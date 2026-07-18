import asyncio
import os
import unittest

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec

from cap.progress import (
    cancel_payload,
    emit_cancel,
    emit_progress,
    progress_payload,
)
from cap.errors import InvalidInputError
from cap.pb.cordum.agent.v1 import buspacket_pb2
from cap.subjects import SUBJECT_CANCEL, SUBJECT_PROGRESS
from cap.validate import validate_bus_packet


class MockNATS:
    def __init__(self):
        self.published = asyncio.Queue()

    async def publish(self, subject, data):
        await self.published.put((subject, data))


class TestProgressPayload(unittest.TestCase):
    def test_progress_payload_constructor(self):
        payload = progress_payload(
            sender_id="worker-1",
            job_id="job-1",
            step_id="step-1",
            percent=50,
            message="halfway",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.trace_id, "job-1")
        self.assertEqual(packet.sender_id, "worker-1")
        self.assertEqual(packet.protocol_version, 1)
        self.assertGreater(packet.created_at.seconds, 0)
        self.assertEqual(packet.job_progress.job_id, "job-1")
        self.assertEqual(packet.job_progress.step_id, "step-1")
        self.assertEqual(packet.job_progress.percent, 50)
        self.assertEqual(packet.job_progress.message, "halfway")
        self.assertEqual(validate_bus_packet(packet), [])

    def test_progress_payload_zero_percent(self):
        payload = progress_payload(
            sender_id="worker-1",
            job_id="job-1",
            step_id="step-1",
            percent=0,
            message="",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.job_progress.job_id, "job-1")
        self.assertEqual(packet.job_progress.percent, 0)
        self.assertEqual(packet.job_progress.message, "")

    def test_progress_payload_full_percent(self):
        payload = progress_payload(
            sender_id="worker-1",
            job_id="job-1",
            step_id="step-done",
            percent=100,
            message="complete",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.job_progress.percent, 100)
        self.assertEqual(packet.job_progress.step_id, "step-done")
        self.assertEqual(packet.job_progress.message, "complete")


class TestCancelPayload(unittest.TestCase):
    def test_cancel_payload_constructor(self):
        payload = cancel_payload(
            sender_id="worker-1",
            job_id="job-1",
            reason="timeout",
            requested_by="user-7",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.trace_id, "job-1")
        self.assertEqual(packet.sender_id, "worker-1")
        self.assertEqual(packet.protocol_version, 1)
        self.assertGreater(packet.created_at.seconds, 0)
        self.assertEqual(packet.job_cancel.job_id, "job-1")
        self.assertEqual(packet.job_cancel.reason, "timeout")
        self.assertEqual(packet.job_cancel.requested_by, "user-7")
        self.assertEqual(validate_bus_packet(packet), [])

    def test_cancel_payload_empty_reason(self):
        payload = cancel_payload(
            sender_id="worker-1",
            job_id="job-1",
            reason="",
            requested_by="scheduler-1",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.job_cancel.job_id, "job-1")
        self.assertEqual(packet.job_cancel.reason, "")
        self.assertEqual(packet.job_cancel.requested_by, "scheduler-1")

    def test_cancel_payload_requires_requester(self):
        with self.assertRaises(InvalidInputError):
            cancel_payload("worker-1", "job-1", "stop", "")


class TestEmitProgress(unittest.IsolatedAsyncioTestCase):
    async def test_emit_progress_without_signing(self):
        nc = MockNATS()
        payload = progress_payload("worker-emit", "job-1", "step-1", 50, "halfway")

        await emit_progress(nc, payload)

        subject, published = await asyncio.wait_for(nc.published.get(), timeout=1)
        self.assertEqual(subject, SUBJECT_PROGRESS)
        self.assertEqual(published, payload)

        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(published)
        self.assertFalse(packet.signature)

    async def test_emit_progress_with_signing(self):
        nc = MockNATS()
        private_key = ec.generate_private_key(ec.SECP256R1())
        payload = progress_payload("worker-sign", "job-1", "step-1", 75, "almost")

        await emit_progress(nc, payload, private_key=private_key)

        subject, published = await asyncio.wait_for(nc.published.get(), timeout=1)
        self.assertEqual(subject, SUBJECT_PROGRESS)

        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(published)
        self.assertTrue(packet.signature)

        signature = packet.signature
        packet.ClearField("signature")
        unsigned_data = packet.SerializeToString(deterministic=True)
        private_key.public_key().verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))


