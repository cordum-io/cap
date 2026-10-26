"""HTTP client for the Cordum API Gateway."""

from __future__ import annotations

import logging
import os
import ssl
import time
from typing import Any, Optional, Union

import httpx

from .cache import PolicyCache, make_cache_key
from .exceptions import (
    CordumAuthError,
    CordumBlockedError,
    CordumConnectionError,
    CordumError,
    CordumTimeoutError,
)
from .types import (
    ApprovalEntry,
    Decision,
    JobResponse,
    JobStatus,
    OnErrorMode,
    SafetyDecision,
)

_TERMINAL_STATES = frozenset({
    "succeeded", "failed", "cancelled", "denied",
})
# Job states that end an approval without letting the action run.
_FAILED_STATES = frozenset({"failed", "cancelled", "denied", "timeout", "quarantined"})
_APPROVAL_OUTCOMES = {
    "approve": "approved",
    "reject": "rejected",
    "expire": "expired",
    "invalidate": "invalidated",
}

_logger = logging.getLogger("cordum_guard")


def _base_url(url: str) -> str:
    return url.rstrip("/")


def _tls_verify(
    verify: Union[bool, ssl.SSLContext, None], ca_cert: Optional[str]
) -> Union[bool, ssl.SSLContext]:
    """httpx ``verify`` value for the gateway connection.

    Only an explicit ``verify=False`` turns verification off. ``True`` (or
    None) verifies against ``ca_cert``, else ``$CORDUM_CA_CERT``, else
    httpx's default CAs. Anything else (an ``ssl.SSLContext``) is passed as is.
    """
    if verify is False:
        _logger.warning(
            "TLS certificate verification is disabled for the Cordum gateway "
            "(verify=False); use this only against a local test stack"
        )
        return False
    if verify is not True and verify is not None:
        return verify
    ca_cert = ca_cert or os.environ.get("CORDUM_CA_CERT")
    if not ca_cert:
        return True
    return ssl.create_default_context(cafile=ca_cert)


