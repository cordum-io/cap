"""Tests for cordum_guard.langchain — LangChain tool wrapping.

These tests mock langchain_core to avoid requiring it as a dependency.
"""

import asyncio
import inspect
import sys
import types
from unittest.mock import AsyncMock, MagicMock

import pytest

from cordum_guard.exceptions import CordumApprovalRequiredError
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


class TestRequireApprovalPath:
    def test_require_approval_raises_and_tool_not_run(self):
        client = _make_client(
            Decision.REQUIRE_APPROVAL,
            reason="Transfers over $10,000 require human approval.",
            rule_id="financial-approval",
            policy_snapshot="snap-1",
            approval_required=True,
            approval_ref="job-42",
        )
        tool = _make_tool("transfer_funds")
        original_run = tool._run

        CordumToolGuard(client).wrap_one(tool)
        with pytest.raises(CordumApprovalRequiredError) as exc_info:
            tool._run(amount=15000, to_account="alice")

        original_run.assert_not_called()
        err = exc_info.value
        assert err.approval_ref == "job-42"
        assert err.job_id == "job-42"
        assert err.rule_id == "financial-approval"
        assert err.policy_snapshot == "snap-1"
        assert err.reason == "Transfers over $10,000 require human approval."
        assert err.decision.decision == Decision.REQUIRE_APPROVAL

    def test_approval_required_flag_blocks_even_when_decision_is_allow(self):
        client = _make_client(Decision.ALLOW, approval_required=True)
        tool = _make_tool()
        original_run = tool._run

        CordumToolGuard(client).wrap_one(tool)
        with pytest.raises(CordumApprovalRequiredError):
            tool._run("input")
        original_run.assert_not_called()

    def test_async_require_approval_raises_and_tool_not_awaited(self):
        client = _make_client(Decision.REQUIRE_APPROVAL, reason="needs sign-off")
        tool = _make_tool()
        tool._arun = AsyncMock(return_value="async result")
        original_arun = tool._arun

        CordumToolGuard(client).wrap_one(tool)
        with pytest.raises(CordumApprovalRequiredError):
            asyncio.run(tool._arun(amount=1))
        original_arun.assert_not_awaited()


class TestFailClosedDecisions:
    def test_unknown_decision_blocks(self):
        client = MagicMock()
        client.evaluate_policy.return_value = SafetyDecision.model_construct(
            decision="quarantine", reason="unrecognised verdict"
        )
        tool = _make_tool("risky_tool")
        original_run = tool._run

        CordumToolGuard(client).wrap_one(tool)
        result = tool._run("input")

        assert result == "[BLOCKED] risky_tool: unrecognised verdict"
        original_run.assert_not_called()

    def test_allow_with_constraints_runs(self):
        client = _make_client(
            Decision.ALLOW_WITH_CONSTRAINTS, constraints={"redactionLevel": "high"}
        )
        tool = _make_tool()

        CordumToolGuard(client).wrap_one(tool)
        assert tool._run("input") == "tool result"


class TestArgumentLabels:
    def test_scalar_arguments_become_prefixed_labels(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool("transfer_funds")

        CordumToolGuard(client, policy="financial_ops").wrap_one(tool)
        tool._run(amount=15000, to_account="alice", dry_run=False, memo=None, meta={"a": 1})

        labels = client.evaluate_policy.call_args.kwargs["labels"]
        assert labels["arg.amount"] == "15000"
        assert labels["arg.to_account"] == "alice"
        assert labels["arg.dry_run"] == "false"
        assert "arg.memo" not in labels  # None carries no value
        assert "arg.meta" not in labels  # non-scalars are never flattened
        assert labels["policy"] == "financial_ops"
        assert labels["tool"] == "transfer_funds"

    def test_nan_amount_is_sent_as_inf(self):
        # json.loads accepts NaN, and NaN never exceeds a label_threshold.
        client = _make_client(Decision.ALLOW)
        tool = _make_tool()

        CordumToolGuard(client).wrap_one(tool)
        tool._run(amount=float("nan"))

        assert client.evaluate_policy.call_args.kwargs["labels"]["arg.amount"] == "inf"

    def test_argument_labels_are_bounded(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool()

        CordumToolGuard(client).wrap_one(tool)
        many = {f"p{i}": i for i in range(40)}
        tool._run(note="x" * 5000, **many, **{"bad-name": 1, "y" * 65: 2})

        labels = client.evaluate_policy.call_args.kwargs["labels"]
        arg_keys = [k for k in labels if k.startswith("arg.")]
        assert len(arg_keys) == 16
        assert labels["arg.note"] == "x" * 256
        assert "arg.bad-name" not in labels
        assert all(len(k) <= len("arg.") + 64 for k in arg_keys)

    def test_secret_arguments_are_never_sent(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool()

        CordumToolGuard(client).wrap_one(tool)
        tool._run(
            password="hunter2",
            api_key="k-1",
            accessToken="t-1",
            client_secret="s-1",
            auth="a-1",
            session_id="sid-1",
            private_key="pk-1",
            amount=5,
        )

        labels = client.evaluate_policy.call_args.kwargs["labels"]
        assert {k for k in labels if k.startswith("arg.")} == {"arg.amount"}
        for value in ("hunter2", "k-1", "t-1", "s-1", "a-1", "sid-1", "pk-1"):
            assert value not in labels.values()

    def test_async_path_sends_same_labels(self):
        client = _make_client(Decision.ALLOW)
        tool = _make_tool("my_tool", "Does something")
        tool._arun = AsyncMock(return_value="async result")

        CordumToolGuard(client, policy="ops").wrap_one(tool)
        assert asyncio.run(tool._arun(amount=3)) == "async result"

        labels = client.evaluate_policy.call_args.kwargs["labels"]
        assert labels["arg.amount"] == "3"
        assert labels["description"] == "Does something"


class TestSignaturePreserved:
    def test_wrapped_run_keeps_signature_for_langchain_injection(self):
        # LangChain only injects `config` / `run_manager` when the tool's
        # `_run` signature declares them, and StructuredTool._run requires
        # `config`; a bare (*args, **kwargs) wrapper breaks every @tool.
        def real_run(*args, config, run_manager=None, **kwargs):
            return "ok"

        tool = _make_tool()
        tool._run = real_run

        CordumToolGuard(_make_client(Decision.ALLOW)).wrap_one(tool)
        assert inspect.signature(tool._run) == inspect.signature(real_run)
        assert tool._run(config={}, run_manager=None, query="q") == "ok"
