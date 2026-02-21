import asyncio
import os
import unittest

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec

from cap.heartbeat import (
    emit_heartbeat,
    heartbeat_loop,
    heartbeat_payload,
    heartbeat_payload_with_memory,
    heartbeat_payload_with_progress,
)
from cap.pb.cordum.agent.v1 import buspacket_pb2
from cap.subjects import SUBJECT_HEARTBEAT


class MockNATS:
    def __init__(self):
        self.published = asyncio.Queue()

    async def publish(self, subject, data):
        await self.published.put((subject, data))


class RecordingMetrics:
    def __init__(self):
        self.heartbeats = []

    def on_heartbeat_sent(self, worker_id):
        self.heartbeats.append(worker_id)


class TestHeartbeatPayload(unittest.TestCase):
    def test_heartbeat_payload_constructor(self):
        payload = heartbeat_payload(
            worker_id="worker-1",
            pool="pool-a",
            active_jobs=3,
            max_parallel=8,
            cpu_load=42.5,
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.sender_id, "worker-1")
        self.assertEqual(packet.protocol_version, 1)
        self.assertGreater(packet.created_at.seconds, 0)
        self.assertEqual(packet.heartbeat.worker_id, "worker-1")
        self.assertEqual(packet.heartbeat.pool, "pool-a")
        self.assertEqual(packet.heartbeat.active_jobs, 3)
        self.assertEqual(packet.heartbeat.max_parallel_jobs, 8)
        self.assertEqual(packet.heartbeat.cpu_load, 42.5)
        self.assertEqual(packet.heartbeat.memory_load, 0.0)
        self.assertEqual(packet.heartbeat.progress_pct, 0)
        self.assertEqual(packet.heartbeat.last_memo, "")

    def test_heartbeat_payload_with_memory_constructor(self):
        payload = heartbeat_payload_with_memory(
            worker_id="worker-2",
            pool="pool-b",
            active_jobs=1,
            max_parallel=2,
            cpu_load=10.0,
            memory_load=75.0,
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.sender_id, "worker-2")
        self.assertEqual(packet.protocol_version, 1)
        self.assertEqual(packet.heartbeat.memory_load, 75.0)
        self.assertEqual(packet.heartbeat.progress_pct, 0)
        self.assertEqual(packet.heartbeat.last_memo, "")

    def test_heartbeat_payload_with_progress_constructor(self):
        payload = heartbeat_payload_with_progress(
            worker_id="worker-3",
            pool="pool-c",
            active_jobs=4,
            max_parallel=10,
            cpu_load=88.0,
            memory_load=64.0,
            progress_pct=67,
            last_memo="checkpoint-42",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.sender_id, "worker-3")
        self.assertEqual(packet.protocol_version, 1)
        self.assertEqual(packet.heartbeat.worker_id, "worker-3")
        self.assertEqual(packet.heartbeat.pool, "pool-c")
        self.assertEqual(packet.heartbeat.active_jobs, 4)
        self.assertEqual(packet.heartbeat.max_parallel_jobs, 10)
        self.assertEqual(packet.heartbeat.cpu_load, 88.0)
        self.assertEqual(packet.heartbeat.memory_load, 64.0)
        self.assertEqual(packet.heartbeat.progress_pct, 67)
        self.assertEqual(packet.heartbeat.last_memo, "checkpoint-42")

    def test_heartbeat_payload_with_zero_values(self):
        payload = heartbeat_payload_with_progress(
            worker_id="",
            pool="",
            active_jobs=0,
            max_parallel=0,
            cpu_load=0.0,
            memory_load=0.0,
            progress_pct=0,
            last_memo="",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)

        self.assertEqual(packet.sender_id, "")
        self.assertEqual(packet.protocol_version, 1)
        self.assertEqual(packet.heartbeat.active_jobs, 0)
        self.assertEqual(packet.heartbeat.max_parallel_jobs, 0)
        self.assertEqual(packet.heartbeat.cpu_load, 0.0)
        self.assertEqual(packet.heartbeat.memory_load, 0.0)


