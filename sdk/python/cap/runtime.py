import asyncio
import json
import logging
import os
import secrets
import time
from dataclasses import dataclass
from typing import Any, Awaitable, Callable, Coroutine, Dict, Optional, Protocol, Type, TypeVar, Union

from google.protobuf import timestamp_pb2
from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2, job_pb2
from cryptography.hazmat.primitives.asymmetric import ec

try:
    import redis.asyncio as redis_async  # type: ignore
except Exception:  # pragma: no cover - optional until runtime used
    redis_async = None

try:
    from pydantic import BaseModel, ValidationError
except Exception:  # pragma: no cover - optional until runtime used
    BaseModel = None
    ValidationError = Exception


from cap.subjects import SUBJECT_HANDSHAKE, SUBJECT_RESULT
from cap.errors import InvalidInputError
from cap.metrics import MetricsHook, NoopMetrics, safe_metrics_call
from cap.heartbeat import heartbeat_loop, heartbeat_payload
from cap.packet_boundary import decode_packet, finalize_packet, parse_packet
from cap.worker_trust import WorkerTrustConfig, WorkerTrustMode
from cap.worker_trust_runtime import (
    DEFAULT_RENEW_MIN_INTERVAL,
    RuntimeTrustSettings,
    WorkerTrustLifecycle,
    WorkerTrustRuntimeError,
)

from cap.constants import DEFAULT_PROTOCOL_VERSION  # noqa: E402 — must be after conditional imports


_HANDLER_FAILED_MESSAGE = "handler failed"
_LOG_FIELD_LIMIT = 256


def _bounded_log_field(value: object) -> str:
    sanitized = str(value).replace("\r", "\\r").replace("\n", "\\n")
    if len(sanitized) <= _LOG_FIELD_LIMIT:
        return sanitized
    return sanitized[: _LOG_FIELD_LIMIT - 3] + "..."


def _normalize_max_parallel(value: int) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise TypeError("max_parallel must be an integer")
    return max(1, value)


def _log_handler_failure(
    logger: logging.Logger,
    packet: buspacket_pb2.BusPacket,
    request: job_pb2.JobRequest,
    error: Exception,
    attempt: int,
    max_attempts: int,
) -> None:
    try:
        logger.warning(
            "handler failed",
            extra={
                "job_id": _bounded_log_field(request.job_id),
                "trace_id": _bounded_log_field(packet.trace_id),
                "topic": _bounded_log_field(request.topic),
                "sender_id": _bounded_log_field(packet.sender_id),
                "attempt": attempt,
                "max_attempts": max_attempts,
                "exception_type": _bounded_log_field(type(error).__name__),
            },
        )
    except Exception:
        pass


class BlobStore(Protocol):
    """Abstraction over payload storage (Redis, in-memory, etc.)."""

    async def get(self, key: str) -> Optional[bytes]:
        ...

    async def set(self, key: str, data: bytes) -> None:
        ...

    async def close(self) -> None:
        ...


def redis_ssl_context_from_env() -> Optional["ssl.SSLContext"]:
    """Build an :class:`ssl.SSLContext` from ``REDIS_TLS_*`` env vars.

    Matches the Go SDK's ``RedisTLSConfigFromEnv()`` pattern. Supports:

    * ``REDIS_TLS_CA`` (or ``SSL_CERT_FILE`` fallback) -- CA certificate path
    * ``REDIS_TLS_CERT`` + ``REDIS_TLS_KEY`` -- client certificate pair
    * ``REDIS_TLS_SERVER_NAME`` -- SNI override
    * ``REDIS_TLS_INSECURE`` -- skip certificate verification (dev/test only)

    Returns ``None`` when no TLS env vars are set.
    """
    import ssl as _ssl

    ca_path = (os.environ.get("REDIS_TLS_CA") or os.environ.get("SSL_CERT_FILE") or "").strip()
    cert_path = os.environ.get("REDIS_TLS_CERT", "").strip()
    key_path = os.environ.get("REDIS_TLS_KEY", "").strip()
    server_name = os.environ.get("REDIS_TLS_SERVER_NAME", "").strip()
    insecure = os.environ.get("REDIS_TLS_INSECURE", "").strip().lower() in ("1", "true")

    if not ca_path and not cert_path and not key_path and not server_name and not insecure:
        return None

    if bool(cert_path) != bool(key_path):
        raise ValueError("REDIS_TLS_CERT and REDIS_TLS_KEY must be set together")

    if insecure:
        ssl_ctx = _ssl.SSLContext(_ssl.PROTOCOL_TLS_CLIENT)
        ssl_ctx.minimum_version = _ssl.TLSVersion.TLSv1_2
        ssl_ctx.check_hostname = False
        ssl_ctx.verify_mode = _ssl.CERT_NONE
        logging.getLogger("cap.runtime").warning("REDIS_TLS_INSECURE enabled, skipping certificate verification")
    elif ca_path:
        if not os.path.isfile(ca_path):
            raise FileNotFoundError(f"REDIS_TLS_CA file not found: {ca_path}")
        ssl_ctx = _ssl.create_default_context(cafile=ca_path)
        ssl_ctx.minimum_version = _ssl.TLSVersion.TLSv1_2
    else:
        ssl_ctx = _ssl.create_default_context()
        ssl_ctx.minimum_version = _ssl.TLSVersion.TLSv1_2

    if cert_path and key_path:
        if not os.path.isfile(cert_path):
            raise FileNotFoundError(f"REDIS_TLS_CERT file not found: {cert_path}")
        if not os.path.isfile(key_path):
            raise FileNotFoundError(f"REDIS_TLS_KEY file not found: {key_path}")
        ssl_ctx.load_cert_chain(cert_path, key_path)

    if server_name:
        ssl_ctx.check_hostname = True
        ssl_ctx.server_hostname = server_name  # type: ignore[attr-defined]

    return ssl_ctx


