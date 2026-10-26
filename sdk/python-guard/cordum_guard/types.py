"""Core types for Cordum Guard SDK."""

from __future__ import annotations

from enum import Enum
from typing import Any, Callable, Optional, Union

from pydantic import BaseModel, Field


class Decision(str, Enum):
    """Safety decision types returned by the Cordum Safety Kernel.

    ``REQUIRE_APPROVAL`` is the kernel's ``DECISION_TYPE_REQUIRE_HUMAN``.
    """

    ALLOW = "allow"
    DENY = "deny"
    REQUIRE_APPROVAL = "require_approval"
    THROTTLE = "throttle"
    ALLOW_WITH_CONSTRAINTS = "allow_with_constraints"


class SafetyDecision(BaseModel):
    """Result of a policy evaluation."""

    decision: Decision
    reason: str = ""
    matched_rules: list[str] = Field(default_factory=list)
    remediations: list[dict[str, Any]] = Field(default_factory=list)
    throttle_duration_seconds: float = 0.0
    rule_id: str = ""
    policy_snapshot: str = ""
    approval_required: bool = False
    # The kernel echoes the request job_id here when approval is required.
    approval_ref: str = ""
    # Raw PolicyConstraints for ALLOW_WITH_CONSTRAINTS; guards do not enforce them.
    constraints: dict[str, Any] = Field(default_factory=dict)


class JobResponse(BaseModel):
    """Response from job submission."""

    job_id: str
    trace_id: str = ""


class JobStatus(BaseModel):
    """Current state of a submitted job."""

    job_id: str
    state: str = ""
    topic: str = ""
    result_ptr: str = ""
    failure_reason: str = ""
    tenant_id: str = ""


class ApprovalEntry(BaseModel):
    """A pending approval from the approvals queue."""

    job_id: str
    topic: str = ""
    tenant_id: str = ""
    risk_tags: list[str] = Field(default_factory=list)
    capability: str = ""
    reason: str = ""
    created_at: Optional[str] = None


# Type alias for the on_error callback parameter.
OnErrorCallback = Callable[[Exception], "SafetyDecision"]

# Union type for the on_error parameter (string literal or callback).
OnErrorMode = Union[str, OnErrorCallback]
