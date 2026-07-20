"""Fail-fast configuration for the asynchronous worker-trust lifecycle."""

import math
import os
from dataclasses import dataclass
from typing import Optional, Type, Union

from cap.worker_trust import (
    WORKER_HANDSHAKE_MAX_LIFETIME,
    WorkerTrustConfig,
    WorkerTrustConfigError,
    WorkerTrustMode,
    parse_worker_trust_mode,
    validate_worker_trust_config,
)


ENV_WORKER_TRUST_MODE = "CORDUM_SDK_HANDSHAKE"
DEFAULT_TRUST_TIMEOUT = 10.0
DEFAULT_TRUST_RETRIES = 3
DEFAULT_RENEW_MIN_INTERVAL = 30.0
MAX_TRUST_RETRIES = 10


@dataclass(frozen=True)
class RuntimeTrustSettings:
    mode: WorkerTrustMode
    config: Optional[WorkerTrustConfig]
    timeout: float
    retries: int
    renew_min_interval: float

    @classmethod
    def resolve(
        cls,
        raw_mode: Optional[Union[str, WorkerTrustMode]],
        config: Optional[WorkerTrustConfig],
        sender_id: str,
        *,
        timeout: Optional[float] = None,
        retries: Optional[int] = None,
        renew_min_interval: float = DEFAULT_RENEW_MIN_INTERVAL,
    ) -> "RuntimeTrustSettings":
        renew_interval = _renew_interval(renew_min_interval)
        raw = raw_mode.value if isinstance(raw_mode, WorkerTrustMode) else raw_mode
        if raw is None:
            raw = os.getenv(ENV_WORKER_TRUST_MODE, "")
        if _is_legacy_default(raw, config, timeout, retries):
            return cls(
                WorkerTrustMode.OFF,
                None,
                DEFAULT_TRUST_TIMEOUT,
                DEFAULT_TRUST_RETRIES,
                renew_interval,
            )
        mode = parse_worker_trust_mode(raw)
        if mode == WorkerTrustMode.OFF:
            _validate_off(config, timeout, retries)
            return cls(
                mode, None, DEFAULT_TRUST_TIMEOUT, DEFAULT_TRUST_RETRIES,
                renew_interval,
            )
        return _resolve_enabled(
            cls, mode, config, sender_id, timeout, retries, renew_interval
        )


def _is_legacy_default(
    raw: Optional[Union[str, WorkerTrustMode]],
    config: Optional[WorkerTrustConfig],
    timeout: Optional[float],
    retries: Optional[int],
) -> bool:
    return (
        isinstance(raw, str)
        and not raw.strip()
        and config is None
        and timeout is None
        and retries is None
    )


def _validate_off(
    config: Optional[WorkerTrustConfig],
    timeout: Optional[float],
    retries: Optional[int],
) -> None:
    if config is not None or timeout is not None or retries is not None:
        raise WorkerTrustConfigError(
            "worker trust mode off conflicts with trust configuration"
        )


def _resolve_enabled(
    cls: Type[RuntimeTrustSettings],
    mode: WorkerTrustMode,
    config: Optional[WorkerTrustConfig],
    sender_id: str,
    timeout: Optional[float],
    retries: Optional[int],
    renew_interval: float,
) -> RuntimeTrustSettings:
    if config is None:
        raise WorkerTrustConfigError("worker trust configuration is required")
    validate_worker_trust_config(config)
    if config.worker_id != sender_id:
        raise WorkerTrustConfigError("sender_id does not match worker trust identity")
    resolved_timeout = DEFAULT_TRUST_TIMEOUT if timeout is None else timeout
    resolved_retries = DEFAULT_TRUST_RETRIES if retries is None else retries
    max_timeout = WORKER_HANDSHAKE_MAX_LIFETIME.total_seconds()
    if not _bounded_number(resolved_timeout, max_timeout):
        raise WorkerTrustConfigError("worker trust timeout is outside allowed bounds")
    if (
        isinstance(resolved_retries, bool)
        or not isinstance(resolved_retries, int)
        or not 1 <= resolved_retries <= MAX_TRUST_RETRIES
    ):
        raise WorkerTrustConfigError("worker trust retries are outside allowed bounds")
    return cls(
        mode, config, float(resolved_timeout), resolved_retries, renew_interval
    )


def _renew_interval(value: float) -> float:
    maximum = WORKER_HANDSHAKE_MAX_LIFETIME.total_seconds()
    if not _bounded_number(value, maximum):
        raise WorkerTrustConfigError(
            "worker trust renew interval is outside allowed bounds"
        )
    return float(value)


def _bounded_number(value: object, maximum: float) -> bool:
    return (
        not isinstance(value, bool)
        and isinstance(value, (int, float))
        and math.isfinite(value)
        and 0 < value <= maximum
    )


__all__ = [
    "DEFAULT_RENEW_MIN_INTERVAL",
    "DEFAULT_TRUST_RETRIES",
    "DEFAULT_TRUST_TIMEOUT",
    "ENV_WORKER_TRUST_MODE",
    "RuntimeTrustSettings",
]
