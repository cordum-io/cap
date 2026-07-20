"""CAP-PRODUCTION authoritative identity validation (admission-layer subset).

Mirrors sdk/go/production_validation.go's ValidateIdentityBinding. Broader
cross-mirror identity normalization (top-level/meta/env/label agreement
across every ingress) is task-a13f83fa step-9 scope; this module covers only
what the raw-packet admission layer (step-7) needs before dispatch.
"""

from __future__ import annotations

from .pb.cordum.agent.v1.job_pb2 import IdentityBinding, JobRequest


class IdentityMismatchError(ValueError):
    """A JobRequest mirror disagrees with the authoritative IdentityBinding."""


def validate_identity_binding(request: JobRequest, authoritative: IdentityBinding) -> None:
    if request is None or authoritative is None or not authoritative.tenant_id:
        raise IdentityMismatchError("missing request or authoritative identity")
    mirrors = {
        "tenant": request.tenant_id,
        "principal": request.principal_id,
        "meta.tenant": request.meta.tenant_id if request.HasField("meta") else "",
        "meta.actor": request.meta.actor_id if request.HasField("meta") else "",
        "env.tenant": request.env.get("tenant_id", ""),
        "env.principal": request.env.get("principal_id", ""),
    }
    for name, actual in mirrors.items():
        if not actual:
            continue
        if "actor" in name:
            expected = authoritative.actor_id
        elif "principal" in name:
            expected = authoritative.principal_id
        else:
            expected = authoritative.tenant_id
        if actual != expected:
            raise IdentityMismatchError(f"{name} mismatch")
    if request.HasField("identity") and not _same_identity(request.identity, authoritative):
        raise IdentityMismatchError("request.identity mismatch")


def _same_identity(a: IdentityBinding, b: IdentityBinding) -> bool:
    return (
        a.tenant_id == b.tenant_id
        and a.principal_id == b.principal_id
        and a.actor_id == b.actor_id
        and a.delegation_id == b.delegation_id
    )
