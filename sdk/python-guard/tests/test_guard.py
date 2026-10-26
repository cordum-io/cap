"""Tests for cordum_guard.guard — @guard decorator paths."""

import asyncio
from unittest.mock import MagicMock, patch

import pytest

from cordum_guard.exceptions import CordumBlockedError
from cordum_guard.guard import guard
from cordum_guard.types import Decision, JobResponse, JobStatus, SafetyDecision


def _make_client(decision: Decision, **extra) -> MagicMock:
    """Create a mock CordumClient that returns a specific decision."""
    client = MagicMock()
    client.evaluate_policy.return_value = SafetyDecision(
        decision=decision, **extra
    )
    return client


class TestSyncAllow:
    def test_allow_executes_function(self):
        client = _make_client(Decision.ALLOW)

        @guard(client, policy="test")
        def do_work(x: int) -> int:
            return x * 2

        assert do_work(5) == 10
        client.evaluate_policy.assert_called_once()

    def test_allow_preserves_function_name(self):
        client = _make_client(Decision.ALLOW)

        @guard(client)
        def my_func():
            pass

        assert my_func.__name__ == "my_func"


class TestSyncDeny:
    def test_deny_raises_blocked_error(self):
        client = _make_client(Decision.DENY, reason="Too risky")

        @guard(client, policy="strict")
        def dangerous():
            return "should not reach"

        with pytest.raises(CordumBlockedError) as exc_info:
            dangerous()
        assert "Too risky" in str(exc_info.value)

    def test_deny_does_not_call_function(self):
        client = _make_client(Decision.DENY, reason="blocked")
        called = False

        @guard(client)
        def side_effect():
            nonlocal called
            called = True

        with pytest.raises(CordumBlockedError):
            side_effect()
        assert not called


class TestSyncThrottle:
    def test_throttle_sleeps_then_executes(self):
        client = _make_client(Decision.THROTTLE, throttle_duration_seconds=0.5)

        @guard(client)
        def slow_op() -> str:
            return "done"

        with patch("cordum_guard.guard.time.sleep") as mock_sleep:
            result = slow_op()
        assert result == "done"
        mock_sleep.assert_called_once_with(0.5)


