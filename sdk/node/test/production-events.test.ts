import { expect } from "chai";

import {
  ProductionEventConflictError,
  bindProductionEvent,
  freezeProductionAuthority,
  sealProductionEvent,
  type ProductionEventAuthority,
} from "../src/production-events";
import { verifyProductionPacket } from "../src/production-signing";
import { busPacketType, keyPair } from "./production-admission-support";

const TENANT = "tenant-a";
const PRINCIPAL = "principal-a";
const ACTOR = "actor-a";
const WORKER = "worker-1";
const JOB = "job-42";
const DISPATCH = "dispatch-abc";
const ATTEMPT = 7;
const KEY_ID = "k1";
const OUT_SUBJECT = "sys.job.result";

const identity = () => ({
  tenantId: TENANT,
  principalId: PRINCIPAL,
  actorId: ACTOR,
  delegationId: "delegation-a",
});
const dispatch = () => ({ dispatchId: DISPATCH, attempt: ATTEMPT, assignedWorkerId: WORKER });
const admittedRequest = () => ({ jobId: JOB, identity: identity(), dispatch: dispatch() });
const authority = (): ProductionEventAuthority => freezeProductionAuthority(admittedRequest());

const trustFor = (audience: string) => ({
  audience,
  tenant: TENANT,
  sender: WORKER,
  publicKeys: { [KEY_ID]: keyPair.publicKey },
});

// validateBusPacket requires traceId/senderId/createdAt/payload, and
// validateJobResult requires a non-UNSPECIFIED status plus workerId. Sealing
// deliberately does not invent those -- they are envelope/payload data the
// caller owns -- so the fixtures supply them.
function envelope(jobResult: Record<string, unknown>): Record<string, unknown> {
  return {
    traceId: "trace-1",
    senderId: WORKER,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    identity: identity(),
    jobResult: { status: 2, workerId: WORKER, ...jobResult },
  };
}

async function seal(jobResult: Record<string, unknown>, audience = OUT_SUBJECT): Promise<Buffer> {
  const type = await busPacketType();
  return sealProductionEvent(
    envelope(jobResult),
    type,
    { privateKey: keyPair.privateKey, keyId: KEY_ID, audience },
  );
}

describe("CAP-PRODUCTION Node event echo", () => {
  // Exact-value assertions against the admitted request. A non-empty check would
  // stay green if the producer echoed a fresh-but-wrong identity, which is
  // precisely the bug class this suite exists to catch.
  it("echoes the admitted identity and dispatch onto result, progress, and cancel", () => {
    for (const event of [{ jobId: JOB }, { jobId: JOB }, { jobId: JOB }] as Record<string, any>[]) {
      bindProductionEvent(event, authority());
      expect(event.identity).to.deep.equal(identity());
      expect(event.dispatch).to.deep.equal(dispatch());
      expect(event.dispatch.dispatchId).to.equal(DISPATCH);
      expect(event.dispatch.attempt).to.equal(ATTEMPT);
      expect(event.dispatch.assignedWorkerId).to.equal(WORKER);
      expect(event.jobId).to.equal(JOB);
    }
  });

  it("does not alias the frozen authority across emitted events", () => {
    const frozen = authority();
    const first: Record<string, any> = { jobId: JOB };
    bindProductionEvent(first, frozen);
    first.dispatch.attempt = 999;
    first.identity.actorId = "actor-IMPOSTER";

    const second: Record<string, any> = { jobId: JOB };
    bindProductionEvent(second, frozen);
    expect(second.dispatch.attempt).to.equal(ATTEMPT);
    expect(second.identity.actorId).to.equal(ACTOR);
  });

  it("fills authority a handler left unset", () => {
    const event: Record<string, any> = {};
    bindProductionEvent(event, authority());
    expect(event.jobId).to.equal(JOB);
    expect(event.identity).to.deep.equal(identity());
    expect(event.dispatch).to.deep.equal(dispatch());
  });

  it("rejects conflicting handler-supplied authority rather than overriding it", () => {
    const conflicts: Record<string, unknown>[] = [
      { jobId: JOB, identity: { ...identity(), tenantId: "tenant-EVIL" } },
      { jobId: JOB, identity: { ...identity(), actorId: "actor-IMPOSTER" } },
      { jobId: JOB, dispatch: { ...dispatch(), dispatchId: "dispatch-OTHER" } },
      { jobId: JOB, dispatch: { ...dispatch(), attempt: ATTEMPT + 1 } },
      { jobId: JOB, dispatch: { ...dispatch(), assignedWorkerId: "worker-OTHER" } },
      { jobId: "job-OTHER" },
    ];
    for (const event of conflicts) {
      expect(() => bindProductionEvent(event, authority())).to.throw(ProductionEventConflictError);
    }
  });

  it("refuses to freeze authority from a request with no job id", () => {
    expect(() => freezeProductionAuthority({ jobId: "" })).to.throw(ProductionEventConflictError);
  });
});