class TestEmitCancel(unittest.IsolatedAsyncioTestCase):
    async def test_emit_cancel_without_signing(self):
        nc = MockNATS()
        payload = cancel_payload("worker-emit", "job-1", "timeout", "user-7")

        await emit_cancel(nc, payload)

        subject, published = await asyncio.wait_for(nc.published.get(), timeout=1)
        self.assertEqual(subject, SUBJECT_CANCEL)
        self.assertEqual(published, payload)

        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(published)
        self.assertFalse(packet.signature)

    async def test_emit_cancel_with_signing(self):
        nc = MockNATS()
        private_key = ec.generate_private_key(ec.SECP256R1())
        payload = cancel_payload("worker-sign", "job-1", "user-requested", "admin-1")

        await emit_cancel(nc, payload, private_key=private_key)

        subject, published = await asyncio.wait_for(nc.published.get(), timeout=1)
        self.assertEqual(subject, SUBJECT_CANCEL)

        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(published)
        self.assertTrue(packet.signature)

        signature = packet.signature
        packet.ClearField("signature")
        unsigned_data = packet.SerializeToString(deterministic=True)
        private_key.public_key().verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))


class TestProgressIntegration(unittest.IsolatedAsyncioTestCase):
    async def test_progress_and_cancel_real_nats(self):
        try:
            import nats  # type: ignore
        except Exception as exc:
            self.skipTest(f"nats-py unavailable: {exc}")
            return

        nats_url = os.getenv("CAP_TEST_NATS_URL", "nats://127.0.0.1:4222")
        try:
            nc = await asyncio.wait_for(
                nats.connect(
                    servers=nats_url,
                    name="cap-progress-integration",
                    allow_reconnect=False,
                    connect_timeout=1,
                ),
                timeout=2,
            )
        except Exception as exc:
            self.skipTest(f"NATS unavailable at {nats_url}: {exc}")
            return

        progress_messages = asyncio.Queue()
        cancel_messages = asyncio.Queue()
        private_key = ec.generate_private_key(ec.SECP256R1())

        async def on_progress(msg):
            await progress_messages.put(msg.data)

        async def on_cancel(msg):
            await cancel_messages.put(msg.data)

        try:
            await nc.subscribe(SUBJECT_PROGRESS, cb=on_progress)
            await nc.subscribe(SUBJECT_CANCEL, cb=on_cancel)
            await nc.flush(timeout=1)

            prog_data = progress_payload("worker-int", "job-int", "step-int", 42, "processing")
            await emit_progress(nc, prog_data, private_key=private_key)

            data = await asyncio.wait_for(progress_messages.get(), timeout=2)
            packet = buspacket_pb2.BusPacket()
            packet.ParseFromString(data)

            self.assertEqual(packet.trace_id, "job-int")
            self.assertEqual(packet.sender_id, "worker-int")
            self.assertEqual(packet.protocol_version, 1)
            self.assertEqual(packet.job_progress.job_id, "job-int")
            self.assertEqual(packet.job_progress.step_id, "step-int")
            self.assertEqual(packet.job_progress.percent, 42)
            self.assertEqual(packet.job_progress.message, "processing")
            self.assertTrue(packet.signature)

            signature = packet.signature
            packet.ClearField("signature")
            unsigned_data = packet.SerializeToString(deterministic=True)
            private_key.public_key().verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))

            canc_data = cancel_payload("worker-int", "job-int", "user-abort", "admin-int")
            await emit_cancel(nc, canc_data, private_key=private_key)

            data = await asyncio.wait_for(cancel_messages.get(), timeout=2)
            packet = buspacket_pb2.BusPacket()
            packet.ParseFromString(data)

            self.assertEqual(packet.trace_id, "job-int")
            self.assertEqual(packet.sender_id, "worker-int")
            self.assertEqual(packet.job_cancel.job_id, "job-int")
            self.assertEqual(packet.job_cancel.reason, "user-abort")
            self.assertEqual(packet.job_cancel.requested_by, "admin-int")
            self.assertTrue(packet.signature)

            signature = packet.signature
            packet.ClearField("signature")
            unsigned_data = packet.SerializeToString(deterministic=True)
            private_key.public_key().verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))
        finally:
            await asyncio.wait_for(nc.drain(), timeout=2)


if __name__ == "__main__":
    unittest.main()
