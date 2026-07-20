import assert from "node:assert/strict";
import crypto from "node:crypto";
import { Events, type NatsConnection } from "nats";

import { Agent, InMemoryBlobStore } from "../src/runtime";
import {
  SUBJECT_HANDSHAKE,
  SUBJECT_HEARTBEAT,
  SUBJECT_RESULT,
  SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
} from "../src/protos";
import { RuntimeTrustConnection, waitFor } from "./runtime-trust-nats-support";
import {
  decodeBusPacket,
  privateKeyPem,
  signedDispatch,
} from "./runtime-trust-packet-support";
import { createTrustFixture } from "./worker-trust-runtime-support";

const silentLogger = { info() {}, warn() {}, error() {} };

function deferred<T>(): { promise: Promise<T>; resolve(value: T): void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

class StartupDisconnectConnection extends RuntimeTrustConnection {
  override async request(subject: string, data: Uint8Array, options?: { timeout?: number }) {
    if (subject === SUBJECT_WORKER_HANDSHAKE_CHALLENGE) {
      this.emitStatus({ type: Events.Disconnect, data: "127.0.0.1:4222" });
      await new Promise((resolve) => setImmediate(resolve));
      this.emitStatus({ type: Events.Reconnect, data: "127.0.0.1:4222" });
    }
    return super.request(subject, data, options);
  }
}

class StartupPublishDisconnectConnection extends RuntimeTrustConnection {
  override publish(subject: string, data: Uint8Array = new Uint8Array()): void {
    super.publish(subject, data);
    if (subject === SUBJECT_HANDSHAKE) {
      this.emitStatus({ type: Events.Disconnect, data: "127.0.0.1:4222" });
      this.emitStatus({ type: Events.Reconnect, data: "127.0.0.1:4222" });
    }
  }
}

describe("Agent authenticated worker trust startup", () => {
  it("rejects invalid trust settings before connecting", async () => {
    let connectCount = 0;
    const agent = new Agent({
      senderId: "worker-node",
      workerTrust: { mode: "audit" },
      connectFn: async () => {
        connectCount += 1;
        throw new Error("must not connect");
      },
    });
    agent.job("job.runtime.trust", async () => ({ ok: true }));

    await assert.rejects(agent.start(), /invalid worker trust mode/);
    assert.equal(connectCount, 0);
  });

  it("authenticates before subscription and attaches the session to every output", async () => {
    const fixture = createTrustFixture();
    const connection = new RuntimeTrustConnection(fixture);
    const store = new InMemoryBlobStore();
    await store.set("ctx:runtime-trust", Buffer.from('{"value":1}'));
    const wrongScheduler = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
    const agent = new Agent({
      senderId: fixture.config.workerId,
      privateKey: privateKeyPem(fixture),
      publicKeyMap: {
        [fixture.config.expectedSchedulerId]: wrongScheduler.publicKey.export({
          type: "spki",
          format: "pem",
        }).toString(),
      },
      store,
      heartbeatInterval: 5,
      logger: silentLogger,
      workerTrust: {
        mode: "enforce",
        config: fixture.config,
        timeoutMs: 100,
        retries: 1,
        renewMinIntervalMs: 30_000,
      },
      connectFn: async () => connection.asNatsConnection(),
    });
    let handled = 0;
    agent.job("job.runtime.trust", async () => {
      handled += 1;
      return { ok: true };
    });

    await agent.start();

    const challengeIndex = connection.events.indexOf("request:sys.worker.handshake.challenge");
    const watcherIndex = connection.events.indexOf("status:watch");
    const authenticateIndex = connection.events.indexOf("request:sys.worker.handshake.authenticate");
    const subscribeIndex = connection.events.indexOf("subscribe:job.runtime.trust");
    const readyIndex = connection.events.indexOf(`publish:${SUBJECT_HANDSHAKE}`);
    assert.equal(agent.sessionToken, "session-1");

    const ready = connection.published.find(({ subject }) => subject === SUBJECT_HANDSHAKE);
    assert.ok(ready);
    assert.equal((await decodeBusPacket(ready.data)).authToken, "session-1");

    await waitFor(() => connection.published.some(({ subject }) => subject === SUBJECT_HEARTBEAT));
    const heartbeat = connection.published.find(({ subject }) => subject === SUBJECT_HEARTBEAT);
    assert.ok(heartbeat);
    assert.equal((await decodeBusPacket(heartbeat.data)).authToken, "session-1");

    connection.deliver(
      "job.runtime.trust",
      await signedDispatch(fixture, "job.runtime.trust", "job-trust", "redis://ctx:runtime-trust")
    );
    await waitFor(() => connection.published.some(({ subject }) => subject === SUBJECT_RESULT));
    assert.equal(handled, 1, "trust pins must override a malicious legacy key map");
    const result = connection.published.find(({ subject }) => subject === SUBJECT_RESULT);
    assert.ok(result);
    assert.equal((await decodeBusPacket(result.data)).authToken, "session-1");

    await agent.close();
    assert.ok(watcherIndex >= 0 && watcherIndex < challengeIndex);
    assert.ok(challengeIndex < authenticateIndex);
    assert.ok(authenticateIndex < subscribeIndex);
    assert.ok(subscribeIndex < readyIndex);
  });

  it("cancels a pending connection without starting subscriptions", async () => {
    const fixture = createTrustFixture();
    const connection = new RuntimeTrustConnection(fixture);
    const pending = deferred<NatsConnection>();
    const agent = new Agent({
      senderId: fixture.config.workerId,
      store: new InMemoryBlobStore(),
      ioTimeoutMs: 10_000,
      logger: silentLogger,
      connectFn: async () => pending.promise,
    });
    agent.job("job.runtime.cancel-start", async () => ({ ok: true }));

    const starting = agent.start();
    const closing = agent.close();
    const outcome = await Promise.race([
      closing.then(() => "closed" as const),
      new Promise<"waiting">((resolve) => setTimeout(() => resolve("waiting"), 25)),
    ]);
    pending.resolve(connection.asNatsConnection());
    const startOutcome = await starting.then(() => "started" as const, () => "cancelled" as const);
    await closing;

    assert.equal(outcome, "closed");
    assert.equal(startOutcome, "cancelled");
    await waitFor(() => connection.isClosed());
    assert.equal(connection.events.some((event) => event.startsWith("subscribe:")), false);
  });

  it("aborts startup when transport continuity is lost during authentication", async () => {
    const fixture = createTrustFixture();
    const connection = new StartupDisconnectConnection(fixture);
    const agent = new Agent({
      senderId: fixture.config.workerId,
      store: new InMemoryBlobStore(),
      logger: silentLogger,
      workerTrust: { mode: "enforce", config: fixture.config, retries: 1 },
      connectFn: async () => connection.asNatsConnection(),
    });
    agent.job("job.runtime.interrupted-start", async () => ({ ok: true }));

    try {
      await assert.rejects(agent.start(), /transport.*startup/i);
    } finally {
      await agent.close();
    }
    assert.equal(connection.events.some((event) => event.startsWith("subscribe:")), false);
  });

  it("aborts startup when transport continuity is lost while publishing readiness", async () => {
    const fixture = createTrustFixture();
    const connection = new StartupPublishDisconnectConnection(fixture);
    const agent = new Agent({
      senderId: fixture.config.workerId,
      store: new InMemoryBlobStore(),
      logger: silentLogger,
      workerTrust: { mode: "enforce", config: fixture.config, retries: 1 },
      connectFn: async () => connection.asNatsConnection(),
    });
    agent.job("job.runtime.interrupted-ready", async () => ({ ok: true }));

    try {
      await assert.rejects(agent.start(), /transport.*startup/i);
    } finally {
      await agent.close();
    }
    assert.ok(connection.events.includes(`publish:${SUBJECT_HANDSHAKE}`));
    assert.ok(connection.events.includes("drain:job.runtime.interrupted-ready"));
  });
});
