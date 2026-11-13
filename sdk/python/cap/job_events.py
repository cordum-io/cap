"""Internal, event-loop-only hooks for async Agent handlers (not run_worker).

Progress awaits its ordered publish. Artifacts require the operator registry;
slots reserve call order and bounds latch the first fixed failure, even if caught.
Retries clear slots, not the latch. Hooks close at invocation exit (in the same
coroutine as the retry loop), terminal publish entry, and request cleanup.
Closed or unbound Context hooks still validate, but do no I/O. Sync handlers
cannot await these hooks; callers must await them before the final attempt ends.
"""
from __future__ import annotations

import re
from typing import Awaitable, Callable, List, NoReturn, Optional

from cap.outcome import JobFailure, _Outcome, _RESOURCE_EXHAUSTED, _failed
from cap.production_resource import MAX_RESOURCE_MEDIA_TYPE_BYTES, _MEDIA_PATTERN
from cap.resource_registry import ResourceTooLargeError

ARTIFACT_PURPOSE_PREFIX = "job-artifact:"
MAX_MARKER_BYTES = 256
_NAME_PATTERN = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}")
_ARTIFACT_COUNT_EXCEEDED = _failed(_RESOURCE_EXHAUSTED, "artifact count exceeds max")
_ARTIFACT_TOO_LARGE = _failed(_RESOURCE_EXHAUSTED, "artifact exceeds max size")
_ARTIFACT_WRITE_FAILED = _failed(_RESOURCE_EXHAUSTED, "artifact write failed")


def check_marker(marker: str) -> None:
    if type(marker) is not str:
        raise TypeError("marker must be a str")
    if not marker.isprintable() or not 1 <= len(marker.encode("utf-8")) <= MAX_MARKER_BYTES:
        raise ValueError("marker must be printable and 1-256 UTF-8 bytes")


def check_artifact(name: str, media_type: str, content: bytes) -> None:
    if type(name) is not str or type(media_type) is not str or type(content) is not bytes:
        raise TypeError("artifact requires str name, str media type and bytes content")
    if _NAME_PATTERN.fullmatch(name) is None:
        raise ValueError("artifact name must match [a-z0-9][a-z0-9._-]{0,63}")
    if (_MEDIA_PATTERN.fullmatch(media_type) is None
            or len(media_type.encode("utf-8")) > MAX_RESOURCE_MEDIA_TYPE_BYTES):
        raise ValueError("artifact media type must be canonical and at most 127 bytes")
    if not content:
        raise ValueError("artifact content must not be empty")


class _JobEvents:
    def __init__(self, publish: Callable[[str], Awaitable[None]],
                 externalize: Optional[Callable[[str, str, bytes], Awaitable[object]]],
                 max_artifacts: int, max_artifact_bytes: int) -> None:
        self._publish = publish
        self._externalize = externalize
        self._max_artifacts = max_artifacts
        self._max_artifact_bytes = max_artifact_bytes
        self._slots: List[Optional[object]] = []
        self.closed = False
        self.failure: Optional[_Outcome] = None

    def close(self) -> None:
        self.closed = True

    def begin_attempt(self) -> None:
        self._slots = []

    def refs(self) -> List[object]:
        return [ref for ref in self._slots if ref is not None]

    async def progress(self, marker: str) -> None:
        if self.closed:
            return
        await self._publish(marker)

    def _latch(self, failure: _Outcome) -> NoReturn:
        if self.failure is None:
            self.failure = failure
        raise JobFailure(_RESOURCE_EXHAUSTED)

    async def artifact(self, name: str, media_type: str, content: bytes) -> None:
        if self.closed:
            return
        if len(self._slots) >= self._max_artifacts:
            self._latch(_ARTIFACT_COUNT_EXCEEDED)
        if len(content) > self._max_artifact_bytes:
            self._latch(_ARTIFACT_TOO_LARGE)
        slots = self._slots  # a late completion cannot fill a subsequent attempt's slots
        index = len(slots)
        slots.append(None)
        failure, ref = None, None
        try:
            if self._externalize is None:
                failure = _ARTIFACT_WRITE_FAILED
            else:
                ref = await self._externalize(name, media_type, content)
        except ResourceTooLargeError:
            failure = _ARTIFACT_TOO_LARGE
        except Exception:
            failure = _ARTIFACT_WRITE_FAILED
        if self.closed:
            return
        if failure is not None:
            self._latch(failure)
        slots[index] = ref
