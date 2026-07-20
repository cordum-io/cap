import { expect } from "chai";

import {
  extractProductionSignature,
  type ProductionTrustStore,
  verifyProductionPacket,
} from "../src/production-signing";
import type { Agent } from "../src/runtime";
import {
  busPacketType,
  encodeVarint,
  keyPair,
  productionAgent,
  RecordingReplayStore,
  sign,
  signedRawPacket,
} from "./production-admission-support";

interface AdmissionHarness {
  busPacketType: unknown;
  decodeProductionPacket(raw: Uint8Array): Promise<unknown | null>;
}

interface MutableExpiryPacket {
  signatureMetadata: { expiresAt: { seconds: number } };
}

function admissionHarness(agent: Agent): AdmissionHarness {
  return agent as unknown as AdmissionHarness;
}

function field(number: number, wireType: number, value: Buffer): Buffer {
  const tag = encodeVarint((number << 3) | wireType);
  if (wireType === 2) {
    return Buffer.concat([tag, encodeVarint(value.length), value]);
  }
  return Buffer.concat([tag, value]);
}

async function signedWireWith(extraField: Buffer): Promise<Buffer> {
  const valid = await signedRawPacket();
  const { unsigned } = extractProductionSignature(valid);
  const malicious = Buffer.concat([Buffer.from(unsigned), extraField]);
  const signature = sign(malicious, keyPair.privateKey);
  return Buffer.concat([malicious, field(14, 2, signature)]);
}

async function signedProtocolVersion(encoded: Buffer): Promise<Buffer> {
  const valid = await signedRawPacket();
  const { unsigned } = extractProductionSignature(valid);
  const canonical = Buffer.from([0x20, 0x01]);
  const offset = Buffer.from(unsigned).indexOf(canonical);
  if (offset < 0) throw new Error("fixture lacks canonical protocol version");
  const malicious = Buffer.concat([
    Buffer.from(unsigned).subarray(0, offset),
    field(4, 0, encoded),
    Buffer.from(unsigned).subarray(offset + canonical.length),
  ]);
  const signature = sign(malicious, keyPair.privateKey);
  return Buffer.concat([malicious, field(14, 2, signature)]);
}

async function expectRejected(extraField: Buffer): Promise<void> {
  const raw = await signedWireWith(extraField);
  expect(() => extractProductionSignature(raw)).to.throw();
}

describe("CAP-PRODUCTION exact BusPacket parser", () => {
  it("rejects a signed unknown top-level field", async () => {
    await expectRejected(field(99, 2, Buffer.from("unknown")));
  });

  it("rejects a duplicate singular sender_id", async () => {
    await expectRejected(field(2, 2, Buffer.from("other-sender")));
  });

  it("rejects more than one distinct oneof payload", async () => {
    await expectRejected(field(11, 2, Buffer.alloc(0)));
  });

  it("rejects a wrong wire type for a known field", async () => {
    await expectRejected(field(4, 2, Buffer.alloc(0)));
  });

  for (const encoded of [
    Buffer.from([0x02]),
    Buffer.from([0x81, 0x80, 0x80, 0x80, 0x10]),
    Buffer.from([0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01]),
  ]) {
    it(`rejects protocol_version parser differential ${encoded.toString("hex")}`, async () => {
      const raw = await signedProtocolVersion(encoded);
      expect(() => extractProductionSignature(raw))
        .to.throw("invalid protocol version wire");
    });
  }
});

describe("CAP-PRODUCTION authoritative verifier trust", () => {
  const incompleteAuthorities: ProductionTrustStore[] = [
    { audience: "", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } },
    { audience: "worker-pool-a", tenant: "", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } },
    { audience: "worker-pool-a", tenant: "tenant-a", sender: "", publicKeys: { k1: keyPair.publicKey } },
  ];
  for (const trust of incompleteAuthorities) {
    it(`rejects incomplete authority ${JSON.stringify([trust.audience, trust.tenant, trust.sender])}`, async () => {
      const type = await busPacketType();
      const raw = await signedRawPacket();
      expect(() => verifyProductionPacket(raw, type, trust))
        .to.throw("production trust authority required");
    });
  }
});

