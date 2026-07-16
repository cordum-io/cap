import assert from "node:assert";
import type {
  Msg,
  NatsConnection,
  NatsError,
  Subscription,
  SubscriptionOptions,
} from "nats";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { encodeDeterministic } from "../src/codec";
import { loadRoot } from "../src/protos";
import type { Logger } from "../src/logger";

type MessageCallback = NonNullable<SubscriptionOptions["callback"]>;

class RecordingStore extends InMemoryBlobStore {
  closeCount = 0;

  constructor(private readonly events: string[]) {
    super();
  }

  async close(): Promise<void> {
    this.closeCount += 1;
    this.events.push("store-close");
  }
}

class LifecycleNatsConnection {
  callback?: MessageCallback;
  connectionDrainCount = 0;
  failResultPublish = false;
  failSubscribe = false;
  subscriptionDrainCount = 0;

  constructor(private readonly events: string[]) {}

  publish(subject: string, _data: Uint8Array): void {
    if (subject === "sys.job.result") {
      this.events.push("result-publish");
      if (this.failResultPublish) {
        throw new Error("injected result publish failure");
      }
    }
  }

  subscribe(
    _subject: string,
    opts: SubscriptionOptions = {},
    legacyCallback?: MessageCallback
  ): Subscription {
    if (this.failSubscribe) {
      throw new Error("injected subscribe failure");
    }
    // The fallback isolates lifecycle behavior while other tests enforce the real API.
    this.callback = opts.callback ?? legacyCallback;
    return {
      drain: async () => {
        this.subscriptionDrainCount += 1;
        this.events.push("subscription-drain");
      },
    } as unknown as Subscription;
  }

  async drain(): Promise<void> {
    this.connectionDrainCount += 1;
    this.events.push("connection-drain");
  }
}

function deferred(): { promise: Promise<void>; resolve: () => void } {
  let resolve = (): void => undefined;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

async function jobMessage(topic: string, jobId: string): Promise<Msg> {
  const root = await loadRoot();
  const busPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const packet = busPacket.fromObject({
    traceId: `trace-${jobId}`,
    senderId: "lifecycle-client",
    protocolVersion: 1,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: { jobId, topic, contextPtr: `redis://ctx:${jobId}` },
  });
  return { data: encodeDeterministic(busPacket, packet) } as Msg;
}

async function tick(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 20));
}

function recordingLogger(errors: string[]): Logger {
  return {
    info: () => undefined,
    warn: () => undefined,
    error: (message, fields) => errors.push(`${message} ${String(fields?.error ?? "")}`),
  };
}

function assertShutdownOrder(events: string[]): void {
  const ordered = [
    "handler-finish",
    "result-publish",
    "connection-drain",
    "store-close",
  ];
  assert.deepStrictEqual(events.filter((event) => ordered.includes(event)), ordered);
}

async function waitsForInFlightBeforeClosing(): Promise<void> {
  const events: string[] = [];
  const store = new RecordingStore(events);
  const nc = new LifecycleNatsConnection(events);
  const releaseHandler = deferred();
  const handlerStarted = deferred();
  const topic = "job.lifecycle.blocked";
  const jobId = "lifecycle-blocked";
  await store.set(`ctx:${jobId}`, Buffer.from("{}"));

  const agent = new Agent({
    store,
    connectFn: async () => nc as unknown as NatsConnection,
    senderId: "lifecycle-worker",
    heartbeatInterval: 60_000,
  });
  agent.job(topic, async () => {
    events.push("handler-start");
    handlerStarted.resolve();
    await releaseHandler.promise;
    events.push("handler-finish");
    return { ok: true };
  });

  await agent.start();
  events.length = 0;
  assert.ok(nc.callback, "Agent must register a subscription callback");
  nc.callback(null, await jobMessage(topic, jobId));
  await handlerStarted.promise;

  let closeSettled = false;
  const closePromise = agent.close().then(() => {
    closeSettled = true;
  });
  try {
    await tick();
    assert.strictEqual(closeSettled, false, "close resolved while handler was blocked");
    assert.ok(events.includes("subscription-drain"), "intake was not drained first");
    assert.ok(!events.includes("store-close"), "store closed before the handler finished");
  } finally {
    releaseHandler.resolve();
    await closePromise;
  }

  assertShutdownOrder(events);
}

