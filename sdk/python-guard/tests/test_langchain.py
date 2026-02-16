"""Tests for cordum_guard.langchain — LangChain tool wrapping.

These tests mock langchain_core to avoid requiring it as a dependency.
"""

import sys
import types
from unittest.mock import MagicMock

import pytest

from cordum_guard.types import Decision, SafetyDecision


def _install_langchain_mock():
    """Install a minimal mock of langchain_core.tools into sys.modules."""
    # Create mock module hierarchy
    lc_core = types.ModuleType("langchain_core")
    lc_tools = types.ModuleType("langchain_core.tools")

    class BaseTool:
        name: str = ""
        description: str = ""

        def _run(self, *args, **kwargs):
            raise NotImplementedError

        _arun = None

    lc_tools.BaseTool = BaseTool
    lc_tools.ToolException = Exception
    lc_core.tools = lc_tools

    sys.modules["langchain_core"] = lc_core
    sys.modules["langchain_core.tools"] = lc_tools
    return BaseTool


# Install mock before importing the module
_BaseTool = _install_langchain_mock()

# Now we can import the langchain integration
# Need to remove cached module if previously loaded without mock
if "cordum_guard.langchain" in sys.modules:
    del sys.modules["cordum_guard.langchain"]

from cordum_guard.langchain import CordumToolGuard


def _make_tool(name: str = "test_tool", description: str = "A test tool"):
    """Create a mock LangChain BaseTool."""
    tool = _BaseTool()
    tool.name = name
    tool.description = description
    tool._run = MagicMock(return_value="tool result")
    tool._arun = None
    return tool


def _make_client(decision: Decision, **extra) -> MagicMock:
    client = MagicMock()
    client.evaluate_policy.return_value = SafetyDecision(
        decision=decision, **extra
    )
    return client


class TestWrap:
    def test_wrap_returns_same_count(self):
        client = _make_client(Decision.ALLOW)
        tools = [_make_tool("t1"), _make_tool("t2"), _make_tool("t3")]
        guarded = CordumToolGuard(client).wrap(tools)
        assert len(guarded) == 3

    def test_wrap_empty_list(self):
        client = _make_client(Decision.ALLOW)
        assert CordumToolGuard(client).wrap([]) == []


class TestAllowPath:
    def test_allow_calls_original(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool()
        original_run = tool._run

        CordumToolGuard(client, policy="test").wrap_one(tool)
        result = tool._run("input")

        assert result == "tool result"
        client.evaluate_policy.assert_called_once()
        original_run.assert_called_once_with("input")


class TestDenyPath:
    def test_deny_returns_blocked_string(self):
        client = _make_client(Decision.DENY, reason="Policy violation")
        tool = _make_tool("dangerous_tool")
        original_run = tool._run

        CordumToolGuard(client).wrap_one(tool)
        result = tool._run("input")

        assert "[BLOCKED]" in result
        assert "dangerous_tool" in result
        assert "Policy violation" in result
        original_run.assert_not_called()


class TestThrottlePath:
    def test_throttle_sleeps_then_calls(self):
        client = _make_client(Decision.THROTTLE, throttle_duration_seconds=0.5)
        tool = _make_tool()
        original_run = tool._run

        from unittest.mock import patch

        CordumToolGuard(client).wrap_one(tool)
        with patch("time.sleep") as mock_sleep:
            tool._run("input")
        mock_sleep.assert_called_once_with(0.5)
        original_run.assert_called_once_with("input")


class TestPolicyLabels:
    def test_labels_include_policy_and_tool(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool("my_tool", "Does something important")

        CordumToolGuard(client, policy="ops", risk_tags=["read"]).wrap_one(tool)
        tool._run()

        call_kwargs = client.evaluate_policy.call_args.kwargs
        assert call_kwargs["labels"]["policy"] == "ops"
        assert call_kwargs["labels"]["tool"] == "my_tool"
        assert call_kwargs["risk_tags"] == ["read"]
        assert call_kwargs["capability"] == "my_tool"
