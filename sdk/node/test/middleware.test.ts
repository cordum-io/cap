import assert from "node:assert";
import { z } from "zod";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { loadRoot } from "../src/protos";
import { encodeDeterministic } from "../src/codec";
import { Middleware, loggingMiddleware } from "../src/middleware";
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

async function waitFor(fn: () => boolean, timeoutMs = 1000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (fn()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error("timeout waiting for condition");
}

async function waitForPublishedSubject(mock: MockNatsConnection, subject: string, timeoutMs = 1000): Promise<void> {
  await waitFor(() => mock.published.some((entry) => entry.subject === subject), timeoutMs);
}

async function sendJob(mock: MockNatsConnection, topic: string, jobId: string, ctxKey: string) {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const packet = BusPacket.fromObject({
    traceId: "trace-mw",
    senderId: "client-mw",
    protocolVersion: 1,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: { jobId, topic, contextPtr: `redis://${ctxKey}` },
  });
  const payload = encodeDeterministic(BusPacket, packet);
  const cb = mock.subscriptions.get(topic);
  assert.ok(cb);
  cb?.(null, { data: payload } as Msg);
}

describe("Middleware (node)", () => {
  it("executes middleware in FIFO order", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const jobId = "job-mw-order";
    const ctxKey = `ctx:${jobId}`;
    await store.set(ctxKey, Buffer.from(JSON.stringify({ prompt: "hello" }), "utf-8"));

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-mw",
    });

    const order: string[] = [];

    const mwA: Middleware = async (ctx, next) => {
      order.push("A-before");
      const result = await next();
      order.push("A-after");
      return result;
    };

    const mwB: Middleware = async (ctx, next) => {
      order.push("B-before");
      const result = await next();
      order.push("B-after");
      return result;
    };

    agent.use(mwA, mwB);

    const Input = z.object({ prompt: z.string() });
    agent.job("job.mw", Input, async (_ctx, data) => {
      order.push("handler");
      return { summary: data.prompt };
    });

    await agent.start();
    await sendJob(mock, "job.mw", jobId, ctxKey);
    await waitForPublishedSubject(mock, "sys.job.result");

    assert.deepStrictEqual(order, ["A-before", "B-before", "handler", "B-after", "A-after"]);
    await agent.close();
  });

  it("allows middleware to short-circuit", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const jobId = "job-mw-short";
    const ctxKey = `ctx:${jobId}`;
    await store.set(ctxKey, Buffer.from(JSON.stringify({ prompt: "hello" }), "utf-8"));

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-short",
    });

    let handlerCalled = false;

    const shortCircuit: Middleware = async (_ctx, _next) => {
      return { shortcut: "yes" };
    };

    agent.use(shortCircuit);

    const Input = z.object({ prompt: z.string() });
    agent.job("job.short", Input, async (_ctx, data) => {
      handlerCalled = true;
      return { summary: data.prompt };
    });

    await agent.start();
    await sendJob(mock, "job.short", jobId, ctxKey);
    await waitForPublishedSubject(mock, "sys.job.result");

    assert.strictEqual(handlerCalled, false);

    const resultData = await store.get(`res:${jobId}`);
    assert.ok(resultData);
    const parsed = JSON.parse(resultData!.toString("utf-8"));
    assert.strictEqual(parsed.shortcut, "yes");

    await agent.close();
  });

  it("loggingMiddleware captures output", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const jobId = "job-mw-log";
    const ctxKey = `ctx:${jobId}`;
    await store.set(ctxKey, Buffer.from(JSON.stringify({ prompt: "hello" }), "utf-8"));

    const logs: string[] = [];
    const testLogger = {
      info: (...args: any[]) => logs.push(args.join(" ")),
      warn: (...args: any[]) => logs.push(args.join(" ")),
      error: (...args: any[]) => logs.push(args.join(" ")),
    };

    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-log",
      logger: testLogger,
    });

    agent.use(loggingMiddleware(testLogger));

    const Input = z.object({ prompt: z.string() });
    agent.job("job.log", Input, async (_ctx, data) => {
      return { summary: data.prompt };
    });

    await agent.start();
    await sendJob(mock, "job.log", jobId, ctxKey);
    await waitForPublishedSubject(mock, "sys.job.result");

    const logOutput = logs.join("\n");
    assert.ok(logOutput.includes(jobId), `log should contain job ID: ${logOutput}`);
    assert.ok(logOutput.includes("ok"), `log should contain 'ok': ${logOutput}`);

    await agent.close();
  });
});
