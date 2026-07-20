import assert from "node:assert/strict";
import { setTimeout as delay } from "node:timers/promises";
import type { NatsConnection, Subscription } from "nats";
import { Agent, InMemoryBlobStore } from "../src/runtime";

class RecordingStore extends InMemoryBlobStore {
  closeCount = 0;

  async close(): Promise<void> {
    this.closeCount += 1;
  }
}

class StalledDrainConnection {
  closeCount = 0;
  failUnsubscribe = false;
  unsubscribeCount = 0;

  publish(): void {}

  subscribe(): Subscription {
    return {
      drain: async () => new Promise<void>(() => undefined),
      unsubscribe: () => {
        this.unsubscribeCount += 1;
        if (this.failUnsubscribe) throw new Error("injected unsubscribe failure");
      },
    } as unknown as Subscription;
  }

  async drain(): Promise<void> {
    return new Promise<void>(() => undefined);
  }

  async close(): Promise<void> {
    this.closeCount += 1;
  }
}

async function failAfter(ms: number): Promise<never> {
  await delay(ms);
  throw new Error("Agent.close stalled on a NATS drain");
}

const silentLogger = {
  info: () => undefined,
  warn: () => undefined,
  error: () => undefined,
};

function newAgent(store: RecordingStore, connection: StalledDrainConnection): Agent {
  const agent = new Agent({
    store,
    connectFn: async () => connection as unknown as NatsConnection,
    heartbeatInterval: 60_000,
    ioTimeoutMs: 10,
    logger: silentLogger,
  });
  agent.job("job.shutdown.timeout", async () => ({}));
  return agent;
}

describe("bounded NATS shutdown", () => {
  it("falls back to unsubscribe and close when drains do not settle", async () => {
    const store = new RecordingStore();
    const connection = new StalledDrainConnection();
    const agent = newAgent(store, connection);
    await agent.start();

    await Promise.race([agent.close(), failAfter(200)]);

    assert.equal(connection.unsubscribeCount, 1);
    assert.equal(connection.closeCount, 1);
    assert.equal(store.closeCount, 1);
  });

  it("continues cleanup when the unsubscribe fallback throws", async () => {
    const store = new RecordingStore();
    const connection = new StalledDrainConnection();
    connection.failUnsubscribe = true;
    const agent = newAgent(store, connection);
    await agent.start();

    await agent.close();

    assert.equal(connection.unsubscribeCount, 1);
    assert.equal(connection.closeCount, 1);
    assert.equal(store.closeCount, 1);
  });
});