describe("CAP-PRODUCTION Node event sealing", () => {
  it("verifies against the actual outbound subject and carries the echoed authority", async () => {
    const type = await busPacketType();
    const result: Record<string, any> = { jobId: JOB };
    bindProductionEvent(result, authority());

    const raw = await seal(result);

    const verified = verifyProductionPacket(raw, type, trustFor(OUT_SUBJECT)) as any;
    expect(verified.signatureMetadata.audience).to.equal(OUT_SUBJECT);
    expect(verified.signatureMetadata.keyId).to.equal(KEY_ID);
    expect(verified.signatureMetadata.messageId.length).to.equal(16);
    expect(verified.jobResult.dispatch.dispatchId).to.equal(DISPATCH);
    // DispatchIdentity.attempt is a 64-bit proto field, so protobufjs decodes it
    // as a Long object rather than a JS number. A consumer doing `attempt === 7`
    // against the decoded value silently gets false and would skip its own fence
    // check -- so normalize explicitly here and keep the trap visible.
    expect(Number(verified.jobResult.dispatch.attempt)).to.equal(ATTEMPT);
    expect(verified.jobResult.dispatch.assignedWorkerId).to.equal(WORKER);
    expect(verified.jobResult.identity.tenantId).to.equal(TENANT);
    expect(verified.jobResult.identity.actorId).to.equal(ACTOR);
  });

  it("is rejected under a subject it was not sealed for", async () => {
    const type = await busPacketType();
    const raw = await seal({ jobId: JOB }, "sys.job.result");
    expect(() => verifyProductionPacket(raw, type, trustFor("sys.job.progress")))
      .to.throw("audience mismatch");
  });

  it("mints a fresh unique message id per message", async () => {
    // Uniqueness across N sends, not merely non-empty: a constant 16-byte id
    // satisfies a length check and still breaks replay de-duplication.
    const type = await busPacketType();
    const seen = new Set<string>();
    for (let i = 0; i < 8; i += 1) {
      const raw = await seal({ jobId: JOB });
      const verified = verifyProductionPacket(raw, type, trustFor(OUT_SUBJECT)) as any;
      const id = Buffer.from(verified.signatureMetadata.messageId).toString("hex");
      expect(verified.signatureMetadata.messageId.length).to.equal(16);
      expect(seen.has(id), "reused production message id").to.equal(false);
      seen.add(id);
    }
    expect(seen.size).to.equal(8);
  });

  it("carries a bounded future expiry", async () => {
    const type = await busPacketType();
    const raw = await seal({ jobId: JOB });
    const verified = verifyProductionPacket(raw, type, trustFor(OUT_SUBJECT)) as any;
    const expiresMs = Number(verified.signatureMetadata.expiresAt.seconds) * 1000;
    expect(expiresMs).to.be.greaterThan(Date.now() - 1000);
  });

  it("requires a key id, an audience, and a bounded lifetime", async () => {
    const type = await busPacketType();
    const base = envelope({ jobId: JOB });
    const opts = { privateKey: keyPair.privateKey, keyId: KEY_ID, audience: OUT_SUBJECT };
    expect(() => sealProductionEvent(base, type, { ...opts, keyId: "" }))
      .to.throw(ProductionEventConflictError);
    expect(() => sealProductionEvent(base, type, { ...opts, audience: "  " }))
      .to.throw(ProductionEventConflictError);
    expect(() => sealProductionEvent(base, type, { ...opts, lifetimeMs: 0 }))
      .to.throw(ProductionEventConflictError);
    expect(() => sealProductionEvent(base, type, { ...opts, lifetimeMs: 60 * 60_000 }))
      .to.throw(ProductionEventConflictError);
  });

  it("does not mutate the caller's packet", async () => {
    const type = await busPacketType();
    const packet: Record<string, any> = envelope({ jobId: JOB });
    sealProductionEvent(packet, type, {
      privateKey: keyPair.privateKey,
      keyId: KEY_ID,
      audience: OUT_SUBJECT,
    });
    expect(packet.signatureMetadata).to.equal(undefined);
    expect(packet.protocolVersion).to.equal(undefined);
  });
});
