"""Cancellation-safe timing and transport failure classification."""

import asyncio
from typing import Any, Awaitable

from nats.errors import (
    ConnectionClosedError,
    ConnectionDrainingError,
    ConnectionReconnectingError,
    NoRespondersError,
    NoServersError,
    OutboundBufferLimitError,
    StaleConnectionError,
)


_OPERATIONAL_ERRORS = (
    OSError,
    ConnectionClosedError,
    ConnectionDrainingError,
    ConnectionReconnectingError,
    NoRespondersError,
    NoServersError,
    OutboundBufferLimitError,
    StaleConnectionError,
)


class WorkerTrustOperationalError(RuntimeError):
    """A transport-boundary failure that WARN mode may fail open for."""

    def __init__(self, cause: Exception) -> None:
        super().__init__("worker trust transport is unavailable")
        self.cause = cause


def is_transport_failure(error: Exception) -> bool:
    """Return whether a request-boundary error is operational transport loss."""
    return isinstance(error, asyncio.TimeoutError) or isinstance(
        error, _OPERATIONAL_ERRORS
    )


def is_operational_failure(error: Exception) -> bool:
    """Return whether a failure was explicitly tagged at the transport boundary."""
    return isinstance(error, WorkerTrustOperationalError)


def _discard_task_result(task: "asyncio.Task[Any]") -> None:
    try:
        task.result()
    except BaseException:
        pass


def _cancel_background(task: "asyncio.Task[Any]") -> None:
    task.add_done_callback(_discard_task_result)
    task.cancel()


async def await_with_timeout(awaitable: Awaitable[Any], timeout: float) -> Any:
    """Bound an awaitable without Python 3.9 wait_for cancellation loss."""
    operation = asyncio.ensure_future(awaitable)
    timer = asyncio.create_task(asyncio.sleep(timeout))
    try:
        done, _ = await asyncio.wait(
            (operation, timer), return_when=asyncio.FIRST_COMPLETED
        )
    except BaseException:
        _cancel_background(operation)
        _cancel_background(timer)
        raise
    if operation in done:
        _cancel_background(timer)
        return operation.result()
    _cancel_background(operation)
    raise asyncio.TimeoutError()


__all__ = [
    "WorkerTrustOperationalError",
    "await_with_timeout",
    "is_operational_failure",
    "is_transport_failure",
]
