import * as assert from "node:assert";
import * as crypto from "node:crypto";

import {
  WORKER_HANDSHAKE_AUDIENCE,
  createWorkerTrustConfig,
  parseWorkerTrustMode,
  validateWorkerTrustConfig,
} from "../src/worker-trust";

function p256Pair(): crypto.KeyPairKeyObjectResult {
  return crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
}

function validInput() {
  const worker = p256Pair();
  const scheduler = p256Pair();
  return {
    workerId: "worker-1",
    expectedAgentId: "agent-1",
    tenantId: "tenant-1",
    audience: WORKER_HANDSHAKE_AUDIENCE,
    proofKeyId: "worker-key-1",
    proofPrivateKey: worker.privateKey,
    expectedSchedulerId: "scheduler-1",
    schedulerPublicKeys: {
      "scheduler-key-1": scheduler.publicKey,
    } as Record<string, crypto.KeyObject>,
    sdkVersion: "cap-node/test",
  };
}

describe("worker trust configuration", () => {
  it("parses only explicit off, warn, and enforce modes", () => {
    assert.strictEqual(parseWorkerTrustMode(" OFF "), "off");
    assert.strictEqual(parseWorkerTrustMode("warn"), "warn");
    assert.strictEqual(parseWorkerTrustMode("Enforce"), "enforce");
    for (const invalid of ["", "optional", "true", undefined]) {
      assert.throws(() => parseWorkerTrustMode(invalid), /invalid worker trust mode/i);
    }
  });

  it("deep-copies and freezes scheduler trust roots", () => {
    const input = validInput();
    const config = createWorkerTrustConfig(input);
    const replacement = p256Pair().publicKey;
    input.schedulerPublicKeys["scheduler-key-1"] = replacement;
    input.schedulerPublicKeys["attacker"] = replacement;

    assert.notStrictEqual(config.schedulerPublicKeys["scheduler-key-1"], replacement);
    assert.deepStrictEqual(Object.keys(config.schedulerPublicKeys), ["scheduler-key-1"]);
    assert.strictEqual(Object.isFrozen(config), true);
    assert.strictEqual(Object.isFrozen(config.schedulerPublicKeys), true);
    assert.throws(
      () => ((config.schedulerPublicKeys as Record<string, crypto.KeyObject>).attacker = replacement),
      /read only|not extensible/i
    );
  });
});

describe("worker trust configuration validation", () => {
  it("rejects partial, empty, wrong-audience, and non-P256 configuration", () => {
    assert.throws(
      () => createWorkerTrustConfig(undefined as unknown as Parameters<typeof createWorkerTrustConfig>[0]),
      /configuration is required/i
    );
    const cases = [
      { field: "workerId", value: "" },
      { field: "expectedAgentId", value: " agent-1" },
      { field: "tenantId", value: "tenant-1 " },
      { field: "audience", value: "other" },
      { field: "proofKeyId", value: "" },
      { field: "expectedSchedulerId", value: "" },
      { field: "sdkVersion", value: "" },
      { field: "schedulerPublicKeys", value: {} },
    ] as const;
    for (const testCase of cases) {
      const input = { ...validInput(), [testCase.field]: testCase.value };
      assert.throws(() => createWorkerTrustConfig(input), /worker trust|scheduler/i);
    }
    const p384 = crypto.generateKeyPairSync("ec", { namedCurve: "secp384r1" });
    assert.throws(
      () => createWorkerTrustConfig({ ...validInput(), proofPrivateKey: p384.privateKey }),
      /P-256/i
    );
    const config = createWorkerTrustConfig(validInput());
    assert.doesNotThrow(() => validateWorkerTrustConfig(config));
  });
});
