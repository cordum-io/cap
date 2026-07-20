/** CAP-PRODUCTION atomic replay admission (in-memory reference store). */

export enum ReplayOutcome {
  First = "first",
  Duplicate = "duplicate",
}

export class ReplayConflictError extends Error {}
/** Callers MUST fail closed on this error, never treat it as first-seen. */
export class ReplayStoreUnavailableError extends Error {}

export interface ReplayStore {
  admit(
    tenant: string,
    audience: string,
    sender: string,
    messageId: Uint8Array,
    digest: Uint8Array,
    expiresAtMs: number,
  ): ReplayOutcome;
}

interface Entry {
  digest: Uint8Array;
  expiresAtMs: number;
}

function bytesEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

/** Reference ReplayStore. Durable deployments (e.g. Cordum) supply a
 * Redis- or other durable-backed implementation of the same interface. */
export class InMemoryReplayStore implements ReplayStore {
  private entries = new Map<string, Entry>();

  admit(
    tenant: string,
    audience: string,
    sender: string,
    messageId: Uint8Array,
    digest: Uint8Array,
    expiresAtMs: number,
  ): ReplayOutcome {
    if (!tenant || !audience || !sender || messageId.length !== 16 || digest.length === 0) {
      throw new ReplayStoreUnavailableError("invalid replay admission input");
    }
    const now = Date.now();
    if (expiresAtMs <= now) {
      throw new ReplayStoreUnavailableError("expiresAtMs must be in the future");
    }
    const key = `${tenant}\0${audience}\0${sender}\0${Buffer.from(messageId).toString("hex")}`;
    const existing = this.entries.get(key);
    if (existing && existing.expiresAtMs > now) {
      if (bytesEqual(existing.digest, digest)) {
        return ReplayOutcome.Duplicate;
      }
      throw new ReplayConflictError(`replay digest conflict for message_id=${Buffer.from(messageId).toString("hex")}`);
    }
    this.entries.set(key, { digest, expiresAtMs });
    return ReplayOutcome.First;
  }
}
