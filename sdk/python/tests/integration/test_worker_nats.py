import asyncio
import os
import unittest
import uuid
from typing import Awaitable, Callable, Optional
from typing import Any, Dict, List, Tuple

import nats
from nats.aio.client import Client as NATSClient
from nats.aio.msg import Msg
from nats.aio.subscription import Subscription
from opentelemetry import trace
from opentelemetry.sdk.trace import ReadableSpan, TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from opentelemetry.sdk.trace.sampling import ALWAYS_ON
from opentelemetry.trace import SpanContext, SpanKind
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

from cap.client import submit_job
from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionTrust
from cap.runtime import Agent, Context, InMemoryBlobStore
from cap.subjects import (
    SUBJECT_HANDSHAKE,
    SUBJECT_RESULT,
    SUBJECT_SUBMIT,
    SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
    SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
)
from cap.telemetry import TelemetryConfig
from cap.worker import run_worker
from tests.integration.test_production_admission_nats import (
    SCHEDULER_ID,
    TENANT_ID,
    WORKER_ID,
    ProductionPacketFactory,
    TrustResponder,
    connect_worker,
)


NATS_URL_ENV = "CAP_TEST_NATS_URL"
TIMEOUT_SECONDS = 5
FAILED_MESSAGE = "handler failed"


def _required_nats_url() -> str:
    value = os.environ.get(NATS_URL_ENV, "").strip()
    if not value:
        raise AssertionError(
            f"{NATS_URL_ENV} must name a real NATS server for this integration test"
        )
    return value


async def _connect(name: str, url: str) -> NATSClient:
    return await asyncio.wait_for(
        nats.connect(
            servers=url,
            name=name,
            allow_reconnect=False,
            connect_timeout=1,
            max_reconnect_attempts=0,
        ),
        timeout=TIMEOUT_SECONDS,
    )


class ObservedWorkerConnection:
    def __init__(self, client: NATSClient) -> None:
        self.client = client
        self.subscription_ready = asyncio.Event()
        self.drain_finished = asyncio.Event()
        self.drain_calls = 0
        self.flush_calls = 0

    async def publish(self, subject: str, data: bytes) -> None:
        await self.client.publish(subject, data)

    async def subscribe(
        self,
        subject: str,
        *,
        queue: str,
        cb: Callable[[Msg], Awaitable[None]],
    ) -> Subscription:
        subscription = await self.client.subscribe(subject, queue=queue, cb=cb)
        await self.client.flush(timeout=TIMEOUT_SECONDS)
        self.subscription_ready.set()
        return subscription

    async def drain(self) -> None:
        self.drain_calls += 1
        await self.client.flush(timeout=TIMEOUT_SECONDS)
        self.flush_calls += 1
        await self.client.drain()
        self.drain_finished.set()


class ObservedWorkerConnector:
    def __init__(self) -> None:
        self.connection: Optional[ObservedWorkerConnection] = None
        self.connected = asyncio.Event()

    async def connect(self, *, servers: str, name: str) -> ObservedWorkerConnection:
        client = await _connect(name, servers)
        self.connection = ObservedWorkerConnection(client)
        self.connected.set()
        return self.connection


class FailureThenSuccessHandler:
    def __init__(self, failed_id: str, secret: str) -> None:
        self.failed_id = failed_id
        self.secret = secret

    async def __call__(self, request: job_pb2.JobRequest) -> job_pb2.JobResult:
        if request.job_id == self.failed_id:
            raise RuntimeError(self.secret)
        return job_pb2.JobResult(
            status=job_pb2.JOB_STATUS_SUCCEEDED,
            result_ptr="memory://integration-success",
        )


def _job_packet(job_id: str, subject: str, trace_id: str) -> bytes:
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id,
        sender_id="cap-python-integration-submitter",
        protocol_version=DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(job_pb2.JobRequest(job_id=job_id, topic=subject))
    return packet.SerializeToString(deterministic=True)


async def _publish_job(
    client: NATSClient, subject: str, job_id: str, trace_id: str
) -> None:
    await client.publish(subject, _job_packet(job_id, subject, trace_id))
    await client.flush(timeout=TIMEOUT_SECONDS)


