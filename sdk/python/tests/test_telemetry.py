"""Contract tests for cap.telemetry against spec/interop/trace-context.md.

OpenTelemetry is imported unconditionally: the ``dev`` extra installs it on
every CI leg, so a skip would only hide these tests. Use only the
opentelemetry 1.41.1 API, the last release that supports Python 3.9.
"""

import ast
import base64
import dataclasses
import json
import os
import re
import subprocess
import sys
import textwrap
from typing import Any, Dict, Iterator, List, Optional

import pytest
from opentelemetry import baggage, propagate, trace
from opentelemetry import context as otel_context
from opentelemetry.baggage.propagation import W3CBaggagePropagator
from opentelemetry.context import Context
from opentelemetry.propagators.composite import CompositePropagator
from opentelemetry.propagators.textmap import TextMapPropagator, default_getter, default_setter
from opentelemetry.sdk.trace import ReadableSpan, TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from opentelemetry.sdk.trace.sampling import ALWAYS_ON
from opentelemetry.trace import SpanContext, SpanKind, StatusCode, TraceFlags
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
from packaging.requirements import Requirement

try:
    import tomllib
except ModuleNotFoundError:  # pragma: no cover - exercised on Python 3.9/3.10
    import tomli as tomllib

from artifact_support import REPO_ROOT, SDK_ROOT
from cap import telemetry
from cap.telemetry import ErrorType, Process, Publish, TelemetryConfig

VECTORS_PATH = REPO_ROOT / "spec" / "interop" / "vectors" / "trace-context-v1.json"
TP = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
STALE_TP = "00-11111111111111111111111111111111-2222222222222222-01"
REMOTE = SpanContext(
    trace_id=0x0AF7651916CD43DD8448EB211C80319C,
    span_id=0xB7AD6B7169203331,
    is_remote=True,
    trace_flags=TraceFlags(TraceFlags.SAMPLED),
)
CONTRACT_ERROR_TYPES = {
    "PROTOCOL_VERSION_MISMATCH",
    "PROTOCOL_MALFORMED_PACKET",
    "PROTOCOL_UNKNOWN_PAYLOAD",
    "PROTOCOL_SIGNATURE_INVALID",
    "PROTOCOL_SIGNATURE_MISSING",
    "ADMISSION_REJECTED",
    "HANDLER_FAILED",
    "TRANSPORT_PUBLISH_FAILED",
    "_OTHER",
}
PROCESS_ERROR_TYPES = CONTRACT_ERROR_TYPES - {"TRANSPORT_PUBLISH_FAILED"}


class Recorder:
    """An always-sampling tracer provider that keeps finished spans in memory."""

    def __init__(self) -> None:
        self.exporter = InMemorySpanExporter()
        self.provider = TracerProvider(sampler=ALWAYS_ON)
        self.provider.add_span_processor(SimpleSpanProcessor(self.exporter))
        self.tracer = self.provider.get_tracer("test")

    def config(self, propagator: Optional[TextMapPropagator] = None) -> TelemetryConfig:
        return telemetry.from_opentelemetry(tracer_provider=self.provider, propagator=propagator)

    def take(self) -> List[ReadableSpan]:
        spans = list(self.exporter.get_finished_spans())
        self.exporter.clear()
        return spans

    def ambient(self) -> Context:
        return trace.set_span_in_context(self.tracer.start_span("ambient", context=Context()), Context())


class StubPropagator(TextMapPropagator):
    """Injects ``values`` verbatim; extract returns its given context, plus ``span_context`` when set."""

    def __init__(self, values: Optional[Dict[str, str]] = None, span_context: Optional[SpanContext] = None) -> None:
        self._values = dict(values or {})
        self._span_context = span_context

    def extract(self, carrier: Any, context: Any = None, getter: Any = default_getter) -> Any:
        if self._span_context is None:
            return context
        return trace.set_span_in_context(trace.NonRecordingSpan(self._span_context), context)

    def inject(self, carrier: Any, context: Any = None, setter: Any = default_setter) -> None:
        for key, value in self._values.items():
            setter.set(carrier, key, value)

    @property
    def fields(self) -> Any:
        return set(self._values)


