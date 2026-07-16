import asyncio
import logging
import time
from dataclasses import dataclass
from typing import Awaitable, Callable, Dict, Optional, Protocol, Sequence

from google.protobuf import timestamp_pb2

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes

from cap.subjects import SUBJECT_RESULT
from cap.errors import MalformedPacketError, SignatureInvalidError
from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.metrics import MetricsHook, NoopMetrics, safe_metrics_call


_HANDLER_FAILED_MESSAGE = "handler failed"
_LOG_FIELD_LIMIT = 256
JobHandler = Callable[[job_pb2.JobRequest], Awaitable[job_pb2.JobResult]]
Middleware = Callable[[job_pb2.JobRequest, JobHandler], Awaitable[job_pb2.JobResult]]


class _Message(Protocol):
    data: bytes


class _NATSConnection(Protocol):
    async def publish(self, subject: str, data: bytes) -> None: ...
    async def subscribe(self, subject: str, *, queue: str,
                        cb: Callable[[_Message], Awaitable[None]]) -> object: ...
    async def drain(self) -> None: ...


ConnectFn = Callable[..., Awaitable[_NATSConnection]]

@dataclass(frozen=True)
class _WorkerContext:
    connection: _NATSConnection
    handler: JobHandler
    public_keys: Optional[Dict[str, ec.EllipticCurvePublicKey]]
    private_key: Optional[ec.EllipticCurvePrivateKey]
    sender_id: str
    middlewares: Optional[Sequence[Middleware]]
    logger: logging.Logger
    metrics: MetricsHook


def _bounded_log_field(value: str) -> str:
    sanitized = value.replace("\r", "\\r").replace("\n", "\\n")
    if len(sanitized) <= _LOG_FIELD_LIMIT:
        return sanitized
    return sanitized[: _LOG_FIELD_LIMIT - 3] + "..."


def _verify_packet(
    packet: buspacket_pb2.BusPacket,
    public_keys: Optional[Dict[str, ec.EllipticCurvePublicKey]],
    logger: logging.Logger,
) -> bool:
    if not public_keys:
        return True
    safe_sender_id = _bounded_log_field(packet.sender_id)
    public_key = public_keys.get(packet.sender_id)
    if not public_key:
        logger.warning("no public key found", extra={"sender_id": safe_sender_id})
        return False
    signature = packet.signature
    packet.ClearField("signature")
    unsigned_data = packet.SerializeToString(deterministic=True)
    packet.signature = signature
    try:
        public_key.verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))
    except Exception:
        error = SignatureInvalidError(f"sender {safe_sender_id}")
        logger.warning("invalid signature: %s", error, extra={"sender_id": safe_sender_id})
        return False
    return True


def _decode_packet(message: _Message, context: _WorkerContext) -> Optional[buspacket_pb2.BusPacket]:
    packet = buspacket_pb2.BusPacket()
    try:
        packet.ParseFromString(message.data)
    except Exception as exc:
        context.logger.warning("%s", MalformedPacketError(str(exc)))
        return None
    if not _verify_packet(packet, context.public_keys, context.logger):
        return None
    return packet


def _middleware_chain(handler: JobHandler, middlewares: Optional[Sequence[Middleware]]) -> JobHandler:
    wrapped = handler
    if not middlewares:
        return wrapped
    for middleware in reversed(middlewares):
        next_handler = wrapped

        async def invoke(
            request: job_pb2.JobRequest,
            current: Middleware = middleware,
            following: JobHandler = next_handler,
        ) -> job_pb2.JobResult:
            return await current(request, following)

        wrapped = invoke
    return wrapped


def _log_handler_failure(
    context: _WorkerContext,
    packet: buspacket_pb2.BusPacket,
    request: job_pb2.JobRequest,
    error: Exception,
) -> None:
    try:
        context.logger.warning(
            "job handler failed",
            extra={
                "job_id": _bounded_log_field(request.job_id),
                "trace_id": _bounded_log_field(packet.trace_id),
                "exception_type": _bounded_log_field(type(error).__name__),
            },
        )
    except Exception:
        pass


def _failed_result(job_id: str, message: str) -> job_pb2.JobResult:
    return job_pb2.JobResult(
        job_id=job_id,
        status=job_pb2.JOB_STATUS_FAILED,
        error_message=message,
    )


async def _execute_handler(
    context: _WorkerContext,
    packet: buspacket_pb2.BusPacket,
    request: job_pb2.JobRequest,
) -> job_pb2.JobResult:
    wrapped = _middleware_chain(context.handler, context.middlewares)
    try:
        result = await wrapped(request)
    except asyncio.CancelledError:
        raise
    except (KeyboardInterrupt, SystemExit):
        raise
    except Exception as exc:
        _log_handler_failure(context, packet, request, exc)
        return _failed_result(request.job_id, _HANDLER_FAILED_MESSAGE)
    if result is None:
        return _failed_result(request.job_id, "handler returned null")
    return result


