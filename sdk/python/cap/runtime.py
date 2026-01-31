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


DEFAULT_PROTOCOL_VERSION = 1
SUBJECT_RESULT = "sys.job.result"


class BlobStore(Protocol):
    async def get(self, key: str) -> Optional[bytes]:
        ...

    async def set(self, key: str, data: bytes) -> None:
        ...

    async def close(self) -> None:
        ...


class RedisBlobStore:
    def __init__(self, redis_url: str) -> None:
        if redis_async is None:
            raise RuntimeError("redis is required for RedisBlobStore")
        self._client = redis_async.from_url(redis_url)

    async def get(self, key: str) -> Optional[bytes]:
        value = await self._client.get(key)
        return value

    async def set(self, key: str, data: bytes) -> None:
        await self._client.set(key, data)

    async def close(self) -> None:
        await self._client.close()


class InMemoryBlobStore:
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
        raise ValueError("empty pointer")
    if not ptr.startswith("redis://"):
        raise ValueError("unsupported pointer scheme")
    key = ptr[len("redis://") :]
    if not key:
        raise ValueError("missing pointer key")
    return key


def _default_logger() -> logging.Logger:
    logger = logging.getLogger("cap.runtime")
    if not logger.handlers:
        handler = logging.StreamHandler()
        formatter = logging.Formatter("%(levelname)s %(message)s")
        handler.setFormatter(formatter)
        logger.addHandler(handler)
        logger.setLevel(logging.INFO)
    return logger


@dataclass
class Context:
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
    def __init__(
        self,
        *,
        nats_url: Optional[str] = None,
        redis_url: Optional[str] = None,
        store: Optional[BlobStore] = None,
        public_keys: Optional[Dict[str, ec.EllipticCurvePublicKey]] = None,
        private_key: Optional[ec.EllipticCurvePrivateKey] = None,
        sender_id: str = "cap-runtime",
        retries: int = 0,
        io_timeout: Optional[float] = 5.0,
        max_context_bytes: Optional[int] = 2 * 1024 * 1024,
        max_result_bytes: Optional[int] = 2 * 1024 * 1024,
        connect_fn: Optional[Callable[..., Awaitable[Any]]] = None,
        logger: Optional[logging.Logger] = None,
    ) -> None:
        self._nats_url = nats_url or os.getenv("NATS_URL", "nats://127.0.0.1:4222")
        self._redis_url = redis_url or os.getenv("REDIS_URL", "redis://127.0.0.1:6379/0")
        self._store = store
        self._public_keys = public_keys
        self._private_key = private_key
        self._sender_id = sender_id
        self._default_retries = max(0, retries)
        self._io_timeout = io_timeout if io_timeout and io_timeout > 0 else None
        self._max_context_bytes = max_context_bytes if max_context_bytes and max_context_bytes > 0 else None
        self._max_result_bytes = max_result_bytes if max_result_bytes and max_result_bytes > 0 else None
        self._connect_fn = connect_fn
        self._logger = logger or _default_logger()
        self._handlers: Dict[str, HandlerSpec] = {}
        self._nc = None

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
            await self._nc.subscribe(topic, queue=topic, cb=lambda msg, s=spec: asyncio.create_task(self._on_msg(msg, s)))

    async def close(self) -> None:
        if self._nc is not None:
            await self._nc.drain()
        if self._store is not None:
            await self._store.close()

    async def run(self) -> None:
        await self.start()
        try:
            while True:
                await asyncio.sleep(1)
        finally:
            await self.close()

    async def _on_msg(self, msg: Any, spec: HandlerSpec) -> None:
        packet = buspacket_pb2.BusPacket()
        try:
            packet.ParseFromString(msg.data)
        except Exception as exc:
            self._logger.error("runtime: decode failed: %s", exc)
            return

        if self._public_keys is not None:
            sender_key = self._public_keys.get(packet.sender_id)
            if not sender_key:
                self._logger.warning("runtime: no public key for sender %s", packet.sender_id)
                return
            if not packet.signature:
                self._logger.warning("runtime: missing signature for sender %s", packet.sender_id)
                return
            signature = packet.signature
            packet.ClearField("signature")
            unsigned = packet.SerializeToString(deterministic=True)
            packet.signature = signature
            try:
                sender_key.verify(signature, unsigned, ec.ECDSA(hashes.SHA256()))
            except Exception:
                self._logger.warning("runtime: invalid signature from sender %s", packet.sender_id)
                return

        req = packet.job_request
        if not req.job_id:
            return

        ctx_logger = logging.LoggerAdapter(
            self._logger,
            {
                "job_id": req.job_id,
                "trace_id": packet.trace_id,
                "topic": req.topic,
            },
        )
        ctx = Context(job=req, packet=packet, logger=ctx_logger)

        store = self._store
        if store is None:
            ctx_logger.error("runtime: blob store not initialized")
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

        start_time = time.monotonic()
        error: Optional[str] = None
        output: Any = None
        for attempt in range(spec.retries + 1):
            try:
                output = spec.func(ctx, input_data)
                if asyncio.iscoroutine(output):
                    output = await output
                output = self._validate_output(spec, output)
                error = None
                break
            except Exception as exc:
                error = str(exc)
                ctx_logger.warning("runtime: handler failed (attempt %d/%d): %s", attempt + 1, spec.retries + 1, exc)
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

    async def _publish_result(self, ctx: Context, result: job_pb2.JobResult) -> None:
        if self._nc is None:
            ctx.logger.error("runtime: NATS not initialized")
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
