import asyncio
import json
import unittest
from unittest.mock import AsyncMock, MagicMock

from cap.metrics import MetricsHook, NoopMetrics
from cap.runtime import Agent, Context, InMemoryBlobStore

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2


class RecordingMetrics:
    """Test metrics hook that records all calls."""

    def __init__(self):
        self.received = []
        self.completed = []
        self.failed = []
        self.heartbeats = []

    def on_job_received(self, job_id, topic):
        self.received.append((job_id, topic))

    def on_job_completed(self, job_id, duration_ms, status):
        self.completed.append((job_id, status))

    def on_job_failed(self, job_id, error_msg):
        self.failed.append((job_id, error_msg))

    def on_heartbeat_sent(self, worker_id):
        self.heartbeats.append(worker_id)


def _make_packet(job_id: str, topic: str, context_ptr: str, trace_id: str = "t1", sender_id: str = "c1"):
    req = job_pb2.JobRequest(job_id=job_id, topic=topic, context_ptr=context_ptr)
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id
    packet.sender_id = sender_id
    packet.protocol_version = 1
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(req)
    return packet


class FakeMsg:
    def __init__(self, data: bytes):
        self.data = data
        self.subject = ""


class TestMetrics(unittest.TestCase):
    def test_metrics_on_success(self):
        async def _run():
            store = InMemoryBlobStore()
            metrics = RecordingMetrics()
            nc = AsyncMock()

            payload = json.dumps({"prompt": "hello"}).encode()
            await store.set("ctx:j1", payload)

            agent = Agent(store=store, connect_fn=AsyncMock(return_value=nc), metrics=metrics)

            @agent.job("job.test", input_model=None)
            async def handler(ctx: Context, data):
                return {"summary": data["prompt"]}

            await agent.start()

            packet = _make_packet("j1", "job.test", "redis://ctx:j1")
            msg = FakeMsg(packet.SerializeToString(deterministic=True))
            spec = agent._handlers["job.test"]
            await agent._on_msg(msg, spec)

            self.assertEqual(len(metrics.received), 1)
            self.assertEqual(metrics.received[0], ("j1", "job.test"))
            self.assertEqual(len(metrics.completed), 1)
            self.assertEqual(metrics.completed[0], ("j1", "SUCCEEDED"))
            self.assertEqual(len(metrics.failed), 0)

        asyncio.run(_run())

    def test_metrics_on_failure(self):
        async def _run():
            store = InMemoryBlobStore()
            metrics = RecordingMetrics()
            nc = AsyncMock()

            agent = Agent(store=store, connect_fn=AsyncMock(return_value=nc), metrics=metrics)

            @agent.job("job.fail", input_model=None)
            async def handler(ctx: Context, data):
                return data

            await agent.start()

            packet = _make_packet("j2", "job.fail", "redis://missing-key")
            msg = FakeMsg(packet.SerializeToString(deterministic=True))
            spec = agent._handlers["job.fail"]
            await agent._on_msg(msg, spec)

            self.assertEqual(len(metrics.received), 1)
            self.assertEqual(len(metrics.failed), 1)
            self.assertEqual(metrics.failed[0][0], "j2")
            self.assertEqual(len(metrics.completed), 0)

        asyncio.run(_run())

    def test_noop_metrics_satisfies_protocol(self):
        """NoopMetrics should satisfy MetricsHook protocol."""
        m = NoopMetrics()
        m.on_job_received("j1", "topic")
        m.on_job_completed("j1", 100, "SUCCEEDED")
        m.on_job_failed("j1", "err")
        m.on_heartbeat_sent("w1")


if __name__ == "__main__":
    unittest.main()
