import asyncio
import json
import logging
import unittest
from types import SimpleNamespace

from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from opentelemetry.sdk.trace.sampling import ALWAYS_ON
from opentelemetry.trace import StatusCode
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.runtime import Context
from cap.subjects import SUBJECT_RESULT, SUBJECT_WORKER_HANDSHAKE_CHALLENGE
from cap.telemetry import TelemetryConfig
from cap.testing import MockNATS, create_test_agent, run_handler
from cap.worker_trust_runtime import WorkerTrustRuntimeError
from worker_trust_runtime_support import TOPIC, WORKER_ID, trust_config


def _trust_agent(mode, logger=None):
    return create_test_agent(
        sender_id=WORKER_ID,
        heartbeat_interval=60,
        worker_trust_mode=mode,
        worker_trust=trust_config(),
        worker_trust_timeout=1.0,
        worker_trust_retries=1,
        logger=logger or logging.getLogger("cap.test.testing.trust"),
    )


class TestTestingUtilities(unittest.IsolatedAsyncioTestCase):
    async def test_mock_nats_publish_accepts_and_records_headers(self):
        mock = MockNATS()
        traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
        headers = {"traceparent": traceparent}
        await mock.publish("subject.a", b"one", headers=headers)
        await mock.publish("subject.b", b"two")
        headers["traceparent"] = "mutated"
        self.assertEqual(mock.published_headers, [
            ("subject.a", {"traceparent": traceparent}), ("subject.b", None),
        ])
        self.assertEqual(await asyncio.wait_for(mock.published.get(), 1), ("subject.a", b"one"))
        self.assertEqual(await asyncio.wait_for(mock.published.get(), 1), ("subject.b", b"two"))

    async def test_create_test_agent_with_telemetry_publishes_result_with_trace_headers(self):
        exporter = InMemorySpanExporter()
        provider = TracerProvider(sampler=ALWAYS_ON)
        provider.add_span_processor(SimpleSpanProcessor(exporter))
        cfg = TelemetryConfig(provider.get_tracer("test"), TraceContextTextMapPropagator())
        agent, mock, store = create_test_agent(telemetry=cfg)

        @agent.job("job.telemetry")
        async def handler(ctx: Context, data):
            return {"ok": True}

        await agent.start()
        try:
            await store.set("ctx:telemetry-job", json.dumps({"input": "test"}).encode("utf-8"))
            packet = buspacket_pb2.BusPacket(
                trace_id="test-trace", sender_id="test-client",
                protocol_version=DEFAULT_PROTOCOL_VERSION,
            )
            packet.created_at.GetCurrentTime()
            packet.job_request.CopyFrom(job_pb2.JobRequest(
                job_id="telemetry-job", topic="job.telemetry",
                context_ptr="redis://ctx:telemetry-job",
            ))
            await mock.subscriptions["job.telemetry"](SimpleNamespace(
                data=packet.SerializeToString(deterministic=True),
                # Keep the incoming W3C flags stable across OTel SDK versions.
                headers={"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
            ))
            while True:
                subject, raw = await asyncio.wait_for(mock.published.get(), timeout=5)
                if subject == SUBJECT_RESULT:
                    break
            result = buspacket_pb2.BusPacket.FromString(raw).job_result
            self.assertEqual(result.status, job_pb2.JOB_STATUS_SUCCEEDED)
            headers = [h for s, h in mock.published_headers if s == SUBJECT_RESULT]
            self.assertEqual(len(headers), 1)
            self.assertIsNotNone(headers[0])
            self.assertRegex(headers[0]["traceparent"], r"^00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]$")
            self.assertNotIn("baggage", headers[0])
            spans = exporter.get_finished_spans()
            self.assertTrue(spans)
            self.assertTrue(all(s.status.status_code != StatusCode.ERROR for s in spans))
        finally:
            await agent.close()
            provider.shutdown()

    async def test_run_handler_echo(self):
        async def echo_handler(ctx: Context, data):
            return {"echo": data["prompt"]}

        result = await run_handler(
            echo_handler,
            {"prompt": "hello"},
            job_id="test-1",
            topic="job.echo",
        )

        self.assertEqual(result.status, job_pb2.JOB_STATUS_SUCCEEDED)
        self.assertEqual(result.job_id, "test-1")

    async def test_run_handler_failure(self):
        async def failing_handler(ctx: Context, data):
            raise RuntimeError("boom")

        result = await run_handler(
            failing_handler,
            {"prompt": "hello"},
            job_id="test-2",
            topic="job.fail",
        )

        self.assertEqual(result.status, job_pb2.JOB_STATUS_FAILED)
        self.assertEqual(result.error_message, "handler failed")
        self.assertNotIn("boom", result.error_message)

    async def test_create_test_agent(self):
        agent, mock, store = create_test_agent()
        self.assertIsNotNone(agent)
        self.assertIsInstance(mock, MockNATS)
        self.assertIsNotNone(store)

    async def test_mock_nats_connect_records_trust_callbacks_and_request_times_out(self):
        async def on_reconnected():
            return None

        async def on_disconnected():
            return None

        mock = MockNATS()
        connected = await mock.connect(
            "nats://mock",
            "worker",
            reconnected_cb=on_reconnected,
            disconnected_cb=on_disconnected,
        )
        self.assertIs(mock, connected)
        self.assertIs(on_reconnected, mock.reconnected_cb)
        self.assertIs(on_disconnected, mock.disconnected_cb)

        legacy = MockNATS()
        self.assertIs(legacy, await legacy.connect("nats://mock", "worker"))
        self.assertIsNone(legacy.reconnected_cb)
        self.assertIsNone(legacy.disconnected_cb)

        with self.assertRaises(asyncio.TimeoutError):
            await mock.request("subject.x", b"payload", timeout=0.5)
        self.assertEqual([("subject.x", b"payload")], mock.requests)

    async def test_create_test_agent_starts_with_worker_trust_warn(self):
        logger = logging.getLogger("cap.test.testing.trust.warn")
        agent, mock, _store = _trust_agent("warn", logger)

        @agent.job(TOPIC)
        async def handler(ctx, data):
            return {"ok": True}

        with self.assertLogs(logger, level="WARNING") as logs:
            await agent.start()
        try:
            self.assertTrue(
                any("continuing without new session" in line for line in logs.output)
            )
            self.assertTrue(callable(mock.reconnected_cb))
            self.assertTrue(callable(mock.disconnected_cb))
            self.assertTrue(mock.requests)
            self.assertTrue(
                all(s == SUBJECT_WORKER_HANDSHAKE_CHALLENGE for s, _ in mock.requests)
            )
            self.assertIn(TOPIC, mock.subscriptions)
            self.assertEqual("", agent.session_token)
        finally:
            await agent.close()

    async def test_create_test_agent_worker_trust_enforce_fails_closed(self):
        agent, mock, _store = _trust_agent("enforce")

        @agent.job(TOPIC)
        async def handler(ctx, data):
            return {"ok": True}

        with self.assertRaises(WorkerTrustRuntimeError):
            await agent.start()
        self.assertNotIn(TOPIC, mock.subscriptions)


if __name__ == "__main__":
    unittest.main()
