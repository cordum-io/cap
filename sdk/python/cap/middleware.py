"""Middleware support for CAP agents and workers."""

import logging
import time
from typing import Any, Awaitable, Callable

from cap.runtime import Context

# NextFn calls the next middleware or the terminal handler.
NextFn = Callable[[Context, Any], Awaitable[Any]]

# Middleware intercepts handler execution. Call ``next_fn(ctx, data)`` to
# invoke the next middleware or the terminal handler. Middleware is applied
# in FIFO order: the first registered middleware is the outermost.
Middleware = Callable[[Context, Any, NextFn], Awaitable[Any]]


def logging_middleware(logger: logging.Logger = None) -> "Middleware":
    """Return a middleware that logs job ID, topic, and duration.

    Args:
        logger: Logger to use. Defaults to ``logging.getLogger("cap.middleware")``.
    """
    if logger is None:
        logger = logging.getLogger("cap.middleware")

    async def middleware(ctx: Context, data: Any, next_fn: NextFn) -> Any:
        start = time.monotonic()
        try:
            result = await next_fn(ctx, data)
            elapsed = int((time.monotonic() - start) * 1000)
            logger.info(
                "middleware: job=%s topic=%s duration=%dms ok",
                ctx.job_id,
                ctx.job.topic,
                elapsed,
            )
            return result
        except Exception as exc:
            elapsed = int((time.monotonic() - start) * 1000)
            logger.info(
                "middleware: job=%s topic=%s duration=%dms error=%s",
                ctx.job_id,
                ctx.job.topic,
                elapsed,
                exc,
            )
            raise

    return middleware
