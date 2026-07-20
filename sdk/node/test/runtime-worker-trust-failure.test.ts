import assert from "node:assert/strict";
import { ErrorCode, NatsError } from "nats";

import { Agent, InMemoryBlobStore } from "../src/runtime";
import {
  SUBJECT_HANDSHAKE,
  SUBJECT_RESULT,
  SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
  SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
} from "../src/protos";
import {
  marshalWorkerTrustPacket,
  unmarshalWorkerTrustPacket,
} from "../src/worker-trust";
import { signWorkerTrustPacket } from "../src/trust-signing";
import { RuntimeTrustConnection, waitFor } from "./runtime-trust-nats-support";
import { decodeBusPacket, privateKeyPem, signedDispatch } from "./runtime-trust-packet-support";
import { createTrustFixture, type TrustFixture } from "./worker-trust-runtime-support";

const TOPIC = "job.runtime.failure";
const silentLogger = { info() {}, warn() {}, error() {} };

class RejectingTrustConnection extends RuntimeTrustConnection {
  override async request(subject: string): Promise<{ data: Uint8Array }> {
    this.events.push(`request:${subject}`);
    throw NatsError.errorForCode(ErrorCode.NoResponders);
  }
}

class TamperingTrustConnection extends RuntimeTrustConnection {
  override async request(
    subject: string,
    data: Uint8Array,
    options?: { timeout?: number }
  ): Promise<{ data: Uint8Array }> {
    const response = await super.request(subject, data, options);
    if (subject !== SUBJECT_WORKER_HANDSHAKE_CHALLENGE) return response;
    const tampered = response.data.slice();
    tampered[tampered.length - 1] = (tampered[tampered.length - 1] ?? 0) ^ 1;
    return { data: tampered };
  }
}

class SignedRejectingTrustConnection extends RuntimeTrustConnection {
  constructor(private readonly trustFixture: TrustFixture) {
    super(trustFixture);
  }

  override async request(
    subject: string,
    data: Uint8Array,
    options?: { timeout?: number }
  ): Promise<{ data: Uint8Array }> {
    const response = await super.request(subject, data, options);
    if (subject !== SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE) return response;
    const packet = await unmarshalWorkerTrustPacket(response.data);
    if (!("workerHandshakeResult" in packet)) throw new Error("result expected");
    const rejected = {
      ...packet,
      authToken: "",
      workerHandshakeResult: {
        ...packet.workerHandshakeResult,
        accepted: false,
        rejectionReason: 1,
        tokenExpiresAt: undefined,
      },
    };
    const signed = await signWorkerTrustPacket(rejected, {
      "scheduler-key": this.trustFixture.schedulerPrivateKey,
    });
    return { data: await marshalWorkerTrustPacket(signed) };
  }
}

class HangingTrustConnection extends RuntimeTrustConnection {
  override async request(subject: string): Promise<{ data: Uint8Array }> {
    this.events.push(`request:${subject}`);
    return new Promise(() => undefined);
  }
}

function agentFor(
  fixture: TrustFixture,
  connection: RuntimeTrustConnection,
  store: InMemoryBlobStore,
  mode: "warn" | "enforce"
): Agent {
  return new Agent({
    senderId: fixture.config.workerId,
    privateKey: privateKeyPem(fixture),
    store,
    heartbeatInterval: 10,
    logger: silentLogger,
    workerTrust: {
      mode,
      config: fixture.config,
      timeoutMs: 50,
      retries: 1,
      renewMinIntervalMs: 30_000,
    },
    connectFn: async () => connection.asNatsConnection(),
  });
}

async function assertWarnSecurityFailure(
  fixture: TrustFixture,
  connection: RuntimeTrustConnection
): Promise<void> {
  const agent = agentFor(fixture, connection, new InMemoryBlobStore(), "warn");
  agent.job(TOPIC, async () => ({ ok: true }));

  await assert.rejects(agent.start(), /authenticated worker trust failed/);

  assert.equal(agent.sessionToken, undefined);
  assert.equal(connection.events.some((event) => event.startsWith("subscribe:")), false);
  assert.equal(connection.events.includes(`publish:${SUBJECT_HANDSHAKE}`), false);
  await agent.close();
}

describe("Agent worker trust failure paths", () => {
  it("warn mode proceeds tokenless but keeps scheduler signature pins", async () => {
    const fixture = createTrustFixture();
    const connection = new RejectingTrustConnection(fixture);
    const store = new InMemoryBlobStore();
    await store.set("ctx:warn", Buffer.from("{}"));
    const agent = agentFor(fixture, connection, store, "warn");
    let handled = 0;
    agent.job(TOPIC, async () => { handled += 1; return { ok: true }; });

    await agent.start();
    assert.equal(agent.sessionToken, undefined);
    assert.ok(connection.events.indexOf(`request:sys.worker.handshake.challenge`) <
      connection.events.indexOf(`subscribe:${TOPIC}`));
    const ready = connection.published.find(({ subject }) => subject === SUBJECT_HANDSHAKE);
    assert.ok(ready);
    assert.equal(String((await decodeBusPacket(ready.data)).authToken ?? ""), "");

    const imposter = createTrustFixture();
    connection.deliver(
      TOPIC,
      await signedDispatch(imposter, TOPIC, "job-imposter", "redis://ctx:warn")
    );
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(handled, 0);

    connection.deliver(
      TOPIC,
      await signedDispatch(fixture, TOPIC, "job-warn", "redis://ctx:warn")
    );
    await waitFor(() => handled === 1);
    const result = connection.published.find(({ subject }) => subject === SUBJECT_RESULT);
    assert.ok(result);
    assert.equal(String((await decodeBusPacket(result.data)).authToken ?? ""), "");
    await agent.close();
  });

  it("close cancels admission while the trust request is hung", async () => {
    const fixture = createTrustFixture();
    const connection = new HangingTrustConnection(fixture);
    const agent = agentFor(fixture, connection, new InMemoryBlobStore(), "enforce");
    agent.job(TOPIC, async () => ({ ok: true }));
    const starting = agent.start();
    await waitFor(() => connection.events.includes("request:sys.worker.handshake.challenge"));

    const closing = agent.close();
    const [startResult, closeResult] = await Promise.allSettled([starting, closing]);

    assert.equal(startResult.status, "rejected");
    assert.equal(closeResult.status, "fulfilled");
    assert.equal(connection.events.some((event) => event.startsWith("subscribe:")), false);
    assert.equal(connection.events.includes(`publish:${SUBJECT_HANDSHAKE}`), false);
  });

  it("warn mode fails closed on a tampered scheduler response", async () => {
    const fixture = createTrustFixture();
    await assertWarnSecurityFailure(fixture, new TamperingTrustConnection(fixture));
  });

  it("warn mode fails closed on a signed scheduler rejection", async () => {
    const fixture = createTrustFixture();
    await assertWarnSecurityFailure(fixture, new SignedRejectingTrustConnection(fixture));
  });
});
