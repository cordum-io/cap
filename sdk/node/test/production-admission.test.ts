import { expect } from "chai";
import * as crypto from "node:crypto";

import { verifyProductionPacket, PRODUCTION_PROFILE_VERSION, PRODUCTION_ALGORITHM } from "../src/production-signing";
import type { Agent } from "../src/runtime";
import { busPacketType, encodeVarint, keyPair, productionAgent, sign, signedRawPacket } from "./production-admission-support";

interface AdmissionHarness {
  busPacketType: unknown;
  decodeProductionPacket(raw: Uint8Array): Promise<{ jobRequest: { jobId: string } } | null>;
}

function admissionHarness(agent: Agent): AdmissionHarness {
  return agent as unknown as AdmissionHarness;
}

describe("CAP-PRODUCTION Node verifier", () => {
  it("accepts a valid signed packet via verifyProductionPacket", async () => {
    const raw = await signedRawPacket();
    const type = await busPacketType();
    const packet = verifyProductionPacket(raw, type, { audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } });
    expect(packet.jobRequest?.jobId).to.equal("job-1");
  });

  it("rejects an unsigned packet", async () => {
    const type = await busPacketType();
    const packet = type.create({ senderId: "scheduler-1", protocolVersion: 1 });
    const raw = type.encode(packet).finish();
    expect(() => verifyProductionPacket(raw, type, { audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } })).to.throw();
  });

  it("rejects a tampered signature", async () => {
    const raw = await signedRawPacket();
    raw[raw.length - 1] ^= 0xff;
    const type = await busPacketType();
    expect(() => verifyProductionPacket(raw, type, { audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } })).to.throw();
  });

  it("rejects an unknown key id", async () => {
    const otherKeys = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    const raw = await signedRawPacket({ privateKeyPem: otherKeys.privateKey });
    const type = await busPacketType();
    expect(() => verifyProductionPacket(raw, type, { audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } })).to.throw();
  });
});

describe("CAP-PRODUCTION Node runtime admission", () => {
  it("Agent.decodeProductionPacket accepts a valid packet end-to-end", async () => {
    const agent = productionAgent();
    const runtime = admissionHarness(agent);
    runtime.busPacketType = await busPacketType();
    const raw = await signedRawPacket();
    const packet = await runtime.decodeProductionPacket(raw);
    expect(packet).to.not.equal(null);
    expect(packet?.jobRequest.jobId).to.equal("job-1");
  });

  it("Agent.decodeProductionPacket allows identical redelivery, rejects replay conflict", async () => {
    const agent = productionAgent();
    const runtime = admissionHarness(agent);
    runtime.busPacketType = await busPacketType();
    const raw = await signedRawPacket();
    const first = await runtime.decodeProductionPacket(raw);
    const second = await runtime.decodeProductionPacket(raw);
    expect(first).to.not.equal(null);
    expect(second).to.equal(null);
  });
});

describe("CAP-PRODUCTION Node identity admission", () => {
  it("Agent.decodeProductionPacket rejects an identity mirror mismatch", async () => {
    const agent = productionAgent();
    const runtime = admissionHarness(agent);
    runtime.busPacketType = await busPacketType();
    const type = await busPacketType();
    const packet = type.create({
      traceId: "trace-1",
      senderId: "scheduler-1",
      createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
      protocolVersion: 1,
      identity: { tenantId: "tenant-a", principalId: "principal-a" },
      signatureMetadata: {
        profileVersion: PRODUCTION_PROFILE_VERSION,
        algorithm: PRODUCTION_ALGORITHM,
        messageId: Buffer.from("fedcba9876543210"),
        audience: "worker-pool-a",
        expiresAt: { seconds: Math.floor(Date.now() / 1000) + 120, nanos: 0 },
        keyId: "k1",
      },
      jobRequest: {
        jobId: "job-1",
        topic: "job.test",
        tenantId: "tenant-a",
        env: { tenant_id: "tenant-B-DIFFERENT" },
      },
    });
    const unsigned = type.encode(packet).finish();
    const signature = sign(unsigned, keyPair.privateKey);
    const sigField = Buffer.concat([Buffer.from([(14 << 3) | 2]), encodeVarint(signature.length), signature]);
    const raw = Buffer.concat([Buffer.from(unsigned), sigField]);

    const result = await runtime.decodeProductionPacket(raw);
    expect(result).to.equal(null);
  });
});
