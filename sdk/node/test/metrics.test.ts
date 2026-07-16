import assert from "node:assert";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { loadRoot } from "../src/protos";
import { encodeDeterministic } from "../src/codec";
import type { MetricsHook } from "../src/metrics";
import { noopMetrics } from "../src/metrics";
import type { Msg, Subscription, SubscriptionOptions } from "nats";

class MockNatsConnection {
  published: Array<{ subject: string; data: Uint8Array }> = [];
  subscriptions: Map<string, NonNullable<SubscriptionOptions["callback"]>> = new Map();

  async publish(subject: string, data: Uint8Array): Promise<void> {
    this.published.push({ subject, data });
  }

  subscribe(subject: string, opts: SubscriptionOptions = {}): Subscription {
    if (opts.callback) {
      this.subscriptions.set(subject, opts.callback);
    }
    return { drain: async () => {} } as unknown as Subscription;
  }

  async drain(): Promise<void> {
    return;
  }
}

class RecordingMetrics implements MetricsHook {
  received: Array<{ jobId: string; topic: string }> = [];
  completed: Array<{ jobId: string; status: string }> = [];
  failed: Array<{ jobId: string; errorMsg: string }> = [];

  onJobReceived(jobId: string, topic: string): void {
    this.received.push({ jobId, topic });
  }
  onJobCompleted(jobId: string, _durationMs: number, status: string): void {
    this.completed.push({ jobId, status });
  }
  onJobFailed(jobId: string, errorMsg: string): void {
    this.failed.push({ jobId, errorMsg });
  }
  onHeartbeatSent(_workerId: string): void {}
}

async function waitFor(fn: () => boolean, timeoutMs = 1000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (fn()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error("timeout waiting for condition");
}

describe("MetricsHook", () => {
  it("receives OnJobReceived + OnJobCompleted on success", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const metrics = new RecordingMetrics();
    const jobId = "metrics-1";
    const ctxKey = `ctx:${jobId}`;
    await store.set(ctxKey, Buffer.from(JSON.stringify({ prompt: "test" }), "utf-8"));

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-m1",
      metrics,
    });

    agent.job("job.metrics", async (_ctx, data: any) => {
      return { summary: data.prompt };
    });

    await agent.start();

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const packetObj = BusPacket.fromObject({
      traceId: "t1",
      senderId: "c1",
      protocolVersion: 1,
      createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
      jobRequest: { jobId, topic: "job.metrics", contextPtr: `redis://${ctxKey}` },
    });
    const data = encodeDeterministic(BusPacket, packetObj);

    const cb = mock.subscriptions.get("job.metrics")!;
    cb(null, { data } as Msg);

    await waitFor(() => metrics.completed.length > 0);

    assert.strictEqual(metrics.received.length, 1);
    assert.strictEqual(metrics.received[0].jobId, jobId);
    assert.strictEqual(metrics.received[0].topic, "job.metrics");
    assert.strictEqual(metrics.completed.length, 1);
    assert.strictEqual(metrics.completed[0].jobId, jobId);
    assert.strictEqual(metrics.completed[0].status, "SUCCEEDED");
    assert.strictEqual(metrics.failed.length, 0);
  });

  it("receives OnJobFailed on failure", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const metrics = new RecordingMetrics();
    const jobId = "metrics-2";

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-m2",
      metrics,
    });

    agent.job("job.metrics.fail", async (_ctx, _data: any) => {
      return {};
    });

    await agent.start();

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    // Use a missing context key to trigger failure
    const packetObj = BusPacket.fromObject({
      traceId: "t2",
      senderId: "c2",
      protocolVersion: 1,
      createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
      jobRequest: { jobId, topic: "job.metrics.fail", contextPtr: "redis://missing-key" },
    });
    const data = encodeDeterministic(BusPacket, packetObj);

    const cb = mock.subscriptions.get("job.metrics.fail")!;
    cb(null, { data } as Msg);

    await waitFor(() => metrics.failed.length > 0);

    assert.strictEqual(metrics.received.length, 1);
    assert.strictEqual(metrics.failed.length, 1);
    assert.strictEqual(metrics.failed[0].jobId, jobId);
    assert.strictEqual(metrics.completed.length, 0);
  });

  it("noopMetrics has zero overhead", () => {
    noopMetrics.onJobReceived("j1", "t1");
    noopMetrics.onJobCompleted("j1", 100, "SUCCEEDED");
    noopMetrics.onJobFailed("j1", "err");
    noopMetrics.onHeartbeatSent("w1");
  });
});
