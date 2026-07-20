import { expect } from "chai";

import type { Logger } from "../src/logger";
import {
  ReplayConflictError,
  ReplayOutcome,
  ReplayStoreUnavailableError,
  type ReplayStore,
} from "../src/index";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import {
  busPacketType,
  keyPair,
  signedRawPacket,
} from "./production-admission-support";

interface AdmissionHarness {
  busPacketType: unknown;
  trust: unknown;
  decodeProductionPacket(raw: Uint8Array): Promise<unknown | null>;
  onMessage(msg: unknown, spec: unknown): Promise<void>;
}

function harness(agent: Agent): AdmissionHarness {
  return agent as unknown as AdmissionHarness;
}

function asyncReplayStore(admit: () => Promise<ReplayOutcome>): ReplayStore {
  return { admit };
}

function recordingLogger(errors: string[]): Logger {
  return {
    info: () => undefined,
    error: () => undefined,
    warn: (_message, fields) => errors.push(String(fields?.error ?? "")),
  };
}

async function agentWith(
  replayStore: ReplayStore,
  errors: string[] = [],
  store: InMemoryBlobStore = new InMemoryBlobStore(),
): Promise<Agent> {
  const agent = new Agent({
    store,
    productionTrust: {
      audience: "worker-pool-a",
      tenant: "tenant-a",
      sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
    },
    replayStore,
    logger: recordingLogger(errors),
  });
  harness(agent).busPacketType = await busPacketType();
  return agent;
}

describe("CAP-PRODUCTION async replay admission", () => {
  it("awaits an async first-seen decision", async () => {
    const agent = await agentWith(asyncReplayStore(async () => ReplayOutcome.First));

    const packet = await harness(agent).decodeProductionPacket(await signedRawPacket());

    expect(packet).not.to.equal(null);
  });

  it("drops an async duplicate decision", async () => {
    const agent = await agentWith(asyncReplayStore(async () => ReplayOutcome.Duplicate));

    const packet = await harness(agent).decodeProductionPacket(await signedRawPacket());

    expect(packet).to.equal(null);
  });

  const secret = "redis://user:password@private-host";
  const failures: Array<[string, Error, string]> = [
    ["conflict", new ReplayConflictError(secret), "replay conflict"],
    ["unavailable", new ReplayStoreUnavailableError(secret), "replay store unavailable"],
    ["unexpected", new Error(secret), "replay store unavailable"],
  ];
  for (const [name, error, category] of failures) {
    it(`normalizes an async ${name} failure`, async () => {
      const errors: string[] = [];
      const replay = asyncReplayStore(async () => { throw error; });
      const agent = await agentWith(replay, errors);

      const packet = await harness(agent).decodeProductionPacket(await signedRawPacket());

      expect(packet).to.equal(null);
      expect(errors).to.deep.equal([category]);
      expect(errors.join(" ")).not.to.contain(secret);
    });
  }
});

describe("CAP-PRODUCTION async replay handler boundary", () => {
  it("does not invoke a handler before replay admission settles", async () => {
    let resolve!: (outcome: ReplayOutcome) => void;
    const decision = new Promise<ReplayOutcome>((accept) => { resolve = accept; });
    const store = new InMemoryBlobStore();
    await store.set("ctx:job-1", Buffer.from("{}"));
    const agent = await agentWith(asyncReplayStore(() => decision), [], store);
    harness(agent).trust = {
      productionSessionActive: true,
      settings: { config: { tenantId: "tenant-a", expectedSchedulerId: "scheduler-1" } },
    };
    let calls = 0;
    const running = harness(agent).onMessage(
      {
        data: await signedRawPacket({ contextPtr: "redis://ctx:job-1" }),
        subject: "worker-pool-a",
      },
      { topic: "job.test", retries: 0, handler: async () => { calls += 1; return {}; } },
    );

    await Promise.resolve();
    expect(calls).to.equal(0);
    resolve(ReplayOutcome.First);
    await running;
    expect(calls).to.equal(1);
  });
});
