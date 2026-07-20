import assert from "node:assert/strict";

import { loadRoot, SUBJECT_RESULT } from "../src/protos";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { verifyInboundPacket } from "../src/security";
import { RuntimeTrustConnection, waitFor } from "./runtime-trust-nats-support";
import { privateKeyPem, signedDispatch } from "./runtime-trust-packet-support";
import { createTrustFixture, type TrustFixture } from "./worker-trust-runtime-support";

const TOPIC = "job.runtime.graceful-trust-close";
const silentLogger = { info() {}, warn() {}, error() {} };

function deferred<T>(): { promise: Promise<T>; resolve(value: T): void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

async function assertTerminalResult(
  fixture: TrustFixture,
  connection: RuntimeTrustConnection
): Promise<void> {
  const results = connection.published.filter(({ subject }) => subject === SUBJECT_RESULT);
  assert.equal(results.length, 1);
  const root = await loadRoot();
  const packetType = root.lookupType("cordum.agent.v1.BusPacket");
  const packet = packetType.decode(results[0].data) as { authToken?: string };
  assert.equal(packet.authToken, "session-1");
  const workerPem = fixture.workerPublicKey.export({ type: "spki", format: "pem" }).toString();
  assert.doesNotThrow(() => verifyInboundPacket(packetType, packet, {
    [fixture.config.workerId]: workerPem,
  }));
  assert.ok(connection.events.indexOf(`publish:${SUBJECT_RESULT}`) <
    connection.events.indexOf("drain:connection"));
}

describe("Agent authenticated graceful shutdown", () => {
  it("keeps the live session until an in-flight terminal result publishes", async () => {
    const fixture = createTrustFixture();
    const connection = new RuntimeTrustConnection(fixture);
    const store = new InMemoryBlobStore();
    await store.set("ctx:graceful-close", Buffer.from("{}"));
    const release = deferred<Record<string, boolean>>();
    let handlerStarted = false;
    const agent = new Agent({
      senderId: fixture.config.workerId,
      privateKey: privateKeyPem(fixture),
      store,
      heartbeatInterval: 30_000,
      logger: silentLogger,
      workerTrust: {
        mode: "enforce",
        config: fixture.config,
        timeoutMs: 50,
        retries: 1,
        renewMinIntervalMs: 30_000,
      },
      connectFn: async () => connection.asNatsConnection(),
    });
    agent.job(TOPIC, async () => {
      handlerStarted = true;
      return release.promise;
    });
    await agent.start();
    connection.deliver(TOPIC, await signedDispatch(
      fixture, TOPIC, "job-graceful-close", "redis://ctx:graceful-close"
    ));
    await waitFor(() => handlerStarted);

    let closeFinished = false;
    const closing = agent.close().then(() => { closeFinished = true; });
    await waitFor(() => connection.events.includes(`drain:${TOPIC}`));
    assert.equal(closeFinished, false);
    assert.equal(agent.sessionToken, "session-1");
    release.resolve({ accepted: true });
    await closing;

    await assertTerminalResult(fixture, connection);
    assert.equal(agent.sessionToken, undefined);
  });
});
