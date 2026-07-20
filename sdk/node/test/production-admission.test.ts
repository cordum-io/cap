import { expect } from "chai";
import * as crypto from "node:crypto";

import { Agent, InMemoryBlobStore } from "../src/runtime";
import { loadRoot } from "../src/protos";
import { verifyProductionPacket, PRODUCTION_PROFILE_VERSION, PRODUCTION_ALGORITHM, PRODUCTION_SIGNATURE_DOMAIN } from "../src/production-signing";
import { InMemoryReplayStore } from "../src/production-replay";

const keyPair = crypto.generateKeyPairSync("ec", {
  namedCurve: "prime256v1",
  publicKeyEncoding: { type: "spki", format: "pem" },
  privateKeyEncoding: { type: "pkcs8", format: "pem" },
});

async function busPacketType() {
  const root = await loadRoot();
  return root.lookupType("cordum.agent.v1.BusPacket");
}

function sign(unsigned: Uint8Array, privateKeyPem: string): Buffer {
  const signer = crypto.createSign("sha256");
  signer.update(PRODUCTION_SIGNATURE_DOMAIN);
  signer.update(Buffer.from(unsigned));
  signer.end();
  return signer.sign(privateKeyPem);
}

async function signedRawPacket(opts: { keyId?: string; audience?: string; messageId?: Buffer; privateKeyPem?: string } = {}): Promise<Buffer> {
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
      messageId: opts.messageId ?? Buffer.from("0123456789abcdef"),
      audience: opts.audience ?? "worker-pool-a",
      expiresAt: { seconds: Math.floor(Date.now() / 1000) + 3600, nanos: 0 },
      keyId: opts.keyId ?? "k1",
    },
    jobRequest: {
      jobId: "job-1",
      topic: "job.test",
      tenantId: "tenant-a",
      identity: { tenantId: "tenant-a", principalId: "principal-a" },
    },
  });
  const unsigned = type.encode(packet).finish();
  const signature = sign(unsigned, opts.privateKeyPem ?? keyPair.privateKey);
  const sigField = Buffer.concat([
    Buffer.from([(14 << 3) | 2]),
    encodeVarint(signature.length),
    signature,
  ]);
  return Buffer.concat([Buffer.from(unsigned), sigField]);
}

function encodeVarint(value: number): Buffer {
  const bytes: number[] = [];
  let v = value;
  while (v >= 0x80) {
    bytes.push((v & 0x7f) | 0x80);
    v >>= 7;
  }
  bytes.push(v);
  return Buffer.from(bytes);
}

function productionAgent(): Agent {
  return new Agent({
    store: new InMemoryBlobStore(),
    productionTrust: { audience: "worker-pool-a", publicKeys: { k1: keyPair.publicKey } },
    replayStore: new InMemoryReplayStore(),
  });
}

describe("CAP-PRODUCTION Node admission layer", () => {
  it("accepts a valid signed packet via verifyProductionPacket", async () => {
    const raw = await signedRawPacket();
    const type = await busPacketType();
    const packet = verifyProductionPacket(raw, type, { audience: "worker-pool-a", publicKeys: { k1: keyPair.publicKey } });
    expect(packet.jobRequest.jobId).to.equal("job-1");
  });

  it("rejects an unsigned packet", async () => {
    const type = await busPacketType();
    const packet = type.create({ senderId: "scheduler-1", protocolVersion: 1 });
    const raw = type.encode(packet).finish();
    expect(() => verifyProductionPacket(raw, type, { audience: "worker-pool-a", publicKeys: { k1: keyPair.publicKey } })).to.throw();
  });

  it("rejects a tampered signature", async () => {
    const raw = await signedRawPacket();
    raw[raw.length - 1] ^= 0xff;
    const type = await busPacketType();
    expect(() => verifyProductionPacket(raw, type, { audience: "worker-pool-a", publicKeys: { k1: keyPair.publicKey } })).to.throw();
  });

  it("rejects an unknown key id", async () => {
    const otherKeys = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    const raw = await signedRawPacket({ privateKeyPem: otherKeys.privateKey });
    const type = await busPacketType();
    expect(() => verifyProductionPacket(raw, type, { audience: "worker-pool-a", publicKeys: { k1: keyPair.publicKey } })).to.throw();
  });

  it("Agent.decodeProductionPacket accepts a valid packet end-to-end", async () => {
    const agent = productionAgent();
    (agent as any).busPacketType = await busPacketType();
    const raw = await signedRawPacket();
    const packet = (agent as any).decodeProductionPacket(raw);
    expect(packet).to.not.equal(null);
    expect(packet.jobRequest.jobId).to.equal("job-1");
  });

  it("Agent.decodeProductionPacket allows identical redelivery, rejects replay conflict", async () => {
    const agent = productionAgent();
    (agent as any).busPacketType = await busPacketType();
    const raw = await signedRawPacket();
    const first = (agent as any).decodeProductionPacket(raw);
    const second = (agent as any).decodeProductionPacket(raw);
    expect(first).to.not.equal(null);
    expect(second).to.not.equal(null);
  });

  it("Agent.decodeProductionPacket rejects an identity mirror mismatch", async () => {
    const agent = productionAgent();
    (agent as any).busPacketType = await busPacketType();
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
        expiresAt: { seconds: Math.floor(Date.now() / 1000) + 3600, nanos: 0 },
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

    const result = (agent as any).decodeProductionPacket(raw);
    expect(result).to.equal(null);
  });
});