@pytest.fixture
def recorder() -> Iterator[Recorder]:
    rec = Recorder()
    try:
        yield rec
    finally:
        rec.provider.shutdown()


def load_vectors() -> Dict[str, Any]:
    with VECTORS_PATH.open(encoding="utf-8") as handle:
        return json.load(handle)


def traceparent(span_context: SpanContext) -> str:
    return "00-{:032x}-{:016x}-{:02x}".format(span_context.trace_id, span_context.span_id, span_context.trace_flags)


def one_span(recorder: Recorder, op: str, context: Optional[Context] = None, **fields: Any) -> ReadableSpan:
    start = telemetry.start_publish if op == "publish" else telemetry.start_process
    description = Publish(**fields) if op == "publish" else Process(**fields)
    _, span = start(recorder.config(), description, Context() if context is None else context)
    span.end()
    spans = recorder.take()
    assert len(spans) == 1, spans
    return spans[0]


def test_extract_trace_context_vectors(recorder: Recorder) -> None:
    data = load_vectors()
    assert data["schema_version"] == 1
    cases = data["traceparent_cases"]
    assert cases
    assert len({case["name"] for case in cases}) == len(cases)
    config = recorder.config()
    counts = {"valid": 0, "invalid": 0}
    for case in cases:
        name, headers, expected = case["name"], case["headers"], case["expected"]
        assert set(headers) <= {"traceparent", "tracestate"}, name
        assert case["verdict"] in counts, name
        counts[case["verdict"]] += 1
        result = telemetry.extract(config, headers)
        if case["verdict"] == "invalid":
            assert result is None and expected is None, name
            continue
        assert result is not None and result.is_valid and result.is_remote, name
        assert format(result.trace_id, "032x") == expected["trace_id"], name
        assert format(result.span_id, "016x") == expected["parent_id"], name
        assert result.trace_flags.sampled is expected["sampled"], name
        assert result.trace_state.to_header() == expected["tracestate"], name
        injected = telemetry.inject_headers(config, None, trace.set_span_in_context(trace.NonRecordingSpan(result)))
        assert injected is not None and injected["traceparent"].startswith("00-"), name
        again = telemetry.extract(config, injected)
        assert again is not None, name
        assert (again.trace_id, again.span_id, again.trace_flags) == (result.trace_id, result.span_id, result.trace_flags)
        assert again.trace_state.to_header() == expected["tracestate"], name
    assert telemetry.extract(config, {"Traceparent": TP}) is None
    assert counts["valid"] > 0 and counts["invalid"] > 0
    assert counts["valid"] + counts["invalid"] == len(cases)
    print("vectors: {} cases, {} valid, {} invalid".format(len(cases), counts["valid"], counts["invalid"]))


def test_span_attribute_maps_match_vectors(recorder: Recorder) -> None:
    maps = load_vectors()["attribute_maps"]
    assert set(maps) == {"publish", "process"}
    kinds = {"PRODUCER": SpanKind.PRODUCER, "CONSUMER": SpanKind.CONSUMER}
    pub = maps["publish"]["input"]
    assert set(pub) <= {"subject", "bus_packet_trace_id"}
    proc = maps["process"]["input"]
    assert set(proc) <= {
        "subject",
        "queue_group",
        "admission",
        "bus_packet_trace_id",
        "signature_metadata_message_id_base64",
    }
    assert proc["admission"] == "cap_production_verified"
    spans = {
        "publish": one_span(recorder, "publish", subject=pub["subject"], conversation_id=pub["bus_packet_trace_id"]),
        "process": one_span(
            recorder,
            "process",
            subject=proc["subject"],
            pool=proc["subject"],
            queue_group=proc["queue_group"],
            production_verified=True,
            message_id=base64.b64decode(proc["signature_metadata_message_id_base64"], validate=True),
            conversation_id=proc["bus_packet_trace_id"],
        ),
    }
    assert spans["publish"].kind is SpanKind.PRODUCER
    assert spans["process"].kind is SpanKind.CONSUMER
    for op, span in spans.items():
        expected = maps[op]
        assert span.name == expected["span_name"]
        assert span.kind is kinds[expected["span_kind"]]
        assert all(isinstance(value, str) for value in expected["attributes"].values())
        assert dict(span.attributes or {}) == expected["attributes"]
        assert span.status.status_code is StatusCode.UNSET
        assert not span.events and not span.links


