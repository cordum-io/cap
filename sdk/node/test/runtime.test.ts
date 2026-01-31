import assert from "node:assert";
import { z } from "zod";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { loadRoot } from "../src/protos";
import { encodeDeterministic } from "../src/codec";

class MockNatsConnection {
  published: Array<{ subject: string; data: Uint8Array }> = [];
  subscriptions: Map<string, (msg: any) => void> = new Map();

  async publish(subject: string, data: Uint8Array): Promise<void> {
    this.published.push({ subject, data });
  }

  subscribe(subject: string, _opts?: any, cb?: (msg: any) => void): any {
    if (cb) {
      this.subscriptions.set(subject, cb);
    }
    return {
      drain: async () => {},
    };
  }

  async drain(): Promise<void> {
    return;
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

describe("CAP runtime (node)", () => {
  it("handles job success with validation and result storage", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const jobId = "job-1";
    const ctxKey = `ctx:${jobId}`;
    await store.set(ctxKey, Buffer.from(JSON.stringify({ prompt: "hello" }), "utf-8"));

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-1",
    });

    const Input = z.object({ prompt: z.string() });
    const Output = z.object({ summary: z.string() });

    agent.job("job.test", Input, async (_ctx, data) => {
      return { summary: data.prompt.toUpperCase() };
    }, { outputSchema: Output });

    await agent.start();

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

    const packet = BusPacket.fromObject({
      traceId: "trace-1",
      senderId: "client-1",
      protocolVersion: 1,
      jobRequest: { jobId, topic: "job.test", contextPtr: `redis://${ctxKey}` },
    });
    const payload = encodeDeterministic(BusPacket, packet);

    const cb = mock.subscriptions.get("job.test");
    assert.ok(cb);
    cb?.({ data: payload });

    await waitFor(() => mock.published.length > 0);

    assert.strictEqual(mock.published[0].subject, "sys.job.result");
    const resultPacket = BusPacket.decode(mock.published[0].data) as any;
    assert.strictEqual(resultPacket.jobResult.status, 5); // JOB_STATUS_SUCCEEDED

    const resultData = await store.get(`res:${jobId}`);
    assert.ok(resultData);
    const parsed = JSON.parse(resultData!.toString("utf-8"));
    assert.strictEqual(parsed.summary, "HELLO");

    await agent.close();
  });

  it("fails on input validation errors", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const jobId = "job-2";
    const ctxKey = `ctx:${jobId}`;
    await store.set(ctxKey, Buffer.from(JSON.stringify({ wrong: "field" }), "utf-8"));

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-2",
    });

    const Input = z.object({ prompt: z.string() });
    const Output = z.object({ summary: z.string() });

    agent.job("job.validate", Input, async (_ctx, data) => {
      return { summary: data.prompt };
    }, { outputSchema: Output });

    await agent.start();

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const packet = BusPacket.fromObject({
      traceId: "trace-2",
      senderId: "client-2",
      protocolVersion: 1,
      jobRequest: { jobId, topic: "job.validate", contextPtr: `redis://${ctxKey}` },
    });
    const payload = encodeDeterministic(BusPacket, packet);

    const cb = mock.subscriptions.get("job.validate");
    assert.ok(cb);
    cb?.({ data: payload });

    await waitFor(() => mock.published.length > 0);
    const resultPacket = BusPacket.decode(mock.published[0].data) as any;
    assert.strictEqual(resultPacket.jobResult.status, 6); // JOB_STATUS_FAILED
    assert.ok(String(resultPacket.jobResult.errorMessage).includes("input validation failed"));

    await agent.close();
  });

  it("retries handler failures", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const jobId = "job-3";
    const ctxKey = `ctx:${jobId}`;
    await store.set(ctxKey, Buffer.from(JSON.stringify({ prompt: "retry" }), "utf-8"));

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-3",
      retries: 1,
    });

    const Input = z.object({ prompt: z.string() });
    const Output = z.object({ summary: z.string() });

    let attempts = 0;
    agent.job("job.retry", Input, async (_ctx, data) => {
      attempts += 1;
      if (attempts === 1) {
        throw new Error("boom");
      }
      return { summary: data.prompt };
    }, { outputSchema: Output });

    await agent.start();

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const packet = BusPacket.fromObject({
      traceId: "trace-3",
      senderId: "client-3",
      protocolVersion: 1,
      jobRequest: { jobId, topic: "job.retry", contextPtr: `redis://${ctxKey}` },
    });
    const payload = encodeDeterministic(BusPacket, packet);

    const cb = mock.subscriptions.get("job.retry");
    assert.ok(cb);
    cb?.({ data: payload });

    await waitFor(() => mock.published.length > 0);
    const resultPacket = BusPacket.decode(mock.published[0].data) as any;
    assert.strictEqual(resultPacket.jobResult.status, 5); // JOB_STATUS_SUCCEEDED
    assert.strictEqual(attempts, 2);

    await agent.close();
  });
});
