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

import asyncio
import functools
import time
from typing import Any, Optional

from .client import CordumClient
from .guard import _gate_tool_call

try:
    from langchain_core.tools import BaseTool, ToolException
except ImportError as _err:
    raise ImportError(
        "LangChain integration requires langchain-core. "
        "Install it with: pip install cordum-guard[langchain]"
    ) from _err


class CordumToolGuard:
    """Wraps LangChain tools with Cordum safety evaluation.

    Per call: DENY or an unknown verdict returns ``[BLOCKED] <tool>: <reason>``
    without running the tool; REQUIRE_APPROVAL raises
    :class:`~cordum_guard.exceptions.CordumApprovalRequiredError`; THROTTLE
    waits, then runs; ALLOW and ALLOW_WITH_CONSTRAINTS run. Scalar tool
    arguments are sent as ``arg.<name>`` labels.
    """

    def __init__(
        self,
        client: CordumClient,
        policy: str = "",
        risk_tags: Optional[list[str]] = None,
        topic: str = "job.guard",
        include_args: bool = True,
    ) -> None:
        self.client = client
        self.policy = policy
        self.risk_tags = risk_tags or []
        self.topic = topic
        # False: send no arg.* labels (tools taking sensitive free text).
        self.include_args = include_args

    def wrap(self, tools: list[BaseTool]) -> list[BaseTool]:
        """Return a new list of guarded tools."""
        return [self.wrap_one(t) for t in tools]

    def wrap_one(self, tool: BaseTool) -> BaseTool:
        """Wrap a single LangChain tool with a safety check."""
        guard_self = self
        original_run = tool._run
        description = tool.description or ""

        # wraps() keeps the original signature: LangChain injects `config` and
        # `run_manager` only when `_run` declares them (StructuredTool needs both).
        @functools.wraps(original_run)
        def guarded_run(*args: Any, **kwargs: Any) -> Any:
            blocked, delay = _gate_tool_call(
                guard_self, tool.name, description, original_run, args, kwargs
            )
            if blocked is not None:
                return blocked
            if delay:
                time.sleep(delay)
            return original_run(*args, **kwargs)  # type: ignore[misc]

        tool._run = guarded_run  # type: ignore[assignment]

        # Wrap async variant if present
        if hasattr(tool, "_arun") and tool._arun is not None:
            original_arun = tool._arun

            @functools.wraps(original_arun)
            async def guarded_arun(*args: Any, **kwargs: Any) -> Any:
                blocked, delay = _gate_tool_call(
                    guard_self, tool.name, description, original_arun, args, kwargs
                )
                if blocked is not None:
                    return blocked
                if delay:
                    await asyncio.sleep(delay)
                return await original_arun(*args, **kwargs)  # type: ignore[misc]

            tool._arun = guarded_arun  # type: ignore[assignment]

        return tool
