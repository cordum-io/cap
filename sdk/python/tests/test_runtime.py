import asyncio
import json
import unittest

from pydantic import BaseModel

from cap.runtime import Agent, InMemoryBlobStore
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2


class MockNATS:
    def __init__(self):
        self.subscriptions = {}
        self.published = asyncio.Queue()

    async def publish(self, subject, data):
        await self.published.put((subject, data))

    async def subscribe(self, subject, queue, cb):
        self.subscriptions[subject] = cb

    async def connect(self, servers, name):
        return self

    async def drain(self):
        return None


class InputModel(BaseModel):
    prompt: str


class OutputModel(BaseModel):
    summary: str


class TestRuntime(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.store = InMemoryBlobStore()
        self.mock = MockNATS()

    async def _send_job(self, job_id: str, topic: str, context_ptr: str):
        req = job_pb2.JobRequest(job_id=job_id, topic=topic, context_ptr=context_ptr)
        packet = buspacket_pb2.BusPacket()
        packet.trace_id = "trace-1"
        packet.sender_id = "client-1"
        packet.protocol_version = 1
        packet.job_request.CopyFrom(req)
        await self.mock.subscriptions[topic](type("obj", (object,), {"data": packet.SerializeToString(deterministic=True)})())

    async def test_runtime_success(self):
        job_id = "job-1"
        ctx_key = f"ctx:{job_id}"
        await self.store.set(ctx_key, json.dumps({"prompt": "hello"}).encode("utf-8"))

        agent = Agent(store=self.store, connect_fn=self.mock.connect, sender_id="worker-1")

        @agent.job("job.test", input_model=InputModel, output_model=OutputModel)
        async def handler(ctx, data: InputModel) -> OutputModel:
            return OutputModel(summary=data.prompt.upper())

        await agent.start()
        await self._send_job(job_id, "job.test", f"redis://{ctx_key}")

        subject, payload = await asyncio.wait_for(self.mock.published.get(), timeout=1)
        self.assertEqual(subject, "sys.job.result")
        result_packet = buspacket_pb2.BusPacket()
        result_packet.ParseFromString(payload)
        self.assertEqual(result_packet.job_result.status, job_pb2.JOB_STATUS_SUCCEEDED)

        result_data = await self.store.get(f"res:{job_id}")
        self.assertIsNotNone(result_data)
        parsed = json.loads(result_data.decode("utf-8"))
        self.assertEqual(parsed["summary"], "HELLO")

        await agent.close()

    async def test_runtime_input_validation_failure(self):
        job_id = "job-2"
        ctx_key = f"ctx:{job_id}"
        await self.store.set(ctx_key, json.dumps({"wrong": "field"}).encode("utf-8"))

        agent = Agent(store=self.store, connect_fn=self.mock.connect, sender_id="worker-2")

        @agent.job("job.validate", input_model=InputModel, output_model=OutputModel)
        async def handler(ctx, data: InputModel) -> OutputModel:
            return OutputModel(summary=data.prompt)

        await agent.start()
        await self._send_job(job_id, "job.validate", f"redis://{ctx_key}")

        subject, payload = await asyncio.wait_for(self.mock.published.get(), timeout=1)
        self.assertEqual(subject, "sys.job.result")
        result_packet = buspacket_pb2.BusPacket()
        result_packet.ParseFromString(payload)
        self.assertEqual(result_packet.job_result.status, job_pb2.JOB_STATUS_FAILED)
        self.assertIn("input validation failed", result_packet.job_result.error_message)

        await agent.close()

    async def test_runtime_retries(self):
        job_id = "job-3"
        ctx_key = f"ctx:{job_id}"
        await self.store.set(ctx_key, json.dumps({"prompt": "retry"}).encode("utf-8"))

        agent = Agent(store=self.store, connect_fn=self.mock.connect, sender_id="worker-3", retries=1)
        attempts = {"count": 0}

        @agent.job("job.retry", input_model=InputModel, output_model=OutputModel)
        async def handler(ctx, data: InputModel) -> OutputModel:
            attempts["count"] += 1
            if attempts["count"] == 1:
                raise RuntimeError("boom")
            return OutputModel(summary=data.prompt)

        await agent.start()
        await self._send_job(job_id, "job.retry", f"redis://{ctx_key}")

        subject, payload = await asyncio.wait_for(self.mock.published.get(), timeout=1)
        self.assertEqual(subject, "sys.job.result")
        result_packet = buspacket_pb2.BusPacket()
        result_packet.ParseFromString(payload)
        self.assertEqual(result_packet.job_result.status, job_pb2.JOB_STATUS_SUCCEEDED)
        self.assertEqual(attempts["count"], 2)

        await agent.close()


if __name__ == "__main__":
    unittest.main()
