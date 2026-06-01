import assert from "node:assert";
import * as crypto from "crypto";
import { connect } from "nats";
import {
  emitHeartbeat,
  heartbeatLoop,
  heartbeatPayload,
  heartbeatPayloadWithMemory,
  heartbeatPayloadWithProgress,
} from "../src/heartbeat";
import { encodeDeterministic, encodeUnsignedForSignature } from "../src/codec";
import { SUBJECT_HEARTBEAT, loadRoot } from "../src/protos";
import { MockNatsConnection } from "../src/testing";
import { validateBusPacket } from "../src/validate";

class RecordingMetrics {
  public readonly heartbeats: string[] = [];

  onJobReceived(): void {}
  onJobCompleted(): void {}
  onJobFailed(): void {}
  onHeartbeatSent(workerId: string): void {
    this.heartbeats.push(workerId);
  }
}

async function waitFor(fn: () => boolean, timeoutMs = 1000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (fn()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error("timeout waiting for condition");
}

describe("heartbeat helpers", () => {
  it("builds heartbeat payload with defaults", async () => {
    const packet = await heartbeatPayload("worker-1", "pool-a", 2, 8, 42.5, " token-123 ");

    assert.strictEqual(packet.senderId, "worker-1");
    assert.strictEqual(packet.traceId, "worker-1");
    assert.strictEqual(packet.protocolVersion, 1);
    assert.strictEqual(packet.heartbeat.workerId, "worker-1");
    assert.strictEqual(packet.heartbeat.pool, "pool-a");
    assert.strictEqual(packet.heartbeat.activeJobs, 2);
    assert.strictEqual(packet.heartbeat.maxParallelJobs, 8);
    assert.strictEqual(packet.heartbeat.cpuLoad, 42.5);
    assert.strictEqual(packet.heartbeat.memoryLoad, 0);
    assert.strictEqual(packet.heartbeat.progressPct, 0);
    assert.strictEqual(packet.heartbeat.lastMemo, "");
    assert.strictEqual(packet.heartbeat.authToken, "token-123");
    assert.deepStrictEqual(validateBusPacket(packet), []);
  });

  it("builds heartbeat payload with sanitized agent name", async () => {
    const packet = await heartbeatPayload("worker-an", "pool-an", 1, 4, 42, "", "  Claude Code\n— Billing  ");

    // Serializes/deserializes; sanitized (trimmed + whitespace-collapsed).
    assert.strictEqual(packet.heartbeat.agentName, "Claude Code — Billing");
    // agent_name is a DISPLAY label, never an auth authority.
    assert.strictEqual(packet.heartbeat.authToken ?? "", "");
  });

  it("omits agent name when blank (wire-compatible with old clients)", async () => {
    const packet = await heartbeatPayload("worker-x", "pool-x", 0, 1, 0);
    assert.strictEqual(packet.heartbeat.agentName ?? "", "");
  });

  it("builds heartbeat payload with memory", async () => {
    const packet = await heartbeatPayloadWithMemory("worker-2", "pool-b", 1, 4, 22.2, 77.7);

    assert.strictEqual(packet.senderId, "worker-2");
    assert.strictEqual(packet.traceId, "worker-2");
    assert.strictEqual(packet.protocolVersion, 1);
    assert.strictEqual(packet.heartbeat.workerId, "worker-2");
    assert.strictEqual(packet.heartbeat.memoryLoad, 77.7);
    assert.strictEqual(packet.heartbeat.progressPct, 0);
    assert.deepStrictEqual(validateBusPacket(packet), []);
  });

  it("builds heartbeat payload with progress fields", async () => {
    const packet = await heartbeatPayloadWithProgress("worker-3", "pool-c", 9, 10, 80, 55, 67, "checkpoint-7");

    assert.strictEqual(packet.senderId, "worker-3");
    assert.strictEqual(packet.traceId, "worker-3");
    assert.strictEqual(packet.protocolVersion, 1);
    assert.strictEqual(packet.heartbeat.workerId, "worker-3");
    assert.strictEqual(packet.heartbeat.pool, "pool-c");
    assert.strictEqual(packet.heartbeat.activeJobs, 9);
    assert.strictEqual(packet.heartbeat.maxParallelJobs, 10);
    assert.strictEqual(packet.heartbeat.cpuLoad, 80);
    assert.strictEqual(packet.heartbeat.memoryLoad, 55);
    assert.strictEqual(packet.heartbeat.progressPct, 67);
    assert.strictEqual(packet.heartbeat.lastMemo, "checkpoint-7");
    assert.deepStrictEqual(validateBusPacket(packet), []);
  });

  it("emits heartbeat without signature", async () => {
    const nc = new MockNatsConnection();
    const packet = await heartbeatPayload("worker-emit", "pool-emit", 1, 2, 10);

    await emitHeartbeat(nc as any, packet);

    assert.strictEqual(nc.published.length, 1);
    assert.strictEqual(nc.published[0].subject, SUBJECT_HEARTBEAT);

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(nc.published[0].data) as any;
    assert.strictEqual(decoded.senderId, "worker-emit");
    assert.strictEqual(decoded.traceId, "worker-emit");
    assert.strictEqual(decoded.signature?.length ?? 0, 0);
  });

  it("emits heartbeat with verifiable signature", async () => {
    const nc = new MockNatsConnection();
    const keyPair = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });

    const packet = await heartbeatPayload("worker-sign", "pool-sign", 1, 2, 10);
    await emitHeartbeat(nc as any, packet, keyPair.privateKey);

    assert.strictEqual(nc.published.length, 1);
    assert.strictEqual(nc.published[0].subject, SUBJECT_HEARTBEAT);

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(nc.published[0].data) as any;
    assert.ok(decoded.signature);
    assert.strictEqual(decoded.traceId, "worker-sign");

    const unsigned = encodeUnsignedForSignature(BusPacket, decoded);
    const verify = crypto.createVerify("sha256");
    verify.update(unsigned);
    assert.strictEqual(verify.verify(keyPair.publicKey, decoded.signature), true);
  });

  it("runs heartbeat loop and stops cleanly", async () => {
    const nc = new MockNatsConnection();
    const metrics = new RecordingMetrics();
    let calls = 0;

    const handle = heartbeatLoop(
      nc as any,
      async () => {
        calls += 1;
        return heartbeatPayload("worker-loop", "pool-loop", calls, 3, 0);
      },
      { interval: 50, metrics: metrics as any }
    );

    await waitFor(() => nc.published.length >= 2, 1000);
    handle.stop();

    assert.ok(calls >= 2);
    assert.strictEqual(nc.published[0].subject, SUBJECT_HEARTBEAT);
    assert.ok(metrics.heartbeats.length >= 2);
    assert.ok(metrics.heartbeats.every((id) => id === "worker-loop"));
  });

  it("continues heartbeat loop after payload errors", async () => {
    const nc = new MockNatsConnection();
    let calls = 0;
    const handle = heartbeatLoop(
      nc as any,
      async () => {
        calls += 1;
        if (calls === 1) {
          throw new Error("transient");
        }
        return heartbeatPayload("worker-retry", "pool-retry", 0, 1, 0);
      },
      { interval: 25 }
    );

    await waitFor(() => nc.published.length >= 1, 1000);
    handle.stop();
    handle.stop(); // idempotent

    assert.ok(calls >= 2);
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(nc.published[0].data) as any;
    assert.strictEqual(decoded.traceId, "worker-retry");
    assert.strictEqual(decoded.heartbeat.workerId, "worker-retry");
  });

  it("produces valid packet for zero/empty values", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const packet = await heartbeatPayloadWithProgress("worker-zero", "", 0, 0, 0, 0, 0, "");
    const encoded = encodeDeterministic(BusPacket, packet);
    const decoded = BusPacket.decode(encoded) as any;

    assert.strictEqual(decoded.senderId, "worker-zero");
    assert.strictEqual(decoded.traceId, "worker-zero");
    assert.strictEqual(decoded.heartbeat.workerId, "worker-zero");
    assert.strictEqual(decoded.heartbeat.activeJobs, 0);
    assert.strictEqual(decoded.heartbeat.maxParallelJobs, 0);
    assert.strictEqual(decoded.heartbeat.cpuLoad, 0);
    assert.strictEqual(decoded.heartbeat.memoryLoad, 0);
  });

  it("integration: publishes heartbeat on NATS subject", async function () {
    this.timeout(10_000);

    const natsUrl = process.env.CAP_TEST_NATS_URL ?? "nats://127.0.0.1:4222";
    let nc: any;
    try {
      nc = await connect({
        servers: natsUrl,
        name: "cap-heartbeat-integration",
        maxReconnectAttempts: 0,
        timeout: 1000,
      });
    } catch (err) {
      this.skip();
      return;
    }

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const keyPair = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    const workerId = `worker-int-${Date.now()}`;
    let matchedPacket: any;

    const sub = nc.subscribe(SUBJECT_HEARTBEAT, {
      callback: (_err: Error | null, msg: any) => {
        const packet = BusPacket.decode(msg.data) as any;
        if (packet?.heartbeat?.workerId === workerId) {
          matchedPacket = packet;
        }
      },
    });

    const handle = heartbeatLoop(
      nc,
      async () => heartbeatPayloadWithProgress(workerId, "pool-int", 2, 5, 33, 55, 80, "integration"),
      { interval: 50, privateKey: keyPair.privateKey }
    );

    try {
      await waitFor(() => matchedPacket != null, 3000);

      assert.strictEqual(matchedPacket.senderId, workerId);
      assert.strictEqual(matchedPacket.traceId, workerId);
      assert.strictEqual(matchedPacket.protocolVersion, 1);
      assert.strictEqual(matchedPacket.heartbeat.workerId, workerId);
      assert.strictEqual(matchedPacket.heartbeat.pool, "pool-int");
      assert.strictEqual(matchedPacket.heartbeat.activeJobs, 2);
      assert.strictEqual(matchedPacket.heartbeat.maxParallelJobs, 5);
      assert.strictEqual(matchedPacket.heartbeat.progressPct, 80);
      assert.strictEqual(matchedPacket.heartbeat.lastMemo, "integration");
      assert.ok(matchedPacket.signature);

      const unsigned = encodeUnsignedForSignature(BusPacket, matchedPacket);
      const verify = crypto.createVerify("sha256");
      verify.update(unsigned);
      assert.strictEqual(verify.verify(keyPair.publicKey, matchedPacket.signature), true);
    } finally {
      handle.stop();
      sub.unsubscribe();
      await nc.drain();
    }
  });
});