def _normalize_result(
    result: job_pb2.JobResult,
    request: job_pb2.JobRequest,
    sender_id: str,
) -> None:
    if not result.job_id:
        result.job_id = request.job_id
    if not result.worker_id:
        result.worker_id = sender_id


def _result_packet(
    source: buspacket_pb2.BusPacket,
    result: job_pb2.JobResult,
    sender_id: str,
    private_key: Optional[ec.EllipticCurvePrivateKey],
) -> buspacket_pb2.BusPacket:
    timestamp = timestamp_pb2.Timestamp()
    timestamp.GetCurrentTime()
    packet = buspacket_pb2.BusPacket(
        trace_id=source.trace_id,
        sender_id=sender_id,
        protocol_version=DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.CopyFrom(timestamp)
    packet.job_result.CopyFrom(result)
    if private_key:
        unsigned_data = packet.SerializeToString(deterministic=True)
        packet.signature = private_key.sign(unsigned_data, ec.ECDSA(hashes.SHA256()))
    return packet


async def _handle_message(message: _Message, context: _WorkerContext) -> None:
    packet = _decode_packet(message, context)
    if packet is None:
        return
    request = packet.job_request
    if not request.job_id:
        return
    safe_metrics_call(
        context.logger,
        lambda: context.metrics.on_job_received(request.job_id, request.topic),
    )
    started_at = time.monotonic()
    result = await _execute_handler(context, packet, request)
    elapsed_ms = int((time.monotonic() - started_at) * 1000)
    _normalize_result(result, request, context.sender_id)
    outgoing = _result_packet(packet, result, context.sender_id, context.private_key)
    await context.connection.publish(
        SUBJECT_RESULT, outgoing.SerializeToString(deterministic=True)
    )
    if result.status == job_pb2.JOB_STATUS_FAILED:
        metric = lambda: context.metrics.on_job_failed(
            result.job_id, result.error_message
        )
    else:
        metric = lambda: context.metrics.on_job_completed(
            result.job_id, elapsed_ms, "SUCCEEDED"
        )
    safe_metrics_call(context.logger, metric)


def _log_cleanup_failure(logger: logging.Logger, error: BaseException) -> None:
    try:
        logger.error(
            "worker connection drain failed",
            extra={"exception_type": _bounded_log_field(type(error).__name__)},
        )
    except BaseException:
        # Cleanup diagnostics must never replace the operation's primary error.
        pass


async def _drain_connection(
    connection: _NATSConnection,
    logger: logging.Logger,
    primary_error: Optional[BaseException],
) -> None:
    try:
        await connection.drain()
    except BaseException as exc:
        if primary_error is None:
            raise
        _log_cleanup_failure(logger, exc)


async def _serve(
    connection: _NATSConnection,
    subject: str,
    callback: Callable[[_Message], Awaitable[None]],
    logger: logging.Logger,
) -> None:
    primary_error: Optional[BaseException] = None
    try:
        await connection.subscribe(subject, queue=subject, cb=callback)
        while True:
            await asyncio.sleep(1)
    except BaseException as exc:
        primary_error = exc
        raise
    finally:
        await _drain_connection(connection, logger, primary_error)


async def run_worker(
    nats_url: str,
    subject: str,
    handler: JobHandler,
    public_keys: Optional[Dict[str, ec.EllipticCurvePublicKey]] = None,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    sender_id: str = "cap-worker",
    connect_fn: Optional[ConnectFn] = None,
    middlewares: Optional[Sequence[Middleware]] = None,
    logger: Optional[logging.Logger] = None,
    metrics: Optional[MetricsHook] = None,
) -> None:
    """Subscribe to jobs until cancelled, then drain the NATS connection."""
    if logger is None:
        logger = logging.getLogger("cap.worker")
    if metrics is None:
        metrics = NoopMetrics()
    if connect_fn is None:
        try:
            import nats  # type: ignore
        except ImportError as exc:
            raise RuntimeError("nats-py is required to connect to NATS") from exc
        connect_fn = nats.connect
    connection = await connect_fn(servers=nats_url, name=sender_id)
    context = _WorkerContext(
        connection=connection,
        handler=handler,
        public_keys=public_keys,
        private_key=private_key,
        sender_id=sender_id,
        middlewares=middlewares,
        logger=logger,
        metrics=metrics,
    )
    async def on_msg(message: _Message) -> None:
        await _handle_message(message, context)

    await _serve(connection, subject, on_msg, logger)
