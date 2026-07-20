import crypto from "node:crypto";

import { encodeDeterministic, encodeUnsignedForSignature } from "../src/codec";
import { loadRoot } from "../src/protos";
import type { TrustFixture } from "./worker-trust-runtime-support";

export function privateKeyPem(fixture: TrustFixture): string {
  return fixture.config.proofPrivateKey.export({
    type: "pkcs8",
    format: "pem",
  }).toString();
}

export async function decodeBusPacket(data: Uint8Array): Promise<Record<string, unknown>> {
  const root = await loadRoot();
  const type = root.lookupType("cordum.agent.v1.BusPacket");
  return type.decode(data) as unknown as Record<string, unknown>;
}

export async function signedDispatch(
  fixture: TrustFixture,
  topic: string,
  jobId: string,
  contextPtr: string
): Promise<Uint8Array> {
  const root = await loadRoot();
  const type = root.lookupType("cordum.agent.v1.BusPacket");
  const packet = type.fromObject({
    traceId: `trace-${jobId}`,
    senderId: fixture.config.expectedSchedulerId,
    protocolVersion: 1,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: { jobId, topic, contextPtr },
  }) as { signature?: Uint8Array };
  const signer = crypto.createSign("sha256");
  signer.update(encodeUnsignedForSignature(type, packet));
  packet.signature = signer.sign(fixture.schedulerPrivateKey);
  return encodeDeterministic(type, packet);
}
