"""Tests for cordum_guard.guard — @guard decorator paths."""

import asyncio
import threading
from unittest.mock import MagicMock, patch

import httpx
import pytest
import respx

from cordum_guard.client import CordumClient
from cordum_guard.exceptions import CordumBlockedError, CordumError, CordumTimeoutError
from cordum_guard.guard import guard
from cordum_guard.types import Decision, JobResponse, JobStatus, SafetyDecision


def _make_client(decision: Decision, **extra) -> MagicMock:
    """Create a mock CordumClient that returns a specific decision."""
    client = MagicMock()
    client.evaluate_policy.return_value = SafetyDecision(
        decision=decision, **extra
    )
    return client


def _approved(job_id: str) -> JobStatus:
    return JobStatus(
        job_id=job_id, state="PENDING", approval_status="approved",
        approval_decision="approve", approval_by="alice",
    )


def _rejected() -> CordumBlockedError:
    return CordumBlockedError(SafetyDecision(decision=Decision.DENY, reason="approval rejected"))


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
        client.wait_for_approval.return_value = _approved("j-1")

        @guard(client, policy="finance", risk_tags=["write"])
        def transfer(amount: float) -> str:
            return f"sent {amount}"

        result = transfer(100.0)
        assert result == "sent 100.0"
        client.submit_job.assert_called_once()
        # Waits for the approval, not for the job: no worker runs job.guard.
        client.wait_for_approval.assert_called_once_with("j-1", timeout=300.0)
        client.wait_for_decision.assert_not_called()
        client.cancel_job.assert_called_once_with("j-1")

    def test_approval_rejected_raises_and_does_not_run(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-2")
        client.wait_for_approval.side_effect = _rejected()
        called = False

        @guard(client, timeout=10.0)
        def risky():
            nonlocal called
            called = True

        with pytest.raises(CordumBlockedError, match="approval rejected"):
            risky()
        assert not called
        client.wait_for_approval.assert_called_once_with("j-2", timeout=10.0)
        client.cancel_job.assert_called_once_with("j-2")

    def test_no_verdict_in_time_cancels_the_approval_job(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-t")
        client.wait_for_approval.side_effect = CordumTimeoutError("j-t", 5.0)
        called = False

        @guard(client, timeout=5.0)
        def risky():
            nonlocal called
            called = True

        with pytest.raises(CordumTimeoutError):
            risky()
        assert not called
        # A late approval can no longer be granted for a call that gave up.
        client.cancel_job.assert_called_once_with("j-t")

    def test_cancel_failure_does_not_undo_an_approval(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-c")
        client.wait_for_approval.return_value = _approved("j-c")
        client.cancel_job.side_effect = CordumError("API error 403")

        @guard(client)
        def op() -> str:
            return "ran"

        assert op() == "ran"


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
        waited_on = []

        def wait(job_id, timeout):
            waited_on.append(threading.current_thread())
            return _approved(job_id)

        client.wait_for_approval.side_effect = wait

        @guard(client)
        async def async_transfer() -> str:
            return "transferred"

        assert asyncio.run(async_transfer()) == "transferred"
        # The poll runs off the event loop thread.
        assert waited_on and waited_on[0] is not threading.main_thread()
        client.cancel_job.assert_called_once_with("j-a")

    def test_async_approval_rejected(self):
        client = _make_client(Decision.REQUIRE_APPROVAL)
        client.submit_job.return_value = JobResponse(job_id="j-ar")
        client.wait_for_approval.side_effect = _rejected()
        called = False

        @guard(client)
        async def async_transfer():
            nonlocal called
            called = True

        with pytest.raises(CordumBlockedError):
            asyncio.run(async_transfer())
        assert not called
        client.cancel_job.assert_called_once_with("j-ar")


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
        client.wait_for_approval.side_effect = _rejected()

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
        client.wait_for_approval.return_value = _approved("j-3")

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
        client.wait_for_approval.return_value = _approved("j-4")

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


GATEWAY = "http://localhost:8081"
REQUIRE_HUMAN = {
    "decision": "DECISION_TYPE_REQUIRE_HUMAN",
    "reason": "transfers over 10k need a human",
    "approvalRequired": True,
}


class TestApprovalAgainstGatewayResponses:
    """@guard on a real CordumClient, with GET /api/v1/jobs/{id} bodies shaped
    like the gateway's for an approval job that no worker serves."""

    def _run(self, job_body: dict, ran: list) -> int:
        with respx.mock:
            respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
                return_value=httpx.Response(200, json=REQUIRE_HUMAN)
            )
            respx.post(f"{GATEWAY}/api/v1/jobs").mock(
                return_value=httpx.Response(
                    200, json={"job_id": "j-e2e", "status": "approval_required"}
                )
            )
            respx.get(f"{GATEWAY}/api/v1/jobs/j-e2e").mock(
                return_value=httpx.Response(
                    200, json={"id": "j-e2e", "topic": "job.guard", **job_body}
                )
            )
            cancel = respx.post(f"{GATEWAY}/api/v1/jobs/j-e2e/cancel").mock(
                return_value=httpx.Response(200, json={"id": "j-e2e", "state": "CANCELLED"})
            )
            with CordumClient(GATEWAY, api_key="k") as client:

                @guard(client, policy="finance", timeout=1.0)
                def transfer(amount: float) -> str:
                    ran.append(amount)
                    return "sent"

                try:
                    transfer(50000.0)
                finally:
                    cancels = cancel.call_count
        return cancels

    def test_approved_job_back_in_the_queue_still_runs_once(self):
        # After approval the scheduler cannot route job.guard, re-checks the
        # policy on redelivery and re-queues the job: approval_status reads
        # "pending" again while approval_decision keeps "approve".
        ran: list = []
        cancels = self._run({
            "state": "APPROVAL_REQUIRED", "approval_status": "pending",
            "approval_decision": "approve", "approval_by": "alice",
        }, ran)
        assert ran == [50000.0]
        assert cancels == 1

    def test_rejected_job_raises_without_running(self):
        ran: list = []
        with pytest.raises(CordumBlockedError, match="approval rejected: too large"):
            self._run({
                "state": "DENIED", "approval_status": "rejected",
                "approval_decision": "reject", "approval_by": "bob",
                "approval_reason": "too large",
            }, ran)
        assert ran == []
