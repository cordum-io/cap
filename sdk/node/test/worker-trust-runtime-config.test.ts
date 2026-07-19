import assert from "node:assert/strict";
import crypto from "node:crypto";

import {
  resolveRuntimeTrustSettings,
  type RuntimeTrustOptions,
} from "../src/worker-trust-runtime-config";
import {
  WORKER_HANDSHAKE_AUDIENCE,
  type WorkerTrustConfig,
} from "../src/worker-trust-contract";

function validConfig(workerId = "worker-node"): WorkerTrustConfig {
  const worker = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  const scheduler = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  return {
    workerId,
    expectedAgentId: "agent-node",
    tenantId: "tenant-node",
    audience: WORKER_HANDSHAKE_AUDIENCE,
    proofKeyId: "worker-key",
    proofPrivateKey: worker.privateKey,
    expectedSchedulerId: "scheduler-node",
    schedulerPublicKeys: { "scheduler-key": scheduler.publicKey },
    sdkVersion: "cap-node/v2",
  };
}

function resolve(options: RuntimeTrustOptions = {}) {
  return resolveRuntimeTrustSettings(options, "worker-node", "");
}

describe("Node runtime worker trust settings", () => {
  it("preserves the legacy disabled default only when no trust options exist", () => {
    const settings = resolve();

    assert.equal(settings.mode, "off");
    assert.equal(settings.config, undefined);
  });

  it("rejects an unknown mode before transport setup", () => {
    assert.throws(
      () => resolve({ mode: "audit" as RuntimeTrustOptions["mode"] }),
      /invalid worker trust mode/
    );
  });

  it("rejects enabled mode without complete configuration", () => {
    assert.throws(() => resolve({ mode: "enforce" }), /configuration is required/);
  });

  it("treats an empty scheduler key map as configured and invalid", () => {
    const config = { ...validConfig(), schedulerPublicKeys: {} };

    assert.throws(
      () => resolve({ mode: "warn", config }),
      /scheduler.*(key|trust)/
    );
  });

  it("binds the configured worker identity to the envelope sender", () => {
    assert.throws(
      () => resolve({ mode: "enforce", config: validConfig("other-worker") }),
      /sender.*worker trust identity/
    );
  });

  it("rejects trust tuning when mode is off", () => {
    assert.throws(
      () => resolve({ mode: "off", config: validConfig() }),
      /mode off conflicts/
    );
    assert.throws(
      () => resolve({ mode: "off", timeoutMs: 1000 }),
      /mode off conflicts/
    );
    assert.throws(
      () => resolve({ mode: "off", renewMinIntervalMs: 1000 }),
      /mode off conflicts/
    );
  });

  it("bounds timeouts, retries, and renewal intervals", () => {
    const config = validConfig();
    for (const options of [
      { mode: "enforce", config, timeoutMs: 0 },
      { mode: "enforce", config, retries: 0 },
      { mode: "enforce", config, retries: 11 },
      { mode: "enforce", config, renewMinIntervalMs: 0 },
    ] satisfies RuntimeTrustOptions[]) {
      assert.throws(() => resolve(options), /outside allowed bounds/);
    }
  });
});
