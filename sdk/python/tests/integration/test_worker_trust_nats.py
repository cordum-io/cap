"""Required real-NATS trust exchange for the Python high-level runtime."""

import asyncio
import os
import unittest
from datetime import datetime, timedelta, timezone

from cryptography.hazmat.primitives.asymmetric import ec

from cap.packet_boundary import finalize_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
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


NATS_URL_ENV = "CAP_TEST_NATS_URL"
WORKER_ID = "python-trust-nats"
TOPIC = "job.python.trust.nats"


def required_nats_url() -> str:
    value = os.environ.get(NATS_URL_ENV, "").strip()
    if not value:
        raise AssertionError(
            "{} must name the declared real NATS service".format(NATS_URL_ENV)
        )
    return value


async def connect_required(name: str, url: str):
    try:
        import nats
    except ImportError as exc:
        raise AssertionError("nats-py is required for declared integration") from exc
    try:
        return await asyncio.wait_for(
            nats.connect(
                servers=url,
                name=name,
                connect_timeout=2,
                max_reconnect_attempts=0,
            ),
            timeout=5,
        )
    except asyncio.CancelledError:
        raise
    except Exception as exc:
        raise AssertionError("declared NATS service is unreachable") from exc


async def connect_worker(**options):
    import nats
    return await nats.connect(
        **options, connect_timeout=2, max_reconnect_attempts=0
    )


def packet(sender: str, trace: str, now: datetime) -> buspacket_pb2.BusPacket:
    value = buspacket_pb2.BusPacket(
        sender_id=sender, trace_id=trace, protocol_version=1
    )
    value.created_at.FromDatetime(now)
    return value


def readiness_callback(observed, responder, scheduler):
    async def on_handshake(message) -> None:
        if observed.done():
            return
        observed.set_result(buspacket_pb2.BusPacket.FromString(message.data))
        dispatch = packet(
            "scheduler-nats", "trace-instant-dispatch", datetime.now(timezone.utc)
        )
        dispatch.job_request.job_id = "job-instant-dispatch"
        dispatch.job_request.topic = TOPIC
        dispatch.job_request.context_ptr = "redis://ctx:instant-dispatch"
        outgoing = finalize_packet(dispatch, responder.scheduler_key)
        await scheduler.publish(
            TOPIC, outgoing.SerializeToString(deterministic=True)
        )
    return on_handshake


class TrustResponder:
    def __init__(self) -> None:
        self.worker_key = ec.generate_private_key(ec.SECP256R1())
        self.scheduler_key = ec.generate_private_key(ec.SECP256R1())
        self.events = []

    def config(self) -> WorkerTrustConfig:
        return WorkerTrustConfig(
            worker_id=WORKER_ID,
            expected_agent_id="agent-python-nats",
            tenant_id="tenant-python-nats",
            audience=WORKER_HANDSHAKE_AUDIENCE,
            proof_key_id="worker-key-nats",
            proof_private_key=self.worker_key,
            expected_scheduler_id="scheduler-nats",
            scheduler_public_keys={
                "scheduler-key-nats": self.scheduler_key.public_key()
            },
            sdk_version="cap-python/v2",
        )

    async def challenge(self, message) -> None:
        request = unmarshal_worker_trust_packet(message.data)
        verify_trust_packet(
            request, {"worker-key-nats": self.worker_key.public_key()}
        )
        source = request.worker_handshake_challenge_request
        now = datetime.now(timezone.utc)
        challenge = handshake_pb2.WorkerHandshakeChallenge(
            request_id=source.request_id,
            challenge_id="challenge-nats",
            trace_id=source.trace_id,
            worker_id=source.worker_id,
            agent_id="agent-python-nats",
            tenant_id="tenant-python-nats",
            proof_key_id=source.proof_key_id,
            proof_algorithm=source.proof_algorithm,
            server_key_id="scheduler-key-nats",
            audience=source.audience,
            purpose=source.purpose,
            client_nonce=source.client_nonce,
            server_nonce=b"S" * 32,
            protocol_version=source.protocol_version,
            sdk_version=source.sdk_version,
        )
        challenge.issued_at.FromDatetime(now)
        challenge.expires_at.FromDatetime(now + timedelta(seconds=30))
        response = packet("scheduler-nats", source.trace_id, now)
        response.worker_handshake_challenge.CopyFrom(challenge)
        sign_trust_packet(response, self.scheduler_key)
        self.events.append("challenge")
        await message.respond(marshal_worker_trust_packet(response))

    async def authenticate(self, message) -> None:
        request = unmarshal_worker_trust_packet(message.data)
        verify_trust_packet(
            request, {"worker-key-nats": self.worker_key.public_key()}
        )
        now = datetime.now(timezone.utc)
        result = handshake_pb2.WorkerHandshakeResult(
            challenge=request.worker_handshake_authenticate.challenge,
            accepted=True,
        )
        result.issued_at.FromDatetime(now)
        result.token_expires_at.FromDatetime(now + timedelta(minutes=5))
        response = packet("scheduler-nats", request.trace_id, now)
        response.auth_token = "python-real-nats-session"
        response.worker_handshake_result.CopyFrom(result)
        sign_trust_packet(response, self.scheduler_key)
        self.events.append("authenticate")
        await message.respond(marshal_worker_trust_packet(response))


class TestWorkerTrustRealNATS(unittest.IsolatedAsyncioTestCase):
    async def test_trust_finishes_before_admission_and_broadcasts_session(self) -> None:
        url = required_nats_url()
        responder = TrustResponder()
        scheduler = await connect_required("python-trust-scheduler", url)
        observed = asyncio.get_running_loop().create_future()
        handled = asyncio.Event()
        on_handshake = readiness_callback(observed, responder, scheduler)

        await scheduler.subscribe(
            SUBJECT_WORKER_HANDSHAKE_CHALLENGE, cb=responder.challenge
        )
        await scheduler.subscribe(
            SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, cb=responder.authenticate
        )
        await scheduler.subscribe(SUBJECT_HANDSHAKE, cb=on_handshake)
        await scheduler.flush()
        store = InMemoryBlobStore()
        await store.set("ctx:instant-dispatch", b"{}")
        agent = Agent(
            nats_url=url,
            store=store,
            connect_fn=connect_worker,
            sender_id=WORKER_ID,
            worker_trust_mode="enforce",
            worker_trust=responder.config(),
            worker_trust_timeout=2,
            worker_trust_retries=1,
            heartbeat_interval=60,
        )

        @agent.job(TOPIC)
        async def handler(context, data):
            del context, data
            handled.set()
            return {"ok": True}

        try:
            await asyncio.wait_for(agent.start(), timeout=10)
            handshake = await asyncio.wait_for(observed, timeout=5)
            self.assertEqual(["challenge", "authenticate"], responder.events)
            self.assertEqual("python-real-nats-session", handshake.auth_token)
            self.assertEqual([TOPIC], list(handshake.handshake.ready_topics))
            self.assertEqual(2, len(agent._subscriptions))
            await asyncio.wait_for(handled.wait(), timeout=5)
        finally:
            await agent.close()
            await scheduler.drain()


if __name__ == "__main__":
    unittest.main()
