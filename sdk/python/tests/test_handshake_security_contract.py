import asyncio
import contextlib
import unittest
from typing import Dict, Optional

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.client import submit_job
from cap.handshake import handshake_payload
from cap.heartbeat import heartbeat_payload
from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2, job_pb2
from cap.progress import cancel_payload, progress_payload
from cap.runtime import Agent, InMemoryBlobStore
from cap.validate import validate_bus_packet
from cap.worker import run_worker


SUBJECT = "job.security.contract"


class _MockNats:
    def __init__(self) -> None:
        self.callbacks = {}
        self.published = []

    async def connect(self, **_kwargs):
        return self

    async def subscribe(self, subject, *, queue="", cb=None):
        del queue
        self.callbacks[subject] = cb
        return object()

    async def publish(self, subject, data) -> None:
        self.published.append((subject, data))

    async def drain(self) -> None:
        return None


def _job_packet(
    *,
    trace_id: str = "trace-security",
    sender_id: str = "client-security",
    protocol_version: int = 1,
    signer: Optional[ec.EllipticCurvePrivateKey] = None,
) -> buspacket_pb2.BusPacket:
    timestamp = timestamp_pb2.Timestamp()
    timestamp.GetCurrentTime()
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id,
        sender_id=sender_id,
        protocol_version=protocol_version,
        job_request=job_pb2.JobRequest(
            job_id="job-security",
            topic=SUBJECT,
            context_ptr="redis://ctx:job-security",
        ),
    )
    packet.created_at.CopyFrom(timestamp)
    if signer is not None:
        unsigned = packet.SerializeToString(deterministic=True)
        packet.signature = signer.sign(unsigned, ec.ECDSA(hashes.SHA256()))
    return packet


async def _exercise_worker(
    packet: buspacket_pb2.BusPacket,
    public_keys: Optional[Dict[str, ec.EllipticCurvePublicKey]],
):
    bus = _MockNats()
    calls = 0

    async def handler(request: job_pb2.JobRequest) -> job_pb2.JobResult:
        nonlocal calls
        calls += 1
        return job_pb2.JobResult(
            job_id=request.job_id,
            status=job_pb2.JOB_STATUS_SUCCEEDED,
        )

    task = asyncio.create_task(
        run_worker(
            nats_url="nats://test",
            subject=SUBJECT,
            handler=handler,
            public_keys=public_keys,
            connect_fn=bus.connect,
        )
    )
    for _ in range(20):
        if SUBJECT in bus.callbacks:
            break
        await asyncio.sleep(0)
    if SUBJECT not in bus.callbacks:
        raise AssertionError("worker did not subscribe")
    try:
        message = type("Message", (), {"data": packet.SerializeToString()})()
        await bus.callbacks[SUBJECT](message)
        return calls, list(bus.published)
    finally:
        task.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await task


async def _exercise_agent(
    packet: buspacket_pb2.BusPacket,
    public_keys: Optional[Dict[str, ec.EllipticCurvePublicKey]],
) -> int:
    bus = _MockNats()
    store = InMemoryBlobStore()
    await store.set("ctx:job-security", b"{}")
    calls = 0
    agent = Agent(
        store=store,
        public_keys=public_keys,
        sender_id="agent-security",
        connect_fn=bus.connect,
        heartbeat_interval=60,
    )

    @agent.job(SUBJECT)
    async def handler(_context, _data):
        nonlocal calls
        calls += 1
        return {}

    await agent.start()
    try:
        message = type("Message", (), {"data": packet.SerializeToString()})()
        await bus.callbacks[SUBJECT](message)
    finally:
        await agent.close()
    return calls


