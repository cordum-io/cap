"""LangChain integration — wrap existing tools with Cordum safety checks.

Requires ``langchain-core`` to be installed::

    pip install cordum-guard[langchain]

Example::

    from cordum_guard import CordumClient
    from cordum_guard.langchain import CordumToolGuard

    client = CordumClient("http://localhost:8081", api_key="...")
    guarded = CordumToolGuard(client, policy="agent_ops").wrap(tools)
    agent = initialize_agent(guarded, llm)
"""

from __future__ import annotations

from typing import Any, Optional

from .client import CordumClient
from .types import Decision

try:
    from langchain_core.tools import BaseTool, ToolException
except ImportError as _err:
    raise ImportError(
        "LangChain integration requires langchain-core. "
        "Install it with: pip install cordum-guard[langchain]"
    ) from _err


class CordumToolGuard:
    """Wraps LangChain tools with Cordum safety evaluation."""

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

    def wrap(self, tools: list[BaseTool]) -> list[BaseTool]:
        """Return a new list of guarded tools."""
        return [self.wrap_one(t) for t in tools]

    def wrap_one(self, tool: BaseTool) -> BaseTool:
        """Wrap a single LangChain tool with a safety check."""
        guard_self = self
        original_run = tool._run

        def guarded_run(
            *args: Any, **kwargs: Any
        ) -> str:
            decision = guard_self.client.evaluate_policy(
                topic=guard_self.topic,
                capability=tool.name,
                risk_tags=guard_self.risk_tags,
                labels={
                    "policy": guard_self.policy,
                    "tool": tool.name,
                    "description": (tool.description or "")[:200],
                },
            )
            if decision.decision == Decision.DENY:
                return f"[BLOCKED] {tool.name}: {decision.reason}"
            if decision.decision == Decision.THROTTLE:
                import time

                time.sleep(decision.throttle_duration_seconds)
            return original_run(*args, **kwargs)  # type: ignore[misc]

        tool._run = guarded_run  # type: ignore[assignment]

        # Wrap async variant if present
        if hasattr(tool, "_arun") and tool._arun is not None:
            original_arun = tool._arun

            async def guarded_arun(
                *args: Any, **kwargs: Any
            ) -> str:
                decision = guard_self.client.evaluate_policy(
                    topic=guard_self.topic,
                    capability=tool.name,
                    risk_tags=guard_self.risk_tags,
                    labels={
                        "policy": guard_self.policy,
                        "tool": tool.name,
                    },
                )
                if decision.decision == Decision.DENY:
                    return f"[BLOCKED] {tool.name}: {decision.reason}"
                if decision.decision == Decision.THROTTLE:
                    import asyncio

                    await asyncio.sleep(decision.throttle_duration_seconds)
                return await original_arun(*args, **kwargs)  # type: ignore[misc]

            tool._arun = guarded_arun  # type: ignore[assignment]

        return tool
