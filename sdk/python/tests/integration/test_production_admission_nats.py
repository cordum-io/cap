"""Real-NATS CAP-PRODUCTION admission tests for the Python Agent runtime."""

import asyncio
import unittest
import uuid
from datetime import datetime, timedelta, timezone

import nats
from cryptography.hazmat.primitives.asymmetric import ec
from nats.aio.client import Client as NATSClient
from nats.aio.msg import Msg

from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionTrust, sign_production_packet
from cap.runtime import Agent, InMemoryBlobStore
from cap.subjects import (
    SUBJECT_HANDSHAKE,
    SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
    SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
)
from cap.trust_signing import sign_trust_packet, verify_trust_packet
from cap.worker_trust import WORKER_HANDSHAKE_AUDIENCE, WorkerTrustConfig
from cap.worker_trust_codec import (
    marshal_worker_trust_packet,
    unmarshal_worker_trust_packet,
)
from tests.docker_nats_support import DockerNATSServer


TIMEOUT = 10.0
WORKER_ID = "python-production-nats"
SCHEDULER_ID = "scheduler-production-nats"
TENANT_ID = "tenant-production-nats"

async def connect_worker(**options):
    return await nats.connect(
        **options, connect_timeout=2, max_reconnect_attempts=0
    )


async def connect_scheduler(url: str) -> NATSClient:
    return await asyncio.wait_for(
        nats.connect(
            servers=url,
            name="python-production-test-scheduler",
            allow_reconnect=False,
            connect_timeout=2,
        ),
        TIMEOUT,
    )


