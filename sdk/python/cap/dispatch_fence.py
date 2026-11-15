"""Internal, synchronous dispatch fence for admitted requests with a dispatch.

The trust mode determines who can steer this fence; it authenticates nothing.
Validate the worker and bounded ids before comparing dispatch and request digest.
An unknown job runs. Exact RUNNING drops; exact DONE replays a recorded result
or runs again without one. A conflicting digest or stale attempt drops. While
RUNNING only active+1 may replace it; once DONE any newer attempt may replace it.
Record bound result bytes and trace id before publishing, only for the matching
RUNNING entry. finish closes only its own attempt. Replays never record or run.
The runtime logs fixed reasons and bounded job ids, never request content.

This store is in memory, per process, and used only on the event-loop thread.
Age since the last write and oldest-write-first size eviction make jobs unknown;
restart or eviction loses deduplication. This is not a durable exactly-once store.
"""
from __future__ import annotations

import math
import time
from collections import OrderedDict
from dataclasses import dataclass
from enum import Enum
from typing import Callable, Optional, Tuple

from cap.resource_registry import MAX_TRUSTED_JOB_ID_BYTES

MAX_DISPATCH_ID_BYTES = 128
DEFAULT_MAX_JOBS, DEFAULT_MAX_AGE = 1024, 3600.0
Dispatch = Tuple[str, int, str]
RecordedResult = Tuple[str, bytes]


class FenceDecision(Enum):
    RUN = "run"
    REPLAY = "replay"
    DROP = "drop"


@dataclass
class _Entry:
    dispatch: Dispatch
    digest: bytes
    written_at: float
    running: bool = True
    result: Optional[RecordedResult] = None


class DispatchFence:
    def __init__(self, max_jobs: int = DEFAULT_MAX_JOBS, max_age: float = DEFAULT_MAX_AGE,
                 clock: Callable[[], float] = time.monotonic) -> None:
        if type(max_jobs) is not int or not 1 <= max_jobs <= 1_000_000:
            raise ValueError("dispatch_fence_max_jobs must be an int from 1 to 1000000")
        message = "dispatch_fence_max_age must be positive and finite"
        if isinstance(max_age, bool) or not isinstance(max_age, (int, float)) or max_age <= 0:
            raise ValueError(message)
        try:
            age = float(max_age)
        except OverflowError as exc:
            raise ValueError(message) from exc
        if not math.isfinite(age):
            raise ValueError(message)
        self.max_jobs, self.max_age, self._clock = max_jobs, age, clock
        self._entries: OrderedDict[str, _Entry] = OrderedDict()

    def admit(self, job_id: str, dispatch: Dispatch, request_digest: bytes, worker_id: str,
              *, replay_only: bool = False) -> Tuple[FenceDecision, str, Optional[RecordedResult]]:
        self._evict()
        reason = ""
        dispatch_id, attempt, assigned_worker_id = dispatch
        if (not 0 < len(dispatch_id.encode("utf-8")) <= MAX_DISPATCH_ID_BYTES
                or attempt < 1 or len(job_id.encode("utf-8")) > MAX_TRUSTED_JOB_ID_BYTES):
            reason = "malformed dispatch"
        elif assigned_worker_id != worker_id:
            reason = "wrong worker"
        entry = self._entries.get(job_id)
        if not reason and entry is not None:
            if dispatch == entry.dispatch:
                if request_digest != entry.digest:
                    reason = "conflicting dispatch"
                elif entry.running:
                    reason = "in flight"
                elif entry.result is not None:
                    return FenceDecision.REPLAY, "recorded result", entry.result
            elif attempt <= entry.dispatch[1]:
                reason = "stale attempt"
            elif entry.running and attempt != entry.dispatch[1] + 1:
                reason = "future attempt"
        if reason or replay_only:
            return FenceDecision.DROP, reason or "unrecorded duplicate", None
        self._write(job_id, _Entry(dispatch, request_digest, self._clock()))
        return FenceDecision.RUN, "new dispatch", None

    def record(self, job_id: str, dispatch: Dispatch, trace_id: str, result_bytes: bytes) -> bool:
        self._evict()
        entry = self._entries.get(job_id)
        if entry is None or not entry.running or entry.dispatch != dispatch:
            return False
        entry.running, entry.result = False, (trace_id, result_bytes)
        self._write(job_id, entry)
        return True

    def finish(self, job_id: str, dispatch: Dispatch) -> None:
        self._evict()
        entry = self._entries.get(job_id)
        if entry is not None and entry.running and entry.dispatch == dispatch:
            entry.running = False
            self._write(job_id, entry)

    def _write(self, job_id: str, entry: _Entry) -> None:
        entry.written_at = self._clock()
        self._entries[job_id] = entry
        self._entries.move_to_end(job_id)
        self._evict()

    def _evict(self) -> None:
        now = self._clock()
        while self._entries:
            first = next(iter(self._entries.values()))
            if now - first.written_at <= self.max_age and len(self._entries) <= self.max_jobs:
                break
            self._entries.popitem(last=False)
