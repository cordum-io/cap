"""HTTP client for the Cordum API Gateway."""

from __future__ import annotations

import logging
import time
from typing import Any, Callable, Optional, Union

import httpx

from .exceptions import (
    CordumAuthError,
    CordumConnectionError,
    CordumError,
    CordumTimeoutError,
)
from .types import ApprovalEntry, Decision, JobResponse, JobStatus, SafetyDecision

_TERMINAL_STATES = frozenset({
    "succeeded", "failed", "cancelled", "denied",
})

_logger = logging.getLogger("cordum_guard")

# Type for the on_error callback.
OnErrorCallback = Callable[
    [Exception],
    SafetyDecision,
]


def _base_url(url: str) -> str:
    return url.rstrip("/")


class CordumClient:
    """Synchronous client for the Cordum API Gateway.

    Example::

        client = CordumClient("http://localhost:8081", api_key="my-key")
        decision = client.evaluate_policy(
            topic="job.finance.transfer",
            capability="bank_transfer",
            risk_tags=["write", "financial"],
        )
    """

    def __init__(
        self,
        gateway_url: str,
        api_key: str,
        tenant_id: str = "default",
        timeout: float = 30.0,
        on_error: Union[str, OnErrorCallback] = "closed",
    ) -> None:
        self.base_url = _base_url(gateway_url)
        self._http = httpx.Client(
            base_url=self.base_url,
            headers={
                "X-API-Key": api_key,
                "X-Tenant-ID": tenant_id,
                "Content-Type": "application/json",
            },
            timeout=timeout,
        )
        self._on_error = on_error

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> CordumClient:
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

    # ------------------------------------------------------------------
    # Jobs
    # ------------------------------------------------------------------

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
        """Submit a job to the Cordum control plane."""
        body: dict[str, Any] = {"prompt": prompt, "topic": topic}
        if capability:
            body["capability"] = capability
        if risk_tags:
            body["risk_tags"] = risk_tags
        if labels:
            body["labels"] = labels
        if priority:
            body["priority"] = priority
        resp = self._request("POST", "/api/v1/jobs", json=body)
        return JobResponse(**resp)

    def get_job(self, job_id: str) -> JobStatus:
        """Get the current status of a job."""
        resp = self._request("GET", f"/api/v1/jobs/{job_id}")
        return JobStatus(
            job_id=resp.get("job_id", job_id),
            state=resp.get("state", ""),
            topic=resp.get("topic", ""),
            result_ptr=resp.get("result_ptr", ""),
            failure_reason=resp.get("failure_reason", ""),
            tenant_id=resp.get("tenant_id", ""),
        )

    def cancel_job(self, job_id: str) -> None:
        """Cancel a running job."""
        self._request("POST", f"/api/v1/jobs/{job_id}/cancel")

    # ------------------------------------------------------------------
    # Policy evaluation
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
        """Evaluate a policy against the Safety Kernel.

        Returns a :class:`SafetyDecision` with the kernel's verdict.

        Connection/timeout errors are handled according to the ``on_error``
        setting. Auth errors and explicit DENY responses are never affected.
        """
        body: dict[str, Any] = {"topic": topic}
        meta: dict[str, Any] = {}
        if capability:
            meta["capability"] = capability
        if risk_tags:
            meta["risk_tags"] = risk_tags
        if labels:
            meta["labels"] = labels
        if meta:
            body["meta"] = meta
        if job_id:
            body["job_id"] = job_id
        try:
            resp = self._request("POST", "/api/v1/policy/evaluate", json=body)
        except (CordumConnectionError, CordumTimeoutError) as exc:
            return self._handle_error(exc)
        return _parse_safety_decision(resp)

    def _handle_error(self, exc: Exception) -> SafetyDecision:
        """Apply the on_error policy to a connection/timeout error."""
        if self._on_error == "closed":
            raise exc
        if self._on_error == "open":
            _logger.warning(
                "fail-open: gateway unreachable, allowing operation",
                exc_info=exc,
            )
            return SafetyDecision(
                decision=Decision.ALLOW,
                reason="fail-open: gateway unreachable",
            )
        if callable(self._on_error):
            return self._on_error(exc)
        raise exc

    # ------------------------------------------------------------------
    # Approvals
    # ------------------------------------------------------------------

    def list_approvals(self) -> list[ApprovalEntry]:
        """List pending approvals."""
        resp = self._request("GET", "/api/v1/approvals")
        items = resp if isinstance(resp, list) else resp.get("items", [])
        return [ApprovalEntry(**item) for item in items]

    def approve_job(self, job_id: str) -> None:
        """Approve a pending job."""
        self._request("POST", f"/api/v1/approvals/{job_id}/approve")

    def reject_job(self, job_id: str) -> None:
        """Reject a pending job."""
        self._request("POST", f"/api/v1/approvals/{job_id}/reject")

    # ------------------------------------------------------------------
    # Polling helpers
    # ------------------------------------------------------------------

    def wait_for_decision(
        self,
        job_id: str,
        timeout: float = 300.0,
        poll_interval: float = 2.0,
    ) -> JobStatus:
        """Poll a job until it reaches a terminal state.

        Raises :class:`CordumTimeoutError` if the timeout is exceeded.
        """
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            status = self.get_job(job_id)
            if status.state.lower() in _TERMINAL_STATES:
                return status
            time.sleep(min(poll_interval, deadline - time.monotonic()))
        raise CordumTimeoutError(job_id, timeout)

    # ------------------------------------------------------------------
    # Internal HTTP plumbing
    # ------------------------------------------------------------------

    def _request(
        self,
        method: str,
        path: str,
        **kwargs: Any,
    ) -> Any:
        try:
            resp = self._http.request(method, path, **kwargs)
        except httpx.ConnectError as exc:
            raise CordumConnectionError(
                f"Cannot connect to Cordum gateway at {self.base_url}"
            ) from exc
        except httpx.TimeoutException as exc:
            raise CordumConnectionError(
                f"Request to {self.base_url}{path} timed out"
            ) from exc

        if resp.status_code in (401, 403):
            raise CordumAuthError(
                f"Authentication failed ({resp.status_code}): {resp.text}"
            )
        if resp.status_code >= 400:
            raise CordumError(
                f"API error {resp.status_code} on {method} {path}: {resp.text}"
            )

        if not resp.content:
            return {}
        return resp.json()


def _parse_safety_decision(data: dict[str, Any]) -> SafetyDecision:
    """Parse a policy evaluation response into a SafetyDecision."""
    decision_str = data.get("decision", "allow")
    # The gateway may return upper-case or snake_case variants.
    decision_str = decision_str.lower().strip()
    try:
        decision = Decision(decision_str)
    except ValueError:
        decision = Decision.ALLOW

    return SafetyDecision(
        decision=decision,
        reason=data.get("reason", ""),
        matched_rules=data.get("matched_rules", []),
        remediations=data.get("remediations", []),
        throttle_duration_seconds=float(
            data.get("throttle_duration_seconds", 0)
        ),
    )