async def _next_result(
    subscription: Subscription, trace_id: str
) -> buspacket_pb2.BusPacket:
    loop = asyncio.get_running_loop()
    deadline = loop.time() + TIMEOUT_SECONDS
    while True:
        remaining = deadline - loop.time()
        if remaining <= 0:
            raise asyncio.TimeoutError("timed out waiting for matching CAP result")
        message = await subscription.next_msg(timeout=remaining)
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(message.data)
        if packet.trace_id == trace_id:
            return packet


async def _cancel_and_wait(task: "asyncio.Task[None]") -> None:
    task.cancel()
    try:
        await asyncio.wait_for(task, timeout=TIMEOUT_SECONDS)
    except asyncio.CancelledError:
        return
    raise AssertionError("cancelled worker task completed without CancelledError")


async def _cleanup_task(task: "asyncio.Task[None]") -> None:
    if not task.done():
        task.cancel()
    try:
        await asyncio.wait_for(task, timeout=TIMEOUT_SECONDS)
    except (asyncio.CancelledError, Exception):
        pass


def _start_worker(
    nats_url: str,
    subject: str,
    handler: FailureThenSuccessHandler,
    connector: ObservedWorkerConnector,
) -> "asyncio.Task[None]":
    return asyncio.create_task(
        run_worker(
            nats_url,
            subject,
            handler,
            sender_id="cap-python-integration-worker",
            connect_fn=connector.connect,
        )
    )


class TestWorkerRealNATS(unittest.IsolatedAsyncioTestCase):
    async def test_handler_failure_is_safe_and_worker_remains_live(self) -> None:
        nats_url = _required_nats_url()
        suffix = uuid.uuid4().hex
        subject = f"test.cap.python.worker.{suffix}"
        trace_id = f"trace-{suffix}"
        failed_id = f"failed-{suffix}"
        success_id = f"success-{suffix}"
        secret = "credential=must-not-leak"
        connector = ObservedWorkerConnector()
        submitter = await _connect("cap-python-integration-submitter", nats_url)
        results = await submitter.subscribe(SUBJECT_RESULT)
        await submitter.flush(timeout=TIMEOUT_SECONDS)
        handler = FailureThenSuccessHandler(failed_id, secret)
        worker_task = _start_worker(nats_url, subject, handler, connector)
        try:
            await asyncio.wait_for(connector.connected.wait(), TIMEOUT_SECONDS)
            worker_connection = connector.connection
            self.assertIsNotNone(worker_connection)
            assert worker_connection is not None
            self.assertIsNot(worker_connection.client, submitter)
            await asyncio.wait_for(
                worker_connection.subscription_ready.wait(), TIMEOUT_SECONDS
            )
            await _publish_job(submitter, subject, failed_id, trace_id)
            failed = await _next_result(results, trace_id)
            self.assertEqual(failed_id, failed.job_result.job_id)
            self.assertEqual(job_pb2.JOB_STATUS_FAILED, failed.job_result.status)
            self.assertEqual(FAILED_MESSAGE, failed.job_result.error_message)
            self.assertNotIn(secret.encode(), failed.SerializeToString())
            self.assertFalse(worker_task.done())

            await _publish_job(submitter, subject, success_id, trace_id)
            succeeded = await _next_result(results, trace_id)
            self.assertEqual(success_id, succeeded.job_result.job_id)
            self.assertEqual(job_pb2.JOB_STATUS_SUCCEEDED, succeeded.job_result.status)
            self.assertEqual(
                "memory://integration-success", succeeded.job_result.result_ptr
            )

            await _cancel_and_wait(worker_task)
            await asyncio.wait_for(
                worker_connection.drain_finished.wait(), TIMEOUT_SECONDS
            )
            self.assertEqual(1, worker_connection.drain_calls)
            self.assertEqual(1, worker_connection.flush_calls)
            self.assertTrue(worker_connection.client.is_closed)
        finally:
            await _cleanup_task(worker_task)
            if not submitter.is_closed:
                await asyncio.wait_for(submitter.drain(), TIMEOUT_SECONDS)


CREATION = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
CREATION_TRACE_ID = 0x4BF92F3577B34DA6A3CE929D0E0E4736
CONVERSATION = "messaging.message.conversation_id"


class CountingPropagator(TraceContextTextMapPropagator):
    """The W3C propagator, counting every extract call."""

    def __init__(self) -> None:
        super().__init__()
        self.extracts = 0

    def extract(self, *args: Any, **kwargs: Any) -> Any:
        self.extracts += 1
        return super().extract(*args, **kwargs)


