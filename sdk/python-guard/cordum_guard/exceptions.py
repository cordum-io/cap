"""Exception types for Cordum Guard SDK."""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .types import SafetyDecision


class CordumError(Exception):
    """Base exception for all Cordum Guard errors."""


class CordumBlockedError(CordumError):
    """Raised when the Safety Kernel denies an action."""

    def __init__(self, decision: SafetyDecision) -> None:
        self.decision = decision
        super().__init__(
            f"Action blocked by safety policy: {decision.reason}"
        )


class CordumTimeoutError(CordumError):
    """Raised when waiting for an approval times out."""

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
