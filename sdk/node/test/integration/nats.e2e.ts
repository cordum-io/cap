import assert from "node:assert/strict";
import { setTimeout as delay } from "node:timers/promises";
import {
  Events,
  Msg,
  NatsConnection,
  NatsError,
  Subscription,
} from "nats";
import { Type } from "protobufjs";
import { connectNATS } from "../../src/bus";
import { submitJob } from "../../src/client";
import {
  loadRoot,
  SUBJECT_RESULT,
  SUBJECT_SUBMIT,
} from "../../src/protos";
import { startWorker } from "../../src/worker";
import {
  NatsServer,
  reservePort,
  waitForCondition,
  withTimeout,
} from "./nats-server";

const EVENT_TIMEOUT_MS = 8_000;

interface DecodedResult {
  jobId: string;
  status: number;
}

interface ResultPacketShape {
  jobResult?: {
    jobId?: unknown;
    status?: unknown;
  };
}

interface ResultCollector {
  records: DecodedResult[];
  error?: Error;
  subscription: Subscription;
}

interface StatusProbe {
  events: string[];
  done: Promise<void>;
}

interface ScenarioStatus {
  worker: StatusProbe;
  client: StatusProbe;
}

interface ScenarioState {
  server: NatsServer;
  calls: Map<string, number>;
  statusTasks: Promise<void>[];
  workerConnection?: NatsConnection;
  clientConnection?: NatsConnection;
  workerSubscription?: Subscription;
  results?: ResultCollector;
}

function watchStatus(connection: NatsConnection): StatusProbe {
  const events: string[] = [];
  const done = (async () => {
    for await (const status of connection.status()) {
      events.push(status.type);
    }
  })();
  return { events, done };
}

async function waitForEvent(
  probe: StatusProbe,
  event: Events,
  afterIndex = -1,
): Promise<number> {
  await waitForCondition(
    () => probe.events.indexOf(event, afterIndex + 1) >= 0,
    EVENT_TIMEOUT_MS,
    `timed out waiting for NATS ${event}; saw ${probe.events.join(",")}`,
  );
  return probe.events.indexOf(event, afterIndex + 1);
}

function decodeResult(type: Type, data: Uint8Array): DecodedResult {
  const packet = type.toObject(type.decode(data), {
    defaults: false,
    enums: Number,
  }) as ResultPacketShape;
  const jobId = packet.jobResult?.jobId;
  const status = packet.jobResult?.status;
  if (typeof jobId !== "string" || typeof status !== "number") {
    throw new Error("result packet lacked typed jobId/status");
  }
  return { jobId, status };
}

async function collectResults(connection: NatsConnection): Promise<ResultCollector> {
  const root = await loadRoot();
  const type = root.lookupType("cordum.agent.v1.BusPacket");
  const collector: Omit<ResultCollector, "subscription"> = { records: [] };
  const subscription = connection.subscribe(SUBJECT_RESULT, {
    callback: (error: NatsError | null, message: Msg) => {
      if (error) {
        collector.error = error;
        return;
      }
      try {
        collector.records.push(decodeResult(type, message.data));
      } catch (decodeError) {
        collector.error = decodeError instanceof Error ? decodeError : new Error(String(decodeError));
      }
    },
  });
  await connection.flush();
  return { ...collector, subscription };
}

function readJobId(value: unknown): string {
  if (!value || typeof value !== "object") throw new Error("job request was not an object");
  const jobId = (value as Record<string, unknown>).jobId;
  if (typeof jobId !== "string") throw new Error("job request lacked jobId");
  return jobId;
}

async function submitAndExpect(
  connection: NatsConnection,
  collector: ResultCollector,
  jobId: string,
  calls: ReadonlyMap<string, number>,
): Promise<void> {
  await submitJob(connection, {
    jobId,
    topic: "job.real-nats",
    contextPtr: `memory://${jobId}`,
  }, `trace-${jobId}`, "e2e-client");
  await waitForCondition(() => {
    if (collector.error) throw collector.error;
    return collector.records.some((record) => record.jobId === jobId);
  }, 4_000, `timed out waiting for result ${jobId}; handlerCalls=${calls.get(jobId) ?? 0}`);
  await delay(150);
  const matches = collector.records.filter((record) => record.jobId === jobId);
  assert.equal(matches.length, 1, `expected exactly one result for ${jobId}`);
  assert.equal(matches[0].status, 5, `expected succeeded result for ${jobId}`);
}

