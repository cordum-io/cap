/**
 * CAP-PRODUCTION managed-worker event echo and sealing for Node.
 *
 * Parity with sdk/go/worker/managed_events.go and sdk/python cap/production_events.py.
 *
 * Node already verifies INCOMING packets in runtime.ts (verifyProductionPacket +
 * validateIdentityBinding + replay admission). The missing half was outgoing:
 * nothing echoed the admitted DispatchIdentity/IdentityBinding onto a
 * Result/Progress/Cancel, and signProductionPacket had zero call sites, so a
 * Node worker could not participate in dispatch fencing at all.
 *
 * The rule: authority is frozen from the ADMITTED request before handler code
 * runs, and a handler that contradicts it is rejected rather than silently
 * corrected. Silent correction turns a handler bug -- or a compromised handler --
 * into a correctly-signed event carrying someone else's identity.
 */

import * as crypto from "node:crypto";
import type { Type } from "protobufjs";

import {
  DEFAULT_PRODUCTION_MAX_LIFETIME_MS,
  PRODUCTION_ALGORITHM,
  PRODUCTION_PROFILE_VERSION,
} from "./production-signing";
import { signProductionPacket } from "./production-signing-producer";

export const PRODUCTION_MESSAGE_ID_BYTES = 16;

export class ProductionEventConflictError extends Error {}

export interface ProductionIdentityView {
  tenantId?: string;
  principalId?: string;
  actorId?: string;
  delegationId?: string;
}

export interface ProductionDispatchView {
  dispatchId?: string;
  attempt?: number;
  assignedWorkerId?: string;
}

export interface ProductionEventAuthority {
  readonly jobId: string;
  readonly identity: ProductionIdentityView;
  readonly dispatch: ProductionDispatchView;
}

interface ProductionEventView {
  jobId?: string;
  identity?: ProductionIdentityView;
  dispatch?: ProductionDispatchView;
}

interface AdmittedRequestView extends ProductionEventView {
  jobId?: string;
}

function cloneIdentity(value?: ProductionIdentityView): ProductionIdentityView {
  return {
    tenantId: value?.tenantId ?? "",
    principalId: value?.principalId ?? "",
    actorId: value?.actorId ?? "",
    delegationId: value?.delegationId ?? "",
  };
}

function cloneDispatch(value?: ProductionDispatchView): ProductionDispatchView {
  return {
    dispatchId: value?.dispatchId ?? "",
    attempt: value?.attempt ?? 0,
    assignedWorkerId: value?.assignedWorkerId ?? "",
  };
}

function identityDiffers(a: ProductionIdentityView, b: ProductionIdentityView): boolean {
  return (
    (a.tenantId ?? "") !== (b.tenantId ?? "") ||
    (a.principalId ?? "") !== (b.principalId ?? "") ||
    (a.actorId ?? "") !== (b.actorId ?? "") ||
    (a.delegationId ?? "") !== (b.delegationId ?? "")
  );
}

function dispatchDiffers(a: ProductionDispatchView, b: ProductionDispatchView): boolean {
  return (
    (a.dispatchId ?? "") !== (b.dispatchId ?? "") ||
    (a.attempt ?? 0) !== (b.attempt ?? 0) ||
    (a.assignedWorkerId ?? "") !== (b.assignedWorkerId ?? "")
  );
}

/**
 * Snapshot the admitted request's authority before any handler runs.
 *
 * Returns deep copies, so neither handler code nor a previously emitted event
 * can reach back and change what later events will echo.
 */
export function freezeProductionAuthority(request: AdmittedRequestView): ProductionEventAuthority {
  if (!request) {
    throw new ProductionEventConflictError("cannot freeze authority from a missing request");
  }
  const jobId = (request.jobId ?? "").trim();
  if (!jobId) {
    throw new ProductionEventConflictError("admitted request carries no job id");
  }
  return Object.freeze({
    jobId,
    identity: Object.freeze(cloneIdentity(request.identity)),
    dispatch: Object.freeze(cloneDispatch(request.dispatch)),
  });
}

