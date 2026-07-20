import { generateKeyPairSync } from "node:crypto";
import {
  WORKER_HANDSHAKE_AUDIENCE,
  WorkerTrustModes,
  buildChallengeRequest,
  createWorkerTrustConfig,
  marshalWorkerTrustPacket,
  parseWorkerTrustMode,
  unmarshalWorkerTrustPacket,
  validateWorkerTrustConfig,
  validateWorkerTrustPacket,
} from "cap-sdk-node";

async function exerciseWorkerTrust(): Promise<number> {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "prime256v1",
  });
  const config = createWorkerTrustConfig({
    workerId: "typed-worker",
    expectedAgentId: "typed-agent",
    tenantId: "typed-tenant",
    audience: WORKER_HANDSHAKE_AUDIENCE,
    proofKeyId: "typed-worker-key",
    proofPrivateKey: privateKey,
    expectedSchedulerId: "typed-scheduler",
    schedulerPublicKeys: { "typed-scheduler-key": publicKey },
    sdkVersion: "typed-consumer",
  });
  validateWorkerTrustConfig(config);
  if (parseWorkerTrustMode("enforce") !== WorkerTrustModes.ENFORCE) return 0;
  const packet = await buildChallengeRequest(config, {
    requestId: "typed-request",
    traceId: "typed-trace",
    purpose: 1,
    clientNonce: new Uint8Array(32).fill(3),
    createdAt: new Date(),
  });
  validateWorkerTrustPacket(packet);
  const encoded = await marshalWorkerTrustPacket(packet);
  const decoded = await unmarshalWorkerTrustPacket(encoded);
  return decoded.senderId === config.workerId ? encoded.length : 0;
}

void exerciseWorkerTrust();