def envelope(sender: str, trace_id: str) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket(
        sender_id=sender,
        trace_id=trace_id,
        protocol_version=DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.GetCurrentTime()
    return packet


class TrustResponder:
    def __init__(self) -> None:
        self.worker_key = ec.generate_private_key(ec.SECP256R1())
        self.scheduler_key = ec.generate_private_key(ec.SECP256R1())
        self.events: list[str] = []

    def config(self) -> WorkerTrustConfig:
        return WorkerTrustConfig(
            worker_id=WORKER_ID,
            expected_agent_id="agent-production-nats",
            tenant_id=TENANT_ID,
            audience=WORKER_HANDSHAKE_AUDIENCE,
            proof_key_id="worker-proof-key",
            proof_private_key=self.worker_key,
            expected_scheduler_id=SCHEDULER_ID,
            scheduler_public_keys={"scheduler-session-key": self.scheduler_key.public_key()},
            sdk_version="cap-python/v2",
        )

    async def challenge(self, message: Msg) -> None:
        request = unmarshal_worker_trust_packet(message.data)
        verify_trust_packet(request, {"worker-proof-key": self.worker_key.public_key()})
        source = request.worker_handshake_challenge_request
        now = datetime.now(timezone.utc)
        challenge = handshake_pb2.WorkerHandshakeChallenge(
            request_id=source.request_id,
            challenge_id="production-challenge",
            trace_id=source.trace_id,
            worker_id=source.worker_id,
            agent_id="agent-production-nats",
            tenant_id=TENANT_ID,
            proof_key_id=source.proof_key_id,
            proof_algorithm=source.proof_algorithm,
            server_key_id="scheduler-session-key",
            audience=source.audience,
            purpose=source.purpose,
            client_nonce=source.client_nonce,
            server_nonce=b"S" * 32,
            protocol_version=source.protocol_version,
            sdk_version=source.sdk_version,
        )
        challenge.issued_at.FromDatetime(now)
        challenge.expires_at.FromDatetime(now + timedelta(seconds=30))
        response = envelope(SCHEDULER_ID, source.trace_id)
        response.worker_handshake_challenge.CopyFrom(challenge)
        sign_trust_packet(response, self.scheduler_key)
        self.events.append("challenge")
        await message.respond(marshal_worker_trust_packet(response))

    async def authenticate(self, message: Msg) -> None:
        request = unmarshal_worker_trust_packet(message.data)
        verify_trust_packet(request, {"worker-proof-key": self.worker_key.public_key()})
        now = datetime.now(timezone.utc)
        result = handshake_pb2.WorkerHandshakeResult(
            challenge=request.worker_handshake_authenticate.challenge,
            accepted=True,
        )
        result.issued_at.FromDatetime(now)
        result.token_expires_at.FromDatetime(now + timedelta(minutes=5))
        response = envelope(SCHEDULER_ID, request.trace_id)
        response.auth_token = "python-production-session"
        response.worker_handshake_result.CopyFrom(result)
        sign_trust_packet(response, self.scheduler_key)
        self.events.append("authenticate")
        await message.respond(marshal_worker_trust_packet(response))


class ProductionPacketFactory:
    def __init__(self, key: ec.EllipticCurvePrivateKey, subject: str) -> None:
        self.key = key
        self.subject = subject

    def packet(self, job_id: str, number: int, audience: str = "") -> buspacket_pb2.BusPacket:
        packet = envelope(SCHEDULER_ID, f"trace-{job_id}")
        packet.identity.tenant_id = TENANT_ID
        packet.identity.principal_id = "principal-production"
        metadata = packet.signature_metadata
        metadata.profile_version = "cap-production-v1"
        metadata.algorithm = "ECDSA-P256-SHA256"
        metadata.message_id = number.to_bytes(16, "big")
        metadata.audience = audience or self.subject
        metadata.expires_at.FromDatetime(datetime.now(timezone.utc) + timedelta(minutes=2))
        metadata.key_id = "production-key"
        request = packet.job_request
        request.job_id = job_id
        request.topic = self.subject
        request.context_ptr = f"redis://ctx:{job_id}"
        request.tenant_id = TENANT_ID
        request.identity.CopyFrom(packet.identity)
        return packet

    def signed(self, job_id: str, number: int, audience: str = "") -> bytes:
        return sign_production_packet(self.packet(job_id, number, audience), self.key)


class TestProductionAdmissionRealNATS(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.server = DockerNATSServer.start()
        self.addCleanup(self.server.close)
        self.scheduler = await connect_scheduler(self.server.url)
        self.addAsyncCleanup(self.scheduler.drain)
        self.responder = TrustResponder()
        self.subject = f"job.python.production.{uuid.uuid4().hex}"
        self.direct_subject = f"worker.{WORKER_ID}.jobs"
        self.packets = ProductionPacketFactory(
            self.responder.scheduler_key, self.subject
        )
        self.store = InMemoryBlobStore()
        self.handled: asyncio.Queue[str] = asyncio.Queue()
        ready = asyncio.Event()

        async def on_ready(message: Msg) -> None:
            packet = buspacket_pb2.BusPacket.FromString(message.data)
            if packet.sender_id == WORKER_ID:
                ready.set()

        await self.scheduler.subscribe(
            SUBJECT_WORKER_HANDSHAKE_CHALLENGE, cb=self.responder.challenge
        )
        await self.scheduler.subscribe(
            SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, cb=self.responder.authenticate
        )
        await self.scheduler.subscribe(SUBJECT_HANDSHAKE, cb=on_ready)
        await self.scheduler.flush(timeout=TIMEOUT)
        self.agent = self.build_agent()
        self.addAsyncCleanup(self.agent.close)

        @self.agent.job(self.subject)
        async def handler(_context, _data):
            await self.handled.put(_context.job.job_id)
            return {"ok": True}

        await asyncio.wait_for(self.agent.start(), TIMEOUT)
        await asyncio.wait_for(ready.wait(), TIMEOUT)

    def build_agent(self) -> Agent:
        return Agent(
            nats_url=self.server.url,
            store=self.store,
            connect_fn=connect_worker,
            sender_id=WORKER_ID,
            max_parallel=1,
            heartbeat_interval=60,
            worker_trust_mode="enforce",
            worker_trust=self.responder.config(),
            worker_trust_timeout=2,
            worker_trust_retries=1,
            production_trust=ProductionTrust(
                audience=self.subject,
                tenant=TENANT_ID,
                sender=SCHEDULER_ID,
                public_keys={
                    "production-key": self.responder.scheduler_key.public_key()
                },
            ),
            replay_store=InMemoryReplayStore(),
        )

    async def phase(
        self, target: str, raw_packets: list[bytes], barrier_id: str, number: int
    ) -> list[str]:
        for raw in raw_packets:
            await self.scheduler.publish(target, raw)
        await self.store.set(f"ctx:{barrier_id}", b"{}")
        barrier = self.packets.signed(barrier_id, number, target)
        await self.scheduler.publish(target, barrier)
        await self.scheduler.flush(timeout=TIMEOUT)
        observed = []
        while not observed or observed[-1] != barrier_id:
            observed.append(await asyncio.wait_for(self.handled.get(), TIMEOUT))
        return observed

    async def prime(self, *job_ids: str) -> None:
        for job_id in job_ids:
            await self.store.set(f"ctx:{job_id}", b"{}")

    async def test_admission_rejects_hostile_wire_before_handler(self) -> None:
        self.assertEqual(["challenge", "authenticate"], self.responder.events)
        self.assertEqual("python-production-session", self.agent.session_token)
        await self.prime("wrong-subject")
        wrong_subject = self.packets.signed("wrong-subject", 1)
        observed = await self.phase(
            self.direct_subject, [wrong_subject], "barrier-subject", 2
        )
        self.assertEqual(["barrier-subject"], observed)
        await self.prime("unsigned")
        unsigned = self.packets.packet("unsigned", 3).SerializeToString()
        observed = await self.phase(
            self.subject, [unsigned], "barrier-unsigned", 4
        )
        self.assertEqual(["barrier-unsigned"], observed)
        await self.prime("tampered")
        tampered = bytearray(self.packets.signed("tampered", 5))
        tampered[tampered.index(b"tampered")] ^= 1
        observed = await self.phase(
            self.subject, [bytes(tampered)], "barrier-tampered", 6
        )
        self.assertEqual(["barrier-tampered"], observed)
        await self.prime("first", "conflict")
        first = self.packets.signed("first", 7)
        conflict = self.packets.signed("conflict", 7)
        observed = await self.phase(
            self.subject, [first, conflict], "barrier-conflict", 8
        )
        self.assertEqual(["first", "barrier-conflict"], observed)
        await self.prime("redelivery")
        redelivery = self.packets.signed("redelivery", 9)
        observed = await self.phase(
            self.subject, [redelivery, redelivery], "barrier-redelivery", 10
        )
        self.assertEqual(["redelivery", "barrier-redelivery"], observed)


if __name__ == "__main__":
    unittest.main()
