"""Tests for MockCordumClient."""

import pytest

from cordum_guard import (
    CordumBlockedError,
    Decision,
    MockCordumClient,
    guard,
)


class TestDefaultAllow:
    def test_evaluate_policy_returns_allow_by_default(self):
        mock = MockCordumClient()
        result = mock.evaluate_policy(topic="job.test", capability="test-cap")
        assert result.decision == Decision.ALLOW

    def test_default_decision_can_be_overridden(self):
        mock = MockCordumClient(default_decision=Decision.DENY, default_reason="global deny")
        result = mock.evaluate_policy(capability="anything")
        assert result.decision == Decision.DENY
        assert result.reason == "global deny"


class TestConfiguredResponses:
    def test_deny_by_capability(self):
        mock = MockCordumClient()
        mock.set_policy_response("dangerous", Decision.DENY, reason="too risky")
        result = mock.evaluate_policy(capability="dangerous")
        assert result.decision == Decision.DENY
        assert result.reason == "too risky"

    def test_deny_by_topic(self):
        mock = MockCordumClient()
        mock.set_policy_response("job.restricted", Decision.DENY)
        result = mock.evaluate_policy(topic="job.restricted")
        assert result.decision == Decision.DENY

    def test_capability_takes_precedence_over_topic(self):
        mock = MockCordumClient()
        mock.set_policy_response("job.test", Decision.DENY)
        mock.set_policy_response("safe-cap", Decision.ALLOW)
        result = mock.evaluate_policy(topic="job.test", capability="safe-cap")
        assert result.decision == Decision.ALLOW

    def test_unmatched_falls_back_to_default(self):
        mock = MockCordumClient(default_decision=Decision.ALLOW)
        mock.set_policy_response("other", Decision.DENY)
        result = mock.evaluate_policy(capability="unrelated")
        assert result.decision == Decision.ALLOW


class TestRequireApproval:
    def test_submit_and_wait_succeeds_for_allow(self):
        mock = MockCordumClient(default_decision=Decision.ALLOW)
        job = mock.submit_job("test prompt", topic="job.test")
        assert job.job_id.startswith("mock-")
        status = mock.wait_for_decision(job.job_id)
        assert status.state == "succeeded"

    def test_submit_and_wait_denied(self):
        mock = MockCordumClient()
        mock.set_policy_response("job.blocked", Decision.DENY)
        job = mock.submit_job("test", topic="job.blocked")
        status = mock.wait_for_decision(job.job_id)
        assert status.state == "denied"

    def test_approve_job_changes_state(self):
        mock = MockCordumClient()
        mock.set_policy_response("job.pending", Decision.REQUIRE_APPROVAL)
        job = mock.submit_job("test", topic="job.pending", capability="job.pending")
        # Simulate approval
        mock.approve_job(job.job_id)
        status = mock.get_job(job.job_id)
        assert status.state == "succeeded"


class TestThrottle:
    def test_throttle_duration_returned(self):
        mock = MockCordumClient()
        mock.set_policy_response("rate-limited", Decision.THROTTLE, throttle_seconds=5.0)
        result = mock.evaluate_policy(capability="rate-limited")
        assert result.decision == Decision.THROTTLE
        assert result.throttle_duration_seconds == 5.0

    def test_default_throttle_seconds(self):
        mock = MockCordumClient(
            default_decision=Decision.THROTTLE,
            default_throttle_seconds=1.5,
        )
        result = mock.evaluate_policy(capability="any")
        assert result.decision == Decision.THROTTLE
        assert result.throttle_duration_seconds == 1.5


class TestGuardDecorator:
    def test_allow_passes_through(self):
        mock = MockCordumClient(default_decision=Decision.ALLOW)

        @guard(mock, capability="safe-op")
        def my_func():
            return "ok"

        assert my_func() == "ok"

    def test_deny_raises_blocked_error(self):
        mock = MockCordumClient()
        mock.set_policy_response("dangerous-op", Decision.DENY, reason="not allowed")

        @guard(mock, capability="dangerous-op")
        def risky_func():
            return "should not reach"

        with pytest.raises(CordumBlockedError):
            risky_func()

    def test_throttle_still_executes(self):
        mock = MockCordumClient()
        mock.set_policy_response("slow-op", Decision.THROTTLE, throttle_seconds=0.0)

        @guard(mock, capability="slow-op")
        def slow_func():
            return "done"

        assert slow_func() == "done"


class TestCallLog:
    def test_call_log_records_evaluations(self):
        mock = MockCordumClient()
        mock.evaluate_policy(topic="job.a", capability="cap-a", risk_tags=["write"])
        mock.evaluate_policy(topic="job.b", capability="cap-b", labels={"env": "test"})

        assert len(mock.call_log) == 2
        assert mock.call_log[0].topic == "job.a"
        assert mock.call_log[0].capability == "cap-a"
        assert mock.call_log[0].risk_tags == ["write"]
        assert mock.call_log[1].topic == "job.b"
        assert mock.call_log[1].labels == {"env": "test"}

    def test_call_log_starts_empty(self):
        mock = MockCordumClient()
        assert mock.call_log == []


class TestContextManager:
    def test_works_as_context_manager(self):
        with MockCordumClient() as mock:
            result = mock.evaluate_policy(capability="test")
            assert result.decision == Decision.ALLOW
