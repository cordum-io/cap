"""Exception types for Cordum Guard SDK."""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .types import SafetyDecision


class CordumError(Exception):
    """Base exception for all Cordum Guard errors."""


class CordumBlockedError(CordumError):
    """Raised when the Safety Kernel denies an action, or returns a verdict
    the guard cannot honor (the guard fails closed)."""

    _summary = "Action blocked by safety policy"

    def __init__(self, decision: SafetyDecision) -> None:
        self.decision = decision
        super().__init__(f"{self._summary}: {decision.reason}")


class CordumApprovalRequiredError(CordumBlockedError):
    """Raised when policy requires human approval. The action did NOT run.

    Subclasses :class:`CordumBlockedError`, so existing ``except
    CordumBlockedError`` handlers keep failing closed.

    Attributes:
        decision: The full :class:`SafetyDecision`.
        approval_ref: Approval reference from the gateway. Cordum keys
            approvals by job ID, so this equals ``job_id``. It is empty
            unless the evaluation was bound to a job: ``/policy/evaluate``
            only echoes the ``job_id`` it was given, and the tool guards
            evaluate without one.
        job_id: Job to approve or poll (``client.approve_job`` /
            ``client.wait_for_approval``) when ``approval_ref`` is set.
        reason: Why approval is required.
        rule_id: The policy rule that matched.
        policy_snapshot: Policy version that produced the verdict.
    """

    _summary = "Human approval required by safety policy"

    def __init__(self, decision: SafetyDecision) -> None:
        super().__init__(decision)
        self.approval_ref = decision.approval_ref
        self.job_id = decision.approval_ref
        self.reason = decision.reason
        self.rule_id = decision.rule_id
        self.policy_snapshot = decision.policy_snapshot


class CordumTimeoutError(CordumError):
    """Raised when waiting for an approval times out. The action did NOT run."""

    def __init__(self, job_id: str, timeout: float) -> None:
        self.job_id = job_id
        self.timeout = timeout
        super().__init__(
            f"Approval for job {job_id} timed out after {timeout}s"
        )


class CordumConnectionError(CordumError):
    """Raised when the gateway is unreachable."""


class CordumAuthError(CordumError):
    """Raised on authentication/authorization failures (401/403)."""
