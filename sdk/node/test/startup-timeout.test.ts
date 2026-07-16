import assert from "node:assert/strict";
import { setTimeout as delay } from "node:timers/promises";
import type { NatsConnection } from "nats";
import { Agent, InMemoryBlobStore } from "../src/runtime";

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (value: T) => void;
}

function deferred<T>(): Deferred<T> {
  let resolve = (_value: T): void => undefined;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

async function waitFor(check: () => boolean, message: string): Promise<void> {
  const deadline = Date.now() + 500;
  while (Date.now() < deadline) {
    if (check()) return;
    await delay(10);
  }
  throw new Error(message);
}

describe("Agent startup timeout cleanup", () => {
  it("closes a NATS connection that resolves after the startup timeout", async () => {
    const pending = deferred<NatsConnection>();
    let closeCount = 0;
    const connection = {
      close: async (): Promise<void> => {
        closeCount += 1;
      },
    } as unknown as NatsConnection;
    const agent = new Agent({
      store: new InMemoryBlobStore(),
      connectFn: async () => pending.promise,
      ioTimeoutMs: 5,
    });
    agent.job("job.startup.timeout", async () => ({}));

    await assert.rejects(agent.start(), /nats connect timed out/);
    pending.resolve(connection);
    await waitFor(() => closeCount === 1, "late NATS connection was leaked");
    await agent.close();

    assert.equal(closeCount, 1);
  });
});