def _redis_tls_kwargs(redis_url: str) -> dict:
    """Build TLS kwargs for redis-py 7+ from environment variables.

    Uses ssl_ca_certs/ssl_certfile/ssl_keyfile kwargs (not ssl=SSLContext)
    because redis-py 7 does not accept SSLContext via from_url().
    """
    kwargs: dict = {}
    if redis_url.startswith("rediss://"):
        ca = os.environ.get("REDIS_TLS_CA", "").strip()
        cert = os.environ.get("REDIS_TLS_CERT", "").strip()
        key = os.environ.get("REDIS_TLS_KEY", "").strip()
        if ca:
            kwargs["ssl_ca_certs"] = ca
        if cert:
            kwargs["ssl_certfile"] = cert
        if key:
            kwargs["ssl_keyfile"] = key
    return kwargs


async def ping_redis(redis_url: str) -> None:
    """Connect to Redis and run PING to verify auth and TLS at startup.

    Raises on failure so connection issues surface immediately instead of
    silently failing on the first blob store read/write.
    """
    if redis_async is None:
        raise RuntimeError("redis is required")
    kwargs = _redis_tls_kwargs(redis_url)
    client = redis_async.from_url(redis_url, **kwargs)
    try:
        await client.ping()
    finally:
        await client.close()


class RedisBlobStore:
    """Redis-backed :class:`BlobStore` implementation."""

    def __init__(self, redis_url: str) -> None:
        if redis_async is None:
            raise RuntimeError("redis is required for RedisBlobStore")
        kwargs = _redis_tls_kwargs(redis_url)
        self._client = redis_async.from_url(redis_url, **kwargs)

    async def get(self, key: str) -> Optional[bytes]:
        value = await self._client.get(key)
        return value

    async def set(self, key: str, data: bytes) -> None:
        await self._client.set(key, data)

    async def close(self) -> None:
        await self._client.close()


class InMemoryBlobStore:
    """In-memory :class:`BlobStore` for testing without external infrastructure."""

    def __init__(self) -> None:
        self._data: Dict[str, bytes] = {}

    async def get(self, key: str) -> Optional[bytes]:
        return self._data.get(key)

    async def set(self, key: str, data: bytes) -> None:
        self._data[key] = data

    async def close(self) -> None:
        return None


def pointer_for_key(key: str) -> str:
    return "redis://" + key


def key_from_pointer(ptr: str) -> str:
    if not ptr:
        raise InvalidInputError("empty pointer")
    if not ptr.startswith("redis://"):
        raise InvalidInputError("unsupported pointer scheme")
    key = ptr[len("redis://") :]
    if not key:
        raise InvalidInputError("missing pointer key")
    return key


class _StructuredFormatter(logging.Formatter):
    """Formatter that appends structured fields (job_id, trace_id, etc.) to log output."""
    _EXTRA_FIELDS = ("job_id", "trace_id", "topic", "sender_id")

    def format(self, record: logging.LogRecord) -> str:
        base = f"{record.levelname} {record.getMessage()}"
        extras = []
        for field in self._EXTRA_FIELDS:
            value = getattr(record, field, None)
            if value:
                extras.append(f"{field}={value}")
        if extras:
            return f"{base} {' '.join(extras)}"
        return base


def _default_logger() -> logging.Logger:
    logger = logging.getLogger("cap.runtime")
    if not logger.handlers:
        handler = logging.StreamHandler()
        handler.setFormatter(_StructuredFormatter())
        logger.addHandler(handler)
        logger.setLevel(logging.INFO)
    return logger


@dataclass
class Context:
    """Per-request context passed to every job handler."""

    job: job_pb2.JobRequest
    packet: buspacket_pb2.BusPacket
    logger: logging.LoggerAdapter

    @property
    def job_id(self) -> str:
        return self.job.job_id

    @property
    def trace_id(self) -> str:
        return self.packet.trace_id


TIn = TypeVar("TIn")
TOut = TypeVar("TOut")
TAny = TypeVar("TAny")


