import assert from "node:assert/strict";
import crypto from "node:crypto";
import type { Msg, NatsConnection, NatsError, Subscription } from "nats";
import type { Type } from "protobufjs";

import { connectNATS } from "../../src/bus";
import {
  PRODUCTION_ALGORITHM,
  PRODUCTION_PROFILE_VERSION,
  PRODUCTION_SIGNATURE_DOMAIN,
} from "../../src/production-signing";
import { InMemoryReplayStore } from "../../src/production-replay";
import {
  SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
  SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
  loadRoot,
} from "../../src/protos";
import { Agent, InMemoryBlobStore, type Logger } from "../../src/runtime";
import { privateKeyPem } from "../runtime-trust-packet-support";
import {
  FakeTrustRequester,
  createTrustFixture,
  type TrustFixture,
} from "../worker-trust-runtime-support";
import {
  NatsServer,
  withTimeout,
} from "./nats-server";
import { productionNatsServer, type ProductionTestServer } from "./production-nats-server";

const TOPIC = "job.production.real-nats";
const TEST_TIMEOUT_MS = 45_000;

interface ScenarioState {
  readonly handled: string[];
  readonly barriers: Map<string, () => void>;
  error?: Error;
}

interface Scenario {
  readonly server: ProductionTestServer;
  readonly fixture: TrustFixture;
  readonly state: ScenarioState;
  readonly store: InMemoryBlobStore;
  readonly subscriptions: Subscription[];
  scheduler?: NatsConnection;
  agent?: Agent;
  requester?: FakeTrustRequester;
}

interface PacketOptions {
  readonly jobId: string;
  readonly messageNumber: number;
  readonly audience?: string;
}

function messageId(number: number): Buffer {
  const id = Buffer.alloc(16);
  id.writeUInt32BE(number, 12);
  return id;
}

function unsignedPacket(
  type: Type,
  fixture: TrustFixture,
  options: PacketOptions,
): Buffer {
  const identity = {
    tenantId: fixture.config.tenantId,
    principalId: "principal-node",
    actorId: "actor-node",
    delegationId: "delegation-node",
  };
  const packet = type.fromObject({
    traceId: `trace-${options.jobId}`,
    senderId: fixture.config.expectedSchedulerId,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    protocolVersion: 1,
    identity,
    signatureMetadata: {
      profileVersion: PRODUCTION_PROFILE_VERSION,
      algorithm: PRODUCTION_ALGORITHM,
      messageId: messageId(options.messageNumber),
      audience: options.audience ?? TOPIC,
      expiresAt: { seconds: Math.floor(Date.now() / 1000) + 120, nanos: 0 },
      keyId: "production-key",
    },
    jobRequest: {
      jobId: options.jobId,
      topic: TOPIC,
      contextPtr: `redis://ctx:${options.jobId}`,
      tenantId: identity.tenantId,
      principalId: identity.principalId,
      identity,
    },
  });
  return Buffer.from(type.encode(packet).finish());
}

function signPacket(unsigned: Buffer, fixture: TrustFixture): Buffer {
  const signer = crypto.createSign("sha256");
  signer.update(PRODUCTION_SIGNATURE_DOMAIN);
  signer.update(unsigned);
  signer.end();
  const signature = signer.sign(fixture.schedulerPrivateKey);
  return Buffer.concat([
    unsigned,
    Buffer.from([(14 << 3) | 2, signature.length]),
    signature,
  ]);
}

function recordError(state: ScenarioState, error: unknown): void {
  if (state.error) return;
  state.error = error instanceof Error ? error : new Error(String(error));
}

function trustResponder(
  connection: NatsConnection,
  requester: FakeTrustRequester,
  subject: string,
  state: ScenarioState,
): Subscription {
  return connection.subscribe(subject, {
    callback: (error: NatsError | null, message: Msg) => {
      if (error) return recordError(state, error);
      void requester.request(subject, message.data, { timeout: 1_000 })
        .then((response) => {
          if (!message.respond(response.data)) {
            throw new Error(`failed to respond on ${subject}`);
          }
        })
        .catch((cause) => recordError(state, cause));
    },
  });
}

function logger(state: ScenarioState): Logger {
  return {
    info: () => undefined,
    warn: () => undefined,
    error: (message, fields) => recordError(
      state,
      new Error(`${message}: ${String(fields?.error ?? "unknown error")}`),
    ),
  };
}

async function createScenario(): Promise<Scenario> {
  const fixture = createTrustFixture("worker-production-node");
  return {
    server: await productionNatsServer(NatsServer),
    fixture,
    state: { handled: [], barriers: new Map() },
    store: new InMemoryBlobStore(),
    subscriptions: [],
  };
}

async function startScenario(scenario: Scenario): Promise<Type> {
  await scenario.server.start();
  scenario.scheduler = await connectNATS({
    url: scenario.server.url,
    name: "node-production-scheduler",
  });
  const requester = new FakeTrustRequester(scenario.fixture);
  scenario.requester = requester;
  scenario.subscriptions.push(
    trustResponder(scenario.scheduler, requester, SUBJECT_WORKER_HANDSHAKE_CHALLENGE, scenario.state),
    trustResponder(scenario.scheduler, requester, SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, scenario.state),
  );
  await scenario.scheduler.flush();
  const root = await loadRoot();
  const type = root.lookupType("cordum.agent.v1.BusPacket");
  const productionPublicKey = crypto.createPublicKey(scenario.fixture.schedulerPrivateKey)
    .export({ type: "spki", format: "pem" }).toString();
  scenario.agent = createAgent(scenario, productionPublicKey);
  await withTimeout(scenario.agent.start(), 8_000, "production Agent.start timed out");
  return type;
}

