import asyncio
import json
import logging
import os
import time
from dataclasses import dataclass
from typing import Any, Awaitable, Callable, Dict, Optional, Protocol, Type, TypeVar, Union

from google.protobuf import timestamp_pb2
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes

try:
    import redis.asyncio as redis_async  # type: ignore
except Exception:  # pragma: no cover - optional until runtime used
    redis_async = None

try:
    from pydantic import BaseModel, ValidationError
except Exception:  # pragma: no cover - optional until runtime used
    BaseModel = None
    ValidationError = Exception


from cap.subjects import SUBJECT_RESULT
from cap.errors import InvalidInputError, MalformedPacketError, SignatureInvalidError, SignatureMissingError
from cap.metrics import MetricsHook, NoopMetrics
from cap.heartbeat import heartbeat_loop, heartbeat_payload

DEFAULT_PROTOCOL_VERSION = 1


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


async def ping_redis(redis_url: str) -> None:
    """Connect to Redis and run PING to verify auth and TLS at startup.

    Raises on failure so connection issues surface immediately instead of
    silently failing on the first blob store read/write.
    """
    if redis_async is None:
        raise RuntimeError("redis is required")
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
            sname = os.environ.get("REDIS_TLS_SERVER_NAME", "").strip()
            if sname:
                kwargs["ssl_check_hostname"] = True
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
        nats_url: Optional[str] = None,
        redis_url: Optional[str] = None,
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
        max_context_bytes: Optional[int] = 2 * 1024 * 1024,
        max_result_bytes: Optional[int] = 2 * 1024 * 1024,
        connect_fn: Optional[Callable[..., Awaitable[Any]]] = None,
        logger: Optional[logging.Logger] = None,
        metrics: Optional[MetricsHook] = None,
    ) -> None:
        self._nats_url = nats_url or os.getenv("NATS_URL", "nats://127.0.0.1:4222")
        self._redis_url = redis_url or os.getenv("REDIS_URL", "redis://127.0.0.1:6379/0")
        self._store = store
        self._public_keys = public_keys
        self._private_key = private_key
        self._sender_id = worker_id or sender_id
        self._pool = pool
        self._max_parallel = max(1, max_parallel)
        self._heartbeat_interval = heartbeat_interval if heartbeat_interval > 0 else 5.0
        self._heartbeat_payload_fn = heartbeat_payload_fn
        self._default_retries = max(0, retries)
        self._io_timeout = io_timeout if io_timeout and io_timeout > 0 else None
        self._max_context_bytes = max_context_bytes if max_context_bytes and max_context_bytes > 0 else None
        self._max_result_bytes = max_result_bytes if max_result_bytes and max_result_bytes > 0 else None
        self._connect_fn = connect_fn
        self._logger = logger or _default_logger()
        self._metrics: MetricsHook = metrics or NoopMetrics()
        self._handlers: Dict[str, HandlerSpec] = {}
        self._middlewares: list = []
        self._nc = None
        self._active_jobs: set[str] = set()
        self._heartbeat_cancel_event: Optional[asyncio.Event] = None
        self._heartbeat_task: Optional[asyncio.Task[Any]] = None

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
    ) -> Callable[
        [Callable[[Context, Any], Union[Awaitable[Any], Any]]],
        Callable[[Context, Any], Union[Awaitable[Any], Any]],
    ]:
        """Decorator that registers a handler for *topic*.

        Args:
            topic: NATS subject the handler subscribes to.
            input_model: Optional Pydantic model or callable for input validation.
            output_model: Optional Pydantic model or callable for output validation.
            retries: Override the default retry count for this handler.
        """
        def decorator(func: Callable[[Context, Any], Union[Awaitable[Any], Any]]):
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
        if self._connect_fn is None:
            try:
                import nats  # type: ignore
            except ImportError as exc:
                raise RuntimeError("nats-py is required to connect to NATS") from exc
            self._connect_fn = nats.connect

        self._nc = await self._with_timeout(
            self._connect_fn(servers=self._nats_url, name=self._sender_id),
            "nats connect",
        )
        if self._store is None:
            self._store = RedisBlobStore(self._redis_url)

        for topic, spec in self._handlers.items():
            async def _on_topic_msg(msg, s=spec):
                asyncio.create_task(self._on_msg(msg, s))
            await self._nc.subscribe(topic, queue=topic, cb=_on_topic_msg)

        # Subscribe to direct worker subject (worker.<id>.jobs) for scheduler dispatch
        if self._sender_id:
            direct_subject = f"worker.{self._sender_id}.jobs"
            async def _direct_cb(msg):
                asyncio.create_task(self._on_direct_msg(msg))
            await self._nc.subscribe(direct_subject, cb=_direct_cb)

        if self._heartbeat_task is None or self._heartbeat_task.done():
            self._heartbeat_cancel_event = asyncio.Event()
            payload_fn = self._heartbeat_payload_fn or self._default_heartbeat_payload
            self._heartbeat_task = asyncio.create_task(
                heartbeat_loop(
                    nc=self._nc,
                    payload_fn=payload_fn,
                    interval=self._heartbeat_interval,
                    private_key=self._private_key,
                    metrics=self._metrics,
                    cancel_event=self._heartbeat_cancel_event,
                )
            )

    async def close(self) -> None:
        """Drain the NATS connection and close the blob store."""
        if self._heartbeat_cancel_event is not None:
            self._heartbeat_cancel_event.set()
        if self._heartbeat_task is not None:
            try:
                await self._heartbeat_task
            finally:
                self._heartbeat_task = None
                self._heartbeat_cancel_event = None

        if self._nc is not None:
            await self._nc.drain()
        if self._store is not None:
            await self._store.close()

    def _default_heartbeat_payload(self) -> bytes:
        return heartbeat_payload(
            worker_id=self._sender_id,
            pool=self._pool,
            active_jobs=len(self._active_jobs),
            max_parallel=self._max_parallel,
            cpu_load=0.0,
        )

    async def run(self) -> None:
        """Start the agent and block until interrupted."""
        await self.start()
        try:
            while True:
                await asyncio.sleep(1)
        finally:
            await self.close()

    async def _on_direct_msg(self, msg: Any) -> None:
        """Route a direct worker message to the correct handler by topic."""
        packet = buspacket_pb2.BusPacket()
        try:
            packet.ParseFromString(msg.data)
        except Exception:
            return
        topic = packet.job_request.topic if packet.job_request else ""
        spec = self._handlers.get(topic)
        if spec is None:
            self._logger.warning("no handler for topic %s on direct subject", topic)
            return
        await self._on_msg(msg, spec)

    async def _on_msg(self, msg: Any, spec: HandlerSpec) -> None:
        packet = buspacket_pb2.BusPacket()
        try:
            packet.ParseFromString(msg.data)
        except Exception as exc:
            self._logger.error("decode failed: %s", MalformedPacketError(str(exc)))
            return

        if self._public_keys is not None:
            sender_key = self._public_keys.get(packet.sender_id)
            if not sender_key:
                self._logger.warning("no public key for sender", extra={"sender_id": packet.sender_id})
                return
            if not packet.signature:
                self._logger.warning("missing signature: %s", SignatureMissingError(f"sender {packet.sender_id}"), extra={"sender_id": packet.sender_id})
                return
            signature = packet.signature
            packet.ClearField("signature")
            unsigned = packet.SerializeToString(deterministic=True)
            packet.signature = signature
            try:
                sender_key.verify(signature, unsigned, ec.ECDSA(hashes.SHA256()))
            except Exception:
                self._logger.warning("invalid signature: %s", SignatureInvalidError(f"sender {packet.sender_id}"), extra={"sender_id": packet.sender_id})
                return

        req = packet.job_request
        if not req.job_id:
            return

        self._active_jobs.add(req.job_id)
        try:
            self._metrics.on_job_received(req.job_id, req.topic)

            ctx_logger = logging.LoggerAdapter(
                self._logger,
                {
                    "job_id": req.job_id,
                    "trace_id": packet.trace_id,
                    "topic": req.topic,
                    "sender_id": packet.sender_id,
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
                    error = str(exc)
                    ctx_logger.warning("handler failed (attempt %d/%d): %s", attempt + 1, spec.retries + 1, exc)
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

            self._metrics.on_job_completed(req.job_id, elapsed_ms, "SUCCEEDED")
            result = job_pb2.JobResult(
                job_id=req.job_id,
                status=job_pb2.JOB_STATUS_SUCCEEDED,
                result_ptr=result_ptr,
                worker_id=self._sender_id,
                execution_ms=elapsed_ms,
            )
            await self._publish_result(ctx, result)
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
        self._metrics.on_job_failed(req.job_id, error)
        result = job_pb2.JobResult(
            job_id=req.job_id,
            status=job_pb2.JOB_STATUS_FAILED,
            error_message=error,
            worker_id=self._sender_id,
            execution_ms=execution_ms,
        )
        await self._publish_result(ctx, result)

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

        if self._private_key is not None:
            unsigned = packet.SerializeToString(deterministic=True)
            packet.signature = self._private_key.sign(unsigned, ec.ECDSA(hashes.SHA256()))

        await self._with_timeout(
            self._nc.publish(SUBJECT_RESULT, packet.SerializeToString(deterministic=True)),
            "result publish",
        )

    async def _with_timeout(self, coro: Awaitable[TAny], label: str) -> TAny:
        if self._io_timeout is None:
            return await coro
        try:
            return await asyncio.wait_for(coro, timeout=self._io_timeout)
        except asyncio.TimeoutError as exc:
            raise TimeoutError(f"{label} timed out") from exc
