import asyncio
import contextlib
import logging
import unittest
from unittest import mock

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.client import submit_job
from cap.errors import InvalidInputError, MalformedPacketError
from cap.heartbeat import emit_heartbeat, heartbeat_loop, heartbeat_payload
from cap.packet_boundary import decode_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.progress import cancel_payload, emit_cancel, emit_progress, progress_payload
from cap.worker import _result_packet, _verify_packet, run_worker

class _MockNats:
    def __init__(self) -> None:
        self.published = []
        self.callbacks = {}

    async def connect(self, **_kwargs):
        return self

    async def subscribe(self, subject, *, queue="", cb=None):
        del queue
        self.callbacks[subject] = cb
        return object()

    async def publish(self, subject: str, data: bytes) -> None:
        self.published.append((subject, data))

    async def drain(self) -> None:
        return None


def _parse(data: bytes) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket()
    packet.ParseFromString(data)
    return packet


def _verify_signature(
    packet: buspacket_pb2.BusPacket,
    public_key: ec.EllipticCurvePublicKey,
) -> None:
    signature = packet.signature
    unsigned = buspacket_pb2.BusPacket()
    unsigned.CopyFrom(packet)
    unsigned.ClearField("signature")
    public_key.verify(
        signature,
        unsigned.SerializeToString(deterministic=True),
        ec.ECDSA(hashes.SHA256()),
    )


def _source_packet() -> buspacket_pb2.BusPacket:
    timestamp = timestamp_pb2.Timestamp()
    timestamp.GetCurrentTime()
    packet = buspacket_pb2.BusPacket(
        trace_id="trace-result",
        sender_id="scheduler",
        protocol_version=1,
    )
    packet.created_at.CopyFrom(timestamp)
    packet.job_request.CopyFrom(
        job_pb2.JobRequest(job_id="job-result", topic="job.secure")
    )
    return packet


