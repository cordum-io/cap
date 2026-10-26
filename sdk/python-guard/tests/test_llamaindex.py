"""Tests for cordum_guard.llamaindex — LlamaIndex FunctionTool wrapping.

These tests mock llama_index.core.tools to avoid requiring it as a dependency.
"""

import asyncio
import sys
import types
from unittest.mock import AsyncMock, MagicMock

import pytest

from cordum_guard.exceptions import CordumApprovalRequiredError
from cordum_guard.types import Decision, SafetyDecision


def _install_llamaindex_mock():
    """Install a minimal mock of llama_index.core.tools into sys.modules."""
    li = types.ModuleType("llama_index")
    li_core = types.ModuleType("llama_index.core")
    li_tools = types.ModuleType("llama_index.core.tools")

    class FunctionTool:
        pass

    li_tools.FunctionTool = FunctionTool
    li_core.tools = li_tools
    li.core = li_core
    sys.modules["llama_index"] = li
    sys.modules["llama_index.core"] = li_core
    sys.modules["llama_index.core.tools"] = li_tools
    return FunctionTool


_FunctionTool = _install_llamaindex_mock()

if "cordum_guard.llamaindex" in sys.modules:
    del sys.modules["cordum_guard.llamaindex"]

from cordum_guard.llamaindex import CordumToolGuard


def _make_tool(name: str = "transfer_funds"):
    """Create a mock FunctionTool with both call paths (call / acall)."""
    tool = _FunctionTool()
    tool.metadata = types.SimpleNamespace(name=name, description="Moves money")
    tool._fn = MagicMock(return_value="sync result")
    tool._async_fn = AsyncMock(return_value="async result")
    return tool


def _make_client(decision: Decision, **extra) -> MagicMock:
    client = MagicMock()
    client.evaluate_policy.return_value = SafetyDecision(decision=decision, **extra)
    return client


class TestAllowPath:
    def test_allow_runs_both_paths(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool()
        original_fn, original_async_fn = tool._fn, tool._async_fn

        CordumToolGuard(client).wrap_one(tool)

        assert tool._fn(amount=1) == "sync result"
        assert asyncio.run(tool._async_fn(amount=1)) == "async result"
        original_fn.assert_called_once_with(amount=1)
        original_async_fn.assert_awaited_once_with(amount=1)
        assert client.evaluate_policy.call_count == 2


class TestDenyPath:
    def test_deny_blocks_sync(self):
        client = _make_client(Decision.DENY, reason="Policy violation")
        tool = _make_tool()
        original_fn = tool._fn

        CordumToolGuard(client).wrap_one(tool)

        assert tool._fn(amount=1) == "[BLOCKED] transfer_funds: Policy violation"
        original_fn.assert_not_called()

    def test_deny_blocks_async_path(self):
        # FunctionTool.acall() runs _async_fn, which LlamaIndex builds from the
        # original function at construction time; wrapping only _fn let every
        # async agent call run unguarded.
        client = _make_client(Decision.DENY, reason="Policy violation")
        tool = _make_tool()
        original_async_fn = tool._async_fn

        CordumToolGuard(client).wrap_one(tool)

        result = asyncio.run(tool._async_fn(amount=15000))
        assert result == "[BLOCKED] transfer_funds: Policy violation"
        original_async_fn.assert_not_awaited()


class TestRealFnPath:
    # CodeActAgent passes tool.real_fn (the original function) to its code
    # executor, bypassing both _fn and _async_fn.
    def test_sync_real_fn_is_guarded(self):
        ran = []

        def transfer_funds(amount: float) -> str:
            ran.append(amount)
            return "sent"

        client = _make_client(Decision.DENY, reason="Policy violation")
        tool = _make_tool()
        tool._real_fn = transfer_funds

        CordumToolGuard(client).wrap_one(tool)

        assert tool._real_fn(15000) == "[BLOCKED] transfer_funds: Policy violation"
        assert ran == []
        # Positional arguments are named from the real function's signature.
        labels = client.evaluate_policy.call_args.kwargs["labels"]
        assert labels["arg.amount"] == "15000"

    def test_async_real_fn_stays_async_and_guarded(self):
        ran = []

        async def transfer_funds(amount: float) -> str:
            ran.append(amount)
            return "sent"

        client = _make_client(Decision.REQUIRE_APPROVAL, reason="needs sign-off")
        tool = _make_tool()
        tool._real_fn = transfer_funds

        CordumToolGuard(client).wrap_one(tool)

        with pytest.raises(CordumApprovalRequiredError):
            asyncio.run(tool._real_fn(amount=15000))
        assert ran == []


class TestRequireApprovalPath:
    def test_require_approval_raises_and_tool_not_run(self):
        client = _make_client(
            Decision.REQUIRE_APPROVAL, reason="needs sign-off", approval_ref="job-7"
        )
        tool = _make_tool()
        original_fn, original_async_fn = tool._fn, tool._async_fn

        CordumToolGuard(client).wrap_one(tool)

        with pytest.raises(CordumApprovalRequiredError) as exc_info:
            tool._fn(amount=15000)
        assert exc_info.value.approval_ref == "job-7"
        with pytest.raises(CordumApprovalRequiredError):
            asyncio.run(tool._async_fn(amount=15000))
        original_fn.assert_not_called()
        original_async_fn.assert_not_awaited()


class TestFailClosedDecisions:
    def test_unknown_decision_blocks(self):
        client = MagicMock()
        client.evaluate_policy.return_value = SafetyDecision.model_construct(
            decision="redact", reason="unrecognised verdict"
        )
        tool = _make_tool()
        original_fn = tool._fn

        CordumToolGuard(client).wrap_one(tool)

        assert tool._fn(amount=1).startswith("[BLOCKED] transfer_funds")
        original_fn.assert_not_called()


class TestLabels:
    def test_argument_labels_and_tool_labels(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool()

        CordumToolGuard(client, policy="financial_ops").wrap_one(tool)
        asyncio.run(tool._async_fn(amount=15000, to_account="alice", api_key="k-1"))

        call_kwargs = client.evaluate_policy.call_args.kwargs
        labels = call_kwargs["labels"]
        assert call_kwargs["capability"] == "transfer_funds"
        assert labels["policy"] == "financial_ops"
        assert labels["tool"] == "transfer_funds"
        assert labels["arg.amount"] == "15000"
        assert labels["arg.to_account"] == "alice"
        assert "arg.api_key" not in labels
