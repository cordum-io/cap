import assert from "node:assert";
import * as crypto from "node:crypto";
import type {
  Msg,
  NatsConnection,
  Subscription,
  SubscriptionOptions,
} from "nats";

import { encodeDeterministic, encodeUnsignedForSignature } from "../src/codec";
import { submitJob } from "../src/client";
import { handshakePayload } from "../src/handshake";
import { heartbeatPayload } from "../src/heartbeat";
import type { Logger } from "../src/logger";
import { DEFAULT_PROTOCOL_VERSION, loadRoot } from "../src/protos";
import { cancelPayload, progressPayload } from "../src/progress";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { startWorker } from "../src/worker";
import { validateBusPacket } from "../src/validate";


const SUBJECT = "job.security.contract";
const keyPair = crypto.generateKeyPairSync("ec", {
  namedCurve: "prime256v1",
  publicKeyEncoding: { type: "spki", format: "pem" },
  privateKeyEncoding: { type: "pkcs8", format: "pem" },
});

type NatsCallback = NonNullable<SubscriptionOptions["callback"]>;

interface PacketView {
  signature?: Uint8Array;
}

interface PacketOptions {
  traceId?: string;
  senderId?: string;
  protocolVersion?: number;
  sign?: boolean;
}

class MockNatsConnection {
  callbacks = new Map<string, NatsCallback>();
  published: Array<{ subject: string; data: Uint8Array }> = [];

  publish(subject: string, data: Uint8Array): void {
    this.published.push({ subject, data });
  }

  subscribe(subject: string, options: SubscriptionOptions = {}): Subscription {
    if (options.callback) this.callbacks.set(subject, options.callback);
    return {
      drain: async () => {},
      unsubscribe: () => {},
    } as unknown as Subscription;
  }

  async deliver(data: Uint8Array): Promise<void> {
    const callback = this.callbacks.get(SUBJECT);
    assert.ok(callback, "worker did not subscribe");
    callback(null, { subject: SUBJECT, data } as unknown as Msg);
    await new Promise((resolve) => setTimeout(resolve, 20));
  }

  async drain(): Promise<void> {}
}

const silentLogger: Logger = {
  info: () => {},
  warn: () => {},
  error: () => {},
};

async function jobPacket(options: PacketOptions = {}): Promise<Uint8Array> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const packet = BusPacket.fromObject({
    traceId: options.traceId ?? "trace-security",
    senderId: options.senderId ?? "client-security",
    protocolVersion: options.protocolVersion ?? DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: {
      jobId: "job-security",
      topic: SUBJECT,
      contextPtr: "redis://ctx:job-security",
    },
  }) as unknown as PacketView;
  if (options.sign) {
    const signer = crypto.createSign("sha256");
    signer.update(encodeUnsignedForSignature(BusPacket, packet));
    packet.signature = signer.sign(keyPair.privateKey);
  }
  return encodeDeterministic(BusPacket, packet);
}

async function exerciseWorker(
  packet: Uint8Array,
  publicKeyMap?: Record<string, string>
): Promise<{ calls: number; published: Array<{ subject: string; data: Uint8Array }> }> {
  const bus = new MockNatsConnection();
  let calls = 0;
  const subscription = await startWorker({
    nc: bus as unknown as NatsConnection,
    subject: SUBJECT,
    senderId: "worker-security",
    publicKeyMap,
    logger: silentLogger,
    handler: async (request: { jobId?: string }) => {
      calls += 1;
      return {
        jobId: request.jobId,
        workerId: "worker-security",
        status: "JOB_STATUS_SUCCEEDED",
      };
    },
  });
  try {
    await bus.deliver(packet);
  } finally {
    await subscription.drain();
  }
  return { calls, published: bus.published };
}

async function exerciseAgent(
  packet: Uint8Array,
  publicKeyMap?: Record<string, string>
): Promise<number> {
  const bus = new MockNatsConnection();
  const store = new InMemoryBlobStore();
  await store.set("ctx:job-security", Buffer.from("{}", "utf-8"));
  let calls = 0;
  const agent = new Agent({
    store,
    publicKeyMap,
    senderId: "agent-security",
    heartbeatInterval: 60_000,
    logger: silentLogger,
    connectFn: async () => bus as unknown as NatsConnection,
  });
  agent.job(SUBJECT, async () => {
    calls += 1;
    return {};
  });
  await agent.start();
  try {
    await bus.deliver(packet);
  } finally {
    await agent.close();
  }
  return calls;
}

