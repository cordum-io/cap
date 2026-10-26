"""LlamaIndex integration — wrap existing tools with Cordum safety checks.

Requires ``llama-index-core`` to be installed::

    pip install cordum-guard[llamaindex]

Example::

    from cordum_guard import CordumClient
    from cordum_guard.llamaindex import CordumToolGuard

    client = CordumClient("http://localhost:8081", api_key="...")
    guarded = CordumToolGuard(client, policy="agent_ops").wrap(tools)
"""

from __future__ import annotations

import asyncio
import functools
import inspect
import time
from typing import Any, Optional

from .client import CordumClient
from .guard import _gate_tool_call

try:
    from llama_index.core.tools import FunctionTool
except ImportError as _err:
    raise ImportError(
        "LlamaIndex integration requires llama-index-core. "
        "Install it with: pip install cordum-guard[llamaindex]"
    ) from _err


class CordumToolGuard:
    """Wraps LlamaIndex tools with Cordum safety evaluation.

    Same per-call behavior as the LangChain guard, on ``call()``, ``acall()``
    and ``real_fn``.
    """

    def __init__(
        self,
        client: CordumClient,
        policy: str = "",
        risk_tags: Optional[list[str]] = None,
        topic: str = "job.guard",
    ) -> None:
        self.client = client
        self.policy = policy
        self.risk_tags = risk_tags or []
        self.topic = topic

    def wrap(self, tools: list[FunctionTool]) -> list[FunctionTool]:
        """Return a new list of guarded tools."""
        return [self.wrap_one(t) for t in tools]

    def wrap_one(self, tool: FunctionTool) -> FunctionTool:
        """Wrap a single LlamaIndex FunctionTool with a safety check.

        Every entry point is guarded: ``call()`` runs ``_fn``, ``acall()`` runs
        ``_async_fn`` (built from the original function at construction time,
        so wrapping ``_fn`` alone left async agents unguarded), and
        CodeActAgent hands ``real_fn`` straight to its code executor.
        """
        guard_self = self
        name = tool.metadata.name or tool._fn.__name__
        description = tool.metadata.description or ""
        real_fn = getattr(tool, "_real_fn", None)
        # The user's function names positional arguments for arg.* labels.
        signature_fn = real_fn or tool._fn

        def guard_sync(fn: Any) -> Any:
            @functools.wraps(fn)
            def guarded(*args: Any, **kwargs: Any) -> Any:
                blocked, delay = _gate_tool_call(
                    guard_self, name, description, signature_fn, args, kwargs
                )
                if blocked is not None:
                    return blocked
                if delay:
                    time.sleep(delay)
                return fn(*args, **kwargs)

            return guarded

        def guard_async(fn: Any) -> Any:
            @functools.wraps(fn)
            async def guarded(*args: Any, **kwargs: Any) -> Any:
                blocked, delay = _gate_tool_call(
                    guard_self, name, description, signature_fn, args, kwargs
                )
                if blocked is not None:
                    return blocked
                if delay:
                    await asyncio.sleep(delay)
                return await fn(*args, **kwargs)

            return guarded

        tool._fn = guard_sync(tool._fn)
        if getattr(tool, "_async_fn", None) is not None:
            tool._async_fn = guard_async(tool._async_fn)
        if real_fn is not None:
            wrap = guard_async if inspect.iscoroutinefunction(real_fn) else guard_sync
            tool._real_fn = wrap(real_fn)
        return tool