function createAgent(scenario: Scenario, productionPublicKey: string): Agent {
  const { fixture, server, state, store } = scenario;
  const agent = new Agent({
    natsUrl: server.url,
    senderId: fixture.config.workerId,
    privateKey: privateKeyPem(fixture),
    store,
    logger: logger(state),
    productionTrust: {
      audience: "configured.alias.must-not-authorize",
      tenant: fixture.config.tenantId,
      sender: fixture.config.expectedSchedulerId,
      publicKeys: { "production-key": productionPublicKey },
    },
    replayStore: new InMemoryReplayStore(),
    workerTrust: {
      mode: "enforce",
      config: fixture.config,
      timeoutMs: 1_000,
      retries: 2,
      renewMinIntervalMs: 30_000,
    },
  });
  agent.job(TOPIC, async (context) => {
    state.handled.push(context.jobId);
    state.barriers.get(context.jobId)?.();
    return { accepted: true };
  });
  return agent;
}

async function publishThroughBarrier(
  scenario: Scenario,
  packets: readonly Uint8Array[],
  barrier: Uint8Array,
  barrierJobId: string,
): Promise<void> {
  const scheduler = scenario.scheduler;
  if (!scheduler) throw new Error("scheduler is unavailable");
  const reached = new Promise<void>((resolve) => {
    scenario.state.barriers.set(barrierJobId, resolve);
  });
  for (const packet of packets) scheduler.publish(TOPIC, packet);
  scheduler.publish(TOPIC, barrier);
  await scheduler.flush();
  await withTimeout(reached, 5_000, `handler barrier ${barrierJobId} timed out`);
  scenario.state.barriers.delete(barrierJobId);
  if (scenario.state.error) throw scenario.state.error;
}

async function seedContexts(scenario: Scenario, jobIds: readonly string[]): Promise<void> {
  await Promise.all(jobIds.map((jobId) =>
    scenario.store.set(`ctx:${jobId}`, Buffer.from("{}"))
  ));
}

async function cleanup(scenario: Scenario): Promise<void> {
  if (scenario.agent) await withTimeout(scenario.agent.close(), 5_000, "agent close timed out");
  await Promise.all(scenario.subscriptions.map(async (subscription) => {
    if (!subscription.isClosed()) await subscription.drain();
  }));
  if (scenario.scheduler && !scenario.scheduler.isClosed()) {
    await withTimeout(scenario.scheduler.drain(), 5_000, "scheduler drain timed out");
  }
  await scenario.server.stop(false);
}

describe("Node CAP-PRODUCTION admission over real NATS", () => {
  it("admits one exact delivery and rejects unsigned, tampered, wrong-audience, duplicate, and conflicting replay", async function () {
    this.timeout(TEST_TIMEOUT_MS);
    const scenario = await createScenario();
    try {
      const type = await startScenario(scenario);
      assert.equal(scenario.agent?.sessionToken, "session-1");
      assert.deepEqual(scenario.requester?.requests.map(({ subject }) => subject), [
        SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
        SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
      ]);
      const jobs = [
        "unsigned", "tampered", "wrong-audience", "accepted", "conflict",
        "barrier-unsigned", "barrier-tampered", "barrier-audience", "barrier-replay",
      ];
      await seedContexts(scenario, jobs);
      const packet = (jobId: string, number: number, audience?: string) =>
        signPacket(unsignedPacket(type, scenario.fixture, { jobId, messageNumber: number, audience }), scenario.fixture);

      await publishThroughBarrier(scenario,
        [unsignedPacket(type, scenario.fixture, { jobId: "unsigned", messageNumber: 1 })],
        packet("barrier-unsigned", 11), "barrier-unsigned");
      assert.deepEqual(scenario.state.handled, ["barrier-unsigned"]);

      const tampered = packet("tampered", 2);
      tampered[tampered.length - 1] ^= 0xff;
      await publishThroughBarrier(scenario, [tampered], packet("barrier-tampered", 12), "barrier-tampered");
      assert.deepEqual(scenario.state.handled, ["barrier-unsigned", "barrier-tampered"]);

      await publishThroughBarrier(scenario,
        [packet("wrong-audience", 3, "configured.alias.must-not-authorize")],
        packet("barrier-audience", 13), "barrier-audience");
      assert.deepEqual(scenario.state.handled, ["barrier-unsigned", "barrier-tampered", "barrier-audience"]);

      const accepted = packet("accepted", 4);
      const conflict = packet("conflict", 4);
      await publishThroughBarrier(scenario,
        [accepted, accepted, conflict], packet("barrier-replay", 14), "barrier-replay");
      assert.deepEqual(scenario.state.handled, [
        "barrier-unsigned", "barrier-tampered", "barrier-audience", "accepted", "barrier-replay",
      ]);
    } finally {
      await cleanup(scenario);
    }
    if (scenario.state.error) throw scenario.state.error;
  });
});
