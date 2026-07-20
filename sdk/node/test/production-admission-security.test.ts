import { expect } from "chai";
import * as crypto from "node:crypto";

import { Agent, InMemoryBlobStore } from "../src/runtime";
import { extractProductionSignature, MAX_PRODUCTION_RAW_BYTES, ProductionWireError, verifyProductionPacket } from "../src/production-signing";
import { InMemoryReplayStore, ReplayOutcome, type ReplayStore } from "../src/production-replay";
import { busPacketType, keyPair, productionAgent, RecordingReplayStore, signedRawPacket } from "./production-admission-support";

interface AdmissionHarness {
  busPacketType: unknown;
  trust: unknown;
  decodeProductionPacket(raw: Uint8Array, audience?: string): Promise<unknown | null>;
  onMessage(message: unknown, spec: unknown): Promise<void>;
}

interface ReplayStoreHarness {
  entries: Map<string, { expiresAtMs: number }>;
}

function admissionHarness(agent: Agent): AdmissionHarness {
  return agent as unknown as AdmissionHarness;
}

describe("CAP-PRODUCTION replay and identity admission", () => {
  it("rejects an invalid envelope before replay admission", async () => {
    const replay = new RecordingReplayStore();
    const agent = productionAgent(replay);
    admissionHarness(agent).busPacketType = await busPacketType();
    const raw = await signedRawPacket({ protocolVersion: 999 });

    expect(await admissionHarness(agent).decodeProductionPacket(raw)).to.equal(null);
    expect(replay.calls).to.equal(0);
  });

  it("keys replay to the exact signed body rather than randomized signature bytes", async () => {
    const replay = new RecordingReplayStore();
    const agent = productionAgent(replay);
    admissionHarness(agent).busPacketType = await busPacketType();
    const first = await signedRawPacket();
    const second = await signedRawPacket();

    expect(await admissionHarness(agent).decodeProductionPacket(first)).to.not.equal(null);
    expect(await admissionHarness(agent).decodeProductionPacket(second)).to.not.equal(null);
    expect(replay.digests).to.have.length(2);
    expect(replay.digests[0].equals(replay.digests[1])).to.equal(true);
  });

  it("binds meta.actor to the authoritative actor rather than principal", async () => {
    const agent = productionAgent();
    admissionHarness(agent).busPacketType = await busPacketType();
    const raw = await signedRawPacket({ actorId: "actor-a", metaActorId: "principal-a" });

    expect(await admissionHarness(agent).decodeProductionPacket(raw)).to.equal(null);
  });

  it("runtime production path requires an authenticated session before replay", async () => {
    const replay = new RecordingReplayStore(ReplayOutcome.Duplicate);
    const agent = productionAgent(replay);
    admissionHarness(agent).busPacketType = await busPacketType();
    const raw = await signedRawPacket();

    await admissionHarness(agent).onMessage({ data: raw, subject: "worker-pool-a" }, {});
    expect(replay.calls).to.equal(0);
  });
});

describe("CAP-PRODUCTION configuration and authority security", () => {
  it("rejects partial production configuration", () => {
    expect(() => new Agent({
      store: new InMemoryBlobStore(),
      productionTrust: { audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } },
    })).to.throw("productionTrust and replayStore");
    expect(() => new Agent({
      store: new InMemoryBlobStore(),
      replayStore: new InMemoryReplayStore(),
    })).to.throw("productionTrust and replayStore");
  });

  it("fails closed on an unknown replay-store outcome", async () => {
    const replay = new RecordingReplayStore("invalid" as ReplayOutcome);
    const agent = productionAgent(replay);
    admissionHarness(agent).busPacketType = await busPacketType();

    expect(await admissionHarness(agent).decodeProductionPacket(await signedRawPacket())).to.equal(null);
  });

  it("binds signature audience to the actual NATS subject", async () => {
    const agent = productionAgent();
    admissionHarness(agent).busPacketType = await busPacketType();
    const raw = await signedRawPacket();

    expect(await admissionHarness(agent).decodeProductionPacket(raw, "other.subject")).to.equal(null);
  });

  it("does not let an identity rejection poison the replay message id", async () => {
    const agent = productionAgent();
    admissionHarness(agent).busPacketType = await busPacketType();
    const invalid = await signedRawPacket({ envTenantId: "tenant-b" });
    const valid = await signedRawPacket();

    expect(await admissionHarness(agent).decodeProductionPacket(invalid)).to.equal(null);
    expect(await admissionHarness(agent).decodeProductionPacket(valid)).to.not.equal(null);
  });
});

describe("CAP-PRODUCTION lifetime and algorithm security", () => {
  it("rejects expiry beyond the default bounded lifetime", async () => {
    const agent = productionAgent();
    admissionHarness(agent).busPacketType = await busPacketType();
    const raw = await signedRawPacket({ expiresInMs: 10 * 60_000 });

    expect(await admissionHarness(agent).decodeProductionPacket(raw)).to.equal(null);
  });

  it("rejects a non-P-256 key for the declared production algorithm", async () => {
    const p384 = crypto.generateKeyPairSync("ec", {
      namedCurve: "secp384r1",
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
      publicKeyEncoding: { type: "spki", format: "pem" },
    });
    const agent = new Agent({
      store: new InMemoryBlobStore(),
      productionTrust: { audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: p384.publicKey } },
      replayStore: new InMemoryReplayStore(),
    });
    admissionHarness(agent).busPacketType = await busPacketType();
    const raw = await signedRawPacket({ privateKeyPem: p384.privateKey });

    expect(await admissionHarness(agent).decodeProductionPacket(raw)).to.equal(null);
  });
});

