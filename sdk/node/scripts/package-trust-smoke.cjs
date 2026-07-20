"use strict";

const assert = require("node:assert/strict");
const { generateKeyPairSync } = require("node:crypto");
const sdk = require("cap-sdk-node");

async function main() {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "prime256v1",
  });
  const config = sdk.createWorkerTrustConfig({
    workerId: "artifact-worker",
    expectedAgentId: "artifact-agent",
    tenantId: "artifact-tenant",
    audience: sdk.WORKER_HANDSHAKE_AUDIENCE,
    proofKeyId: "worker-key",
    proofPrivateKey: privateKey,
    expectedSchedulerId: "artifact-scheduler",
    schedulerPublicKeys: { "scheduler-key": publicKey },
    sdkVersion: "artifact-smoke",
  });
  sdk.validateWorkerTrustConfig(config);
  assert.equal(sdk.parseWorkerTrustMode("enforce"), sdk.WorkerTrustModes.ENFORCE);
  const request = await sdk.buildChallengeRequest(config, {
    requestId: "artifact-request",
    traceId: "artifact-trace",
    purpose: 1,
    clientNonce: new Uint8Array(32).fill(7),
    createdAt: new Date(),
  });
  sdk.validateWorkerTrustPacket(request);
  const encoded = await sdk.marshalWorkerTrustPacket(request);
  const decoded = await sdk.unmarshalWorkerTrustPacket(encoded);
  assert.equal(decoded.senderId, config.workerId);
  assert.equal(decoded.protocolVersion, 1);
  assert.ok(decoded.signature.length > 0);
  assert.ok(encoded.length > 0 && encoded.length <= 65536);
  console.log(JSON.stringify({ workerTrust: "ok", encodedBytes: encoded.length }));
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
