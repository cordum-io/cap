"""Tests for cordum_guard.types — Pydantic model validation."""

import pytest
from pydantic import ValidationError

from cordum_guard.types import (
    ApprovalEntry,
    Decision,
    JobResponse,
    JobStatus,
    SafetyDecision,
)


class TestDecision:
    def test_enum_values(self):
        assert Decision.ALLOW == "allow"
        assert Decision.DENY == "deny"
        assert Decision.REQUIRE_APPROVAL == "require_approval"
        assert Decision.THROTTLE == "throttle"

    def test_from_string(self):
        assert Decision("allow") is Decision.ALLOW
        assert Decision("deny") is Decision.DENY

    def test_invalid_value(self):
        with pytest.raises(ValueError):
            Decision("invalid")


class TestSafetyDecision:
    def test_minimal(self):
        d = SafetyDecision(decision=Decision.ALLOW)
        assert d.decision == Decision.ALLOW
        assert d.reason == ""
        assert d.matched_rules == []
        assert d.throttle_duration_seconds == 0.0

    def test_full(self):
        d = SafetyDecision(
            decision=Decision.DENY,
            reason="Policy forbids this",
            matched_rules=["rule-1", "rule-2"],
            remediations=[{"action": "contact admin"}],
            throttle_duration_seconds=5.0,
        )
        assert d.reason == "Policy forbids this"
        assert len(d.matched_rules) == 2
        assert d.throttle_duration_seconds == 5.0

    def test_missing_decision_raises(self):
        with pytest.raises(ValidationError):
            SafetyDecision()  # type: ignore[call-arg]


class TestJobResponse:
    def test_minimal(self):
        r = JobResponse(job_id="j-123")
        assert r.job_id == "j-123"
        assert r.trace_id == ""

    def test_full(self):
        r = JobResponse(job_id="j-1", trace_id="t-1")
        assert r.trace_id == "t-1"

    def test_missing_job_id(self):
        with pytest.raises(ValidationError):
            JobResponse()  # type: ignore[call-arg]


class TestJobStatus:
    def test_defaults(self):
        s = JobStatus(job_id="j-1")
        assert s.state == ""
        assert s.topic == ""
        assert s.result_ptr == ""

    def test_full(self):
        s = JobStatus(
            job_id="j-1",
            state="succeeded",
            topic="job.finance",
            result_ptr="ptr-abc",
            failure_reason="",
            tenant_id="t-1",
        )
        assert s.state == "succeeded"
        assert s.tenant_id == "t-1"


class TestApprovalEntry:
    def test_minimal(self):
        a = ApprovalEntry(job_id="j-1")
        assert a.risk_tags == []
        assert a.capability == ""

    def test_full(self):
        a = ApprovalEntry(
            job_id="j-1",
            topic="job.ops",
            tenant_id="default",
            risk_tags=["write"],
            capability="bank_transfer",
            reason="high risk",
            created_at="2026-01-01T00:00:00Z",
        )
        assert a.risk_tags == ["write"]
        assert a.created_at == "2026-01-01T00:00:00Z"
