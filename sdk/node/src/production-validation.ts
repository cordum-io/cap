/**
 * CAP-PRODUCTION authoritative identity validation (admission-layer subset).
 *
 * Mirrors sdk/go/production_validation.go's ValidateIdentityBinding. Broader
 * cross-mirror identity normalization is task-a13f83fa step-9 scope; this
 * module covers only what the raw-packet admission layer (step-7) needs.
 */

export class IdentityMismatchError extends Error {}

interface IdentityBindingView {
  tenantId?: string;
  principalId?: string;
  actorId?: string;
  delegationId?: string;
}

interface JobRequestView {
  tenantId?: string;
  principalId?: string;
  meta?: { tenantId?: string; actorId?: string };
  env?: Record<string, string>;
  identity?: IdentityBindingView;
}

function sameIdentity(a?: IdentityBindingView, b?: IdentityBindingView): boolean {
  if (!a || !b) return false;
  return (
    a.tenantId === b.tenantId &&
    a.principalId === b.principalId &&
    a.actorId === b.actorId &&
    a.delegationId === b.delegationId
  );
}

export function validateIdentityBinding(request: JobRequestView, authoritative?: IdentityBindingView): void {
  if (!request || !authoritative || !authoritative.tenantId) {
    throw new IdentityMismatchError("missing request or authoritative identity");
  }
  const mirrors: Record<string, string | undefined> = {
    tenant: request.tenantId,
    principal: request.principalId,
    "meta.tenant": request.meta?.tenantId,
    "meta.actor": request.meta?.actorId,
    "env.tenant": request.env?.["tenant_id"],
    "env.principal": request.env?.["principal_id"],
  };
  for (const [name, actual] of Object.entries(mirrors)) {
    if (!actual) continue;
    let expected: string | undefined = authoritative.tenantId;
    if (name.includes("actor")) expected = authoritative.actorId;
    else if (name.includes("principal")) expected = authoritative.principalId;
    if (actual !== expected) {
      throw new IdentityMismatchError(`${name} mismatch`);
    }
  }
  if (request.identity && !sameIdentity(request.identity, authoritative)) {
    throw new IdentityMismatchError("request.identity mismatch");
  }
}