describe("CAP worker trust schema", () => {
  it("defines the append-only worker trust handshake proto contract", async () => {
    const root = await loadRoot();
    for (const name of [
      "WorkerHandshakeChallengeRequest",
      "WorkerHandshakeChallenge",
      "WorkerHandshakeAuthenticate",
      "WorkerHandshakeResult",
    ]) {
      assert.ok(root.lookupTypeOrEnum(`cordum.agent.v1.${name}`));
    }
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    for (const name of [
      "workerHandshakeChallengeRequest",
      "workerHandshakeChallenge",
      "workerHandshakeAuthenticate",
      "workerHandshakeResult",
    ]) {
      assert.ok(BusPacket.fields[name]);
      assert.ok(BusPacket.fields[name].id >= 19);
    }
  });

  it("requires exactly protocol v1", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const packet = BusPacket.decode(await jobPacket({ protocolVersion: 2 }));
    const errors = validateBusPacket(packet);
    assert.ok(
      errors.some((error) => error.field === "protocol_version"),
      "protocolVersion=2 must be rejected; CAP currently supports exactly v1"
    );
  });
});

describe("CAP handshake identity contract", () => {
  it("makes the legacy handshake builder pass its own validator", async () => {
    const packet = await handshakePayload("worker-security");
    assert.deepStrictEqual(validateBusPacket(packet), []);
  });

  it("requires handshake sender identity to equal component identity", async () => {
    const packet = await handshakePayload(
      "worker-security",
      {},
      "forged-worker"
    );
    const errors = validateBusPacket(packet);
    assert.ok(
      errors.some((error) => error.field === "handshake.component_id"),
      "handshake componentId must equal the authenticated envelope senderId"
    );
  });
});

describe("CAP inbound envelope boundary", () => {
  it("never invokes a worker handler for an invalid envelope", async () => {
    const calls = {
      missingTrace: (await exerciseWorker(await jobPacket({ traceId: "" }))).calls,
      protocolV2: (await exerciseWorker(await jobPacket({ protocolVersion: 2 }))).calls,
    };
    assert.deepStrictEqual(calls, { missingTrace: 0, protocolV2: 0 });
  });

  it("never invokes an Agent handler for an invalid envelope", async () => {
    const calls = {
      missingTrace: await exerciseAgent(await jobPacket({ traceId: "" })),
      protocolV2: await exerciseAgent(await jobPacket({ protocolVersion: 2 })),
    };
    assert.deepStrictEqual(calls, { missingTrace: 0, protocolV2: 0 });
  });
});

describe("CAP inbound signature boundary", () => {
  it("treats a configured empty trust map as deny-all", async () => {
    assert.strictEqual((await exerciseWorker(await jobPacket(), {})).calls, 0);
  });

  it("does not allow an empty sender to bypass signature verification", async () => {
    const packet = await jobPacket({ senderId: "", sign: true });
    assert.strictEqual(
      (await exerciseWorker(packet, { "": keyPair.publicKey })).calls,
      0
    );
  });

  it("never invokes a worker handler for an invalid signature", async () => {
    const packet = await jobPacket({ sign: true });
    const otherKey = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    assert.strictEqual(
      (await exerciseWorker(packet, { "client-security": otherKey.publicKey })).calls,
      0
    );
  });
});

describe("CAP public outbound builders", () => {
  it("validates every public outbound builder with its own validator", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const bus = new MockNatsConnection();
    await submitJob(
      bus as unknown as NatsConnection,
      { jobId: "job-builder", topic: SUBJECT },
      "trace-builder",
      "client-builder"
    );
    const worker = await exerciseWorker(await jobPacket());
    const packets = {
      jobRequest: BusPacket.decode(bus.published[0].data),
      heartbeat: await heartbeatPayload("worker-builder", "pool", 0, 1, 0),
      progress: await progressPayload("worker-builder", "job-builder", "step", 1, "ok"),
      cancel: await cancelPayload("worker-builder", "job-builder", "stop", "tester"),
      legacyHandshake: await handshakePayload("worker-builder"),
      jobResult: BusPacket.decode(worker.published[0].data),
    };
    const failures = Object.fromEntries(
      Object.entries(packets).map(([name, packet]) => [
        name,
        validateBusPacket(packet).map((error) => error.field),
      ])
    );
    const expected = Object.fromEntries(Object.keys(packets).map((name) => [name, []]));
    assert.deepStrictEqual(failures, expected);
  });
});