/**
 * Echo the frozen authority onto an outgoing Result/Progress/Cancel.
 *
 * Unset fields are filled -- that is the normal path. A field the handler set to
 * something DIFFERENT is a conflict and throws.
 */
export function bindProductionEvent(
  event: ProductionEventView,
  authority: ProductionEventAuthority,
): void {
  if (!event) throw new ProductionEventConflictError("cannot bind a missing event");
  if (!authority) {
    throw new ProductionEventConflictError("cannot bind without an admitted authority");
  }

  const jobId = (event.jobId ?? "").trim();
  if (jobId && jobId !== authority.jobId) {
    throw new ProductionEventConflictError(
      `event job id ${jobId} conflicts with admitted ${authority.jobId}`,
    );
  }
  event.jobId = authority.jobId;

  if (event.identity && identityDiffers(event.identity, authority.identity)) {
    throw new ProductionEventConflictError("event identity conflicts with admitted authority");
  }
  event.identity = cloneIdentity(authority.identity);

  if (event.dispatch && dispatchDiffers(event.dispatch, authority.dispatch)) {
    throw new ProductionEventConflictError("event dispatch conflicts with admitted authority");
  }
  event.dispatch = cloneDispatch(authority.dispatch);
}

export interface SealProductionEventOptions {
  privateKey: crypto.KeyLike;
  keyId: string;
  /** MUST be the subject about to be published on. */
  audience: string;
  lifetimeMs?: number;
  nowMs?: () => number;
}

/**
 * Stamp fresh signature metadata and produce signed production wire bytes.
 *
 * `audience` must be the subject the caller is about to publish on. Binding to
 * anything else -- a configured default, or the inbound subject -- would let a
 * packet captured on one subject be replayed onto another.
 */
export function sealProductionEvent(
  packet: Record<string, unknown>,
  busPacketType: Type,
  options: SealProductionEventOptions,
): Buffer {
  const keyId = (options.keyId ?? "").trim();
  if (!keyId) throw new ProductionEventConflictError("production sealing requires a key id");
  const audience = (options.audience ?? "").trim();
  if (!audience) {
    throw new ProductionEventConflictError(
      "production sealing requires the outbound subject as audience",
    );
  }
  const lifetimeMs = options.lifetimeMs ?? DEFAULT_PRODUCTION_MAX_LIFETIME_MS;
  if (!Number.isFinite(lifetimeMs) || lifetimeMs <= 0 || lifetimeMs > DEFAULT_PRODUCTION_MAX_LIFETIME_MS) {
    throw new ProductionEventConflictError(
      "production sealing requires a bounded positive lifetime",
    );
  }

  const issuedMs = options.nowMs ? options.nowMs() : Date.now();
  const expiresMs = issuedMs + lifetimeMs;
  const outgoing: Record<string, unknown> = { ...packet };
  delete outgoing.signature;
  // signProductionPacket rejects anything without protocolVersion 1, so an unset
  // value yields bytes nobody can verify. Default it rather than merely document it.
  if (!outgoing.protocolVersion) outgoing.protocolVersion = 1;
  outgoing.signatureMetadata = {
    profileVersion: PRODUCTION_PROFILE_VERSION,
    algorithm: PRODUCTION_ALGORITHM,
    // Fresh per message: a reused id silently breaks replay de-duplication for
    // every consumer, so it is generated here rather than accepted from a caller.
    messageId: crypto.randomBytes(PRODUCTION_MESSAGE_ID_BYTES),
    audience,
    expiresAt: {
      seconds: Math.floor(expiresMs / 1000),
      nanos: (expiresMs % 1000) * 1_000_000,
    },
    keyId,
  };

  return signProductionPacket(outgoing, busPacketType, options.privateKey);
}