class TestBuilderSecurityBoundary(unittest.IsolatedAsyncioTestCase):
    def test_invalid_envelope_is_rejected_before_signature_check(self) -> None:
        packet = _source_packet()
        packet.trace_id = ""
        logger = mock.Mock()
        logger.warning.side_effect = RuntimeError("logging unavailable")
        with mock.patch("cap.packet_boundary.verify_packet") as verify:
            decoded = decode_packet(
                packet.SerializeToString(deterministic=True),
                {},
                logger,
            )
        self.assertIsNone(decoded)
        verify.assert_not_called()

    async def test_public_builders_attach_outer_session_token(self) -> None:
        bus = _MockNats()
        private_key = ec.generate_private_key(ec.SECP256R1())
        request = job_pb2.JobRequest(job_id="job-submit", topic="job.secure")
        await submit_job(
            bus,
            request,
            "trace-submit",
            "client-secure",
            private_key=private_key,
            session_token="session-submit",
        )
        payloads = (
            bus.published[0][1],
            heartbeat_payload(
                "worker-secure", "pool", 0, 1, 0, session_token="session-heartbeat"
            ),
            progress_payload(
                "worker-secure",
                "job-progress",
                "step",
                10,
                "working",
                session_token="session-progress",
            ),
            cancel_payload(
                "worker-secure",
                "job-cancel",
                "stop",
                "operator",
                session_token="session-cancel",
            ),
        )
        self.assertEqual(
            [_parse(data).auth_token for data in payloads],
            [
                "session-submit",
                "session-heartbeat",
                "session-progress",
                "session-cancel",
            ],
        )
        _verify_signature(_parse(payloads[0]), private_key.public_key())

    async def test_empty_session_token_is_omitted(self) -> None:
        packet = _parse(
            progress_payload(
                "worker-secure", "job-empty", "step", 1, "ok", session_token=""
            )
        )
        self.assertEqual(packet.auth_token, "")

    async def test_session_token_must_be_a_string(self) -> None:
        with self.assertRaises(TypeError):
            progress_payload("worker", "job", "step", 1, "ok", session_token=b"bad")  # type: ignore[arg-type]

    async def test_emitters_validate_before_publish(self) -> None:
        bus = _MockNats()
        invalid = buspacket_pb2.BusPacket(
            trace_id="trace-invalid",
            protocol_version=1,
            job_cancel=job_pb2.JobCancel(job_id="job-invalid"),
        )
        invalid.created_at.GetCurrentTime()

        with self.assertRaises(InvalidInputError):
            await emit_cancel(bus, invalid.SerializeToString(deterministic=True))
        with self.assertRaises(MalformedPacketError):
            await emit_cancel(bus, b"\xff")
        self.assertEqual(bus.published, [])

    async def test_emitters_inject_session_before_signing(self) -> None:
        private_key = ec.generate_private_key(ec.SECP256R1())
        emissions = (
            (
                emit_heartbeat,
                heartbeat_payload("worker-secure", "pool", 0, 1, 0),
                "session-heartbeat",
            ),
            (
                emit_progress,
                progress_payload("worker-secure", "job-progress", "step", 1, "ok"),
                "session-progress",
            ),
            (
                emit_cancel,
                cancel_payload("worker-secure", "job-cancel", "stop", "operator"),
                "session-cancel",
            ),
        )
        for emitter, payload, session_token in emissions:
            with self.subTest(session_token=session_token):
                bus = _MockNats()
                await emitter(
                    bus,
                    payload,
                    private_key=private_key,
                    session_token=session_token,
                )
                packet = _parse(bus.published[0][1])
                self.assertEqual(packet.auth_token, session_token)
                _verify_signature(packet, private_key.public_key())
                packet.auth_token += "-tampered"
                self.assertFalse(_verify_packet(packet, {packet.sender_id: private_key.public_key()}, logging.getLogger()))

    async def test_heartbeat_loop_forwards_session_token(self) -> None:
        bus = _MockNats()
        cancel_event = asyncio.Event()

        def payload() -> bytes:
            cancel_event.set()
            return heartbeat_payload("worker-secure", "pool", 0, 1, 0)

        await asyncio.wait_for(
            heartbeat_loop(
                bus,
                payload,
                interval=0,
                cancel_event=cancel_event,
                session_token="session-heartbeat",
            ),
            timeout=1,
        )
        self.assertEqual(_parse(bus.published[0][1]).auth_token, "session-heartbeat")

    async def test_submit_job_rejects_invalid_envelope(self) -> None:
        bus = _MockNats()
        request = job_pb2.JobRequest(job_id="job-submit", topic="job.secure")
        with self.assertRaises(InvalidInputError):
            await submit_job(bus, request, "", "client-secure")
        self.assertEqual(bus.published, [])

    def test_payload_builders_reject_invalid_fields(self) -> None:
        builders = (
            lambda: heartbeat_payload("", "pool", 0, 1, 0),
            lambda: progress_payload("worker", "job-progress", "step", 101, "ok"),
            lambda: cancel_payload("worker-secure", "job", "stop", ""),
        )
        for builder in builders:
            with self.subTest(builder=builder):
                with self.assertRaises(InvalidInputError):
                    builder()

    def test_result_packet_attaches_session_before_signing(self) -> None:
        private_key = ec.generate_private_key(ec.SECP256R1())
        result = job_pb2.JobResult(
            job_id="job-result",
            worker_id="worker-secure",
            status=job_pb2.JOB_STATUS_SUCCEEDED,
        )
        packet = _result_packet(
            _source_packet(),
            result,
            "worker-secure",
            private_key,
            session_token="session-result",
        )
        self.assertEqual(packet.auth_token, "session-result")
        _verify_signature(packet, private_key.public_key())

    async def test_run_worker_attaches_session_to_published_result(self) -> None:
        bus = _MockNats()
        private_key = ec.generate_private_key(ec.SECP256R1())

        async def handler(request: job_pb2.JobRequest) -> job_pb2.JobResult:
            return job_pb2.JobResult(
                job_id=request.job_id,
                status=job_pb2.JOB_STATUS_SUCCEEDED,
            )

        worker = asyncio.create_task(
            run_worker(
                "nats://test",
                "job.secure",
                handler,
                private_key=private_key,
                sender_id="worker-secure",
                connect_fn=bus.connect,
                session_token="session-result",
            )
        )
        try:
            for _ in range(20):
                if "job.secure" in bus.callbacks:
                    break
                await asyncio.sleep(0)
            self.assertIn("job.secure", bus.callbacks)
            message = type("Message", (), {
                "data": _source_packet().SerializeToString(deterministic=True)
            })()
            await bus.callbacks["job.secure"](message)
            packet = _parse(bus.published[0][1])
            self.assertEqual(packet.auth_token, "session-result")
            _verify_signature(packet, private_key.public_key())
        finally:
            worker.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await worker

    def test_configured_key_never_accepts_empty_identity_proof(self) -> None:
        private_key = ec.generate_private_key(ec.SECP256R1())
        packet = _source_packet()
        packet.sender_id = ""
        unsigned = packet.SerializeToString(deterministic=True)
        packet.signature = private_key.sign(unsigned, ec.ECDSA(hashes.SHA256()))

        self.assertFalse(
            _verify_packet(packet, {"": private_key.public_key()}, logging.getLogger())
        )
        keys = {"scheduler": private_key.public_key()}
        self.assertFalse(_verify_packet(_source_packet(), keys, logging.getLogger()))
