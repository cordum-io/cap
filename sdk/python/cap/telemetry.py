"""W3C trace context and OpenTelemetry messaging spans for CAP over NATS.

This module implements ``spec/interop/trace-context.md`` with the
OpenTelemetry API only. It imports without OpenTelemetry installed; a
non-None :class:`TelemetryConfig` needs it
(``pip install 'cap-sdk-python[otel]'``).

Rules this module enforces:

- Header values are untrusted correlation hints outside the signed bytes.
  They never select tenant, principal, key, policy or approval.
- Only the lowercase ``traceparent`` and ``tracestate`` headers are written
  or read. ``baggage`` is never written or read, whatever propagator is
  configured.
- Call :func:`extract` only after CAP-PRODUCTION admission or legacy
  validation has accepted the packet. There is no span for rejected packets.
- A ``None`` config is the off switch: it writes no headers, extracts
  nothing and starts no spans.
- A legacy packet's ``trace_id`` and message ID are never recorded.

The Python client and runtime do not call this module yet.
"""

import enum
from dataclasses import dataclass
from typing import TYPE_CHECKING, Dict, FrozenSet, List, Mapping, Optional, Tuple, Union, cast

from . import subjects

if TYPE_CHECKING:
    from opentelemetry.context import Context
    from opentelemetry.propagators.textmap import Getter, Setter, TextMapPropagator
    from opentelemetry.trace import Span, SpanContext, SpanKind, Tracer, TracerProvider

__all__ = [
    "ErrorType",
    "FIXED_SUBJECTS",
    "MessagingSpan",
    "Process",
    "Publish",
    "TelemetryConfig",
    "extract",
    "from_opentelemetry",
    "inject_headers",
    "start_process",
    "start_publish",
]

_INSTALL_HINT = "cap.telemetry needs OpenTelemetry: pip install 'cap-sdk-python[otel]'"
_TRACE_KEYS = ("traceparent", "tracestate")
_POOL_FORBIDDEN = frozenset("*> \t\r\n")
_Attributes = Dict[str, Union[str, bool]]


class ErrorType(str, enum.Enum):
    """The closed ``error.type`` values of the trace-context contract."""

    PROTOCOL_VERSION_MISMATCH = "PROTOCOL_VERSION_MISMATCH"
    PROTOCOL_MALFORMED_PACKET = "PROTOCOL_MALFORMED_PACKET"
    PROTOCOL_UNKNOWN_PAYLOAD = "PROTOCOL_UNKNOWN_PAYLOAD"
    PROTOCOL_SIGNATURE_INVALID = "PROTOCOL_SIGNATURE_INVALID"
    PROTOCOL_SIGNATURE_MISSING = "PROTOCOL_SIGNATURE_MISSING"
    ADMISSION_REJECTED = "ADMISSION_REJECTED"
    HANDLER_FAILED = "HANDLER_FAILED"
    TRANSPORT_PUBLISH_FAILED = "TRANSPORT_PUBLISH_FAILED"
    OTHER = "_OTHER"


_PUBLISH_ERRORS = frozenset({ErrorType.TRANSPORT_PUBLISH_FAILED.value, ErrorType.OTHER.value})
_PROCESS_ERRORS = frozenset(member.value for member in ErrorType) - {ErrorType.TRANSPORT_PUBLISH_FAILED.value}

FIXED_SUBJECTS: FrozenSet[str] = frozenset(
    {
        subjects.SUBJECT_SUBMIT,
        subjects.SUBJECT_RESULT,
        subjects.SUBJECT_HEARTBEAT,
        subjects.SUBJECT_ALERT,
        subjects.SUBJECT_PROGRESS,
        subjects.SUBJECT_CANCEL,
        subjects.SUBJECT_DLQ,
        subjects.SUBJECT_WORKFLOW_EVENT,
        subjects.SUBJECT_HANDSHAKE,
        subjects.SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
        subjects.SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
    }
)
"""The fixed ``sys.*`` subjects of the spec/09 subject table."""


def _require_opentelemetry() -> None:
    try:
        import opentelemetry.trace  # noqa: F401
    except ImportError as exc:
        raise ImportError(_INSTALL_HINT) from exc


