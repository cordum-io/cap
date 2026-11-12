from google.protobuf import timestamp_pb2
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from typing import TYPE_CHECKING, Dict, Optional, Protocol, cast


from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.packet_boundary import Publisher, finalize_packet
from cap.subjects import SUBJECT_SUBMIT
from cap.telemetry import ErrorType, Publish, TelemetryConfig, inject_headers, start_publish

if TYPE_CHECKING:
    from opentelemetry.context import Context as OtelContext


class _HeaderPublisher(Protocol):
    async def publish(
        self, subject: str, payload: bytes, headers: Optional[Dict[str, str]] = None
    ) -> None: ...


async def submit_job(
    nc: Publisher,
    job_request: job_pb2.JobRequest,
    trace_id: str,
    sender_id: str,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    *,
    session_token: Optional[str] = None,
    telemetry: Optional[TelemetryConfig] = None,
    context: "Optional[OtelContext]" = None,
) -> None:
    """Publish a JobRequest onto the CAP submit subject.

    Args:
        nc: An active NATS connection.
        job_request: A protobuf JobRequest message.
        trace_id: Distributed trace identifier propagated through the bus.
        sender_id: Identity of the sender (used in the BusPacket envelope).
        private_key: Optional ECDSA private key for signing the packet.
        session_token: Optional trusted runtime token for the outer envelope.
        telemetry: Optional trace-context configuration. ``None`` sends no
            headers and starts no span. When set, a ``publish sys.job.submit``
            span is started and its W3C trace context is sent in the
            ``traceparent`` and ``tracestate`` headers, so ``nc.publish`` must
            accept a ``headers=`` keyword.
        context: The OpenTelemetry parent context of the publish span; ``None``
            means the current context. Used only when ``telemetry`` is set.
    """
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id
    packet.sender_id = sender_id
    packet.created_at.CopyFrom(ts)
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.job_request.CopyFrom(job_request)

    outgoing = finalize_packet(
        packet, private_key, session_token=session_token
    )
    data = outgoing.SerializeToString(deterministic=True)
    if telemetry is None:
        await nc.publish(SUBJECT_SUBMIT, data)
        return
    span_context, span = start_publish(
        telemetry, Publish(subject=SUBJECT_SUBMIT, conversation_id=trace_id), context
    )
    headers = inject_headers(telemetry, None, span_context)
    try:
        await cast(_HeaderPublisher, nc).publish(SUBJECT_SUBMIT, data, headers=headers)
    except Exception:
        span.fail(ErrorType.TRANSPORT_PUBLISH_FAILED)
        raise
    finally:
        span.end()
