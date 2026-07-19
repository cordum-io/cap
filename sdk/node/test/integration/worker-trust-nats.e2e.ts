import assert from "node:assert/strict";
import type { Msg, NatsConnection, NatsError, Subscription } from "nats";

import { connectNATS } from "../../src/bus";
import { SUBJECT_HANDSHAKE, SUBJECT_RESULT } from "../../src/protos";
import { Agent, InMemoryBlobStore } from "../../src/runtime";
import { decodeBusPacket, privateKeyPem, signedDispatch } from "../runtime-trust-packet-support";
import {
  FakeTrustRequester,
  createTrustFixture,
  type TrustFixture,
} from "../worker-trust-runtime-support";
import { NatsServer, reservePort, waitForCondition, withTimeout } from "./nats-server";

const TOPIC = "job.runtime.trust";
const TEST_TIMEOUT_MS = 45_000;

interface ScenarioState {
  error?: Error;
  readyTokens: string[];
  resultTokens: string[];
  handled: string[];
  phase: string;
}

function recordError(state: ScenarioState, error: unknown): void {
  if (state.error) return;
  const cause = error instanceof Error ? error : new Error(String(error));
  state.error = new Error(`${state.phase}: ${cause.message}`);
}

function handleAsync(state: ScenarioState, work: Promise<void>): void {
  void work.catch((error) => recordError(state, error));
}

function subscribeTrustResponder(
  scheduler: NatsConnection,
  responder: FakeTrustRequester,
  subject: string,
  state: ScenarioState
): Subscription {
  return scheduler.subscribe(subject, {
    callback: (error: NatsError | null, message: Msg) => {
      if (error) return recordError(state, error);
      handleAsync(state, (async () => {
        const response = await responder.request(subject, message.data, { timeout: 1000 });
        if (!message.respond(response.data)) throw new Error(`failed to respond on ${subject}`);
      })());
    },
  });
}

async function waitForResults(state: ScenarioState, expected: number): Promise<void> {
  await waitForCondition(() => {
    if (state.error) throw state.error;
    return state.resultTokens.length >= expected;
  }, 12_000, `timed out in ${state.phase}: expected=${expected} ` +
    `ready=${state.readyTokens.join(",")} results=${state.resultTokens.join(",")} ` +
    `handled=${state.handled.join(",")}`);
}

async function boundedCleanup(
  state: ScenarioState,
  label: string,
  work: Promise<unknown>
): Promise<void> {
  await withTimeout(work, 5_000, `${label} cleanup timed out`)
    .catch((error) => recordError(state, error));
}

interface ScenarioHarness {
  readonly fixture: TrustFixture;
  readonly responder: FakeTrustRequester;
  readonly server: NatsServer;
  readonly state: ScenarioState;
  readonly subscriptions: Subscription[];
  scheduler?: NatsConnection;
  agent?: Agent;
}

async function createScenario(): Promise<ScenarioHarness> {
  const fixture = createTrustFixture();
  return {
    fixture,
    responder: new FakeTrustRequester(fixture),
    server: new NatsServer(await reservePort()),
    state: { readyTokens: [], resultTokens: [], handled: [], phase: "server-start" },
    subscriptions: [],
  };
}

async function subscribeOutputs(harness: ScenarioHarness): Promise<InMemoryBlobStore> {
  const scheduler = harness.scheduler;
  if (!scheduler) throw new Error("scheduler is unavailable");
  const { fixture, state, subscriptions } = harness;
  const store = new InMemoryBlobStore();
  await store.set("ctx:real-trust-1", Buffer.from("{}"));
  await store.set("ctx:real-trust-2", Buffer.from("{}"));
  subscriptions.push(scheduler.subscribe(SUBJECT_RESULT, {
    callback: (error, message) => {
      if (error) return recordError(state, error);
      handleAsync(state, decodeBusPacket(message.data).then((packet) => {
        state.resultTokens.push(String(packet.authToken ?? ""));
      }));
    },
  }));
  subscriptions.push(scheduler.subscribe(SUBJECT_HANDSHAKE, {
    callback: (error, message) => {
      if (error) return recordError(state, error);
      handleAsync(state, (async () => {
        const packet = await decodeBusPacket(message.data);
        state.readyTokens.push(String(packet.authToken ?? ""));
        const sequence = state.readyTokens.length;
        if (sequence > 2) return;
        scheduler.publish(TOPIC, await signedDispatch(
          fixture, TOPIC, `real-trust-${sequence}`, `redis://ctx:real-trust-${sequence}`
        ));
      })());
    },
  }));
  await scheduler.flush();
  return store;
}