describe("CAP-PRODUCTION exact-wire error security", () => {
  it("rejects an oversize wire packet before parsing", () => {
    expect(() => extractProductionSignature(Buffer.alloc(MAX_PRODUCTION_RAW_BYTES + 1))).to.throw("size");
  });

  it("normalizes malformed embedded protobuf decode errors", async () => {
    const type = await busPacketType();
    const raw = Buffer.from([0x52, 0x01, 0x80, 0x72, 0x01, 0x01]);

    expect(() => verifyProductionPacket(raw, type, {
      audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
    })).to.throw(ProductionWireError);
  });

  it("rejects non-minimal outer wire varints", () => {
    const cases = [
      Buffer.from([0x20, 0x81, 0x00, 0x72, 0x01, 0x01]),
      Buffer.from([0x0a, 0x81, 0x00, 0x00, 0x72, 0x01, 0x01]),
    ];
    for (const raw of cases) {
      expect(() => extractProductionSignature(raw)).to.throw("non-minimal");
    }
  });
});

describe("CAP-PRODUCTION replay backend and session security", () => {
  it("fails closed on an unexpected replay backend error", async () => {
    const replay: ReplayStore = {
      admit: () => { throw new Error("secret backend detail"); },
    };
    const agent = productionAgent(replay);
    admissionHarness(agent).busPacketType = await busPacketType();

    expect(await admissionHarness(agent).decodeProductionPacket(await signedRawPacket())).to.equal(null);
  });

  it("binds runtime trust to authenticated session tenant and sender", async () => {
    const replay = new RecordingReplayStore(ReplayOutcome.Duplicate);
    const agent = productionAgent(replay);
    admissionHarness(agent).busPacketType = await busPacketType();
    admissionHarness(agent).trust = {
      productionSessionActive: true,
      settings: { config: { tenantId: "tenant-a", expectedSchedulerId: "scheduler-1" } },
    };
    const raw = await signedRawPacket({ tenantId: "tenant-b" });

    await admissionHarness(agent).onMessage({ data: raw, subject: "worker-pool-a" }, {});
    expect(replay.calls).to.equal(0);
  });

  it("allows runtime admission after an authenticated session", async () => {
    const replay = new RecordingReplayStore(ReplayOutcome.Duplicate);
    const agent = productionAgent(replay);
    admissionHarness(agent).busPacketType = await busPacketType();
    admissionHarness(agent).trust = {
      productionSessionActive: true,
      settings: { config: { tenantId: "tenant-a", expectedSchedulerId: "scheduler-1" } },
    };
    const raw = await signedRawPacket();

    await admissionHarness(agent).onMessage({ data: raw, subject: "worker-pool-a" }, {});
    expect(replay.calls).to.equal(1);
  });
});

describe("CAP-PRODUCTION startup security", () => {
  it("rejects production startup before connecting unless worker trust is ENFORCE", async () => {
    let connected = false;
    const agent = new Agent({
      store: new InMemoryBlobStore(),
      productionTrust: { audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } },
      replayStore: new InMemoryReplayStore(),
      connectFn: async () => {
        connected = true;
        throw new Error("NATS connect must not run");
      },
    });
    agent.job("job.test", async () => ({}));

    let failure: unknown;
    try {
      await agent.start();
    } catch (error) {
      failure = error;
    }
    expect(String(failure)).to.contain("ENFORCE");
    expect(connected).to.equal(false);
  });
});

describe("CAP-PRODUCTION in-memory replay store security", () => {
  it("purges expired replay entries and copies caller-owned digests", () => {
    const store = new InMemoryReplayStore();
    const now = Date.now();
    const firstId = Buffer.from("0123456789abcdef");
    const original = Buffer.from("digest-original");
    const supplied = Buffer.from(original);
    store.admit("tenant-a", "audience-a", "sender-a", firstId, supplied, now + 60_000);
    supplied[0] ^= 0xff;
    expect(store.admit("tenant-a", "audience-a", "sender-a", firstId, original, now + 60_000)).to.equal(ReplayOutcome.Duplicate);

    const entries = (store as unknown as ReplayStoreHarness).entries;
    const firstKey = [...entries.keys()][0];
    entries.get(firstKey)!.expiresAtMs = now - 1;
    store.admit("tenant-a", "audience-a", "sender-a", Buffer.from("fedcba9876543210"), Buffer.from("new"), now + 60_000);
    expect(entries.has(firstKey)).to.equal(false);
  });

  it("uses an unambiguous replay tuple key", () => {
    const store = new InMemoryReplayStore();
    const expires = Date.now() + 60_000;
    const messageId = Buffer.from("0123456789abcdef");

    expect(store.admit("a\0b", "c", "d", messageId, Buffer.from("one"), expires)).to.equal(ReplayOutcome.First);
    expect(store.admit("a", "b", "c\0d", messageId, Buffer.from("two"), expires)).to.equal(ReplayOutcome.First);
  });
});
