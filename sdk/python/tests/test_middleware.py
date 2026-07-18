import asyncio
import json
import logging
import unittest

from pydantic import BaseModel

from cap.runtime import Agent, InMemoryBlobStore
from cap.middleware import logging_middleware
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2


class MockNATS:
    def __init__(self):
        self.subscriptions = {}
        self.published = asyncio.Queue()

    async def publish(self, subject, data):
        await self.published.put((subject, data))

    async def subscribe(self, subject, queue="", cb=None):
        self.subscriptions[subject] = cb

    async def connect(self, servers, name):
        return self

    async def drain(self):
        return None


class InputModel(BaseModel):
    prompt: str


class OutputModel(BaseModel):
    summary: str


class TestMiddleware(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.store = InMemoryBlobStore()
        self.mock = MockNATS()

    async def _next_published(self, expected_subject: str):
        while True:
            subject, payload = await asyncio.wait_for(self.mock.published.get(), timeout=1)
            if subject == expected_subject:
                return payload

    async def _send_job(self, job_id: str, topic: str, context_ptr: str):
        req = job_pb2.JobRequest(job_id=job_id, topic=topic, context_ptr=context_ptr)
        packet = buspacket_pb2.BusPacket()
        packet.trace_id = "trace-mw"
        packet.sender_id = "client-mw"
        packet.protocol_version = 1
        packet.created_at.GetCurrentTime()
        packet.job_request.CopyFrom(req)
        await self.mock.subscriptions[topic](
            type("obj", (object,), {"data": packet.SerializeToString(deterministic=True)})()
        )

    async def test_middleware_execution_order(self):
        job_id = "job-mw-order"
        ctx_key = f"ctx:{job_id}"
        await self.store.set(ctx_key, json.dumps({"prompt": "hello"}).encode("utf-8"))

        agent = Agent(store=self.store, connect_fn=self.mock.connect, sender_id="worker-mw")

        order = []

        async def mw_a(ctx, data, next_fn):
            order.append("A-before")
            result = await next_fn(ctx, data)
            order.append("A-after")
            return result

        async def mw_b(ctx, data, next_fn):
            order.append("B-before")
            result = await next_fn(ctx, data)
            order.append("B-after")
            return result

        agent.use(mw_a, mw_b)

        @agent.job("job.mw", input_model=InputModel, output_model=OutputModel)
        async def handler(ctx, data: InputModel) -> OutputModel:
            order.append("handler")
            return OutputModel(summary=data.prompt)

        await agent.start()
        await self._send_job(job_id, "job.mw", f"redis://{ctx_key}")

        await self._next_published("sys.job.result")
        self.assertEqual(order, ["A-before", "B-before", "handler", "B-after", "A-after"])

        await agent.close()

    async def test_middleware_short_circuit(self):
        job_id = "job-mw-short"
        ctx_key = f"ctx:{job_id}"
        await self.store.set(ctx_key, json.dumps({"prompt": "hello"}).encode("utf-8"))

        agent = Agent(store=self.store, connect_fn=self.mock.connect, sender_id="worker-short")

        handler_called = {"value": False}

        async def short_circuit(ctx, data, next_fn):
            return OutputModel(summary="shortcut")

        agent.use(short_circuit)

        @agent.job("job.short", input_model=InputModel, output_model=OutputModel)
        async def handler(ctx, data: InputModel) -> OutputModel:
            handler_called["value"] = True
            return OutputModel(summary=data.prompt)

        await agent.start()
        await self._send_job(job_id, "job.short", f"redis://{ctx_key}")

        await self._next_published("sys.job.result")
        self.assertFalse(handler_called["value"])

        result_data = await self.store.get(f"res:{job_id}")
        self.assertIsNotNone(result_data)
        parsed = json.loads(result_data.decode("utf-8"))
        self.assertEqual(parsed["summary"], "shortcut")

        await agent.close()

    async def test_logging_middleware(self):
        job_id = "job-mw-log"
        ctx_key = f"ctx:{job_id}"
        await self.store.set(ctx_key, json.dumps({"prompt": "hello"}).encode("utf-8"))

        test_logger = logging.getLogger("test.middleware")
        test_logger.setLevel(logging.DEBUG)

        log_records = []

        class ListHandler(logging.Handler):
            def emit(self, record):
                log_records.append(record.getMessage())

        test_logger.addHandler(ListHandler())

        agent = Agent(store=self.store, connect_fn=self.mock.connect, sender_id="worker-log")
        agent.use(logging_middleware(test_logger))

        @agent.job("job.log", input_model=InputModel, output_model=OutputModel)
        async def handler(ctx, data: InputModel) -> OutputModel:
            return OutputModel(summary=data.prompt)

        await agent.start()
        await self._send_job(job_id, "job.log", f"redis://{ctx_key}")

        await self._next_published("sys.job.result")

        log_output = "\n".join(log_records)
        self.assertIn(job_id, log_output)
        self.assertIn("ok", log_output)

        await agent.close()


if __name__ == "__main__":
    unittest.main()