BARE_DESTINATIONS = [
    ("sys.worker.handshake", ""),
    ("sys.worker.handshake.renew", ""),
    ("job.gpu", ""),
    ("job.gpu", "job.*"),
    ("job.gpu", "job.cpu"),
    ("job.", "job."),
    ("job.*", "job.*"),
    ("job.>", "job.>"),
    ("job.g pu", "job.g pu"),
    ("job.g\tpu", "job.g\tpu"),
    ("orders.created", ""),
    ("orders.created", "orders.created"),
]


@pytest.mark.parametrize("op", ["publish", "process"])
def test_destination_naming(recorder: Recorder, op: str) -> None:
    named = [(subject, "") for subject in sorted(telemetry.FIXED_SUBJECTS)] + [("job.gpu", "job.gpu")]
    for subject, pool in named:
        span = one_span(recorder, op, subject=subject, pool=pool)
        assert span.name == "{} {}".format(op, subject)
        assert (span.attributes or {})["messaging.destination.name"] == subject
    for subject, pool in BARE_DESTINATIONS:
        span = one_span(recorder, op, subject=subject, pool=pool)
        attributes = dict(span.attributes or {})
        assert span.name == op, (subject, pool)
        assert "messaging.destination.name" not in attributes, (subject, pool)
        assert "messaging.destination.temporary" not in attributes, (subject, pool)
    for subject in ("_INBOX.abc.1", "sys.job.submit"):
        span = one_span(recorder, op, subject=subject, pool=subject, reply_inbox=True)
        attributes = dict(span.attributes or {})
        assert span.name == op
        assert "messaging.destination.name" not in attributes
        assert attributes["messaging.destination.temporary"] is True
        assert attributes["messaging.destination.anonymous"] is True


def test_fixed_subjects_match_spec09() -> None:
    text = (REPO_ROOT / "spec" / "09-transport-profile.md").read_text(encoding="utf-8").replace("\r", "")
    found = re.findall(r"^\|\s*`(sys\.[^`]+)`\s*\|", text, re.MULTILINE)
    assert len(found) == len(set(found)) == 11
    assert set(found) == telemetry.FIXED_SUBJECTS


def test_process_attributes_by_admission(recorder: Recorder) -> None:
    def attrs(**fields: Any) -> Dict[str, Any]:
        return dict(one_span(recorder, "process", subject="job.gpu", pool="job.gpu", **fields).attributes or {})

    legacy = attrs(message_id=bytes(16), conversation_id="trace-1")
    assert "messaging.message.id" not in legacy and "messaging.message.conversation_id" not in legacy
    for size in (0, 15, 17):
        short = attrs(production_verified=True, message_id=bytes(size), conversation_id="trace-1")
        assert "messaging.message.id" not in short, size
        assert short["messaging.message.conversation_id"] == "trace-1"
    full = attrs(production_verified=True, message_id=bytes(range(16)))
    assert full["messaging.message.id"] == "000102030405060708090a0b0c0d0e0f"
    assert "messaging.message.conversation_id" not in full
    assert attrs(durable="dur-1", queue_group="q-1")["messaging.destination.subscription.name"] == "dur-1"
    assert attrs(queue_group="q-1")["messaging.consumer.group.name"] == "q-1"
    plain = attrs()
    assert "messaging.consumer.group.name" not in plain
    assert "messaging.destination.subscription.name" not in plain


