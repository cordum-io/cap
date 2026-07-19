import assert from "node:assert/strict";
import crypto from "node:crypto";

import { Agent, InMemoryBlobStore } from "../src/runtime";
import { SUBJECT_HANDSHAKE, SUBJECT_HEARTBEAT, SUBJECT_RESULT } from "../src/protos";
import { RuntimeTrustConnection, waitFor } from "./runtime-trust-nats-support";
import {
  decodeBusPacket,
  privateKeyPem,
  signedDispatch,
} from "./runtime-trust-packet-support";
import { createTrustFixture } from "./worker-trust-runtime-support";

const silentLogger = { info() {}, warn() {}, error() {} };

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
    const authenticateIndex = connection.events.indexOf("request:sys.worker.handshake.authenticate");
    const subscribeIndex = connection.events.indexOf("subscribe:job.runtime.trust");
    const readyIndex = connection.events.indexOf(`publish:${SUBJECT_HANDSHAKE}`);
    assert.ok(challengeIndex >= 0 && challengeIndex < authenticateIndex);
    assert.ok(authenticateIndex < subscribeIndex);
    assert.ok(subscribeIndex < readyIndex);
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
  });
});
