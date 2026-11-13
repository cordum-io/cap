"""Internal attempt-local cancellation, after normal packet admission.

The entire admitted dispatch must match, including an empty dispatch. Attempts
sharing a job id remain independent. The unverified peek is routing, not trust.
Synchronous handlers cannot be pre-empted on the event loop; their result wins.
Threaded work finishes independently after its terminal result. A handler that
suppresses cancellation keeps its slot until it returns; its outcome is discarded.
Closing drains cancel subscriptions. cap.worker.run_worker is unchanged.
"""
import asyncio
import math
import threading
from copy import deepcopy
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Protocol

from cap.errors import MalformedPacketError
from cap.outcome import _Outcome, _enum_number, _error_code
from cap.packet_boundary import parse_packet

_JOB_TIMEOUT = _Outcome(_enum_number("JobStatus", "JOB_STATUS_FAILED"),
                        _error_code("JOB_TIMEOUT"), "TIMEOUT", "job deadline exceeded")
_JOB_CANCELLED = _Outcome(_enum_number("JobStatus", "JOB_STATUS_CANCELLED"),
                          _error_code("UNSPECIFIED"), "", "job cancelled")


class _Dispatch(Protocol):
    def SerializeToString(self, *, deterministic: bool = False) -> bytes: ...


class _Cancel(Protocol):
    # Generated protobuf messages have no stubs. Describe only the read surface.
    @property
    def job_id(self) -> str: ...

    @property
    def dispatch(self) -> _Dispatch: ...


@dataclass(eq=False)
class RunningAttempt:
    job_id: str
    dispatch: _Dispatch
    stopped: threading.Event = field(default_factory=threading.Event)
    requested: asyncio.Event = field(default_factory=asyncio.Event)


class CancelRegistry:
    def __init__(self) -> None:
        self._running: Dict[str, List[RunningAttempt]] = {}

    def register(self, job_id: str, dispatch: _Dispatch) -> RunningAttempt:
        snapshot = deepcopy(dispatch)
        attempt = RunningAttempt(job_id, snapshot)  # Called inside the running loop.
        self._running.setdefault(job_id, []).append(attempt)
        return attempt

    def unregister(self, attempt: RunningAttempt) -> None:
        attempts = self._running[attempt.job_id]
        attempts.remove(attempt)
        if not attempts:
            del self._running[attempt.job_id]

    def is_running(self, job_id: str) -> bool:
        return job_id in self._running

    def apply(self, cancel: _Cancel) -> int:
        matched = 0
        for attempt in self._running.get(cancel.job_id, ()):
            if attempt.dispatch == cancel.dispatch:
                attempt.requested.set()
                matched += 1
        return matched


def peek_cancel_job_id(payload: bytes) -> str:
    try:
        packet = parse_packet(payload)
    except MalformedPacketError:
        return ""
    return str(packet.job_cancel.job_id) if packet.HasField("job_cancel") else ""


def validate_max_deadline(value: Optional[float]) -> Optional[float]:
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, (int, float)) or value <= 0:
        raise ValueError("max_deadline must be positive and finite, or None")
    try:
        value = float(value)
    except OverflowError as exc:
        raise ValueError("max_deadline must be positive and finite, or None") from exc
    if not math.isfinite(value):
        raise ValueError("max_deadline must be positive and finite, or None")
    return value


def handler_deadline(start: float, deadline_ms: int, maximum: Optional[float]) -> Optional[float]:
    seconds = deadline_ms / 1000 if deadline_ms > 0 else maximum
    if seconds is None:
        return None
    return start + (min(seconds, maximum) if maximum is not None else seconds)