def test_baggage_is_never_propagated(recorder: Recorder) -> None:
    ctx = baggage.set_baggage("tenant", "acme", recorder.ambient())
    want = traceparent(trace.get_current_span(ctx).get_span_context())
    composite = CompositePropagator([TraceContextTextMapPropagator(), W3CBaggagePropagator()])
    previous = propagate.get_global_textmap()
    try:
        propagate.set_global_textmap(composite)
        for config in (recorder.config(), recorder.config(composite)):
            assert telemetry.inject_headers(config, None, ctx) == {"traceparent": want}
            extracted = telemetry.extract(config, {"traceparent": TP, "baggage": "tenant=acme"})
            assert extracted is not None and extracted.is_valid
        propagate.set_global_textmap(StubPropagator({"traceparent": "global"}))
        assert telemetry.inject_headers(recorder.config(), None, ctx) == {"traceparent": want}
    finally:
        propagate.set_global_textmap(previous)
    custom = StubPropagator({"traceparent": "fixed", "baggage": "tenant=acme", "X-Other": "1"})
    assert telemetry.inject_headers(recorder.config(custom), None, ctx) == {"traceparent": "fixed"}


def test_extract_ignores_ambient_and_local_contexts(recorder: Recorder) -> None:
    token = otel_context.attach(trace.set_span_in_context(trace.NonRecordingSpan(REMOTE)))
    try:
        assert telemetry.extract(recorder.config(StubPropagator()), {"traceparent": TP}) is None
        assert telemetry.extract(recorder.config(), {"traceparent": "garbage"}) is None
    finally:
        otel_context.detach(token)
    local = SpanContext(REMOTE.trace_id, REMOTE.span_id, is_remote=False, trace_flags=REMOTE.trace_flags)
    assert telemetry.extract(recorder.config(StubPropagator(span_context=local)), {"traceparent": TP}) is None


