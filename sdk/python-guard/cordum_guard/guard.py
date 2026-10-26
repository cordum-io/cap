"""@guard decorator — intercept function calls with Cordum safety checks."""

from __future__ import annotations

import asyncio
import functools
import inspect
import re
import time
from decimal import Decimal
from typing import Any, Callable, Optional, TypeVar, overload

from .client import CordumClient
from .exceptions import CordumApprovalRequiredError, CordumBlockedError
from .types import Decision, SafetyDecision

F = TypeVar("F", bound=Callable[..., Any])

# Call arguments reach policy as ``arg.<name>`` labels. The prefix keeps
# LLM-chosen values out of label keys the kernel interprets itself
# (agent_id, secrets_present, mcp.*, _delegation.*).
_ARG_LABEL_PREFIX = "arg."
_MAX_ARG_LABELS = 16
_MAX_ARG_LABEL_VALUE = 256
_ARG_NAME = re.compile(r"[A-Za-z_][A-Za-z0-9_]{0,63}")
# Word-bounded secret names, the cordum-packs agent-adapter convention plus a
# few more; camelCase is split first so ``accessToken`` matches too.
_SECRET_NAME = re.compile(
    r"(?:^|_)(?:password|passwd|passphrase|secret|token|api_?key|access_?key|"
    r"private_?key|authorization|auth|credentials?|cookie|session)(?:_|$)"
)


def _is_secret_name(name: str) -> bool:
    return bool(_SECRET_NAME.search(re.sub(r"(?<=[a-z0-9])(?=[A-Z])", "_", name).lower()))


def _named_args(
    fn: Callable[..., Any], args: tuple[Any, ...], kwargs: dict[str, Any]
) -> dict[str, Any]:
    """Name a call's arguments by ``fn``'s signature (``**kwargs`` flattened,
    ``*args`` dropped)."""
    try:
        bound = inspect.signature(fn).bind_partial(*args, **kwargs)
    except (TypeError, ValueError):
        return dict(kwargs)
    named: dict[str, Any] = {}
    for name, value in bound.arguments.items():
        kind = bound.signature.parameters[name].kind
        if kind is inspect.Parameter.VAR_KEYWORD:
            named.update(value)
        elif kind is not inspect.Parameter.VAR_POSITIONAL:
            named[name] = value
    return named


def _arg_labels(named: dict[str, Any]) -> dict[str, str]:
    """Expose scalar call arguments to policy as bounded ``arg.<name>`` labels.

    Only bool / int / float / Decimal / str values are sent (bools as
    ``true`` / ``false``, NaN as ``inf``); None and containers are skipped.
    Names must match ``[A-Za-z_][A-Za-z0-9_]{0,63}`` and secret-like names
    are never sent. At most 16 labels, in argument order; values are cut at
    256 chars.
    """
    labels: dict[str, str] = {}
    for name, value in named.items():
        if len(labels) >= _MAX_ARG_LABELS:
            break
        if not _ARG_NAME.fullmatch(name) or _is_secret_name(name):
            continue
        if isinstance(value, bool):
            text = "true" if value else "false"
        elif isinstance(value, (int, float, Decimal)):
            # NaN exceeds no label_threshold; send it as +inf so thresholds trip.
            text = "inf" if value != value else str(value)
        elif isinstance(value, str):
            text = value
        else:
            continue
        labels[_ARG_LABEL_PREFIX + name] = text[:_MAX_ARG_LABEL_VALUE]
    return labels


def _check_decision(decision: SafetyDecision) -> float:
    """Gate a call on a verdict, failing closed; return seconds to wait first.

    ALLOW and ALLOW_WITH_CONSTRAINTS run (constraints are NOT enforced
    in-process), THROTTLE runs after its delay, REQUIRE_APPROVAL (or
    ``approval_required``) raises :class:`CordumApprovalRequiredError`, and
    DENY or any other verdict raises :class:`CordumBlockedError`.
    """
    verdict = decision.decision
    if verdict == Decision.DENY:
        raise CordumBlockedError(decision)
    if verdict == Decision.REQUIRE_APPROVAL or decision.approval_required:
        raise CordumApprovalRequiredError(decision)
    if verdict == Decision.THROTTLE:
        return decision.throttle_duration_seconds
    if verdict in (Decision.ALLOW, Decision.ALLOW_WITH_CONSTRAINTS):
        return 0.0
    raise CordumBlockedError(decision)


