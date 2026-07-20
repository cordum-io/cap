"""Async request/reply and renewal lifecycle for authenticated workers."""
import asyncio
import logging
import secrets
from datetime import datetime, timezone
from typing import Any, Awaitable, Callable, Optional, Protocol

from cap.pb.cordum.agent.v1 import handshake_pb2
from cap.subjects import SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, SUBJECT_WORKER_HANDSHAKE_CHALLENGE
from cap.worker_trust import (
    WORKER_HANDSHAKE_NONCE_SIZE,
    WorkerHandshakeRequestOptions,
    WorkerHandshakeSession,
    WorkerTrustConfig,
    WorkerTrustMode,
    build_authenticate,
    build_challenge_request,
    verify_challenge,
    verify_result,
)
from cap.worker_trust_codec import (
    marshal_worker_trust_packet,
    unmarshal_worker_trust_packet,
)
from cap.worker_trust_async import (
    WorkerTrustOperationalError,
    await_with_timeout,
    is_operational_failure,
    is_transport_failure,
)
from cap.worker_trust_runtime_config import (
    DEFAULT_RENEW_MIN_INTERVAL,
    DEFAULT_TRUST_RETRIES,
    DEFAULT_TRUST_TIMEOUT,
    ENV_WORKER_TRUST_MODE,
    RuntimeTrustSettings,
)


class _Requester(Protocol):
    async def request(self, subject: str, data: bytes, timeout: float) -> Any:
        ...


class WorkerTrustRuntimeError(RuntimeError):
    """Raised when enforce mode cannot establish or renew trust."""