function createAgent(harness: ScenarioHarness, store: InMemoryBlobStore): Agent {
  const { fixture, server, state } = harness;
  const agent = new Agent({
    natsUrl: server.url,
    senderId: fixture.config.workerId,
    privateKey: privateKeyPem(fixture),
    store,
    heartbeatInterval: 25,
    ioTimeoutMs: 2000,
    workerTrust: {
      mode: "enforce",
      config: fixture.config,
      timeoutMs: 1000,
      retries: 10,
      renewMinIntervalMs: 30_000,
    },
  });
  agent.job(TOPIC, async (context) => {
    state.handled.push(context.jobId);
    return { accepted: true };
  });
  return agent;
}

async function startScenario(harness: ScenarioHarness): Promise<void> {
  await harness.server.start();
  const scheduler = await connectNATS({
    url: harness.server.url,
    name: "node-trust-scheduler",
  });
  harness.scheduler = scheduler;
  harness.subscriptions.push(
    subscribeTrustResponder(scheduler, harness.responder,
      "sys.worker.handshake.challenge", harness.state),
    subscribeTrustResponder(scheduler, harness.responder,
      "sys.worker.handshake.authenticate", harness.state)
  );
  harness.agent = createAgent(harness, await subscribeOutputs(harness));
}

async function exerciseScenario(harness: ScenarioHarness): Promise<void> {
  const { agent, responder, server, state } = harness;
  if (!agent) throw new Error("agent is unavailable");
  state.phase = "initial-authenticate";
  await withTimeout(agent.start(), 8_000, "initial authenticated Agent.start timed out");
  state.phase = "initial-dispatch";
  await waitForResults(state, 1);
  assert.deepEqual(responder.requests.map(({ subject }) => subject), [
    "sys.worker.handshake.challenge",
    "sys.worker.handshake.authenticate",
  ]);
  assert.deepEqual(state.readyTokens, ["session-1"]);
  assert.deepEqual(state.resultTokens, ["session-1"]);
  state.phase = "server-stop";
  await server.stop();
  state.phase = "server-restart";
  await server.start();
  state.phase = "reconnect-authenticate-dispatch";
  await waitForResults(state, 2);
  assert.equal(agent.sessionToken, "session-2");
  assert.deepEqual(state.readyTokens, ["session-1", "session-2"]);
  assert.deepEqual(state.resultTokens, ["session-1", "session-2"]);
  assert.deepEqual(state.handled, ["real-trust-1", "real-trust-2"]);
}

async function cleanupScenario(harness: ScenarioHarness): Promise<void> {
  const { agent, scheduler, server, state, subscriptions } = harness;
  state.phase = "cleanup";
  if (agent) await boundedCleanup(state, "agent close", agent.close());
  await Promise.all(subscriptions.map(async (subscription) => {
    if (subscription.isClosed()) return;
    await boundedCleanup(state, "subscription drain", subscription.drain());
    if (!subscription.isClosed()) subscription.unsubscribe();
  }));
  if (scheduler && !scheduler.isClosed()) {
    await boundedCleanup(state, "scheduler drain", scheduler.drain());
    if (!scheduler.isClosed()) await scheduler.close();
  }
  await server.stop(false);
}

describe("Node authenticated worker trust over real NATS", () => {
  it("authenticates before ready, dispatches immediately, and reauthenticates", async function () {
    this.timeout(TEST_TIMEOUT_MS);
    const harness = await createScenario();
    try {
      await startScenario(harness);
      await exerciseScenario(harness);
    } finally {
      await cleanupScenario(harness);
    }
    if (harness.state.error) throw harness.state.error;
  });
});