class SpanRecorder:
    def __init__(self) -> None:
        self.exporter = InMemorySpanExporter()
        provider = TracerProvider(sampler=ALWAYS_ON)
        provider.add_span_processor(SimpleSpanProcessor(self.exporter))
        self.tracer = provider.get_tracer("test")

    async def spans(self, count: int) -> Dict[str, ReadableSpan]:
        """Wait for ``count`` finished spans: spans end after the result is sent."""
        loop = asyncio.get_running_loop()
        deadline = loop.time() + TIMEOUT_SECONDS
        while len(self.exporter.get_finished_spans()) < count:
            if loop.time() >= deadline:
                raise asyncio.TimeoutError(f"timed out waiting for {count} spans")
            await asyncio.sleep(0.01)
        spans = self.exporter.get_finished_spans()
        by_name = {span.name: span for span in spans}
        assert len(spans) == count == len(by_name), [span.name for span in spans]
        return by_name


def _header_ids(value: str) -> Tuple[int, int]:
    """Return the trace-id and parent-id of a traceparent header value."""
    parts = value.split("-")
    return int(parts[1], 16), int(parts[2], 16)


def _traced_request(job_id: str, subject: str) -> job_pb2.JobRequest:
    return job_pb2.JobRequest(
        job_id=job_id, topic=subject, context_ptr=f"redis://ctx:{job_id}"
    )