class TestSyncRequireApproval:
    def test_approval_granted_executes(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-1")
        client.wait_for_decision.return_value = JobStatus(
            job_id="j-1", state="succeeded"
        )

        @guard(client, policy="finance", risk_tags=["write"])
        def transfer(amount: float) -> str:
            return f"sent {amount}"

        result = transfer(100.0)
        assert result == "sent 100.0"
        client.submit_job.assert_called_once()
        client.wait_for_decision.assert_called_once_with("j-1", timeout=300.0)

    def test_approval_denied_raises(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-2")
        client.wait_for_decision.return_value = JobStatus(
            job_id="j-2", state="denied"
        )

        @guard(client, timeout=10.0)
        def risky():
            return "nope"

        with pytest.raises(CordumBlockedError):
            risky()


class TestSyncCustomCapability:
    def test_custom_capability_name(self):
        client = _make_client(Decision.ALLOW)

        @guard(client, capability="custom_cap", topic="job.custom")
        def generic_fn():
            return True

        generic_fn()
        call_kwargs = client.evaluate_policy.call_args.kwargs
        assert call_kwargs["capability"] == "custom_cap"
        assert call_kwargs["topic"] == "job.custom"


class TestSyncRiskTags:
    def test_risk_tags_passed(self):
        client = _make_client(Decision.ALLOW)

        @guard(client, risk_tags=["write", "financial"])
        def tagged_fn():
            pass

        tagged_fn()
        call_kwargs = client.evaluate_policy.call_args.kwargs
        assert call_kwargs["risk_tags"] == ["write", "financial"]


class TestAsyncAllow:
    def test_async_allow(self):
        client = _make_client(Decision.ALLOW)

        @guard(client)
        async def async_work(x: int) -> int:
            return x + 1

        result = asyncio.get_event_loop().run_until_complete(async_work(10))
        assert result == 11

    def test_async_preserves_name(self):
        client = _make_client(Decision.ALLOW)

        @guard(client)
        async def my_async():
            pass

        assert my_async.__name__ == "my_async"


class TestAsyncDeny:
    def test_async_deny_raises(self):
        client = _make_client(Decision.DENY, reason="blocked")

        @guard(client)
        async def async_risky():
            return "nope"

        with pytest.raises(CordumBlockedError):
            asyncio.get_event_loop().run_until_complete(async_risky())


class TestAsyncThrottle:
    def test_async_throttle(self):
        client = _make_client(Decision.THROTTLE, throttle_duration_seconds=0.01)

        @guard(client)
        async def throttled() -> str:
            return "ok"

        result = asyncio.get_event_loop().run_until_complete(throttled())
        assert result == "ok"


class TestAsyncRequireApproval:
    def test_async_approval_granted(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-a")
        client.wait_for_decision.return_value = JobStatus(
            job_id="j-a", state="succeeded"
        )

        @guard(client)
        async def async_transfer() -> str:
            return "transferred"

        result = asyncio.get_event_loop().run_until_complete(async_transfer())
        assert result == "transferred"


class TestDecisionMapping:
    def test_allow_with_constraints_executes(self):
        client = _make_client(Decision.ALLOW_WITH_CONSTRAINTS)

        @guard(client)
        def op() -> str:
            return "ran"

        assert op() == "ran"

    def test_unknown_decision_raises_and_does_not_run(self):
        client = MagicMock()
        client.evaluate_policy.return_value = SafetyDecision.model_construct(
            decision="quarantine", reason="unrecognised verdict"
        )
        called = False

        @guard(client)
        def op():
            nonlocal called
            called = True

        with pytest.raises(CordumBlockedError):
            op()
        assert not called

    def test_approval_required_flag_routes_to_approval(self):
        client = _make_client(Decision.ALLOW, approval_required=True)
        client.submit_job.return_value = JobResponse(job_id="j-f")
        client.wait_for_decision.return_value = JobStatus(job_id="j-f", state="denied")

        @guard(client)
        def op():
            return "nope"

        with pytest.raises(CordumBlockedError):
            op()
        client.submit_job.assert_called_once()

    def test_async_unknown_decision_raises(self):
        client = MagicMock()
        client.evaluate_policy.return_value = SafetyDecision.model_construct(
            decision="redact"
        )

        @guard(client)
        async def op():
            return "nope"

        with pytest.raises(CordumBlockedError):
            asyncio.run(op())


class TestArgumentLabels:
    def test_evaluate_receives_argument_labels(self):
        client = _make_client(Decision.ALLOW)

        @guard(client, policy="financial_ops")
        def transfer(amount: float, to_account: str, password: str = "") -> str:
            return "sent"

        transfer(15000.0, to_account="bob", password="hunter2")

        labels = client.evaluate_policy.call_args.kwargs["labels"]
        assert labels["arg.amount"] == "15000.0"  # positional, named by signature
        assert labels["arg.to_account"] == "bob"
        assert "arg.password" not in labels
        assert labels["policy"] == "financial_ops"
        assert labels["function"].endswith("transfer")

    def test_approval_job_carries_same_labels_and_no_secrets(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-3")
        client.wait_for_decision.return_value = JobStatus(job_id="j-3", state="succeeded")

        @guard(client)
        def transfer(amount: float, api_key: str) -> str:
            return "sent"

        assert transfer(15000.0, api_key="sk-live-123") == "sent"

        evaluated = client.evaluate_policy.call_args.kwargs["labels"]
        submitted = client.submit_job.call_args.kwargs
        assert submitted["labels"] == evaluated
        assert submitted["labels"]["arg.amount"] == "15000.0"
        assert "sk-live-123" not in submitted["prompt"]
        assert "amount=15000.0" in submitted["prompt"]

    def test_include_args_false_keeps_arguments_out_of_labels_and_prompt(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-4")
        client.wait_for_decision.return_value = JobStatus(job_id="j-4", state="succeeded")

        @guard(client, policy="support", include_args=False)
        def send_email(to: str, body: str) -> str:
            return "sent"

        assert send_email("bob@example.com", body="private medical details") == "sent"

        evaluated = client.evaluate_policy.call_args.kwargs["labels"]
        submitted = client.submit_job.call_args.kwargs
        assert not [k for k in evaluated if k.startswith("arg.")]
        assert evaluated["policy"] == "support"
        assert submitted["labels"] == evaluated
        assert "private medical details" not in submitted["prompt"]
        assert "bob@example.com" not in submitted["prompt"]
        assert submitted["prompt"] == "Approval required: send_email(...)"
