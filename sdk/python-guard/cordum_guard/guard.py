"""@guard decorator — intercept function calls with Cordum safety checks."""

from __future__ import annotations

import asyncio
import functools
import inspect
import time
from typing import Any, Callable, Optional, TypeVar, overload

from .client import CordumClient
from .exceptions import CordumBlockedError
from .types import Decision

F = TypeVar("F", bound=Callable[..., Any])


@overload
def guard(
    client: CordumClient,
    *,
    policy: str = "",
    risk_tags: Optional[list[str]] = None,
    capability: str = "",
    topic: str = "job.guard",
    timeout: float = 300.0,
) -> Callable[[F], F]: ...


def guard(
    client: CordumClient,
    *,
    policy: str = "",
    risk_tags: Optional[list[str]] = None,
    capability: str = "",
    topic: str = "job.guard",
    timeout: float = 300.0,
) -> Callable[[F], F]:
    """Decorator that enforces Cordum safety policy before executing a function.

    Example::

        @guard(client, policy="financial_ops", risk_tags=["write", "financial"])
        def execute_transfer(amount: float, to_account: str):
            bank_api.transfer(amount, to_account)

    Args:
        client: A :class:`CordumClient` instance.
        policy: Policy name (used as label for traceability).
        risk_tags: Risk tags sent to the Safety Kernel.
        capability: Override the capability name (defaults to function name).
        topic: NATS topic for the policy evaluation.
        timeout: Max seconds to wait for an approval decision.
    """
    tags = risk_tags or []

    def decorator(fn: F) -> F:
        cap = capability or fn.__name__
        labels = {"policy": policy, "function": fn.__qualname__} if policy else {"function": fn.__qualname__}

        if inspect.iscoroutinefunction(fn):

            @functools.wraps(fn)
            async def async_wrapper(*args: Any, **kwargs: Any) -> Any:
                decision = client.evaluate_policy(
                    topic=topic,
                    capability=cap,
                    risk_tags=tags,
                    labels=labels,
                )
                return await _handle_decision_async(
                    decision, client, fn, args, kwargs, topic, cap, tags, labels, timeout
                )

            return async_wrapper  # type: ignore[return-value]

        @functools.wraps(fn)
        def sync_wrapper(*args: Any, **kwargs: Any) -> Any:
            decision = client.evaluate_policy(
                topic=topic,
                capability=cap,
                risk_tags=tags,
                labels=labels,
            )
            return _handle_decision_sync(
                decision, client, fn, args, kwargs, topic, cap, tags, labels, timeout
            )

        return sync_wrapper  # type: ignore[return-value]

    return decorator  # type: ignore[return-value]


def _handle_decision_sync(
    decision: Any,
    client: CordumClient,
    fn: Callable[..., Any],
    args: tuple[Any, ...],
    kwargs: dict[str, Any],
    topic: str,
    cap: str,
    tags: list[str],
    labels: dict[str, str],
    timeout: float,
) -> Any:
    if decision.decision == Decision.ALLOW:
        return fn(*args, **kwargs)

    if decision.decision == Decision.DENY:
        raise CordumBlockedError(decision)

    if decision.decision == Decision.THROTTLE:
        time.sleep(decision.throttle_duration_seconds)
        return fn(*args, **kwargs)

    if decision.decision == Decision.REQUIRE_APPROVAL:
        # Submit a job so the approval appears in the dashboard.
        job = client.submit_job(
            prompt=f"Approval required: {cap}({_summarize_args(args, kwargs)})",
            topic=topic,
            capability=cap,
            risk_tags=tags,
            labels=labels,
        )
        status = client.wait_for_decision(job.job_id, timeout=timeout)
        if status.state.lower() in ("succeeded", "running"):
            return fn(*args, **kwargs)
        raise CordumBlockedError(decision)

    # Unknown decision — fail-safe deny.
    raise CordumBlockedError(decision)


async def _handle_decision_async(
    decision: Any,
    client: CordumClient,
    fn: Callable[..., Any],
    args: tuple[Any, ...],
    kwargs: dict[str, Any],
    topic: str,
    cap: str,
    tags: list[str],
    labels: dict[str, str],
    timeout: float,
) -> Any:
    if decision.decision == Decision.ALLOW:
        return await fn(*args, **kwargs)

    if decision.decision == Decision.DENY:
        raise CordumBlockedError(decision)

    if decision.decision == Decision.THROTTLE:
        await asyncio.sleep(decision.throttle_duration_seconds)
        return await fn(*args, **kwargs)

    if decision.decision == Decision.REQUIRE_APPROVAL:
        job = client.submit_job(
            prompt=f"Approval required: {cap}({_summarize_args(args, kwargs)})",
            topic=topic,
            capability=cap,
            risk_tags=tags,
            labels=labels,
        )
        status = client.wait_for_decision(job.job_id, timeout=timeout)
        if status.state.lower() in ("succeeded", "running"):
            return await fn(*args, **kwargs)
        raise CordumBlockedError(decision)

    raise CordumBlockedError(decision)


def _summarize_args(args: tuple[Any, ...], kwargs: dict[str, Any]) -> str:
    """Create a short summary of function arguments for the approval prompt."""
    parts: list[str] = []
    for a in args:
        s = repr(a)
        parts.append(s[:50] + "..." if len(s) > 50 else s)
    for k, v in kwargs.items():
        s = repr(v)
        parts.append(f"{k}={s[:50]}..." if len(s) > 50 else f"{k}={s}")
    summary = ", ".join(parts)
    return summary[:200] + "..." if len(summary) > 200 else summary