class TestHeartbeatAsync(unittest.IsolatedAsyncioTestCase):
    async def test_emit_heartbeat_without_signing(self):
        nc = MockNATS()
        payload = heartbeat_payload("worker-emit", "pool-emit", 2, 4, 11.0)

        await emit_heartbeat(nc, payload)

        subject, published = await asyncio.wait_for(nc.published.get(), timeout=1)
        self.assertEqual(subject, SUBJECT_HEARTBEAT)
        self.assertEqual(published, payload)

        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(published)
        self.assertFalse(packet.signature)

    async def test_emit_heartbeat_with_signing(self):
        nc = MockNATS()
        private_key = ec.generate_private_key(ec.SECP256R1())
        payload = heartbeat_payload("worker-sign", "pool-sign", 1, 2, 22.0)

        await emit_heartbeat(nc, payload, private_key=private_key)

        subject, published = await asyncio.wait_for(nc.published.get(), timeout=1)
        self.assertEqual(subject, SUBJECT_HEARTBEAT)

        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(published)
        self.assertTrue(packet.signature)

        signature = packet.signature
        packet.ClearField("signature")
        unsigned_data = packet.SerializeToString(deterministic=True)
        private_key.public_key().verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))

    async def test_heartbeat_loop_emits_and_stops(self):
        nc = MockNATS()
        metrics = RecordingMetrics()
        cancel_event = asyncio.Event()
        calls = {"count": 0}

        def payload_fn() -> bytes:
            calls["count"] += 1
            return heartbeat_payload("worker-loop", "pool-loop", calls["count"], 5, 12.0)

        loop_task = asyncio.create_task(
            heartbeat_loop(
                nc=nc,
                payload_fn=payload_fn,
                interval=0.1,
                metrics=metrics,
                cancel_event=cancel_event,
            )
        )

        await asyncio.sleep(0.35)
        cancel_event.set()
        await asyncio.wait_for(loop_task, timeout=1)

        self.assertGreaterEqual(calls["count"], 2)
        self.assertGreaterEqual(len(metrics.heartbeats), 2)
        self.assertTrue(all(worker_id == "worker-loop" for worker_id in metrics.heartbeats))

        published = []
        while not nc.published.empty():
            published.append(await nc.published.get())

        self.assertGreaterEqual(len(published), 2)
        for subject, payload in published:
            self.assertEqual(subject, SUBJECT_HEARTBEAT)
            packet = buspacket_pb2.BusPacket()
            packet.ParseFromString(payload)
            self.assertEqual(packet.heartbeat.worker_id, "worker-loop")

    async def test_heartbeat_loop_continues_after_payload_error(self):
        nc = MockNATS()
        cancel_event = asyncio.Event()
        calls = {"count": 0}

        def payload_fn() -> bytes:
            calls["count"] += 1
            if calls["count"] == 1:
                raise ValueError("transient payload error")
            return heartbeat_payload("worker-retry", "pool-retry", 0, 1, 0.0)

        loop_task = asyncio.create_task(
            heartbeat_loop(
                nc=nc,
                payload_fn=payload_fn,
                interval=0.05,
                cancel_event=cancel_event,
            )
        )

        subject, payload = await asyncio.wait_for(nc.published.get(), timeout=1)
        self.assertEqual(subject, SUBJECT_HEARTBEAT)
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)
        self.assertEqual(packet.heartbeat.worker_id, "worker-retry")

        cancel_event.set()
        await asyncio.wait_for(loop_task, timeout=1)
        self.assertGreaterEqual(calls["count"], 2)


class TestHeartbeatIntegration(unittest.IsolatedAsyncioTestCase):
    async def test_heartbeat_loop_real_nats(self):
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
                    name="cap-heartbeat-integration",
                    allow_reconnect=False,
                    connect_timeout=1,
                ),
                timeout=2,
            )
        except Exception as exc:
            self.skipTest(f"NATS unavailable at {nats_url}: {exc}")
            return

        messages = asyncio.Queue()
        cancel_event = asyncio.Event()
        private_key = ec.generate_private_key(ec.SECP256R1())
        loop_task = None

        async def on_msg(msg):
            await messages.put(msg.data)

        try:
            await nc.subscribe(SUBJECT_HEARTBEAT, cb=on_msg)
            await nc.flush(timeout=1)

            def payload_fn() -> bytes:
                return heartbeat_payload_with_progress(
                    worker_id="worker-int",
                    pool="pool-int",
                    active_jobs=2,
                    max_parallel=5,
                    cpu_load=33.0,
                    memory_load=55.0,
                    progress_pct=80,
                    last_memo="integration",
                )

            loop_task = asyncio.create_task(
                heartbeat_loop(
                    nc=nc,
                    payload_fn=payload_fn,
                    interval=0.05,
                    private_key=private_key,
                    cancel_event=cancel_event,
                )
            )

            data = await asyncio.wait_for(messages.get(), timeout=2)
            packet = buspacket_pb2.BusPacket()
            packet.ParseFromString(data)

            self.assertEqual(packet.sender_id, "worker-int")
            self.assertEqual(packet.protocol_version, 1)
            self.assertEqual(packet.heartbeat.worker_id, "worker-int")
            self.assertEqual(packet.heartbeat.pool, "pool-int")
            self.assertEqual(packet.heartbeat.active_jobs, 2)
            self.assertEqual(packet.heartbeat.max_parallel_jobs, 5)
            self.assertEqual(packet.heartbeat.progress_pct, 80)
            self.assertEqual(packet.heartbeat.last_memo, "integration")
            self.assertTrue(packet.signature)

            signature = packet.signature
            packet.ClearField("signature")
            unsigned_data = packet.SerializeToString(deterministic=True)
            private_key.public_key().verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))
        finally:
            cancel_event.set()
            if loop_task is not None:
                await asyncio.wait_for(loop_task, timeout=1)
            await asyncio.wait_for(nc.drain(), timeout=2)


if __name__ == "__main__":
    unittest.main()