async function coalescesConcurrentClose(): Promise<void> {
  const events: string[] = [];
  const store = new RecordingStore(events);
  const nc = new LifecycleNatsConnection(events);
  const agent = new Agent({
    store,
    connectFn: async () => nc as unknown as NatsConnection,
    heartbeatInterval: 60_000,
  });
  agent.job("job.lifecycle.close", async () => ({}));
  await agent.start();

  await Promise.all([agent.close(), agent.close()]);

  assert.strictEqual(nc.connectionDrainCount, 1);
  assert.strictEqual(store.closeCount, 1);
}

async function rejectsDuplicateStarts(): Promise<void> {
  const events: string[] = [];
  const store = new RecordingStore(events);
  let connectCount = 0;
  const agent = new Agent({
    store,
    connectFn: async () => {
      connectCount += 1;
      return new LifecycleNatsConnection(events) as unknown as NatsConnection;
    },
    heartbeatInterval: 60_000,
  });
  agent.job("job.lifecycle.start", async () => ({}));
  await agent.start();
  try {
    await assert.rejects(agent.start(), /already started/i);
    assert.strictEqual(connectCount, 1);
  } finally {
    await agent.close();
  }
}

async function cleansUpFailedStartup(): Promise<void> {
  const events: string[] = [];
  const store = new RecordingStore(events);
  const nc = new LifecycleNatsConnection(events);
  nc.failSubscribe = true;
  const agent = new Agent({
    store,
    connectFn: async () => nc as unknown as NatsConnection,
    heartbeatInterval: 60_000,
  });
  agent.job("job.lifecycle.start-failure", async () => ({}));

  await assert.rejects(agent.start(), /injected subscribe failure/);

  assert.strictEqual(nc.connectionDrainCount, 1);
  assert.strictEqual(store.closeCount, 1);
  await agent.close();
  assert.strictEqual(nc.connectionDrainCount, 1);
  assert.strictEqual(store.closeCount, 1);
}

async function handlesSubscriptionCallbackErrors(): Promise<void> {
  const errors: string[] = [];
  const nc = new LifecycleNatsConnection([]);
  let handlerCalls = 0;
  const agent = new Agent({
    store: new InMemoryBlobStore(),
    connectFn: async () => nc as unknown as NatsConnection,
    heartbeatInterval: 60_000,
    logger: recordingLogger(errors),
  });
  agent.job("job.lifecycle.callback-error", async () => {
    handlerCalls += 1;
    return {};
  });
  await agent.start();
  try {
    assert.ok(nc.callback);
    nc.callback(
      new Error("injected callback failure") as unknown as NatsError,
      {} as Msg
    );
    await tick();
    assert.strictEqual(handlerCalls, 0);
    assert.ok(errors.some((entry) => entry.includes("injected callback failure")));
  } finally {
    await agent.close();
  }
}

async function catchesRejectedMessageProcessing(): Promise<void> {
  const errors: string[] = [];
  const unhandled: unknown[] = [];
  const nc = new LifecycleNatsConnection([]);
  const store = new InMemoryBlobStore();
  const jobId = "lifecycle-rejection";
  await store.set(`ctx:${jobId}`, Buffer.from("{}"));
  const agent = new Agent({
    store,
    connectFn: async () => nc as unknown as NatsConnection,
    heartbeatInterval: 60_000,
    logger: recordingLogger(errors),
  });
  agent.job("job.lifecycle.rejection", async () => ({ ok: true }));
  const capture = (reason: unknown): void => {
    unhandled.push(reason);
  };
  process.on("unhandledRejection", capture);
  try {
    await agent.start();
    nc.failResultPublish = true;
    assert.ok(nc.callback);
    nc.callback(null, await jobMessage("job.lifecycle.rejection", jobId));
    await tick();
    assert.deepStrictEqual(unhandled, []);
    assert.ok(errors.some((entry) => entry.includes("message processing failed")));
  } finally {
    process.off("unhandledRejection", capture);
    nc.failResultPublish = false;
    await agent.close();
  }
}

describe("Agent lifecycle", () => {
  it(
    "waits for an in-flight handler and result before closing resources",
    waitsForInFlightBeforeClosing
  );
  it("coalesces concurrent close calls", coalescesConcurrentClose);
  it("rejects duplicate starts without opening another connection", rejectsDuplicateStarts);
  it("cleans up an opened connection and store when startup fails", cleansUpFailedStartup);
  it(
    "logs subscription callback errors without invoking a handler",
    handlesSubscriptionCallbackErrors
  );
  it(
    "catches rejected message processing instead of leaking an unhandled rejection",
    catchesRejectedMessageProcessing
  );
});