@dataclass(frozen=True)
class TelemetryConfig:
    """An OpenTelemetry tracer plus an optional trace-context propagator.

    ``propagator`` defaults to the W3C trace-context propagator, never the
    global textmap. Whatever propagator is set, only ``traceparent`` and
    ``tracestate`` reach the headers.
    """

    tracer: "Tracer"
    propagator: "Optional[TextMapPropagator]" = None

    def __post_init__(self) -> None:
        _require_opentelemetry()
        from opentelemetry.propagators.textmap import TextMapPropagator
        from opentelemetry.trace import Tracer

        if not isinstance(self.tracer, Tracer):
            raise TypeError("tracer must be an opentelemetry.trace.Tracer")
        if self.propagator is not None and not isinstance(self.propagator, TextMapPropagator):
            raise TypeError("propagator must be None or an opentelemetry TextMapPropagator")


@dataclass(frozen=True)
class Publish:
    """What a publish span describes.

    ``pool`` is the ``job.<pool>`` subject the participant is configured to
    publish to; the span names the destination only when it equals
    ``subject``. Leave ``conversation_id`` empty when the packet's
    ``trace_id`` was copied from an unauthenticated packet (spec/10 Safe
    Observability).
    """

    subject: str
    pool: str = ""
    reply_inbox: bool = False
    conversation_id: str = ""


@dataclass(frozen=True)
class Process:
    """What a process span describes, for an admitted packet only.

    ``production_verified`` is True only when the packet passed
    CAP-PRODUCTION admission; only then are ``message_id`` (16 bytes) and
    ``conversation_id`` recorded. ``creation`` is the result of
    :func:`extract`.
    """

    subject: str
    pool: str = ""
    reply_inbox: bool = False
    queue_group: str = ""
    durable: str = ""
    production_verified: bool = False
    message_id: bytes = b""
    conversation_id: str = ""
    creation: "Optional[SpanContext]" = None


class MessagingSpan:
    """A started publish or process span that accepts only contract data."""

    __slots__ = ("_span", "_allowed")

    def __init__(self, span: "Optional[Span]" = None, allowed: FrozenSet[str] = frozenset()) -> None:
        self._span = span
        self._allowed = allowed

    def fail(self, error_type: Union[ErrorType, str]) -> None:
        """Mark the span failed with a closed ``error.type`` value.

        A value outside the span's closed set is recorded as ``_OTHER``. No
        exception, message or event is recorded.
        """
        if self._span is None:
            return
        from opentelemetry.trace import Status, StatusCode

        value = error_type.value if isinstance(error_type, ErrorType) else error_type
        if not isinstance(value, str) or value not in self._allowed:
            value = ErrorType.OTHER.value
        self._span.set_attribute("error.type", value)
        self._span.set_status(Status(StatusCode.ERROR, value))

    def end(self) -> None:
        """End the span."""
        if self._span is not None:
            self._span.end()


_NOOP_SPAN = MessagingSpan()


class _TraceHeaders:
    """Getter and setter for exactly the lowercase trace-context header keys."""

    def get(self, carrier: Mapping[str, str], key: str) -> Optional[List[str]]:
        value = carrier.get(key) if key in _TRACE_KEYS else None
        return [value] if isinstance(value, str) else None

    def keys(self, carrier: Mapping[str, str]) -> List[str]:
        return [key for key in _TRACE_KEYS if key in carrier]

    def set(self, carrier: Dict[str, str], key: str, value: str) -> None:
        if key in _TRACE_KEYS:
            carrier[key] = value


_CARRIER = _TraceHeaders()


def _propagator(config: TelemetryConfig) -> "TextMapPropagator":
    if config.propagator is not None:
        return config.propagator
    from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

    return TraceContextTextMapPropagator()


def from_opentelemetry(
    tracer_provider: "Optional[TracerProvider]" = None,
    propagator: "Optional[TextMapPropagator]" = None,
) -> TelemetryConfig:
    """Build a config whose tracer comes from ``tracer_provider``.

    A ``None`` provider resolves the global tracer provider at call time.
    """
    _require_opentelemetry()
    import opentelemetry.trace

    return TelemetryConfig(opentelemetry.trace.get_tracer("cap.telemetry", tracer_provider=tracer_provider), propagator)


def inject_headers(
    config: Optional[TelemetryConfig],
    headers: Optional[Dict[str, str]],
    context: "Optional[Context]" = None,
) -> Optional[Dict[str, str]]:
    """Write the trace context of ``context`` into ``headers``.

    ``context`` ``None`` means the current OpenTelemetry context. Existing
    ``traceparent`` and ``tracestate`` values are replaced or removed; other
    keys are kept. ``headers`` is updated in place and returned; when it is
    ``None``, a new dict is returned, or ``None`` if nothing was written.
    """
    if config is None:
        return headers
    carrier: Dict[str, str] = {} if headers is None else headers
    for key in _TRACE_KEYS:
        carrier.pop(key, None)
    _propagator(config).inject(carrier, context=context, setter=cast("Setter[Dict[str, str]]", _CARRIER))
    if headers is None and not carrier:
        return None
    return carrier