def _gate_tool_call(
    tool_guard: Any,
    name: str,
    description: str,
    fn: Callable[..., Any],
    args: tuple[Any, ...],
    kwargs: dict[str, Any],
) -> tuple[Optional[str], float]:
    """Evaluate one framework tool call (LangChain / LlamaIndex).

    Returns ``(blocked, delay)``: ``blocked`` is the ``[BLOCKED]`` message the
    agent sees instead of the tool result (DENY or an unknown verdict), else
    None and ``delay`` is the seconds to wait before running. Raises
    :class:`CordumApprovalRequiredError` when a human must approve.
    """
    decision = tool_guard.client.evaluate_policy(
        topic=tool_guard.topic,
        capability=name,
        risk_tags=tool_guard.risk_tags,
        labels={
            "policy": tool_guard.policy,
            "tool": name,
            "description": description[:200],
            **_arg_labels(_named_args(fn, args, kwargs)),
        },
    )
    try:
        return None, _check_decision(decision)
    except CordumApprovalRequiredError:
        raise
    except CordumBlockedError:
        return f"[BLOCKED] {name}: {decision.reason}", 0.0


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

    Scalar arguments are sent as ``arg.<name>`` labels (see README).
    """
    tags = risk_tags or []

    def decorator(fn: F) -> F:
        cap = capability or fn.__name__
        labels = {"policy": policy, "function": fn.__qualname__} if policy else {"function": fn.__qualname__}

        if inspect.iscoroutinefunction(fn):

            @functools.wraps(fn)
            async def async_wrapper(*args: Any, **kwargs: Any) -> Any:
                call_labels = {**labels, **_arg_labels(_named_args(fn, args, kwargs))}
                decision = client.evaluate_policy(
                    topic=topic,
                    capability=cap,
                    risk_tags=tags,
                    labels=call_labels,
                )
                return await _handle_decision_async(
                    decision, client, fn, args, kwargs, topic, cap, tags, call_labels, timeout
                )

            return async_wrapper  # type: ignore[return-value]

        @functools.wraps(fn)
        def sync_wrapper(*args: Any, **kwargs: Any) -> Any:
            call_labels = {**labels, **_arg_labels(_named_args(fn, args, kwargs))}
            decision = client.evaluate_policy(
                topic=topic,
                capability=cap,
                risk_tags=tags,
                labels=call_labels,
            )
            return _handle_decision_sync(
                decision, client, fn, args, kwargs, topic, cap, tags, call_labels, timeout
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
    try:
        delay = _check_decision(decision)
    except CordumApprovalRequiredError:
        # Submit a job (same labels as the evaluation) so the approval
        # appears in the dashboard, then wait for the human verdict.
        job = client.submit_job(
            prompt=f"Approval required: {cap}({_summarize_args(_named_args(fn, args, kwargs))})",
            topic=topic,
            capability=cap,
            risk_tags=tags,
            labels=labels,
        )
        status = client.wait_for_decision(job.job_id, timeout=timeout)
        if status.state.lower() in ("succeeded", "running"):
            return fn(*args, **kwargs)
        raise CordumBlockedError(decision) from None
    if delay:
        time.sleep(delay)
    return fn(*args, **kwargs)


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
    try:
        delay = _check_decision(decision)
    except CordumApprovalRequiredError:
        job = client.submit_job(
            prompt=f"Approval required: {cap}({_summarize_args(_named_args(fn, args, kwargs))})",
            topic=topic,
            capability=cap,
            risk_tags=tags,
            labels=labels,
        )
        status = client.wait_for_decision(job.job_id, timeout=timeout)
        if status.state.lower() in ("succeeded", "running"):
            return await fn(*args, **kwargs)
        raise CordumBlockedError(decision) from None
    if delay:
        await asyncio.sleep(delay)
    return await fn(*args, **kwargs)


def _summarize_args(named: dict[str, Any]) -> str:
    """Short summary of named arguments for the approval prompt; values of
    secret-like names are masked."""
    parts: list[str] = []
    for k, v in named.items():
        s = "***" if _is_secret_name(k) else repr(v)
        parts.append(f"{k}={s[:50]}..." if len(s) > 50 else f"{k}={s}")
    summary = ", ".join(parts)
    return summary[:200] + "..." if len(summary) > 200 else summary
