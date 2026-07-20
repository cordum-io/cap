import * as crypto from "node:crypto";

import { Agent, InMemoryBlobStore } from "../src/runtime";
import { loadRoot } from "../src/protos";
import { PRODUCTION_PROFILE_VERSION, PRODUCTION_ALGORITHM, PRODUCTION_SIGNATURE_DOMAIN, type ProductionTrustStore } from "../src/production-signing";
import { InMemoryReplayStore, ReplayOutcome, type ReplayStore } from "../src/production-replay";

export const keyPair = crypto.generateKeyPairSync("ec", {
  namedCurve: "prime256v1",
  publicKeyEncoding: { type: "spki", format: "pem" },
  privateKeyEncoding: { type: "pkcs8", format: "pem" },
});

export async function busPacketType() {
  const root = await loadRoot();
  return root.lookupType("cordum.agent.v1.BusPacket");
}

export function sign(unsigned: Uint8Array, privateKeyPem: string): Buffer {
  const signer = crypto.createSign("sha256");
  signer.update(PRODUCTION_SIGNATURE_DOMAIN);
  signer.update(Buffer.from(unsigned));
  signer.end();
  return signer.sign(privateKeyPem);
}

export interface SignedPacketOptions {
  keyId?: string;
  audience?: string;
  messageId?: Buffer;
  privateKeyPem?: string;
  protocolVersion?: number;
  actorId?: string;
  metaActorId?: string;
  envTenantId?: string;
  tenantId?: string;
  senderId?: string;
  expiresInMs?: number;
  expiresSeconds?: number;
  expiresNanos?: number;
  contextPtr?: string;
}

export async function signedRawPacket(opts: SignedPacketOptions = {}): Promise<Buffer> {
  const type = await busPacketType();
  const packet = type.create({
    traceId: "trace-1",
    senderId: opts.senderId ?? "scheduler-1",
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    protocolVersion: opts.protocolVersion ?? 1,
    identity: { tenantId: opts.tenantId ?? "tenant-a", principalId: "principal-a", actorId: opts.actorId },
    signatureMetadata: {
      profileVersion: PRODUCTION_PROFILE_VERSION,
      algorithm: PRODUCTION_ALGORITHM,
      messageId: opts.messageId ?? Buffer.from("0123456789abcdef"),
      audience: opts.audience ?? "worker-pool-a",
      expiresAt: {
        seconds: opts.expiresSeconds
          ?? Math.floor((Date.now() + (opts.expiresInMs ?? 120_000)) / 1000),
        nanos: opts.expiresNanos ?? 0,
      },
      keyId: opts.keyId ?? "k1",
    },
    jobRequest: {
      jobId: "job-1",
      topic: "job.test",
      contextPtr: opts.contextPtr,
      tenantId: opts.tenantId ?? "tenant-a",
      meta: opts.metaActorId ? { actorId: opts.metaActorId } : undefined,
      env: opts.envTenantId ? { tenant_id: opts.envTenantId } : undefined,
      identity: { tenantId: opts.tenantId ?? "tenant-a", principalId: "principal-a", actorId: opts.actorId },
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

export function encodeVarint(value: number): Buffer {
  const bytes: number[] = [];
  let v = value;
  while (v >= 0x80) {
    bytes.push((v & 0x7f) | 0x80);
    v >>= 7;
  }
  bytes.push(v);
  return Buffer.from(bytes);
}

export class RecordingReplayStore implements ReplayStore {
  calls = 0;
  digests: Buffer[] = [];
  expiriesMs: number[] = [];

  constructor(private readonly outcome: ReplayOutcome = ReplayOutcome.First) {}

  admit(_tenant: string, _audience: string, _sender: string, _messageId: Uint8Array, digest: Uint8Array, expiresAtMs: number): ReplayOutcome {
    this.calls += 1;
    this.digests.push(Buffer.from(digest));
    this.expiriesMs.push(expiresAtMs);
    return this.outcome;
  }
}

export function productionAgent(
  replayStore: ReplayStore = new InMemoryReplayStore(),
  trust: Partial<ProductionTrustStore> = {},
): Agent {
  return new Agent({
    store: new InMemoryBlobStore(),
    productionTrust: {
      audience: "worker-pool-a",
      tenant: "tenant-a",
      sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
      ...trust,
    },
    replayStore,
  });
}

