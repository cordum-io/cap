"""Trace-context wiring of ``submit_job`` and the ``Agent`` runtime.

Fake NATS transports, the real OpenTelemetry SDK and real CAP-PRODUCTION
signing. Use only the opentelemetry 1.41.1 API, the last release that
supports Python 3.9.
"""

import asyncio
import dataclasses
import logging
from types import SimpleNamespace
from typing import Any, Dict, List, Optional, Tuple

import pytest
from cryptography.hazmat.primitives.asymmetric import ec
from opentelemetry import trace
from opentelemetry.sdk.trace import ReadableSpan, TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from opentelemetry.sdk.trace.sampling import ALWAYS_ON
from opentelemetry.trace import SpanContext, SpanKind, StatusCode
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

from cap.client import submit_job
from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionTrust, sign_production_packet
from cap.runtime import Agent, Context, InMemoryBlobStore
from cap.subjects import SUBJECT_RESULT, SUBJECT_SUBMIT
from cap.telemetry import TelemetryConfig
from cap.worker_trust import WorkerTrustMode
from tests.production_admission_support import make_packet

TOPIC = "job.telemetry"
CREATION = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
CREATION_TRACE_ID = 0x4BF92F3577B34DA6A3CE929D0E0E4736
CREATION_SPAN_ID = 0x00F067AA0BA902B7
WORKER_ID = "worker-telemetry"
SCHEDULER_ID = "scheduler-1"
TENANT_ID = "tenant-a"
CONVERSATION = "messaging.message.conversation_id"


class CountingPropagator(TraceContextTextMapPropagator):
    """The W3C propagator, counting every extract call."""

    def __init__(self) -> None:
        super().__init__()
        self.extracts = 0

    def extract(self, *args: Any, **kwargs: Any) -> Any:
        self.extracts += 1
        return super().extract(*args, **kwargs)


class HeaderNats:
    def __init__(self) -> None:
        self.calls: List[Tuple[str, bytes, Optional[Dict[str, str]]]] = []

    async def publish(self, subject: str, payload: bytes, headers: Optional[Dict[str, str]] = None) -> None:
        self.calls.append((subject, payload, headers))


class StrictNats:
    """A publisher whose ``publish`` takes no ``headers`` parameter."""

    def __init__(self) -> None:
        self.calls: List[Tuple[str, bytes]] = []

    async def publish(self, subject: str, payload: bytes) -> None:
        self.calls.append((subject, payload))


class FailingNats:
    async def publish(self, subject: str, payload: bytes, headers: Optional[Dict[str, str]] = None) -> None:
        raise ConnectionError("nats unavailable")


Seen = List[Tuple[Optional[Any], SpanContext]]


def recorder(propagator: Optional[CountingPropagator] = None) -> Tuple[TelemetryConfig, InMemorySpanExporter]:
    exporter = InMemorySpanExporter()
    provider = TracerProvider(sampler=ALWAYS_ON)
    provider.add_span_processor(SimpleSpanProcessor(exporter))
    return TelemetryConfig(provider.get_tracer("test"), propagator), exporter


def register_handler(agent: Agent, fail: bool = False) -> Seen:
    seen: Seen = []

    @agent.job(TOPIC)
    async def handler(ctx: Context, _data: Any) -> Dict[str, bool]:
        seen.append((ctx.otel_context, trace.get_current_span(ctx.otel_context).get_span_context()))
        if fail:
            raise RuntimeError("handler exploded")
        return {"ok": True}

    return seen


def legacy_agent(nc: Any, telemetry: Optional[TelemetryConfig], fail: bool = False) -> Tuple[Agent, Seen]:
    agent = Agent(sender_id=WORKER_ID, telemetry=telemetry)
    agent._nc = nc
    agent._store = InMemoryBlobStore()
    agent._trust_admitting = True
    return agent, register_handler(agent, fail)