def extract(
    config: Optional[TelemetryConfig],
    headers: Optional[Mapping[str, str]],
) -> "Optional[SpanContext]":
    """Return the valid remote creation context in ``headers``, else None.

    Call it only for a packet that admission or validation accepted. The
    result is a correlation hint, never authority.
    """
    if config is None or headers is None:
        return None
    from opentelemetry import trace
    from opentelemetry.context import Context

    getter = cast("Getter[Mapping[str, str]]", _CARRIER)
    extracted = _propagator(config).extract(headers, context=Context(), getter=getter)
    span_context = trace.get_current_span(extracted).get_span_context()
    return span_context if span_context.is_valid and span_context.is_remote else None


def _attributes(op: str, op_type: str, subject: str, pool: str, reply_inbox: bool) -> Tuple[str, _Attributes]:
    attributes: _Attributes = {
        "messaging.system": "nats",
        "messaging.operation.name": op,
        "messaging.operation.type": op_type,
    }
    if reply_inbox:
        attributes["messaging.destination.temporary"] = True
        attributes["messaging.destination.anonymous"] = True
        return op, attributes
    is_pool = pool.startswith("job.") and len(pool) > len("job.") and not _POOL_FORBIDDEN.intersection(pool)
    if subject in FIXED_SUBJECTS or (subject == pool and is_pool):
        attributes["messaging.destination.name"] = subject
        return op + " " + subject, attributes
    return op, attributes


def _start(
    config: TelemetryConfig,
    name: str,
    kind: "SpanKind",
    attributes: _Attributes,
    context: "Optional[Context]",
    creation: "Optional[SpanContext]",
    allowed: FrozenSet[str],
) -> "Tuple[Optional[Context], MessagingSpan]":
    from opentelemetry import trace

    links = None
    if creation is not None and creation.is_valid:
        links = [trace.Link(creation)]
        if not trace.get_current_span(context).get_span_context().is_valid:
            context = trace.set_span_in_context(trace.NonRecordingSpan(creation), context)
    span = config.tracer.start_span(
        name,
        context=context,
        kind=kind,
        attributes=attributes,
        links=links,
        record_exception=False,
        set_status_on_exception=False,
    )
    return trace.set_span_in_context(span, context), MessagingSpan(span, allowed)


def start_publish(
    config: Optional[TelemetryConfig],
    publish: Publish,
    context: "Optional[Context]" = None,
) -> "Tuple[Optional[Context], MessagingSpan]":
    """Start the PRODUCER ``publish`` span.

    Inject the returned context into the message headers: the publish span
    is the creation context. ``context`` ``None`` means the current context.
    A caller leaves ``publish.conversation_id`` empty when the packet's
    ``trace_id`` was copied from an unauthenticated packet (spec/10 Safe
    Observability).
    """
    if config is None:
        return context, _NOOP_SPAN
    from opentelemetry.trace import SpanKind

    name, attributes = _attributes("publish", "send", publish.subject, publish.pool, publish.reply_inbox)
    if publish.conversation_id:
        attributes["messaging.message.conversation_id"] = publish.conversation_id
    return _start(config, name, SpanKind.PRODUCER, attributes, context, None, _PUBLISH_ERRORS)


def start_process(
    config: Optional[TelemetryConfig],
    process: Process,
    context: "Optional[Context]" = None,
) -> "Tuple[Optional[Context], MessagingSpan]":
    """Start the CONSUMER ``process`` span for an admitted packet.

    A valid ``process.creation`` becomes a link, and also the parent when
    ``context`` has no active span. ``context`` ``None`` means the current
    context.
    """
    if config is None:
        return context, _NOOP_SPAN
    from opentelemetry.trace import SpanKind

    name, attributes = _attributes("process", "process", process.subject, process.pool, process.reply_inbox)
    if process.queue_group:
        attributes["messaging.consumer.group.name"] = process.queue_group
    if process.durable:
        attributes["messaging.destination.subscription.name"] = process.durable
    if process.production_verified:
        if len(process.message_id) == 16:
            attributes["messaging.message.id"] = process.message_id.hex()
        if process.conversation_id:
            attributes["messaging.message.conversation_id"] = process.conversation_id
    return _start(config, name, SpanKind.CONSUMER, attributes, context, process.creation, _PROCESS_ERRORS)
