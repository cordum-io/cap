import assert from "node:assert/strict";
import * as crypto from "node:crypto";
import type { KeyObject } from "node:crypto";
import type { Type } from "protobufjs";
import { encodeUnsignedForSignature } from "../src/codec";
import { SignatureInvalidError } from "../src/errors";
import { DEFAULT_PROTOCOL_VERSION, loadRoot } from "../src/protos";
import { verifyInboundPacket } from "../src/security";

interface SigningKeys {
  privateKey: KeyObject;
  publicKey: KeyObject;
}

async function signedPacket(keys: SigningKeys): Promise<{
  packetType: Type;
  packet: unknown;
}> {
  const root = await loadRoot();
  const packetType = root.lookupType("cordum.agent.v1.BusPacket");
  const packet = packetType.fromObject({
    traceId: "trace-key-policy",
    senderId: "trusted-client",
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: {
      jobId: "job-key-policy",
      topic: "job.security.key-policy",
      contextPtr: "redis://ctx:job-key-policy",
    },
  }) as unknown as { signature?: Uint8Array };
  const signer = crypto.createSign("sha256");
  signer.update(encodeUnsignedForSignature(packetType, packet));
  packet.signature = signer.sign(keys.privateKey);
  return { packetType, packet };
}

async function assertKeyRejected(keys: SigningKeys): Promise<void> {
  const { packetType, packet } = await signedPacket(keys);
  const publicKey = keys.publicKey.export({ type: "spki", format: "pem" }).toString();
  assert.throws(
    () => verifyInboundPacket(packetType, packet, { "trusted-client": publicKey }),
    SignatureInvalidError
  );
}

describe("trusted inbound key policy", () => {
  it("rejects an RSA trusted key", async () => {
    await assertKeyRejected(crypto.generateKeyPairSync("rsa", { modulusLength: 2048 }));
  });

  it("rejects a non-P-256 EC trusted key", async () => {
    await assertKeyRejected(
      crypto.generateKeyPairSync("ec", { namedCurve: "secp384r1" })
    );
  });
});