class _JobDecorator(Protocol):
    def __call__(
        self, func: Callable[[Context, TIn], TOut]
    ) -> Callable[[Context, TIn], TOut]:
        ...


@dataclass
class HandlerSpec:
    topic: str
    func: Callable[[Context, Any], Union[Awaitable[Any], Any]]
    input_model: Optional[Type[Any]]
    output_model: Optional[Type[Any]]
    retries: int


class Agent:
    """High-level runtime that manages typed job handlers, blob storage, and NATS subscriptions.

    Register handlers with :meth:`job`, then call :meth:`run` to start processing.
    """

    def __init__(
        self,
        *,
        nats_url: Optional[str] = None, redis_url: Optional[str] = None,
        store: Optional[BlobStore] = None,
        public_keys: Optional[Dict[str, ec.EllipticCurvePublicKey]] = None,
        private_key: Optional[ec.EllipticCurvePrivateKey] = None,
        sender_id: str = "cap-runtime",
        worker_id: Optional[str] = None,
        pool: str = "",
        max_parallel: int = 1,
        heartbeat_interval: float = 5.0,
        heartbeat_payload_fn: Optional[Callable[[], bytes]] = None,
        retries: int = 0,
        io_timeout: Optional[float] = 5.0,
        shutdown_timeout: Optional[float] = 30.0,
        max_context_bytes: Optional[int] = 2 * 1024 * 1024,
        max_result_bytes: Optional[int] = 2 * 1024 * 1024,
        connect_fn: Optional[Callable[..., Awaitable[Any]]] = None,
        logger: Optional[logging.Logger] = None,
        metrics: Optional[MetricsHook] = None,
        worker_trust_mode: Optional[Union[str, WorkerTrustMode]] = None,
        worker_trust: Optional[WorkerTrustConfig] = None,
        worker_trust_timeout: Optional[float] = None, worker_trust_retries: Optional[int] = None,
        worker_trust_renew_min_interval: float = DEFAULT_RENEW_MIN_INTERVAL,
    ) -> None:
        self._nats_url = nats_url or os.getenv("NATS_URL", "nats://127.0.0.1:4222")
        self._redis_url = redis_url or os.getenv("REDIS_URL", "redis://127.0.0.1:6379/0")
        self._store = store
        self._public_keys = public_keys
        self._private_key = private_key
        self._sender_id = worker_id or sender_id
        self._pool = pool
        self._max_parallel = _normalize_max_parallel(max_parallel)
        self._heartbeat_interval = heartbeat_interval if heartbeat_interval > 0 else 5.0
        self._heartbeat_payload_fn = heartbeat_payload_fn
        self._default_retries = max(0, retries)
        self._io_timeout = io_timeout if io_timeout and io_timeout > 0 else None
        if shutdown_timeout is not None and shutdown_timeout <= 0:
            raise ValueError("shutdown_timeout must be positive or None")
        self._shutdown_timeout = shutdown_timeout
        self._max_context_bytes = max_context_bytes if max_context_bytes and max_context_bytes > 0 else None
        self._max_result_bytes = max_result_bytes if max_result_bytes and max_result_bytes > 0 else None
        self._connect_fn = connect_fn
        self._logger = logger or _default_logger()
        self._metrics: MetricsHook = metrics or NoopMetrics()
        self._configure_worker_trust(worker_trust_mode, worker_trust,
            worker_trust_timeout, worker_trust_retries,
            worker_trust_renew_min_interval)
        Agent._initialize_lifecycle(self)

    def _configure_worker_trust(
        self,
        mode: Optional[Union[str, WorkerTrustMode]],
        config: Optional[WorkerTrustConfig],
        timeout: Optional[float],
        retries: Optional[int],
        renew_min_interval: float,
    ) -> None:
        self._worker_trust_mode = mode
        self._worker_trust_config = config
        self._worker_trust_timeout = timeout
        self._worker_trust_retries = retries
        self._worker_trust_renew_min_interval = renew_min_interval

    def _initialize_lifecycle(self) -> None:
        self._handlers: Dict[str, HandlerSpec] = {}
        self._middlewares: list = []
        self._nc = None
        self._active_jobs: set[str] = set()
        self._heartbeat_cancel_event: Optional[asyncio.Event] = None
        self._heartbeat_task: Optional[asyncio.Task[Any]] = None
        self._subscriptions: list[Any] = []
        self._handler_slots = asyncio.BoundedSemaphore(self._max_parallel)
        self._dispatch_waiters: set[asyncio.Task[Any]] = set()
        self._dispatch_closed = False
        self._handler_tasks: set[asyncio.Task[Any]] = set()
        self._lifecycle_state = "idle"
        self._close_task: Optional[asyncio.Task[Any]] = None
        self._trust_settings: Optional[RuntimeTrustSettings] = None
        self._worker_trust: Optional[WorkerTrustLifecycle] = None
        self._capability: Optional[handshake_pb2.Handshake] = None
        self._trust_admitting = False
        self._trust_startup_interrupted = False
        self._trust_reconnect_lock = asyncio.Lock()

    def use(self, *middlewares) -> None:
        """Append middleware to the agent. Middleware executes in registration order before the handler."""
        self._middlewares.extend(middlewares)

    def job(
        self,
        topic: str,
        *,
        input_model: Optional[Type[Any]] = None,
        output_model: Optional[Type[Any]] = None,
        retries: Optional[int] = None,
    ) -> _JobDecorator:
        """Decorator that registers a handler for *topic*.

        Args:
            topic: NATS subject the handler subscribes to.
            input_model: Optional Pydantic model or callable for input validation.
            output_model: Optional Pydantic model or callable for output validation.
            retries: Override the default retry count for this handler.
        """
        def decorator(
            func: Callable[[Context, TIn], TOut]
        ) -> Callable[[Context, TIn], TOut]:
            spec = HandlerSpec(
                topic=topic,
                func=func,
                input_model=input_model,
                output_model=output_model,
                retries=self._default_retries if retries is None else max(0, retries),
            )
            self._handlers[topic] = spec
            return func

        return decorator

    async def start(self) -> None:
        """Connect to NATS/Redis and register subscriptions for all handlers."""
        if not self._handlers:
            raise RuntimeError("no handlers registered")
        if self._lifecycle_state != "idle":
            raise RuntimeError(f"agent cannot start while {self._lifecycle_state}")
        self._trust_settings = RuntimeTrustSettings.resolve(
            self._worker_trust_mode,
            self._worker_trust_config,
            self._sender_id,
            timeout=self._worker_trust_timeout,
            retries=self._worker_trust_retries,
            renew_min_interval=self._worker_trust_renew_min_interval,
        )
        self._capability = self._build_capability_handshake()
        self._lifecycle_state = "starting"
        try:
            await self._open_resources()
            self._require_trust_startup_continuity()
        except BaseException as exc:
            await self._cleanup_resources(exc)
            raise
        self._lifecycle_state = "running"

    @property
    def session_token(self) -> str:
        """Return only the current, still-live authenticated session token."""
        if self._worker_trust is None:
            return ""
        return self._worker_trust.session_token()

    async def close(self) -> None:
        """Stop intake, flush tracked jobs, and close resources exactly once."""
        if self._lifecycle_state == "starting":
            raise RuntimeError("agent start is still in progress")
        if self._lifecycle_state == "closed" and self._close_task is None:
            return
        if self._close_task is None:
            self._lifecycle_state = "closing"
            self._close_task = asyncio.create_task(self._cleanup_resources())
        await asyncio.shield(self._close_task)

    def _resolve_connect_fn(self) -> Callable[..., Awaitable[Any]]:
        if self._connect_fn is not None:
            return self._connect_fn
        try:
            import nats  # type: ignore
        except ImportError as exc:
            raise RuntimeError("nats-py is required to connect to NATS") from exc
        self._connect_fn = nats.connect
        return self._connect_fn

    async def _open_resources(self) -> None:
        connect_fn = self._resolve_connect_fn()
        connect_options: Dict[str, Any] = {
            "servers": self._nats_url,
            "name": self._sender_id,
        }
        if self._trust_enabled():
            connect_options["reconnected_cb"] = self._on_nats_reconnected
            connect_options["disconnected_cb"] = self._on_nats_disconnected
        self._nc = await self._with_timeout(
            connect_fn(**connect_options),
            "nats connect",
        )
        if self._store is None:
            self._store = RedisBlobStore(self._redis_url)
        await self._establish_worker_trust()
        self._require_trust_startup_continuity()
        await self._subscribe_handlers()
        self._require_trust_startup_continuity()
        await self._publish_startup_handshake()
        self._require_trust_startup_continuity()
        self._start_heartbeat()
        if self._worker_trust is not None:
            self._worker_trust.start_renewal(self._on_trust_failure)

    def _build_capability_handshake(self) -> handshake_pb2.Handshake:
        sdk_version = "cap-python/v2"
        if self._trust_settings is not None and self._trust_settings.config is not None:
            sdk_version = self._trust_settings.config.sdk_version
        topics = sorted(self._handlers)
        return handshake_pb2.Handshake(
            component_id=self._sender_id,
            role=handshake_pb2.COMPONENT_ROLE_WORKER,
            supported_versions=[DEFAULT_PROTOCOL_VERSION],
            capabilities={topic: True for topic in topics},
            sdk_version=sdk_version,
            ready_topics=topics,
        )

    def _trust_enabled(self) -> bool:
        return (
            self._trust_settings is not None
            and self._trust_settings.mode != WorkerTrustMode.OFF
        )

    def _enforce_trust(self) -> bool:
        return (
            self._trust_settings is not None
            and self._trust_settings.mode == WorkerTrustMode.ENFORCE
        )

    def _outbound_session_token(self) -> str:
        token = self.session_token
        if self._enforce_trust() and (not token or not self._trust_admitting):
            raise WorkerTrustRuntimeError(
                "authenticated session is required for outbound packet"
            )
        return token

    async def _establish_worker_trust(self) -> None:
        if not self._trust_enabled():
            self._trust_admitting = True
            return
        if self._capability is None:
            raise RuntimeError("worker capability is unavailable")
        self._worker_trust = WorkerTrustLifecycle(
            self._nc,
            self._trust_settings,
            self._capability,
            logger=self._logger,
        )
        await self._worker_trust.authenticate()
        self._require_trust_startup_continuity()
        self._trust_admitting = True

    def _require_trust_startup_continuity(self) -> None:
        if self._trust_enabled() and self._trust_startup_interrupted:
            raise WorkerTrustRuntimeError(
                "worker trust transport interrupted during startup"
            )

    async def _subscribe_handlers(self) -> None:
        for topic, spec in self._handlers.items():
            async def on_topic(message: Any, current: HandlerSpec = spec) -> None:
                await self._dispatch_handler(lambda: self._on_msg(message, current))

            handle = await self._nc.subscribe(topic, queue=topic, cb=on_topic)
            self._remember_subscription(handle)
        if self._sender_id:
            subject = f"worker.{self._sender_id}.jobs"

            async def on_direct(message: Any) -> None:
                await self._dispatch_handler(lambda: self._on_direct_msg(message))

            handle = await self._nc.subscribe(subject, cb=on_direct)
            self._remember_subscription(handle)

    def _remember_subscription(self, handle: Any) -> None:
        if handle is not None and callable(getattr(handle, "drain", None)):
            self._subscriptions.append(handle)

    async def _on_trust_failure(self, error: Exception) -> None:
        self._trust_admitting = False
        await self._stop_heartbeat(error)
        primary = await self._drain_subscriptions(error)
        if primary is not error:
            self._safe_cleanup_log("trust admission stop", primary)
        self._logger.error(
            "authenticated session renewal failed; admissions stopped",
            extra={"exception_type": type(error).__name__},
        )

    async def _on_nats_reconnected(self) -> None:
        async with self._trust_reconnect_lock:
            await self._reauthenticate_after_reconnect()

    async def _on_nats_disconnected(self) -> None:
        if self._trust_enabled():
            self._trust_admitting = False
            if self._lifecycle_state == "starting":
                self._trust_startup_interrupted = True

    async def _reauthenticate_after_reconnect(self) -> None:
        if self._lifecycle_state != "running" or self._worker_trust is None:
            return
        enforce = (
            self._trust_settings is not None
            and self._trust_settings.mode == WorkerTrustMode.ENFORCE
        )
        if enforce:
            self._trust_admitting = False
            failure = await self._drain_subscriptions(None)
            if failure is not None:
                await self._on_trust_failure(failure)
                return
        try:
            await self._worker_trust.reauthenticate()
            if self._lifecycle_state != "running":
                return
            if enforce or not self._subscriptions:
                await self._subscribe_handlers()
            self._trust_admitting = True
            await self._publish_startup_handshake()
            if self._heartbeat_task is None:
                self._start_heartbeat()
            self._worker_trust.start_renewal(self._on_trust_failure)
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            if enforce:
                await self._on_trust_failure(exc)
            else:
                self._safe_cleanup_log("worker trust reconnect", exc)

    async def _dispatch_handler(
        self, factory: Callable[[], Coroutine[Any, Any, None]]
    ) -> None:
        if self._dispatch_closed:
            return
        waiter = asyncio.current_task()
        if waiter is not None:
            self._dispatch_waiters.add(waiter)
        acquired = False
        try:
            await self._handler_slots.acquire()
            acquired = True
            task = asyncio.create_task(factory())
            self._handler_tasks.add(task)
            task.add_done_callback(self._handler_finished)
            acquired = False
        except BaseException:
            if acquired:
                self._handler_slots.release()
            raise
        finally:
            if waiter is not None:
                self._dispatch_waiters.discard(waiter)

    def _handler_finished(self, task: asyncio.Task[Any]) -> None:
        self._handler_tasks.discard(task)
        self._handler_slots.release()
        if task.cancelled():
            return
        error = task.exception()
        if error is not None:
            self._safe_cleanup_log("job task", error)

    async def _publish_startup_handshake(self) -> None:
        if self._capability is None:
            raise RuntimeError("worker capability is unavailable")
        session_token = self._outbound_session_token()
        packet = buspacket_pb2.BusPacket(
            trace_id=secrets.token_hex(16),
            sender_id=self._sender_id,
            protocol_version=DEFAULT_PROTOCOL_VERSION,
        )
        packet.created_at.GetCurrentTime()
        packet.handshake.CopyFrom(self._capability)
        try:
            outgoing = finalize_packet(
                packet, self._private_key, session_token=session_token
            )
            await self._with_timeout(
                self._nc.publish(
                    SUBJECT_HANDSHAKE,
                    outgoing.SerializeToString(deterministic=True),
                ),
                "handshake publish",
            )
        except Exception as exc:
            if self._trust_enabled():
                raise
            self._logger.warning(
                "handshake publish failed",
                extra={"sender_id": self._sender_id,
                       "exception_type": type(exc).__name__},
            )

    def _start_heartbeat(self) -> None:
        self._heartbeat_cancel_event = asyncio.Event()
        payload_fn = self._heartbeat_payload_fn or self._default_heartbeat_payload

        def secured_payload() -> bytes:
            session_token = self._outbound_session_token()
            packet = parse_packet(payload_fn())
            outgoing = finalize_packet(
                packet, self._private_key, session_token=session_token
            )
            return outgoing.SerializeToString(deterministic=True)

        self._heartbeat_task = asyncio.create_task(
            heartbeat_loop(
                nc=self._nc,
                payload_fn=secured_payload,
                interval=self._heartbeat_interval,
                private_key=None,
                metrics=self._metrics,
                cancel_event=self._heartbeat_cancel_event,
            )
        )

    def _safe_cleanup_log(self, stage: str, error: BaseException) -> None:
        try:
            self._logger.error(
                "agent cleanup stage failed",
                extra={"stage": stage, "exception_type": type(error).__name__},
            )
        except Exception:
            pass

    def _record_cleanup_error(
        self,
        stage: str,
        error: BaseException,
        primary: Optional[BaseException],
    ) -> BaseException:
        if primary is not None:
            self._safe_cleanup_log(stage, error)
            return primary
        return error

    async def _drain_subscriptions(
        self, primary: Optional[BaseException]
    ) -> Optional[BaseException]:
        handles = tuple(self._subscriptions)
        self._subscriptions.clear()
        for handle in handles:
            primary = await self._bounded_cleanup(
                "subscription drain",
                self._cleanup_call("subscription drain", handle.drain, primary),
                primary,
            )
        return primary

    async def _stop_heartbeat(
        self, primary: Optional[BaseException]
    ) -> Optional[BaseException]:
        cancel_event = self._heartbeat_cancel_event
        heartbeat_task = self._heartbeat_task
        self._heartbeat_task = None
        self._heartbeat_cancel_event = None
        if cancel_event is not None:
            cancel_event.set()
        if heartbeat_task is not None:
            try:
                await heartbeat_task
            except Exception as exc:
                primary = self._record_cleanup_error("heartbeat", exc, primary)
        return primary

    async def _await_handlers(
        self, primary: Optional[BaseException]
    ) -> Optional[BaseException]:
        current = asyncio.current_task()
        while True:
            waiters = tuple(
                task for task in self._dispatch_waiters if task is not current
            )
            tasks = tuple(self._handler_tasks)
            if not waiters and not tasks:
                break
            try:
                await asyncio.gather(*waiters, *tasks, return_exceptions=True)
            except Exception as exc:
                primary = self._record_cleanup_error("job tasks", exc, primary)
        return primary

    @staticmethod
    def _consume_detached(task: asyncio.Future[object]) -> None:
        if task.cancelled():
            return
        try:
            task.exception()
        except BaseException:
            pass

    async def _bounded_cleanup(
        self,
        stage: str,
        operation: Awaitable[Optional[BaseException]],
        primary: Optional[BaseException],
    ) -> Optional[BaseException]:
        task = asyncio.ensure_future(operation)
        try:
            if self._shutdown_timeout is None:
                return await task
            done, _ = await asyncio.wait({task}, timeout=self._shutdown_timeout)
            if task in done:
                return task.result()
        except BaseException as exc:
            return self._record_cleanup_error(stage, exc, primary)
        task.cancel()
        task.add_done_callback(self._consume_detached)
        error = asyncio.TimeoutError(f"{stage} timed out")
        return self._record_cleanup_error(stage, error, primary)

    async def _cleanup_call(
        self,
        stage: str,
        operation: Callable[[], Awaitable[Any]],
        primary: Optional[BaseException],
    ) -> Optional[BaseException]:
        try:
            await operation()
        except Exception as exc:
            return self._record_cleanup_error(stage, exc, primary)
        return primary

    async def _cleanup_resources(
        self, primary: Optional[BaseException] = None
    ) -> None:
        original = primary
        if self._worker_trust is not None:
            primary = await self._bounded_cleanup(
                "worker trust renewal",
                self._cleanup_call(
                    "worker trust renewal",
                    self._stop_worker_trust_renewal,
                    primary,
                ),
                primary,
            )
        primary = await self._drain_subscriptions(primary)
        self._dispatch_closed = True
        primary = await self._bounded_cleanup(
            "heartbeat", self._stop_heartbeat(primary), primary
        )
        primary = await self._bounded_cleanup(
            "job tasks", self._await_handlers(primary), primary
        )
        if self._worker_trust is not None:
            primary = await self._bounded_cleanup(
                "worker trust",
                self._cleanup_call(
                    "worker trust", self._close_worker_trust, primary
                ),
                primary,
            )
        self._worker_trust = None
        if self._nc is not None:
            primary = await self._bounded_cleanup(
                "NATS drain",
                self._cleanup_call("NATS drain", self._nc.drain, primary),
                primary,
            )
        if self._store is not None:
            primary = await self._bounded_cleanup(
                "blob store",
                self._cleanup_call("blob store", self._store.close, primary),
                primary,
            )
        self._nc = None
        self._lifecycle_state = "closed"
        if original is None and primary is not None:
            raise primary

    async def _close_worker_trust(self) -> None:
        async with self._trust_reconnect_lock:
            lifecycle = self._worker_trust
            if lifecycle is not None:
                await lifecycle.close()

    async def _stop_worker_trust_renewal(self) -> None:
        async with self._trust_reconnect_lock:
            lifecycle = self._worker_trust
            if lifecycle is not None:
                await lifecycle.stop_renewal()

    def _default_heartbeat_payload(self) -> bytes:
        return heartbeat_payload(
            worker_id=self._sender_id,
            pool=self._pool,
            active_jobs=len(self._active_jobs),
            max_parallel=self._max_parallel,
            cpu_load=0.0,
            session_token=self._outbound_session_token(),
        )

    async def run(self) -> None:
        """Start the agent and block until interrupted."""
        startup = asyncio.create_task(self.start())
        primary: Optional[BaseException] = None
        try:
            await asyncio.shield(startup)
            while True:
                await asyncio.sleep(1)
        except BaseException as exc:
            primary = exc
            if isinstance(exc, asyncio.CancelledError) and not startup.done():
                startup.cancel()
                await asyncio.gather(startup, return_exceptions=True)
            raise
        finally:
            if not startup.done():
                startup.cancel()
                await asyncio.gather(startup, return_exceptions=True)
            try:
                await self.close()
            except BaseException as exc:
                if primary is None:
                    raise
                self._safe_cleanup_log("agent close", exc)

    def _decode_admitted_packet(
        self, payload: bytes
    ) -> Optional[buspacket_pb2.BusPacket]:
        if not self._trust_admitting:
            return None
        settings = self._trust_settings
        if settings is None or settings.mode == WorkerTrustMode.OFF:
            return decode_packet(payload, self._public_keys, self._logger)
        if settings.mode == WorkerTrustMode.ENFORCE and not self.session_token:
            return None
        config = settings.config
        if config is None:
            return None
        for public_key in config.scheduler_public_keys.values():
            pins = {config.expected_scheduler_id: public_key}
            packet = decode_packet(payload, pins, self._logger)
            if packet is not None:
                return packet
        return None

    async def _on_direct_msg(self, msg: Any) -> None:
        """Route a direct worker message to the correct handler by topic."""
        packet = self._decode_admitted_packet(msg.data)
        if packet is None:
            return
        topic = packet.job_request.topic if packet.job_request else ""
        spec = self._handlers.get(topic)
        if spec is None:
            self._logger.warning("no handler for topic %s on direct subject", topic)
            return
        await self._on_msg(msg, spec, packet)

    async def _on_msg(
        self,
        msg: Any,
        spec: HandlerSpec,
        packet: Optional[buspacket_pb2.BusPacket] = None,
    ) -> None:
        if packet is None:
            packet = self._decode_admitted_packet(msg.data)
        if packet is None:
            return

        req = packet.job_request
        if not req.job_id:
            return

        self._active_jobs.add(req.job_id)
        try:
            safe_metrics_call(
                self._logger,
                lambda: self._metrics.on_job_received(req.job_id, req.topic),
            )

            ctx_logger = logging.LoggerAdapter(
                self._logger,
                {
                    "job_id": _bounded_log_field(req.job_id),
                    "trace_id": _bounded_log_field(packet.trace_id),
                    "topic": _bounded_log_field(req.topic),
                    "sender_id": _bounded_log_field(packet.sender_id),
                },
            )
            ctx = Context(job=req, packet=packet, logger=ctx_logger)

            store = self._store
            if store is None:
                ctx_logger.error("blob store not initialized")
                return

            try:
                key = key_from_pointer(req.context_ptr)
                payload = await self._with_timeout(store.get(key), "context fetch")
                if payload is None:
                    raise ValueError("context not found")
                if self._max_context_bytes is not None and len(payload) > self._max_context_bytes:
                    raise ValueError("context exceeds max size")
            except Exception as exc:
                await self._publish_failure(ctx, req, str(exc), execution_ms=0)
                return

            try:
                raw = json.loads(payload.decode("utf-8"))
            except Exception as exc:
                await self._publish_failure(ctx, req, f"context decode failed: {exc}", execution_ms=0)
                return

            try:
                input_data = self._validate_input(spec, raw)
            except Exception as exc:
                await self._publish_failure(ctx, req, f"input validation failed: {exc}", execution_ms=0)
                return

            # Build middleware chain: outermost first, terminal calls handler.
            async def _terminal(c: Context, d: Any) -> Any:
                result = spec.func(c, d)
                if asyncio.iscoroutine(result):
                    result = await result
                return result

            chain = _terminal
            for mw in reversed(self._middlewares):
                _next = chain
                chain = (lambda m, n: (lambda c, d: m(c, d, n)))(mw, _next)

            start_time = time.monotonic()
            error: Optional[str] = None
            output: Any = None
            for attempt in range(spec.retries + 1):
                try:
                    output = await chain(ctx, input_data)
                    output = self._validate_output(spec, output)
                    error = None
                    break
                except Exception as exc:
                    error = _HANDLER_FAILED_MESSAGE
                    _log_handler_failure(
                        self._logger,
                        packet,
                        req,
                        exc,
                        attempt + 1,
                        spec.retries + 1,
                    )
                    if attempt >= spec.retries:
                        break

            elapsed_ms = int((time.monotonic() - start_time) * 1000)
            if error is not None:
                await self._publish_failure(ctx, req, error, execution_ms=elapsed_ms)
                return

            try:
                result_payload = self._serialize_output(output)
                if self._max_result_bytes is not None and len(result_payload) > self._max_result_bytes:
                    raise ValueError("result exceeds max size")
                result_key = f"res:{req.job_id}"
                await self._with_timeout(store.set(result_key, result_payload), "result write")
                result_ptr = pointer_for_key(result_key)
            except Exception as exc:
                await self._publish_failure(ctx, req, f"result write failed: {exc}", execution_ms=elapsed_ms)
                return

            result = job_pb2.JobResult(
                job_id=req.job_id,
                status=job_pb2.JOB_STATUS_SUCCEEDED,
                result_ptr=result_ptr,
                worker_id=self._sender_id,
                execution_ms=elapsed_ms,
            )
            await self._publish_result(ctx, result)
            safe_metrics_call(
                self._logger,
                lambda: self._metrics.on_job_completed(
                    req.job_id, elapsed_ms, "SUCCEEDED"
                ),
            )
        finally:
            self._active_jobs.discard(req.job_id)

    def _validate_input(self, spec: HandlerSpec, data: Any) -> Any:
        if spec.input_model is None:
            return data
        if BaseModel is not None and isinstance(spec.input_model, type) and issubclass(spec.input_model, BaseModel):
            return spec.input_model.model_validate(data)
        return spec.input_model(**data)

    def _validate_output(self, spec: HandlerSpec, data: Any) -> Any:
        if spec.output_model is None:
            return data
        if BaseModel is not None and isinstance(spec.output_model, type) and issubclass(spec.output_model, BaseModel):
            return spec.output_model.model_validate(data)
        return spec.output_model(**data)

    def _serialize_output(self, data: Any) -> bytes:
        if BaseModel is not None and isinstance(data, BaseModel):
            return json.dumps(data.model_dump(mode="json")).encode("utf-8")
        if isinstance(data, (dict, list, str, int, float, bool)) or data is None:
            return json.dumps(data).encode("utf-8")
        if hasattr(data, "__dict__"):
            return json.dumps(data.__dict__).encode("utf-8")
        raise ValueError("output is not JSON serializable")

    async def _publish_failure(
        self,
        ctx: Context,
        req: job_pb2.JobRequest,
        error: str,
        execution_ms: int,
    ) -> None:
        result = job_pb2.JobResult(
            job_id=req.job_id,
            status=job_pb2.JOB_STATUS_FAILED,
            error_message=error,
            worker_id=self._sender_id,
            execution_ms=execution_ms,
        )
        await self._publish_result(ctx, result)
        safe_metrics_call(
            self._logger,
            lambda: self._metrics.on_job_failed(req.job_id, error),
        )

    async def _publish_result(self, ctx: Context, result: job_pb2.JobResult) -> None:
        if self._nc is None:
            ctx.logger.error("NATS not initialized")
            return
        packet = buspacket_pb2.BusPacket()
        packet.trace_id = ctx.packet.trace_id
        packet.sender_id = self._sender_id
        packet.protocol_version = DEFAULT_PROTOCOL_VERSION
        ts = timestamp_pb2.Timestamp()
        ts.GetCurrentTime()
        packet.created_at.CopyFrom(ts)
        packet.job_result.CopyFrom(result)
        outgoing = finalize_packet(
            packet,
            self._private_key,
            session_token=self._outbound_session_token(),
        )

        await self._with_timeout(
            self._nc.publish(
                SUBJECT_RESULT, outgoing.SerializeToString(deterministic=True)
            ),
            "result publish",
        )

    async def _with_timeout(self, coro: Awaitable[TAny], label: str) -> TAny:
        if self._io_timeout is None:
            return await coro
        try:
            return await asyncio.wait_for(coro, timeout=self._io_timeout)
        except asyncio.TimeoutError as exc:
            raise TimeoutError(f"{label} timed out") from exc