def _traced_packet(job_id: str, subject: str) -> bytes:
    packet = buspacket_pb2.BusPacket(
        trace_id=f"trace-{job_id}",
        sender_id="cap-python-trace-submitter",
        protocol_version=DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(_traced_request(job_id, subject))
    return packet.SerializeToString(deterministic=True)


async def _result_message(subscription: Subscription, trace_id: str) -> Msg:
    loop = asyncio.get_running_loop()
    deadline = loop.time() + TIMEOUT_SECONDS
    while True:
        remaining = deadline - loop.time()
        if remaining <= 0:
            raise asyncio.TimeoutError("timed out waiting for matching CAP result")
        message = await subscription.next_msg(timeout=remaining)
        if buspacket_pb2.BusPacket.FromString(message.data).trace_id == trace_id:
            return message


async def _drain(client: NATSClient) -> None:
    if not client.is_closed:
        await asyncio.wait_for(client.drain(), TIMEOUT_SECONDS)


class TestAgentTraceContextRealNATS(unittest.IsolatedAsyncioTestCase):
    """Every message a sync subscription receives is consumed, or drain stalls."""

    async def asyncSetUp(self) -> None:
        self.nats_url = _required_nats_url()
        self.topic = f"job.telemetry.{uuid.uuid4().hex}"
        self.recorder = SpanRecorder()
        self.store = InMemoryBlobStore()
        self.seen: List[SpanContext] = []
        self.client = await _connect("cap-python-trace-client", self.nats_url)
        self.addAsyncCleanup(_drain, self.client)
        self.jobs = await self.client.subscribe(self.topic)
        self.results = await self.client.subscribe(SUBJECT_RESULT)
        await self.client.flush(timeout=TIMEOUT_SECONDS)

    async def _start_agent(self, telemetry: Optional[TelemetryConfig]) -> None:
        sender_id = f"cap-python-trace-worker-{uuid.uuid4().hex}"
        agent = Agent(
            nats_url=self.nats_url,
            store=self.store,
            sender_id=sender_id,
            heartbeat_interval=60,
            telemetry=telemetry,
        )

        @agent.job(self.topic)
        async def handler(ctx: Context, _data: Any) -> Dict[str, bool]:
            span = trace.get_current_span(ctx.otel_context)
            self.seen.append(span.get_span_context())
            return {"ok": True}

        handshakes = await self.client.subscribe(SUBJECT_HANDSHAKE)
        await self.client.flush(timeout=TIMEOUT_SECONDS)
        await asyncio.wait_for(agent.start(), TIMEOUT_SECONDS)
        self.addAsyncCleanup(agent.close)
        while True:
            message = await handshakes.next_msg(timeout=TIMEOUT_SECONDS)
            if buspacket_pb2.BusPacket.FromString(message.data).sender_id == sender_id:
                break
        await handshakes.unsubscribe()

    async def _relay_submits(self, extra: Dict[str, str]) -> None:
        """Stand in for the scheduler: forward each submit, keeping its headers."""

        async def relay(message: Msg) -> None:
            headers = dict(message.headers or {})
            headers.update(extra)
            await self.client.publish(self.topic, message.data, headers=headers or None)

        await self.client.subscribe(SUBJECT_SUBMIT, cb=relay)
        await self.client.flush(timeout=TIMEOUT_SECONDS)

    async def _submit(self, telemetry: Optional[TelemetryConfig]) -> Tuple[str, Msg, Msg]:
        job_id = f"job-{uuid.uuid4().hex}"
        trace_id = f"trace-{job_id}"
        await self.store.set(f"ctx:{job_id}", b"{}")
        await submit_job(
            self.client,
            _traced_request(job_id, self.topic),
            trace_id,
            "cap-python-trace-submitter",
            telemetry=telemetry,
        )
        await self.client.flush(timeout=TIMEOUT_SECONDS)
        job = await self.jobs.next_msg(timeout=TIMEOUT_SECONDS)
        return trace_id, job, await _result_message(self.results, trace_id)

    async def test_submit_process_and_result_share_one_trace(self) -> None:
        telemetry = TelemetryConfig(self.recorder.tracer)
        await self._start_agent(telemetry)
        await self._relay_submits({"baggage": "k=v"})

        trace_id, job, result = await self._submit(telemetry)

        spans = await self.recorder.spans(3)
        submit = spans["publish sys.job.submit"]
        process = spans[f"process {self.topic}"]
        publish = spans["publish sys.job.result"]
        self.assertEqual({"traceparent", "baggage"}, set(job.headers or {}))
        job_trace, job_parent = _header_ids(job.headers["traceparent"])
        self.assertEqual((submit.context.trace_id, submit.context.span_id), (job_trace, job_parent))
        self.assertEqual(
            [(process.context.trace_id, process.context.span_id)],
            [(seen.trace_id, seen.span_id) for seen in self.seen],
        )
        self.assertIsNotNone(process.parent)
        assert process.parent is not None
        self.assertTrue(process.parent.is_remote)
        self.assertEqual(submit.context.span_id, process.parent.span_id)
        self.assertEqual(submit.context.trace_id, process.context.trace_id)
        self.assertEqual([submit.context.span_id], [link.context.span_id for link in process.links])
        self.assertEqual({"traceparent"}, set(result.headers or {}))
        result_trace, result_parent = _header_ids(result.headers["traceparent"])
        self.assertEqual((submit.context.trace_id, publish.context.span_id), (result_trace, result_parent))
        self.assertIsNotNone(publish.parent)
        assert publish.parent is not None
        self.assertEqual(process.context.span_id, publish.parent.span_id)
        self.assertEqual(
            [SpanKind.PRODUCER, SpanKind.CONSUMER, SpanKind.PRODUCER],
            [submit.kind, process.kind, publish.kind],
        )
        self.assertNotIn(CONVERSATION, publish.attributes)
        self.assertEqual(trace_id, submit.attributes[CONVERSATION])

    async def test_invalid_traceparent_runs_handler_under_new_root(self) -> None:
        await self._start_agent(TelemetryConfig(self.recorder.tracer))
        invalid = ("00-" + "0" * 32 + "-" + "0" * 16 + "-01", "garbage")
        for index, bad in enumerate(invalid):
            job_id = f"bad-{index}-{uuid.uuid4().hex}"
            await self.store.set(f"ctx:{job_id}", b"{}")
            await self.client.publish(
                self.topic, _traced_packet(job_id, self.topic), headers={"traceparent": bad}
            )
            await self.client.flush(timeout=TIMEOUT_SECONDS)
            job = await self.jobs.next_msg(timeout=TIMEOUT_SECONDS)
            self.assertEqual({"traceparent": bad}, job.headers)
            await _result_message(self.results, f"trace-{job_id}")

            process = (await self.recorder.spans(2))[f"process {self.topic}"]
            self.assertIsNone(process.parent)
            self.assertEqual(0, len(process.links))
            self.assertEqual(index + 1, len(self.seen))
            self.assertEqual(process.context.trace_id, self.seen[-1].trace_id)
            self.assertEqual(process.context.span_id, self.seen[-1].span_id)
            self.recorder.exporter.clear()

    async def test_without_telemetry_no_headers_are_sent(self) -> None:
        await self._start_agent(None)
        await self._relay_submits({})

        _, job, result = await self._submit(None)

        self.assertIsNone(job.headers)
        self.assertIsNone(result.headers)
        self.assertEqual(1, len(self.seen))
        self.assertFalse(self.seen[0].is_valid)
        self.assertEqual(0, len(self.recorder.exporter.get_finished_spans()))


class TestAgentTraceAdmissionRealNATS(unittest.IsolatedAsyncioTestCase):
    async def test_rejected_packet_creates_no_span_and_is_never_extracted(self) -> None:
        nats_url = _required_nats_url()
        subject = f"job.telemetry.production.{uuid.uuid4().hex}"
        recorder = SpanRecorder()
        propagator = CountingPropagator()
        responder = TrustResponder()
        packets = ProductionPacketFactory(responder.scheduler_key, subject)
        scheduler = await _connect("cap-python-trace-scheduler", nats_url)
        self.addAsyncCleanup(_drain, scheduler)
        ready = asyncio.Event()

        async def on_ready(message: Msg) -> None:
            if buspacket_pb2.BusPacket.FromString(message.data).sender_id == WORKER_ID:
                ready.set()

        await scheduler.subscribe(SUBJECT_WORKER_HANDSHAKE_CHALLENGE, cb=responder.challenge)
        await scheduler.subscribe(SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, cb=responder.authenticate)
        await scheduler.subscribe(SUBJECT_HANDSHAKE, cb=on_ready)
        results = await scheduler.subscribe(SUBJECT_RESULT)
        await scheduler.flush(timeout=TIMEOUT_SECONDS)
        store = InMemoryBlobStore()
        handled: List[str] = []
        agent = Agent(
            nats_url=nats_url,
            store=store,
            connect_fn=connect_worker,
            sender_id=WORKER_ID,
            max_parallel=1,
            heartbeat_interval=60,
            worker_trust_mode="enforce",
            worker_trust=responder.config(),
            worker_trust_timeout=2,
            worker_trust_retries=1,
            production_trust=ProductionTrust(
                audience=subject,
                tenant=TENANT_ID,
                sender=SCHEDULER_ID,
                public_keys={"production-key": responder.scheduler_key.public_key()},
            ),
            replay_store=InMemoryReplayStore(),
            telemetry=TelemetryConfig(recorder.tracer, propagator),
        )

        @agent.job(subject)
        async def handler(ctx: Context, _data: Any) -> Dict[str, bool]:
            handled.append(ctx.job.job_id)
            return {"ok": True}

        await asyncio.wait_for(agent.start(), TIMEOUT_SECONDS)
        self.addAsyncCleanup(agent.close)
        await asyncio.wait_for(ready.wait(), TIMEOUT_SECONDS)
        await store.set("ctx:tampered", b"{}")
        await store.set("ctx:barrier", b"{}")
        tampered = bytearray(packets.signed("tampered", 1))
        tampered[tampered.index(b"tampered")] ^= 1
        await scheduler.publish(subject, bytes(tampered), headers={"traceparent": CREATION})
        await scheduler.publish(subject, packets.signed("barrier", 2), headers={"traceparent": CREATION})
        await scheduler.flush(timeout=TIMEOUT_SECONDS)

        result = await _result_message(results, "trace-barrier")

        spans = await recorder.spans(2)
        self.assertEqual(["barrier"], handled)
        self.assertEqual(1, propagator.extracts)
        self.assertEqual({f"process {subject}", "publish sys.job.result"}, set(spans))
        process = spans[f"process {subject}"]
        self.assertEqual((2).to_bytes(16, "big").hex(), process.attributes["messaging.message.id"])
        self.assertEqual("trace-barrier", process.attributes[CONVERSATION])
        self.assertIsNotNone(process.parent)
        assert process.parent is not None
        self.assertEqual(CREATION_TRACE_ID, process.parent.trace_id)
        publish = spans["publish sys.job.result"]
        self.assertEqual("trace-barrier", publish.attributes[CONVERSATION])
        self.assertEqual({"traceparent"}, set(result.headers or {}))
        self.assertEqual(publish.context.span_id, _header_ids(result.headers["traceparent"])[1])
        packet = buspacket_pb2.BusPacket.FromString(result.data)
        self.assertEqual(job_pb2.JOB_STATUS_SUCCEEDED, packet.job_result.status)


if __name__ == "__main__":
    unittest.main()