class TestEnvelopeSecurityContract(unittest.IsolatedAsyncioTestCase):
    def test_worker_trust_handshake_proto_contract_exists(self) -> None:
        messages = handshake_pb2.DESCRIPTOR.message_types_by_name
        for name in (
            "WorkerHandshakeChallengeRequest",
            "WorkerHandshakeChallenge",
            "WorkerHandshakeAuthenticate",
            "WorkerHandshakeResult",
        ):
            self.assertIn(name, messages)
        fields = buspacket_pb2.BusPacket.DESCRIPTOR.fields_by_name
        for name in (
            "worker_handshake_challenge_request",
            "worker_handshake_challenge",
            "worker_handshake_authenticate",
            "worker_handshake_result",
        ):
            self.assertIn(name, fields)
            self.assertGreaterEqual(fields[name].number, 19)

    def test_validator_requires_exact_protocol_v1(self) -> None:
        errors = validate_bus_packet(_job_packet(protocol_version=2))
        self.assertTrue(
            any(error.field == "protocol_version" for error in errors),
            "protocol_version=2 must be rejected; CAP currently supports exactly v1",
        )

    def test_legacy_handshake_builder_validates_its_own_envelope(self) -> None:
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(handshake_payload(component_id="worker-security"))
        self.assertEqual(validate_bus_packet(packet), [])

    def test_handshake_sender_must_match_component_identity(self) -> None:
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(
            handshake_payload(
                component_id="worker-security",
                sender_id="forged-worker",
            )
        )
        errors = validate_bus_packet(packet)
        self.assertTrue(
            any(error.field == "handshake.component_id" for error in errors),
            "handshake component_id must equal the authenticated envelope sender_id",
        )

    async def test_invalid_envelope_never_reaches_worker_handler(self) -> None:
        vectors = {
            "missing_trace": _job_packet(trace_id=""),
            "protocol_v2": _job_packet(protocol_version=2),
        }
        for name, packet in vectors.items():
            with self.subTest(name=name):
                calls, _published = await _exercise_worker(packet, None)
                self.assertEqual(calls, 0)

    async def test_invalid_envelope_never_reaches_agent_handler(self) -> None:
        vectors = {
            "missing_trace": _job_packet(trace_id=""),
            "protocol_v2": _job_packet(protocol_version=2),
        }
        for name, packet in vectors.items():
            with self.subTest(name=name):
                self.assertEqual(await _exercise_agent(packet, None), 0)

    async def test_configured_empty_trust_map_is_deny_all(self) -> None:
        calls, _published = await _exercise_worker(_job_packet(), {})
        self.assertEqual(calls, 0)

    async def test_invalid_signature_never_reaches_worker_handler(self) -> None:
        trusted = ec.generate_private_key(ec.SECP256R1())
        attacker = ec.generate_private_key(ec.SECP256R1())
        packet = _job_packet(signer=attacker)
        calls, _published = await _exercise_worker(
            packet,
            {"client-security": trusted.public_key()},
        )
        self.assertEqual(calls, 0)

    async def test_public_outbound_builder_validator_matrix(self) -> None:
        bus = _MockNats()
        request = job_pb2.JobRequest(job_id="job-builder", topic=SUBJECT)
        await submit_job(bus, request, "trace-builder", "client-builder")
        _calls, worker_messages = await _exercise_worker(_job_packet(), None)
        payloads = {
            "job_request": bus.published[0][1],
            "heartbeat": heartbeat_payload("worker-builder", "pool", 0, 1, 0),
            "progress": progress_payload("worker-builder", "job-builder", "step", 1, "ok"),
            "cancel": cancel_payload("worker-builder", "job-builder", "stop", "tester"),
            "legacy_handshake": handshake_payload("worker-builder"),
            "job_result": worker_messages[0][1],
        }
        failures = {}
        for name, payload in payloads.items():
            packet = buspacket_pb2.BusPacket()
            packet.ParseFromString(payload)
            failures[name] = [error.field for error in validate_bus_packet(packet)]
        self.assertEqual(failures, {name: [] for name in payloads})


if __name__ == "__main__":
    unittest.main()
