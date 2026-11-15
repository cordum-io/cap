"""Typed job outcomes with spec/13 error codes.

Handlers raise :class:`JobFailure` to publish a structured failure instead of
an opaque ``"handler failed"``. The runtime publishes only fixed text from the
table below, so exception text, input bytes and pointer text never reach a
published :class:`JobResult` field.
"""

import re
from dataclasses import dataclass
from functools import partial
from typing import Callable, Dict, FrozenSet, Mapping, Optional, Tuple, Union

from cap.pb.cordum.agent.v1 import job_pb2


def _enum_number(enum_name: str, value_name: str) -> int:
    # The generated module has no stubs; its descriptor is the typed way in.
    number: int = job_pb2.DESCRIPTOR.enum_types_by_name[enum_name].values_by_name[value_name].number
    return number


def _error_code(name: str) -> int:
    return _enum_number("ErrorCode", "ERROR_CODE_" + name)


_STATUS_FAILED = _enum_number("JobStatus", "JOB_STATUS_FAILED")
_STATUS_RETRYABLE = _enum_number("JobStatus", "JOB_STATUS_FAILED_RETRYABLE")
_STATUS_FATAL = _enum_number("JobStatus", "JOB_STATUS_FAILED_FATAL")
_UNSPECIFIED = _error_code("UNSPECIFIED")
_INVALID_INPUT = _error_code("JOB_INVALID_INPUT")
_NOT_FOUND = _error_code("JOB_NOT_FOUND")
_RESOURCE_EXHAUSTED = _error_code("JOB_RESOURCE_EXHAUSTED")

_DEFINED_ERROR_CODES: FrozenSet[int] = frozenset(
    value.number for value in job_pb2.DESCRIPTOR.enum_types_by_name["ErrorCode"].values
) - {_UNSPECIFIED}
_APPLICATION_CODES = range(1000, 10000)
_STRING_CODE = re.compile(r"[A-Z][A-Z0-9_]{0,63}")

# spec/13 "Legacy String Mapping": the error_code string older consumers read.
_LEGACY_ERROR_STRINGS: Mapping[int, str] = {
    _error_code("JOB_TIMEOUT"): "TIMEOUT",
    _error_code("SAFETY_DENIED"): "DENIED",
    _error_code("JOB_PERMISSION_DENIED"): "PERMISSION_DENIED",
}


@dataclass(frozen=True)
class _Outcome:
    """The spec/13 fields of one published failure. Every field is fixed text."""

    status: int
    error_code_enum: int
    error_code: str
    error_message: str


def _failed(error_code_enum: int, error_message: str) -> _Outcome:
    return _Outcome(_STATUS_FAILED, error_code_enum, "", error_message)


# The SDK failure table (task-013d478b, comment-e33d348f).
_CONTEXT_POINTER_INVALID = _failed(_INVALID_INPUT, "context pointer invalid")
# With a ResourceRegistry the input is JobRequest.context_ref (task-d35bf16f).
_CONTEXT_REF_REQUIRED = _failed(_INVALID_INPUT, "context ref required")
_CONTEXT_REF_REFUSED = _failed(_INVALID_INPUT, "context ref refused")
_CONTEXT_FETCH_FAILED = _failed(_UNSPECIFIED, "context fetch failed")
_CONTEXT_NOT_FOUND = _failed(_NOT_FOUND, "context not found")
_CONTEXT_TOO_LARGE = _failed(_RESOURCE_EXHAUSTED, "context exceeds max size")
_CONTEXT_DECODE_FAILED = _failed(_INVALID_INPUT, "context decode failed")
_INPUT_INVALID = _failed(_INVALID_INPUT, "input validation failed")
_HANDLER_FAILED = _failed(_UNSPECIFIED, "handler failed")
_RESULT_ENCODE_FAILED = _failed(_UNSPECIFIED, "result encode failed")
_RESULT_TOO_LARGE = _failed(_RESOURCE_EXHAUSTED, "result exceeds max size")
_RESULT_WRITE_FAILED = _failed(_UNSPECIFIED, "result write failed")

_REPORTED_MESSAGE = "handler reported failure"


def _normalize_code(code: Union[int, str]) -> Union[int, str]:
    # Exact-type copies: a subclass cannot carry its own __str__ or enum name.
    if isinstance(code, bool) or not isinstance(code, (int, str)):
        raise TypeError("JobFailure code must be an int or a str")
    if isinstance(code, int):
        value = int.__int__(code)
        if value not in _DEFINED_ERROR_CODES and value not in _APPLICATION_CODES:
            raise ValueError(
                "JobFailure int code must be a defined ErrorCode other than "
                "UNSPECIFIED, or an application code in 1000-9999"
            )
        return value
    text = str.__str__(code)
    if _STRING_CODE.fullmatch(text) is None:
        raise ValueError("JobFailure str code must match [A-Z][A-Z0-9_]{0,63}")
    return text


class JobFailure(Exception):
    """Raise from a handler or middleware to publish a typed spec/13 failure.

    ``code`` is an ``ErrorCode`` value (or an application code in 1000-9999),
    which sets ``error_code_enum``, or an upper-snake string, which sets
    ``error_code``. ``retryable`` selects the status: ``None`` publishes
    ``JOB_STATUS_FAILED``, ``True`` ``JOB_STATUS_FAILED_RETRYABLE`` and
    ``False`` ``JOB_STATUS_FAILED_FATAL``. The SDK never retries a
    ``JobFailure``, and its ``error_message`` is always the fixed
    ``"handler reported failure"``.
    """

    def __init__(self, code: Union[int, str], *, retryable: Optional[bool] = None) -> None:
        normalized = _normalize_code(code)
        if retryable is not None and not isinstance(retryable, bool):
            raise TypeError("JobFailure retryable must be None or a bool")
        super().__init__("job failure: %s" % normalized)
        self._code = normalized
        self._retryable = retryable
        # Name-mangled so a subclass attribute cannot replace what is published.
        self.__outcome = _Outcome(
            _status_for(retryable),
            normalized if isinstance(normalized, int) else _UNSPECIFIED,
            normalized if isinstance(normalized, str) else _LEGACY_ERROR_STRINGS.get(normalized, ""),
            _REPORTED_MESSAGE,
        )

    def __reduce__(self) -> Tuple[Callable[..., "JobFailure"], Tuple[Union[int, str]], Dict[str, object]]:
        # The default rebuilds from args (the fixed text), which is not a valid code.
        return partial(type(self), retryable=self._retryable), (self._code,), dict(vars(self))

    @property
    def code(self) -> Union[int, str]:
        return self._code

    @property
    def retryable(self) -> Optional[bool]:
        return self._retryable

    @property
    def status(self) -> int:
        return self.__outcome.status

    @property
    def error_code_enum(self) -> int:
        return self.__outcome.error_code_enum

    @property
    def error_code(self) -> str:
        return self.__outcome.error_code


def _status_for(retryable: Optional[bool]) -> int:
    if retryable is None:
        return _STATUS_FAILED
    return _STATUS_RETRYABLE if retryable else _STATUS_FATAL


def _failure_outcome(failure: JobFailure) -> _Outcome:
    """The validated outcome; a JobFailure that skipped __init__ fails closed."""
    outcome = vars(failure).get("_JobFailure__outcome")
    return outcome if isinstance(outcome, _Outcome) else _HANDLER_FAILED