async function drainSubscription(subscription?: Subscription): Promise<void> {
  if (!subscription || subscription.isClosed()) return;
  try {
    await withTimeout(subscription.drain(), 2_000, "subscription drain timed out");
  } catch {
    subscription.unsubscribe();
  }
}

async function drainConnection(connection?: NatsConnection): Promise<void> {
  if (!connection || connection.isClosed()) return;
  try {
    await withTimeout(connection.drain(), 2_000, "connection drain timed out");
  } catch {
    await connection.close();
  }
}

async function startScenario(state: ScenarioState): Promise<ScenarioStatus> {
  await state.server.start();
  state.workerConnection = await connectNATS({
    url: state.server.url,
    name: "cap-e2e-worker",
  });
  state.clientConnection = await connectNATS({
    url: state.server.url,
    name: "cap-e2e-client",
  });
  const workerStatus = watchStatus(state.workerConnection);
  const clientStatus = watchStatus(state.clientConnection);
  state.statusTasks.push(workerStatus.done, clientStatus.done);
  state.results = await collectResults(state.clientConnection);
  state.workerSubscription = await startWorker({
    nc: state.workerConnection,
    subject: SUBJECT_SUBMIT,
    senderId: "e2e-worker",
    handler: async (request: unknown) => {
      const jobId = readJobId(request);
      state.calls.set(jobId, (state.calls.get(jobId) ?? 0) + 1);
      return {
        jobId,
        status: "JOB_STATUS_SUCCEEDED",
        resultPtr: `memory://${jobId}/result`,
      };
    },
  });
  await state.workerConnection.flush();
  await submitAndExpect(
    state.clientConnection,
    state.results,
    "job-before-restart",
    state.calls,
  );
  return { worker: workerStatus, client: clientStatus };
}

async function exerciseReconnect(state: ScenarioState): Promise<void> {
  const status = await startScenario(state);
  const { clientConnection, results, workerSubscription } = state;
  if (!clientConnection || !results || !workerSubscription) {
    throw new Error("NATS scenario setup did not initialize required resources");
  }
  await state.server.stop();
  const workerDisconnected = await waitForEvent(status.worker, Events.Disconnect);
  const clientDisconnected = await waitForEvent(status.client, Events.Disconnect);
  await state.server.start();
  await waitForEvent(status.worker, Events.Reconnect, workerDisconnected);
  await waitForEvent(status.client, Events.Reconnect, clientDisconnected);
  assert.equal(
    workerSubscription.isClosed(),
    false,
    "original worker subscription closed",
  );
  await submitAndExpect(
    clientConnection,
    results,
    "job-after-restart",
    state.calls,
  );

  assert.deepEqual(results.records.map((record) => record.jobId), [
    "job-before-restart",
    "job-after-restart",
  ]);
  assert.deepEqual([...state.calls.entries()], [
    ["job-before-restart", 1],
    ["job-after-restart", 1],
  ]);
}

async function cleanupScenario(state: ScenarioState): Promise<void> {
  await drainSubscription(state.results?.subscription);
  await drainSubscription(state.workerSubscription);
  await drainConnection(state.clientConnection);
  await drainConnection(state.workerConnection);
  await state.server.stop(false);
  await withTimeout(
    Promise.allSettled(state.statusTasks),
    1_000,
    "NATS status watchers did not close",
  ).catch(() => undefined);
}

async function runReconnectScenario(): Promise<void> {
  const state: ScenarioState = {
    server: new NatsServer(await reservePort()),
    calls: new Map<string, number>(),
    statusTasks: [],
  };
  try {
    await exerciseReconnect(state);
  } finally {
    await cleanupScenario(state);
  }
}

describe("Node SDK real NATS transport", () => {
  it("keeps the original worker subscription after a disconnect and reconnect", async function () {
    this.timeout(30_000);
    await runReconnectScenario();
  });
});