class WorkerTrustLifecycle:
    """Owns one verified session and its cancel-safe renewal task."""

    def __init__(
        self,
        connection: _Requester,
        settings: RuntimeTrustSettings,
        capability: handshake_pb2.Handshake,
        *,
        logger: Optional[logging.Logger] = None,
        clock: Callable[[], datetime] = lambda: datetime.now(timezone.utc),
    ) -> None:
        self._connection = connection
        self._settings = settings
        self._capability = _copy_capability(capability)
        self._logger = logger or logging.getLogger("cap.worker_trust")
        self._renew_min_interval = settings.renew_min_interval
        self._clock = clock
        self._session: Optional[WorkerHandshakeSession] = None
        self._renew_task: Optional[asyncio.Task[None]] = None
        self._closed = False
        self._exchange_lock: Optional[asyncio.Lock] = None

    def _lock(self) -> asyncio.Lock:
        """Lazily create the exchange lock inside the running loop (py3.9)."""
        if self._exchange_lock is None:
            self._exchange_lock = asyncio.Lock()
        return self._exchange_lock

    def session_token(self) -> str:
        session = self._active_session()
        return "" if session is None else session.token

    @property
    def renewal_running(self) -> bool:
        return self._renew_task is not None and not self._renew_task.done()

    async def authenticate(self) -> bool:
        if self._settings.mode == WorkerTrustMode.OFF:
            return False
        return await self._exchange(
            handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE, ""
        )

    async def renew(self) -> bool:
        token = self.session_token()
        if not token:
            return self._handle_failure(
                WorkerTrustRuntimeError("renew requires a current live session"), True
            )
        return await self._exchange(
            handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW, token
        )

    async def reauthenticate(self) -> bool:
        if self.session_token():
            return await self.renew()
        return await self.authenticate()

    async def _exchange(self, purpose: int, current_token: str) -> bool:
        if self._closed: raise WorkerTrustRuntimeError("worker trust lifecycle is closed")
        async with self._lock():
            if self._closed: raise WorkerTrustRuntimeError("worker trust lifecycle is closed")
            if purpose == handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW:
                active_token = self.session_token()
                if not active_token:
                    return self._handle_failure(
                        WorkerTrustRuntimeError("renew requires a live session"), True
                    )
                if active_token != current_token:
                    return True
            error: Optional[Exception] = None
            for attempt in range(self._settings.retries):
                try:
                    verified = await self._exchange_once(purpose, current_token)
                    if self._closed: raise WorkerTrustRuntimeError("worker trust lifecycle is closed")
                    self._session = verified
                    return True
                except asyncio.CancelledError:
                    raise
                except Exception as exc:
                    error = exc
                    if not is_operational_failure(exc):
                        break
                    if attempt + 1 < self._settings.retries:
                        await asyncio.sleep(_retry_delay(attempt))
            renewal = purpose == handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW
            return self._handle_failure(
                error or WorkerTrustRuntimeError("failed"), renewal
            )

    async def _exchange_once(
        self, purpose: int, current_token: str
    ) -> WorkerHandshakeSession:
        config = self._required_config()
        now = self._clock()
        options = WorkerHandshakeRequestOptions(
            request_id=secrets.token_hex(16),
            trace_id=secrets.token_hex(16),
            purpose=purpose,
            client_nonce=secrets.token_bytes(WORKER_HANDSHAKE_NONCE_SIZE),
            created_at=now,
        )
        request = build_challenge_request(config, options)
        challenge = await self._request(SUBJECT_WORKER_HANDSHAKE_CHALLENGE, request)
        verified = verify_challenge(config, request, challenge, self._clock())
        authenticate = build_authenticate(
            config, verified, self._capability, current_token, self._clock()
        )
        result = await self._request(SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, authenticate)
        return verify_result(config, verified, authenticate, result, self._clock())

    async def _request(self, subject: str, packet):
        data = marshal_worker_trust_packet(packet)
        try:
            response = await await_with_timeout(
                self._connection.request(
                    subject, data, timeout=self._settings.timeout
                ),
                self._settings.timeout,
            )
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            if is_transport_failure(exc):
                raise WorkerTrustOperationalError(exc) from exc
            raise
        if response is None or not isinstance(getattr(response, "data", None), bytes):
            raise WorkerTrustOperationalError(
                WorkerTrustRuntimeError("worker trust request returned no data")
            )
        return unmarshal_worker_trust_packet(response.data)

    def _handle_failure(self, error: Exception, renewal: bool) -> bool:
        operational = is_operational_failure(error)
        if not operational or (
            renewal and self._settings.mode == WorkerTrustMode.ENFORCE
        ):
            self._session = None
        else:
            self._active_session()
        if self._settings.mode == WorkerTrustMode.ENFORCE or not operational:
            raise WorkerTrustRuntimeError("authenticated worker trust failed") from error
        self._logger.warning(
            "authenticated worker trust failed; continuing without new session",
            extra={"exception_type": type(error).__name__},
        )
        return False

    def start_renewal(
        self, on_enforce_failure: Callable[[Exception], Awaitable[None]]
    ) -> None:
        if self._closed or self._settings.mode == WorkerTrustMode.OFF:
            return
        if not self.session_token() or self.renewal_running:
            return
        self._renew_task = asyncio.create_task(self._renew_loop(on_enforce_failure))

    async def _renew_loop(
        self, on_enforce_failure: Callable[[Exception], Awaitable[None]]
    ) -> None:
        try:
            while not self._closed:
                session = self._active_session()
                if session is None:
                    return
                await asyncio.sleep(self._renew_wait(session))
                try:
                    if await self.renew():
                        continue
                except asyncio.CancelledError:
                    raise
                except Exception as exc:
                    await on_enforce_failure(exc)
                    return
                if self._active_session() is None:
                    return
                await asyncio.sleep(self._renew_min_interval)
        finally:
            if asyncio.current_task() is self._renew_task:
                self._renew_task = None

    async def close(self) -> None:
        self._closed = True
        await self.stop_renewal()
        self._session = None

    async def stop_renewal(self) -> None:
        """Cancel renewal while retaining a live session for in-flight work."""
        task, self._renew_task = self._renew_task, None
        if task is not None and task is not asyncio.current_task():
            task.cancel()
            await asyncio.gather(task, return_exceptions=True)

    def _active_session(self) -> Optional[WorkerHandshakeSession]:
        if self._session is not None and self._session.expires_at <= self._clock():
            self._session = None
        return self._session

    def _renew_wait(self, session: WorkerHandshakeSession) -> float:
        lifetime = max(0.0, (session.expires_at - self._clock()).total_seconds())
        target = min(lifetime / 2.0, max(0.0, lifetime - 60.0))
        target = max(self._renew_min_interval, target)
        return min(target, max(0.001, lifetime * 0.9))

    def _required_config(self) -> WorkerTrustConfig:
        if self._settings.config is None:
            raise WorkerTrustRuntimeError("worker trust configuration is unavailable")
        return self._settings.config


def _copy_capability(value: handshake_pb2.Handshake) -> handshake_pb2.Handshake:
    copied = handshake_pb2.Handshake()
    copied.CopyFrom(value)
    return copied


def _retry_delay(attempt: int) -> float:
    return min(5.0, 0.1 * (3 ** attempt))


__all__ = ["ENV_WORKER_TRUST_MODE", "RuntimeTrustSettings",
           "WorkerTrustLifecycle", "WorkerTrustOperationalError",
           "WorkerTrustRuntimeError"]
