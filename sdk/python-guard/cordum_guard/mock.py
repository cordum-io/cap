"""Mock client for testing without a live Cordum gateway."""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from typing import Any, Optional

from .types import Decision, JobResponse, JobStatus, SafetyDecision


@dataclass
class _PolicyCall:
    """Record of an evaluate_policy call for test assertions."""

    topic: str
    capability: str
    risk_tags: list[str]
    labels: dict[str, str]
    job_id: str


class MockCordumClient:
    """Drop-in replacement for :class:`CordumClient` that returns
    configurable decisions locally — no HTTP, no gateway needed.

    Example::

        mock = MockCordumClient(default_decision=Decision.ALLOW)
        mock.set_policy_response("dangerous-ops", Decision.DENY, reason="blocked by policy")

        decision = mock.evaluate_policy(capability="dangerous-ops")
        assert decision.decision == Decision.DENY
    """

    def __init__(
        self,
        default_decision: Decision = Decision.ALLOW,
        default_reason: str = "",
        default_throttle_seconds: float = 0.0,
    ) -> None:
        self._default_decision = default_decision
        self._default_reason = default_reason
        self._default_throttle_seconds = default_throttle_seconds
        self._responses: dict[str, SafetyDecision] = {}
        self._jobs: dict[str, _MockJob] = {}
        self.call_log: list[_PolicyCall] = []

    # ------------------------------------------------------------------
    # Configuration
    # ------------------------------------------------------------------

    def set_policy_response(
        self,
        topic_or_capability: str,
        decision: Decision,
        reason: str = "",
        throttle_seconds: float = 0.0,
    ) -> None:
        """Configure the decision returned for a given topic or capability.

        The key is matched against both the ``topic`` and ``capability``
        arguments of :meth:`evaluate_policy`.
        """
        self._responses[topic_or_capability] = SafetyDecision(
            decision=decision,
            reason=reason,
            throttle_duration_seconds=throttle_seconds,
        )

    # ------------------------------------------------------------------
    # CordumClient interface
    # ------------------------------------------------------------------

    def evaluate_policy(
        self,
        *,
        topic: str = "job.guard",
        capability: str = "",
        risk_tags: Optional[list[str]] = None,
        labels: Optional[dict[str, str]] = None,
        job_id: str = "",
    ) -> SafetyDecision:
        self.call_log.append(
            _PolicyCall(
                topic=topic,
                capability=capability,
                risk_tags=risk_tags or [],
                labels=labels or {},
                job_id=job_id,
            )
        )

        # Match on capability first, then topic.
        if capability and capability in self._responses:
            return self._responses[capability]
        if topic in self._responses:
            return self._responses[topic]

        return SafetyDecision(
            decision=self._default_decision,
            reason=self._default_reason,
            throttle_duration_seconds=self._default_throttle_seconds,
        )

    def submit_job(
        self,
        prompt: str,
        *,
        topic: str = "job.default",
        capability: str = "",
        risk_tags: Optional[list[str]] = None,
        labels: Optional[dict[str, str]] = None,
        priority: str = "",
    ) -> JobResponse:
        job_id = f"mock-{uuid.uuid4().hex[:12]}"
        # Determine state from the matching policy response.
        decision = self._resolve_decision(topic, capability)
        state = "denied" if decision == Decision.DENY else "succeeded"
        self._jobs[job_id] = _MockJob(job_id=job_id, topic=topic, state=state)
        return JobResponse(job_id=job_id, trace_id=f"mock-trace-{job_id}")

    def get_job(self, job_id: str) -> JobStatus:
        job = self._jobs.get(job_id)
        if job is None:
            return JobStatus(job_id=job_id, state="not_found")
        return JobStatus(job_id=job.job_id, state=job.state, topic=job.topic)

    def wait_for_decision(
        self,
        job_id: str,
        timeout: float = 300.0,
        poll_interval: float = 2.0,
    ) -> JobStatus:
        return self.get_job(job_id)

    def cancel_job(self, job_id: str) -> None:
        job = self._jobs.get(job_id)
        if job is not None:
            job.state = "cancelled"

    def list_approvals(self) -> list[Any]:
        return []

    def approve_job(self, job_id: str) -> None:
        job = self._jobs.get(job_id)
        if job is not None:
            job.state = "succeeded"

    def reject_job(self, job_id: str) -> None:
        job = self._jobs.get(job_id)
        if job is not None:
            job.state = "denied"

    def close(self) -> None:
        pass

    def __enter__(self) -> MockCordumClient:
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

    # ------------------------------------------------------------------
    # Internal
    # ------------------------------------------------------------------

    def _resolve_decision(self, topic: str, capability: str) -> Decision:
        if capability and capability in self._responses:
            return self._responses[capability].decision
        if topic in self._responses:
            return self._responses[topic].decision
        return self._default_decision


@dataclass
class _MockJob:
    job_id: str
    topic: str = ""
    state: str = "succeeded"
