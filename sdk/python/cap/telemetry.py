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
from typing import TYPE_CHECKING, Dict, FrozenSet, Mapping, Optional, Tuple, Union

from . import subjects

if TYPE_CHECKING:
    from opentelemetry.context import Context
    from opentelemetry.propagators.textmap import TextMapPropagator
    from opentelemetry.trace import Span, SpanContext, Tracer, TracerProvider

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


@dataclass(frozen=True)
class TelemetryConfig:
    """An OpenTelemetry tracer plus an optional trace-context propagator.

    ``propagator`` defaults to the W3C trace-context propagator, never the
    global textmap. Whatever propagator is set, only ``traceparent`` and
    ``tracestate`` reach the headers.
    """

    tracer: "Tracer"
    propagator: "Optional[TextMapPropagator]" = None


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
        """Mark the span failed with a closed ``error.type`` value."""

    def end(self) -> None:
        """End the span."""


def from_opentelemetry(
    tracer_provider: "Optional[TracerProvider]" = None,
    propagator: "Optional[TextMapPropagator]" = None,
) -> TelemetryConfig:
    """Build a config whose tracer comes from ``tracer_provider``.

    A ``None`` provider resolves the global tracer provider at call time.
    """
    raise NotImplementedError


def inject_headers(
    config: Optional[TelemetryConfig],
    headers: Optional[Dict[str, str]],
    context: "Optional[Context]" = None,
) -> Optional[Dict[str, str]]:
    """Write the trace context of ``context`` into ``headers``.

    ``context`` ``None`` means the current OpenTelemetry context.
    """
    return headers


def extract(
    config: Optional[TelemetryConfig],
    headers: Optional[Mapping[str, str]],
) -> "Optional[SpanContext]":
    """Return the valid remote creation context in ``headers``, else None."""
    return None


def start_publish(
    config: Optional[TelemetryConfig],
    publish: Publish,
    context: "Optional[Context]" = None,
) -> "Tuple[Optional[Context], MessagingSpan]":
    """Start the PRODUCER ``publish`` span."""
    return context, MessagingSpan()


def start_process(
    config: Optional[TelemetryConfig],
    process: Process,
    context: "Optional[Context]" = None,
) -> "Tuple[Optional[Context], MessagingSpan]":
    """Start the CONSUMER ``process`` span for an admitted packet."""
    return context, MessagingSpan()
