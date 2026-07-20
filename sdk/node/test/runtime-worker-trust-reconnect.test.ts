import assert from "node:assert/strict";
import crypto from "node:crypto";
import { Events } from "nats";

import { Agent, InMemoryBlobStore, type AgentOptions } from "../src/runtime";
import { SUBJECT_HANDSHAKE, SUBJECT_HEARTBEAT, SUBJECT_RESULT } from "../src/protos";
import { RuntimeTrustConnection, waitFor } from "./runtime-trust-nats-support";
import { decodeBusPacket, privateKeyPem, signedDispatch } from "./runtime-trust-packet-support";
import { createTrustFixture, type TrustFixture } from "./worker-trust-runtime-support";

const TOPIC = "job.runtime.reconnect";
const silentLogger = { info() {}, warn() {}, error() {} };

function legacyWrongKey(fixture: TrustFixture): Record<string, string> {
  const wrong = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  return {
    [fixture.config.expectedSchedulerId]: wrong.publicKey.export({
      type: "spki",
      format: "pem",
    }).toString(),
  };
}

function trustedOptions(
  fixture: TrustFixture,
  connection: RuntimeTrustConnection,
  store: InMemoryBlobStore,
  mode: "warn" | "enforce"
): AgentOptions {
  return {
    senderId: fixture.config.workerId,
    privateKey: privateKeyPem(fixture),
    publicKeyMap: legacyWrongKey(fixture),
    store,
    heartbeatInterval: 5,
    logger: silentLogger,
    workerTrust: {
      mode,
      config: fixture.config,
      timeoutMs: 100,
      retries: 1,
      renewMinIntervalMs: 30_000,
    },
    connectFn: async () => connection.asNatsConnection(),
  };
}

describe("Agent worker trust reconnect", () => {
  it("stops admission on disconnect and reauthenticates before resubscribe", async () => {
    const fixture = createTrustFixture();
    const connection = new RuntimeTrustConnection(fixture);
    const store = new InMemoryBlobStore();
    await store.set("ctx:reconnect", Buffer.from("{}"));
    const agent = new Agent(trustedOptions(fixture, connection, store, "enforce"));
    let handled = 0;
    agent.job(TOPIC, async () => { handled += 1; return { ok: true }; });
    await agent.start();

    connection.emitStatus({ type: Events.Disconnect, data: "127.0.0.1:4222" });
    await new Promise((resolve) => setImmediate(resolve));
    connection.deliver(
      TOPIC,
      await signedDispatch(fixture, TOPIC, "job-disconnected", "redis://ctx:reconnect")
    );
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(handled, 0);

    connection.emitStatus({ type: Events.Reconnect, data: "127.0.0.1:4222" });
    await waitFor(() => agent.sessionToken === "session-2");
    const subscribeEvents = connection.events.filter((event) => event === `subscribe:${TOPIC}`);
    assert.equal(subscribeEvents.length, 2);
    assert.equal(connection.events.filter((event) => event === `publish:${SUBJECT_HANDSHAKE}`).length, 2);

    connection.deliver(
      TOPIC,
      await signedDispatch(fixture, TOPIC, "job-reconnected", "redis://ctx:reconnect")
    );
    await waitFor(() => handled === 1);
    const results = connection.published.filter(({ subject }) => subject === SUBJECT_RESULT);
    assert.equal((await decodeBusPacket(results[results.length - 1]!.data)).authToken, "session-2");
    await waitFor(() => connection.published.filter(({ subject }) => subject === SUBJECT_HEARTBEAT)
      .some(({ data }) => data.length > 0));
    const heartbeats = connection.published.filter(({ subject }) => subject === SUBJECT_HEARTBEAT);
    assert.equal(
      (await decodeBusPacket(heartbeats[heartbeats.length - 1]!.data)).authToken,
      "session-2"
    );

    await agent.close();
  });
});