def test_none_config_produces_no_headers_or_spans(recorder: Recorder, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(trace, "get_tracer_provider", lambda: recorder.provider)
    other = TracerProvider(sampler=ALWAYS_ON)
    ctx = trace.set_span_in_context(other.get_tracer("other").start_span("ambient", context=Context()), Context())
    assert telemetry.inject_headers(None, None, ctx) is None
    headers = {"traceparent": STALE_TP, "x-other": "1"}
    assert telemetry.inject_headers(None, headers, ctx) is headers
    assert headers == {"traceparent": STALE_TP, "x-other": "1"}
    assert telemetry.extract(None, {"traceparent": TP}) is None
    starts = [
        (telemetry.start_publish, Publish("sys.job.submit", conversation_id="trace-1")),
        (telemetry.start_process, Process("job.gpu", production_verified=True, message_id=bytes(16), creation=REMOTE)),
    ]
    for start, description in starts:
        returned, span = start(None, description, ctx)
        assert returned is ctx
        span.fail(ErrorType.HANDLER_FAILED)
        span.end()
        returned, span = start(None, description)
        assert returned is None
        span.fail("boom")
        span.end()
    assert recorder.take() == []
    telemetry.from_opentelemetry().tracer.start_span("probe").end()
    assert [span.name for span in recorder.take()] == ["probe"]
    other.shutdown()


def test_inject_headers_replaces_trace_headers(recorder: Recorder) -> None:
    config = recorder.config()
    ctx = recorder.ambient()
    headers = {"traceparent": STALE_TP, "tracestate": "stale=1", "x-other": "keep"}
    assert telemetry.inject_headers(config, headers, ctx) is headers
    assert headers == {"traceparent": traceparent(trace.get_current_span(ctx).get_span_context()), "x-other": "keep"}
    headers = {"traceparent": STALE_TP, "tracestate": "stale=1", "x-other": "keep"}
    assert telemetry.inject_headers(config, headers, Context()) is headers
    assert headers == {"x-other": "keep"}
    assert telemetry.inject_headers(config, None, Context()) is None


def test_error_type_only_on_failure_and_closed(recorder: Recorder) -> None:
    assert {member.value for member in ErrorType} == CONTRACT_ERROR_TYPES
    assert len(ErrorType) == 9
    config = recorder.config()

    def run(op: str, error: Any = None) -> ReadableSpan:
        if op == "publish":
            _, span = telemetry.start_publish(config, Publish("sys.job.submit"), Context())
        else:
            _, span = telemetry.start_process(config, Process("job.gpu", pool="job.gpu"), Context())
        if error is not None:
            span.fail(error)
        span.end()
        (finished,) = recorder.take()
        return finished

    base = {op: dict(run(op).attributes or {}) for op in ("publish", "process")}
    for op, attributes in base.items():
        assert "error.type" not in attributes
        assert run(op).status.status_code is StatusCode.UNSET
    cases = [("publish", error, "TRANSPORT_PUBLISH_FAILED") for error in (ErrorType.TRANSPORT_PUBLISH_FAILED, "TRANSPORT_PUBLISH_FAILED")]
    cases.append(("publish", ErrorType.HANDLER_FAILED, "_OTHER"))
    cases += [("process", ErrorType(value), value) for value in sorted(PROCESS_ERROR_TYPES)]
    cases.append(("process", ErrorType.TRANSPORT_PUBLISH_FAILED, "_OTHER"))
    cases += [(op, raw, "_OTHER") for op in ("publish", "process") for raw in ("boom: secret", "", "handler_failed")]
    for op, error, want in cases:
        span = run(op, error)
        assert dict(span.attributes or {}) == dict(base[op], **{"error.type": want}), (op, error)
        assert span.status.status_code is StatusCode.ERROR, (op, error)
        assert span.status.description == want, (op, error)
        assert not span.events, (op, error)


def test_process_link_and_parent(recorder: Recorder) -> None:
    config = recorder.config()
    creation = telemetry.extract(config, {"traceparent": TP})
    assert creation is not None
    ambient_ctx = recorder.ambient()
    ambient = trace.get_current_span(ambient_ctx).get_span_context()

    def run(link: Optional[SpanContext], ctx: Context) -> ReadableSpan:
        return one_span(recorder, "process", ctx, subject="job.gpu", pool="job.gpu", creation=link)

    span = run(creation, Context())
    assert span.parent == creation and span.parent is not None and span.parent.is_remote
    assert [link.context for link in span.links] == [creation]
    span = run(creation, ambient_ctx)
    assert span.parent == ambient
    assert [link.context for link in span.links] == [creation]
    for missing in (None, trace.INVALID_SPAN_CONTEXT):
        span = run(missing, Context())
        assert span.parent is None and not span.links, missing
        span = run(missing, ambient_ctx)
        assert span.parent == ambient and not span.links, missing


def test_publish_span_is_injected_creation_context(recorder: Recorder) -> None:
    config = recorder.config()
    ambient_ctx = recorder.ambient()
    ctx, span = telemetry.start_publish(config, Publish("sys.job.submit"), ambient_ctx)
    headers = telemetry.inject_headers(config, None, ctx)
    span.end()
    (finished,) = recorder.take()
    assert finished.parent == trace.get_current_span(ambient_ctx).get_span_context()
    assert finished.context is not None
    assert headers == {"traceparent": traceparent(finished.context)}


def test_config_validation(recorder: Recorder) -> None:
    with pytest.raises(TypeError):
        TelemetryConfig(object())  # type: ignore[arg-type]
    with pytest.raises(TypeError):
        TelemetryConfig(recorder.tracer, object())  # type: ignore[arg-type]
    config = TelemetryConfig(recorder.tracer, StubPropagator())
    with pytest.raises(dataclasses.FrozenInstanceError):
        config.tracer = recorder.tracer  # type: ignore[misc]


NO_OTEL_SCRIPT = textwrap.dedent(
    """
    import importlib.abc
    import sys


    class BlockOpenTelemetry(importlib.abc.MetaPathFinder):
        def find_spec(self, name, path=None, target=None):
            if name == "opentelemetry" or name.startswith("opentelemetry."):
                raise ModuleNotFoundError("blocked " + name, name=name)
            return None


    sys.meta_path.insert(0, BlockOpenTelemetry())
    from cap import telemetry

    TP = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
    headers = {"traceparent": TP}
    assert telemetry.inject_headers(None, None) is None
    assert telemetry.inject_headers(None, headers) is headers and headers == {"traceparent": TP}
    assert telemetry.extract(None, headers) is None
    ctx, span = telemetry.start_publish(None, telemetry.Publish("sys.job.submit"))
    assert ctx is None
    span.fail(telemetry.ErrorType.TRANSPORT_PUBLISH_FAILED)
    span.end()
    ctx, span = telemetry.start_process(None, telemetry.Process("job.gpu", production_verified=True, message_id=bytes(16)))
    assert ctx is None
    span.fail("HANDLER_FAILED")
    span.end()
    for build in (telemetry.from_opentelemetry, lambda: telemetry.TelemetryConfig(object())):
        try:
            build()
        except ImportError as exc:
            assert "cap-sdk-python[otel]" in str(exc), exc
        else:
            raise AssertionError("configured without OpenTelemetry")
    leaked = sorted(m for m in sys.modules if m == "opentelemetry" or m.startswith("opentelemetry."))
    assert not leaked, leaked
    print("cap.telemetry without opentelemetry: OK")
    """
)


def test_module_works_without_opentelemetry() -> None:
    env = dict(os.environ, PYTHONPATH=str(SDK_ROOT))
    result = subprocess.run(
        [sys.executable, "-c", NO_OTEL_SCRIPT],
        cwd=str(SDK_ROOT),
        env=env,
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert result.stdout.strip() == "cap.telemetry without opentelemetry: OK"


def _no_bitor(annotation: ast.expr) -> None:
    for node in ast.walk(annotation):
        assert not (isinstance(node, ast.BinOp) and isinstance(node.op, ast.BitOr)), ast.dump(annotation)
        if isinstance(node, ast.Constant) and isinstance(node.value, str):
            _no_bitor(ast.parse(node.value, mode="eval", feature_version=(3, 9)).body)


def test_no_pep604_annotations() -> None:
    source = (SDK_ROOT / "cap" / "telemetry.py").read_text(encoding="utf-8")
    annotations: List[ast.expr] = []
    for node in ast.walk(ast.parse(source, feature_version=(3, 9))):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            args = node.args
            for arg in args.posonlyargs + args.args + args.kwonlyargs + [args.vararg, args.kwarg]:
                if arg is not None and arg.annotation is not None:
                    annotations.append(arg.annotation)
            if node.returns is not None:
                annotations.append(node.returns)
        elif isinstance(node, ast.AnnAssign):
            annotations.append(node.annotation)
    for annotation in annotations:
        _no_bitor(annotation)
    assert len(annotations) > 10


def test_pyproject_declares_optional_otel() -> None:
    with (SDK_ROOT / "pyproject.toml").open("rb") as handle:
        project = tomllib.load(handle)["project"]
    base = [Requirement(item).name.lower() for item in project["dependencies"]]
    assert not [name for name in base if name.startswith("opentelemetry")]
    extras = project["optional-dependencies"]
    assert extras["otel"] == ["opentelemetry-api>=1.41.1,<2"]
    dev = {Requirement(item).name: Requirement(item).specifier for item in extras["dev"]}
    for name in ("opentelemetry-api", "opentelemetry-sdk"):
        assert dev[name].contains("1.41.1"), name
        assert not dev[name].contains("1.41.0") and not dev[name].contains("2.0.0"), name
