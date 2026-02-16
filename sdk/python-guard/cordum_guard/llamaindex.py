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

import functools
from typing import Any, Optional

from .client import CordumClient
from .types import Decision

try:
    from llama_index.core.tools import FunctionTool
except ImportError as _err:
    raise ImportError(
        "LlamaIndex integration requires llama-index-core. "
        "Install it with: pip install cordum-guard[llamaindex]"
    ) from _err


class CordumToolGuard:
    """Wraps LlamaIndex tools with Cordum safety evaluation."""

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
        """Wrap a single LlamaIndex FunctionTool with a safety check."""
        guard_self = self
        original_fn = tool._fn

        @functools.wraps(original_fn)
        def guarded_fn(*args: Any, **kwargs: Any) -> Any:
            decision = guard_self.client.evaluate_policy(
                topic=guard_self.topic,
                capability=tool.metadata.name or original_fn.__name__,
                risk_tags=guard_self.risk_tags,
                labels={
                    "policy": guard_self.policy,
                    "tool": tool.metadata.name or "",
                    "description": (tool.metadata.description or "")[:200],
                },
            )
            if decision.decision == Decision.DENY:
                return f"[BLOCKED] {tool.metadata.name}: {decision.reason}"
            if decision.decision == Decision.THROTTLE:
                import time

                time.sleep(decision.throttle_duration_seconds)
            return original_fn(*args, **kwargs)

        tool._fn = guarded_fn
        return tool