class CordumClient:
    """Synchronous client for the Cordum API Gateway.

    Example::

        client = CordumClient("http://localhost:8081", api_key="my-key")
        decision = client.evaluate_policy(
            topic="job.finance.transfer",
            capability="bank_transfer",
            risk_tags=["write", "financial"],
        )

    TLS: ``ca_cert`` (default ``$CORDUM_CA_CERT``) is a PEM bundle the client
    trusts for the gateway's certificate instead of the default CAs.
    ``verify`` is passed to httpx: ``True`` verifies, an ``ssl.SSLContext`` is
    used as is, and ``False`` disables verification and logs a warning.
    """

    def __init__(
        self,
        gateway_url: str,
        api_key: str,
        tenant_id: str = "default",
        timeout: float = 30.0,
        cache_ttl: float = 0,
        cache_max_size: int = 1000,
        on_error: OnErrorMode = "closed",
        production_profile: bool = False,
        ca_cert: Optional[str] = None,
        verify: Union[bool, ssl.SSLContext] = True,
    ) -> None:
        if production_profile and on_error != "closed":
            raise ValueError(
                "CAP-PRODUCTION requires the guard to fail closed: on_error "
                f'must be "closed" (got {on_error!r})'
            )
        self._production_profile = production_profile
        self.base_url = _base_url(gateway_url)
        self._http = httpx.Client(
            base_url=self.base_url,
            headers={
                "X-API-Key": api_key,
                "X-Tenant-ID": tenant_id,
                "Content-Type": "application/json",
            },
            timeout=timeout,
            verify=_tls_verify(verify, ca_cert),
        )
        self._cache: PolicyCache | None = None
        if cache_ttl > 0:
            self._cache = PolicyCache(max_size=cache_max_size, ttl_seconds=cache_ttl)
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
            approval_status=resp.get("approval_status") or "",
            approval_decision=resp.get("approval_decision") or "",
            approval_by=resp.get("approval_by") or "",
            approval_reason=resp.get("approval_reason") or "",
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
        cache: bool = True,
    ) -> SafetyDecision:
        """Evaluate a policy against the Safety Kernel.

        Returns a :class:`SafetyDecision` with the kernel's verdict.

        Args:
            cache: When True (default) and caching is enabled, return a
                cached decision if available. Set to False to bypass.

        Connection/timeout errors are handled according to the ``on_error``
        setting. Auth errors and explicit DENY responses are never affected.
        """
        production = getattr(self, "_production_profile", False)
        cache_key = make_cache_key(topic, capability, risk_tags, labels)

        if not production and cache and self._cache is not None:
            cached = self._cache.get(cache_key)
            if cached is not None:
                return cached

        body: dict[str, Any] = {"topic": topic}
        meta: dict[str, Any] = {}
        if capability:
            meta["capability"] = capability
        if risk_tags:
            meta["risk_tags"] = risk_tags
        if labels:
            # The Safety Kernel matches rules on top-level labels only;
            # meta.labels is kept for older consumers.
            body["labels"] = labels
            meta["labels"] = labels
        if meta:
            body["meta"] = meta
        if job_id:
            body["job_id"] = job_id
        try:
            resp = self._request("POST", "/api/v1/policy/evaluate", json=body)
        except (CordumConnectionError, CordumTimeoutError) as exc:
            return self._handle_error(exc)
        decision = _parse_safety_decision(resp, production=production)

        # CAP-PRODUCTION cannot safely cache any verdict without a signed lease
        # bound to the complete input and active policy snapshot.
        if not production and self._cache is not None:
            self._cache.put(cache_key, decision)

        return decision

    def clear_cache(self) -> None:
        """Purge all cached policy decisions."""
        if self._cache is not None:
            self._cache.clear()

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

        Raises :class:`CordumTimeoutError` if the timeout is exceeded. To wait
        for a human approval rather than for the job to finish, use
        :meth:`wait_for_approval`.
        """
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            status = self.get_job(job_id)
            if status.state.lower() in _TERMINAL_STATES:
                return status
            time.sleep(min(poll_interval, deadline - time.monotonic()))
        raise CordumTimeoutError(job_id, timeout)

    def wait_for_approval(
        self,
        job_id: str,
        timeout: float = 300.0,
        poll_interval: float = 2.0,
    ) -> JobStatus:
        """Poll a job until a human resolves its approval.

        Returns the :class:`JobStatus` once the job is approved. Raises
        :class:`CordumBlockedError` when the approval is rejected, expires or
        is invalidated, or the job ends without one, and
        :class:`CordumTimeoutError` if nobody decides within ``timeout``.

        Unlike :meth:`wait_for_decision` this does not wait for the job to
        finish, so it works for approval-only jobs that no worker runs, such
        as the ones ``@guard`` submits.
        """
        deadline = time.monotonic() + timeout
        while True:
            status = self.get_job(job_id)
            state = status.state.lower()
            # approval_decision is the last transition. It survives the
            # scheduler re-queuing an approved job for approval when its
            # dispatch fails; approval_status flips back to "pending".
            outcome = _APPROVAL_OUTCOMES.get(
                status.approval_decision.lower(), status.approval_status.lower()
            )
            if state == "timeout" and outcome == "pending":
                outcome = "expired"
            if outcome in ("rejected", "expired", "invalidated"):
                reason = f"approval {outcome}"
                if outcome == "rejected" and status.approval_reason:
                    reason += f": {status.approval_reason}"
                raise CordumBlockedError(SafetyDecision(decision=Decision.DENY, reason=reason))
            if state in _FAILED_STATES:
                raise CordumBlockedError(SafetyDecision(
                    decision=Decision.DENY, reason=f"approval job ended {status.state}",
                ))
            # approval_by is set only by a real approve/reject; the gateway
            # can derive "approved" from the job state alone.
            if outcome == "approved" and status.approval_by:
                return status
            if state == "succeeded":
                raise CordumBlockedError(SafetyDecision(
                    decision=Decision.DENY, reason="approval job succeeded without an approval",
                ))
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise CordumTimeoutError(job_id, timeout)
            time.sleep(min(poll_interval, remaining))

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


def _parse_decision(raw: str) -> Optional[Decision]:
    """Map a gateway verdict string to a Decision, or None if unknown.

    The gateway serialises the kernel response with protojson, so verdicts
    arrive as enum names (``DECISION_TYPE_REQUIRE_HUMAN``); plain, upper-case
    and kebab-case spellings are accepted too.
    """
    name = raw.strip().lower().replace("-", "_").removeprefix("decision_type_")
    if name == "require_human":
        name = Decision.REQUIRE_APPROVAL.value
    try:
        return Decision(name)
    except ValueError:
        return None


def _parse_safety_decision(
    data: dict[str, Any], production: bool = False
) -> SafetyDecision:
    """Parse a policy evaluation response into a SafetyDecision.

    A missing verdict, or an unknown verdict string (including
    ``DECISION_TYPE_UNSPECIFIED`` and output-only verdicts), FAILS CLOSED
    (DENY) in every profile. A null or non-string verdict fails closed under
    CAP-PRODUCTION and raises outside it, so a malformed gateway response
    never authorizes work.
    """
    if "decision" not in data:
        return SafetyDecision(
            decision=Decision.DENY,
            reason="missing gateway decision; failing closed",
        )
    raw = data["decision"]
    if not isinstance(raw, str):
        if production:
            return SafetyDecision(
                decision=Decision.DENY,
                reason=(
                    "CAP-PRODUCTION: non-string gateway decision; failing closed"
                ),
            )
        raise TypeError("gateway decision must be a string")
    decision = _parse_decision(raw)
    if decision is None:
        return SafetyDecision(
            decision=Decision.DENY,
            reason=f"unknown gateway decision {raw!r}; failing closed",
        )

    # protojson emits camelCase field names; accept snake_case as well.
    return SafetyDecision(
        decision=decision,
        reason=data.get("reason", ""),
        matched_rules=data.get("matched_rules", []),
        remediations=data.get("remediations", []),
        throttle_duration_seconds=float(
            data.get("throttle_duration_seconds", 0)
        ),
        rule_id=data.get("rule_id") or data.get("ruleId") or "",
        policy_snapshot=data.get("policy_snapshot") or data.get("policySnapshot") or "",
        approval_required=bool(
            data.get("approval_required") or data.get("approvalRequired")
        ),
        approval_ref=data.get("approval_ref") or data.get("approvalRef") or "",
        constraints=data.get("constraints") or {},
    )
