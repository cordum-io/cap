import { expect } from "chai";
import * as crypto from "node:crypto";
import type { Type } from "protobufjs";

import {
  DEFAULT_PRODUCTION_MAX_LIFETIME_MS,
  MAX_PRODUCTION_RAW_BYTES,
  extractProductionSignature,
  verifyProductionPacket,
} from "../src/production-signing";
import { signProductionPacket } from "../src/index";
import {
  busPacketType,
  keyPair,
  signedRawPacket,
} from "./production-admission-support";

interface ProductionPacketFixture {
  protocolVersion: number;
  signature: Uint8Array;
  signatureMetadata: {
    profileVersion: string;
    audience: string;
    messageId: Uint8Array;
    expiresAt: { seconds: number; nanos: number };
  };
  jobRequest: { contextPtr: string };
}

async function fixture(): Promise<{ type: Type; packet: ProductionPacketFixture }> {
  const type = await busPacketType();
  const { unsigned } = extractProductionSignature(await signedRawPacket());
  return {
    type,
    packet: type.decode(unsigned) as unknown as ProductionPacketFixture,
  };
}

const trust = {
  audience: "worker-pool-a",
  tenant: "tenant-a",
  sender: "scheduler-1",
  publicKeys: { k1: keyPair.publicKey },
};

describe("CAP-PRODUCTION Node producer", () => {
  it("signs exact wire without mutating the caller", async () => {
    const { type, packet } = await fixture();
    const callerSignature = Buffer.from("caller-signature");
    packet.signature = callerSignature;

    const raw = signProductionPacket(packet, type, keyPair.privateKey);

    expect(Buffer.from(packet.signature).equals(callerSignature)).to.equal(true);
    expect(verifyProductionPacket(raw, type, trust)).to.not.equal(null);
  });

  it("rejects non-P-256 private keys", async () => {
    const { type, packet } = await fixture();
    const rsa = crypto.generateKeyPairSync("rsa", { modulusLength: 2048 });
    const p384 = crypto.generateKeyPairSync("ec", { namedCurve: "secp384r1" });
    for (const key of [rsa.privateKey, p384.privateKey]) {
      expect(() => signProductionPacket(packet, type, key))
        .to.throw("ECDSA P-256");
    }
  });
});

describe("CAP-PRODUCTION Node producer validation", () => {
  it("rejects unknown top-level and nested input properties", async () => {
    for (const location of ["packet", "metadata"] as const) {
      const { type, packet } = await fixture();
      const target = location === "packet" ? packet : packet.signatureMetadata;
      (target as unknown as Record<string, unknown>).unknownAuthority = "secret";
      expect(() => signProductionPacket(packet, type, keyPair.privateKey))
        .to.throw("unknown production packet field");
    }
  });

  it("rejects invalid production metadata and protocol", async () => {
    const mutations: Array<(packet: ProductionPacketFixture) => void> = [
      (packet) => { packet.protocolVersion = 2; },
      (packet) => { packet.signatureMetadata.profileVersion = "future"; },
      (packet) => { packet.signatureMetadata.audience = ""; },
      (packet) => { packet.signatureMetadata.messageId = Buffer.alloc(15); },
    ];
    for (const mutate of mutations) {
      const { type, packet } = await fixture();
      mutate(packet);
      expect(() => signProductionPacket(packet, type, keyPair.privateKey)).to.throw();
    }
  });

  it("rejects expired and overlong production expiry", async () => {
    const offsets = [-60_000, DEFAULT_PRODUCTION_MAX_LIFETIME_MS + 60_000];
    for (const offset of offsets) {
      const { type, packet } = await fixture();
      packet.signatureMetadata.expiresAt = {
        seconds: Math.floor((Date.now() + offset) / 1000),
        nanos: 0,
      };
      expect(() => signProductionPacket(packet, type, keyPair.privateKey))
        .to.throw("expiry");
    }
  });

  it("rejects an oversized encoded packet", async () => {
    const { type, packet } = await fixture();
    packet.jobRequest.contextPtr = "x".repeat(MAX_PRODUCTION_RAW_BYTES);
    expect(() => signProductionPacket(packet, type, keyPair.privateKey))
      .to.throw("size");
  });
});