describe("CAP-PRODUCTION replay retention", () => {
  it("extends metadata expiry by the configured clock skew", async () => {
    const replay = new RecordingReplayStore();
    const agent = productionAgent(replay, { clockSkewMs: 30_000 });
    admissionHarness(agent).busPacketType = await busPacketType();
    const raw = await signedRawPacket();
    const { unsigned } = extractProductionSignature(raw);
    const packet = (await busPacketType()).decode(unsigned) as unknown as MutableExpiryPacket;
    const expiryMs = Number(packet.signatureMetadata.expiresAt.seconds) * 1000;

    expect(await admissionHarness(agent).decodeProductionPacket(raw)).to.not.equal(null);
    expect(replay.expiriesMs).to.deep.equal([expiryMs + 30_000]);
  });

  it("fails closed on a negative clock skew", async () => {
    const type = await busPacketType();
    const raw = await signedRawPacket();
    expect(() => verifyProductionPacket(raw, type, {
      audience: "worker-pool-a",
      tenant: "tenant-a",
      sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
      clockSkewMs: -1,
    })).to.throw("bounds");
  });
});

describe("CAP-PRODUCTION expiry timestamp validation", () => {
  for (const nanos of [-1, 1_000_000_000]) {
    it(`rejects invalid expiry nanos ${nanos}`, async () => {
      const type = await busPacketType();
      const raw = await signedRawPacket({ expiresNanos: nanos });
      expect(() => verifyProductionPacket(raw, type, {
        audience: "worker-pool-a",
        tenant: "tenant-a",
        sender: "scheduler-1",
        publicKeys: { k1: keyPair.publicKey },
      })).to.throw("invalid signature expiry");
    });
  }

  it("rejects an expiry beyond the protobuf Timestamp range", async () => {
    const type = await busPacketType();
    const raw = await signedRawPacket({ expiresSeconds: 253_402_300_800 });
    expect(() => verifyProductionPacket(raw, type, {
      audience: "worker-pool-a",
      tenant: "tenant-a",
      sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
    })).to.throw("invalid signature expiry");
  });

  for (const seconds of [Number.NEGATIVE_INFINITY, Number.POSITIVE_INFINITY, 1.5]) {
    it(`rejects non-integral or non-finite expiry seconds ${seconds}`, async () => {
      const type = await busPacketType();
      const raw = await signedRawPacket();
      const packet = type.decode(
        extractProductionSignature(raw).unsigned,
      ) as unknown as MutableExpiryPacket;
      packet.signatureMetadata.expiresAt.seconds = seconds;
      const decoder = { decode: () => packet, fieldsById: type.fieldsById };

      expect(() => verifyProductionPacket(raw, decoder, {
        audience: "worker-pool-a",
        tenant: "tenant-a",
        sender: "scheduler-1",
        publicKeys: { k1: keyPair.publicKey },
      })).to.throw("invalid signature expiry");
    });
  }
});

describe("CAP-PRODUCTION lifetime trust bounds", () => {
  for (const nowMs of [Number.NaN, Number.POSITIVE_INFINITY, 1.5]) {
    it(`rejects an invalid trust clock value ${nowMs}`, async () => {
      const type = await busPacketType();
      const raw = await signedRawPacket();
      expect(() => verifyProductionPacket(raw, type, {
        audience: "worker-pool-a",
        tenant: "tenant-a",
        sender: "scheduler-1",
        publicKeys: { k1: keyPair.publicKey },
        nowMs: () => nowMs,
      })).to.throw("invalid production lifetime bounds");
    });
  }

  it("rejects clock skew greater than maximum lifetime", async () => {
    const type = await busPacketType();
    const raw = await signedRawPacket();
    expect(() => verifyProductionPacket(raw, type, {
      audience: "worker-pool-a",
      tenant: "tenant-a",
      sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
      maxLifetimeMs: 30_000,
      clockSkewMs: 30_001,
    })).to.throw("invalid production lifetime bounds");
  });
});

describe("CAP-PRODUCTION absolute lifetime caps", () => {
  const invalidCaps: Array<[number, number]> = [
    [300_001, 0],
    [300_000, 60_001],
    [0, 0],
    [-1, 0],
  ];
  for (const [maxLifetimeMs, clockSkewMs] of invalidCaps) {
    it(`rejects lifetime bounds ${maxLifetimeMs}/${clockSkewMs}`, async () => {
      const type = await busPacketType();
      const raw = await signedRawPacket();
      expect(() => verifyProductionPacket(raw, type, {
        audience: "worker-pool-a",
        tenant: "tenant-a",
        sender: "scheduler-1",
        publicKeys: { k1: keyPair.publicKey },
        maxLifetimeMs,
        clockSkewMs,
      })).to.throw("invalid production lifetime bounds");
    });
  }

  it("accepts the absolute lifetime and skew boundaries", async () => {
    const type = await busPacketType();
    const raw = await signedRawPacket();
    expect(verifyProductionPacket(raw, type, {
      audience: "worker-pool-a",
      tenant: "tenant-a",
      sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
      maxLifetimeMs: 300_000,
      clockSkewMs: 60_000,
    })).to.not.equal(null);
  });
});