def legacy_message(job_id: str, headers: Optional[Dict[str, str]], subject: str = TOPIC) -> SimpleNamespace:
    packet = buspacket_pb2.BusPacket(
        trace_id=f"trace-{job_id}",
        sender_id="scheduler-telemetry",
        protocol_version=DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(job_pb2.JobRequest(job_id=job_id, topic=TOPIC, context_ptr=f"redis://ctx:{job_id}"))
    return SimpleNamespace(data=packet.SerializeToString(deterministic=True), subject=subject, headers=headers)


async def run_job(agent: Agent, msg: SimpleNamespace, job_id: str) -> None:
    await agent._store.set(f"ctx:{job_id}", b"{}")
    await agent._on_msg(msg, agent._handlers[TOPIC])


def spans_by_name(exporter: InMemorySpanExporter) -> Dict[str, ReadableSpan]:
    spans = exporter.get_finished_spans()
    by_name = {span.name: span for span in spans}
    assert len(by_name) == len(spans), [span.name for span in spans]
    return by_name


def traceparent(span: ReadableSpan) -> str:
    context = span.get_span_context()
    return f"00-{context.trace_id:032x}-{context.span_id:016x}-{int(context.trace_flags):02x}"


def parent_span_id(header: str) -> int:
    return int(header.split("-")[2], 16)


def production_agent(
    telemetry: TelemetryConfig, scheduler_key: ec.EllipticCurvePrivateKey
) -> Tuple[Agent, HeaderNats, Seen]:
    agent = Agent(
        sender_id=WORKER_ID,
        production_trust=ProductionTrust(
            audience=TOPIC,
            tenant=TENANT_ID,
            sender=SCHEDULER_ID,
            public_keys={"k1": scheduler_key.public_key()},
        ),
        replay_store=InMemoryReplayStore(),
        telemetry=telemetry,
    )
    nc = HeaderNats()
    agent._nc = nc
    agent._store = InMemoryBlobStore()
    agent._trust_settings = SimpleNamespace(
        mode=WorkerTrustMode.ENFORCE,
        config=SimpleNamespace(
            tenant_id=TENANT_ID,
            expected_scheduler_id=SCHEDULER_ID,
            proof_key_id="worker-key",
            proof_private_key=ec.generate_private_key(ec.SECP256R1()),
        ),
    )
    agent._worker_trust = SimpleNamespace(session_token=lambda: "live-session")
    agent._trust_admitting = True
    return agent, nc, register_handler(agent)


def test_production_rejection_creates_no_span_and_never_extracts() -> None:
    propagator = CountingPropagator()
    telemetry, exporter = recorder(propagator)
    key = ec.generate_private_key(ec.SECP256R1())
    agent, nc, seen = production_agent(telemetry, key)
    packet = make_packet(audience=TOPIC)
    packet.job_request.topic = TOPIC
    packet.job_request.context_ptr = "redis://ctx:job-1"
    valid = sign_production_packet(packet, key)
    tampered = bytearray(valid)
    tampered[-1] ^= 1

    async def scenario() -> None:
        await agent._store.set("ctx:job-1", b"{}")
        spec = agent._handlers[TOPIC]
        await agent._on_msg(SimpleNamespace(data=bytes(tampered), subject=TOPIC, headers={"traceparent": CREATION}), spec)
        assert propagator.extracts == 0
        assert len(exporter.get_finished_spans()) == 0
        assert nc.calls == [] and seen == []
        await agent._on_msg(SimpleNamespace(data=valid, subject=TOPIC, headers={"traceparent": CREATION}), spec)

    asyncio.run(scenario())

    assert propagator.extracts == 1
    spans = spans_by_name(exporter)
    assert set(spans) == {"process job.telemetry", "publish sys.job.result"}
    process, publish = spans["process job.telemetry"], spans["publish sys.job.result"]
    assert process.attributes["messaging.message.id"] == b"0123456789abcdef".hex()
    assert process.attributes[CONVERSATION] == "trace-1"
    assert process.attributes["messaging.consumer.group.name"] == TOPIC
    assert process.parent is not None and process.parent.is_remote
    assert (process.parent.trace_id, process.parent.span_id) == (CREATION_TRACE_ID, CREATION_SPAN_ID)
    assert publish.attributes[CONVERSATION] == "trace-1"
    assert publish.parent is not None and publish.parent.span_id == process.context.span_id
    [(subject, payload, headers)] = nc.calls
    assert subject == SUBJECT_RESULT
    assert headers == {"traceparent": traceparent(publish)}
    assert parent_span_id(headers["traceparent"]) == publish.context.span_id
    assert buspacket_pb2.BusPacket.FromString(payload).job_result.status == job_pb2.JOB_STATUS_SUCCEEDED
    assert [span_context.span_id for _, span_context in seen] == [process.context.span_id]


def test_handler_failure_marks_process_span() -> None:
    telemetry, exporter = recorder()
    nc = HeaderNats()
    agent, _ = legacy_agent(nc, telemetry, fail=True)

    asyncio.run(run_job(agent, legacy_message("fail", {"traceparent": CREATION}), "fail"))

    spans = spans_by_name(exporter)
    process, publish = spans["process job.telemetry"], spans["publish sys.job.result"]
    assert process.attributes["error.type"] == "HANDLER_FAILED"
    assert process.status.status_code == StatusCode.ERROR
    assert len(process.events) == 0
    assert "messaging.message.id" not in process.attributes
    assert CONVERSATION not in process.attributes
    assert "error.type" not in publish.attributes
    assert CONVERSATION not in publish.attributes
    [(subject, payload, headers)] = nc.calls
    assert subject == SUBJECT_RESULT
    assert headers == {"traceparent": traceparent(publish)}
    assert buspacket_pb2.BusPacket.FromString(payload).job_result.status == job_pb2.JOB_STATUS_FAILED


def test_no_telemetry_sends_no_headers_and_records_nothing(monkeypatch: pytest.MonkeyPatch) -> None:
    exporter = InMemorySpanExporter()
    provider = TracerProvider(sampler=ALWAYS_ON)
    provider.add_span_processor(SimpleSpanProcessor(exporter))
    monkeypatch.setattr(trace, "get_tracer_provider", lambda: provider)
    nc = StrictNats()
    agent, seen = legacy_agent(nc, None)

    asyncio.run(run_job(agent, legacy_message("plain", {"traceparent": CREATION}), "plain"))

    assert [subject for subject, _ in nc.calls] == [SUBJECT_RESULT]
    [(otel_context, span_context)] = seen
    assert otel_context is None and not span_context.is_valid
    submit_nc = StrictNats()
    request = job_pb2.JobRequest(job_id="plain", topic=TOPIC)
    asyncio.run(submit_job(submit_nc, request, "trace-plain", "client-telemetry"))
    [(subject, payload)] = submit_nc.calls
    assert subject == SUBJECT_SUBMIT
    assert buspacket_pb2.BusPacket.FromString(payload).trace_id == "trace-plain"
    assert len(exporter.get_finished_spans()) == 0


def test_submit_with_telemetry_injects_publish_span() -> None:
    telemetry, exporter = recorder()
    nc = HeaderNats()
    request = job_pb2.JobRequest(job_id="submit", topic=TOPIC)

    asyncio.run(submit_job(nc, request, "trace-submit", "client-telemetry", telemetry=telemetry))

    [span] = exporter.get_finished_spans()
    assert span.name == "publish sys.job.submit"
    assert span.kind == SpanKind.PRODUCER
    assert span.attributes[CONVERSATION] == "trace-submit"
    assert span.parent is None
    [(subject, payload, headers)] = nc.calls
    assert subject == SUBJECT_SUBMIT
    assert headers == {"traceparent": traceparent(span)}
    assert buspacket_pb2.BusPacket.FromString(payload).trace_id == "trace-submit"

    exporter.clear()
    remote = SpanContext(CREATION_TRACE_ID, CREATION_SPAN_ID, is_remote=True)
    parent = trace.set_span_in_context(trace.NonRecordingSpan(remote))
    asyncio.run(submit_job(nc, request, "trace-submit", "client-telemetry", telemetry=telemetry, context=parent))
    [child] = exporter.get_finished_spans()
    assert child.parent is not None and child.parent.span_id == CREATION_SPAN_ID
    assert child.context.trace_id == CREATION_TRACE_ID
    assert nc.calls[-1][2] == {"traceparent": traceparent(child)}


def test_context_otel_context_defaults_to_none() -> None:
    logger = logging.LoggerAdapter(logging.getLogger("cap.test.telemetry"), {})
    ctx = Context(job=job_pb2.JobRequest(), packet=buspacket_pb2.BusPacket(), logger=logger)
    assert ctx.otel_context is None
    assert [field.name for field in dataclasses.fields(Context)][-1] == "otel_context"


def test_ambient_span_never_parents_the_process_span() -> None:
    telemetry, exporter = recorder()
    agent, _ = legacy_agent(HeaderNats(), telemetry)

    async def scenario() -> None:
        with telemetry.tracer.start_as_current_span("ambient"):
            await run_job(agent, legacy_message("ambient", {"traceparent": CREATION}), "ambient")

    asyncio.run(scenario())

    process = spans_by_name(exporter)["process job.telemetry"]
    assert process.parent is not None and process.parent.is_remote
    assert (process.parent.trace_id, process.parent.span_id) == (CREATION_TRACE_ID, CREATION_SPAN_ID)
    assert [link.context.span_id for link in process.links] == [CREATION_SPAN_ID]


def test_direct_subject_process_span_has_no_queue_group() -> None:
    telemetry, exporter = recorder()
    agent, seen = legacy_agent(HeaderNats(), telemetry)
    direct = f"worker.{WORKER_ID}.jobs"

    async def scenario() -> None:
        await agent._store.set("ctx:direct", b"{}")
        await agent._on_direct_msg(legacy_message("direct", {"traceparent": CREATION}, subject=direct))

    asyncio.run(scenario())

    spans = spans_by_name(exporter)
    assert set(spans) == {"process", "publish sys.job.result"}
    process = spans["process"]
    assert "messaging.consumer.group.name" not in process.attributes
    assert "messaging.destination.name" not in process.attributes
    assert process.parent is not None and process.parent.span_id == CREATION_SPAN_ID
    assert [span_context.span_id for _, span_context in seen] == [process.context.span_id]


def test_transport_failure_marks_publish_spans() -> None:
    telemetry, exporter = recorder()
    request = job_pb2.JobRequest(job_id="down", topic=TOPIC)
    with pytest.raises(ConnectionError):
        asyncio.run(submit_job(FailingNats(), request, "trace-down", "client-telemetry", telemetry=telemetry))
    agent, _ = legacy_agent(FailingNats(), telemetry)
    with pytest.raises(ConnectionError):
        asyncio.run(run_job(agent, legacy_message("down", None), "down"))

    spans = spans_by_name(exporter)
    assert set(spans) == {"publish sys.job.submit", "process job.telemetry", "publish sys.job.result"}
    for name in ("publish sys.job.submit", "publish sys.job.result"):
        assert spans[name].attributes["error.type"] == "TRANSPORT_PUBLISH_FAILED"
        assert spans[name].status.status_code == StatusCode.ERROR
        assert len(spans[name].events) == 0
    process = spans["process job.telemetry"]
    assert process.parent is None and len(process.links) == 0
