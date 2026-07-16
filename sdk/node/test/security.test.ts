import assert from "node:assert";
import * as crypto from "node:crypto";
import type {
  Msg,
  NatsConnection,
  Subscription,
  SubscriptionOptions,
} from "nats";
import { encodeDeterministic, encodeUnsignedForSignature } from "../src/codec";
import type { Logger } from "../src/logger";
import { DEFAULT_PROTOCOL_VERSION, loadRoot, SUBJECT_RESULT } from "../src/protos";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { startWorker } from "../src/worker";

const TOPIC = "job.security";
const TRUSTED_SENDER = "trusted-client";
const privateKeyMap = crypto.generateKeyPairSync("ec", {
  namedCurve: "prime256v1",
  publicKeyEncoding: { type: "spki", format: "pem" },
  privateKeyEncoding: { type: "pkcs8", format: "pem" },
});

type Surface = "startWorker" | "Agent";
type KeyMode = "unset" | "trusted" | "empty-map" | "empty-key" | "bad-key";
type NatsCallback = NonNullable<SubscriptionOptions["callback"]>;
type LegacyCallback = (msg: Msg) => void | Promise<void>;

interface SecurityVector {
  name: string;
  keyMode: KeyMode;
  senderId: string;
  signed: boolean;
  allowed: boolean;
  omitTrace?: boolean;
  tamperAfterSigning?: boolean;
}

interface PacketView {
  signature?: Uint8Array;
  jobRequest?: { jobId?: string };
}

interface CallbackEntry {
  callback: NatsCallback;
  mode: "options" | "legacy";
}

interface SurfaceHarness {
  bus: SecurityBus;
  handlerCalls: () => number;
  close: () => Promise<void>;
}

const silentLogger: Logger = {
  info: () => {},
  warn: () => {},
  error: () => {},
};

const vectors: SecurityVector[] = [
  { name: "accepts a valid signed packet", keyMode: "trusted", senderId: TRUSTED_SENDER, signed: true, allowed: true },
  { name: "allows legacy unsigned only when key map is undefined", keyMode: "unset", senderId: TRUSTED_SENDER, signed: false, allowed: true },
  { name: "treats an empty key map as deny-all", keyMode: "empty-map", senderId: TRUSTED_SENDER, signed: true, allowed: false },
  { name: "rejects an unsigned packet in strict mode", keyMode: "trusted", senderId: TRUSTED_SENDER, signed: false, allowed: false },
  { name: "rejects an empty sender with a nonempty signature", keyMode: "trusted", senderId: "", signed: true, allowed: false },
  { name: "rejects an unknown sender", keyMode: "trusted", senderId: "unknown-client", signed: true, allowed: false },
  { name: "rejects a tampered signed packet", keyMode: "trusted", senderId: TRUSTED_SENDER, signed: true, allowed: false, tamperAfterSigning: true },
  { name: "rejects a prototype-like sender", keyMode: "trusted", senderId: "__proto__", signed: true, allowed: false },
  { name: "rejects a constructor-like sender", keyMode: "trusted", senderId: "constructor", signed: true, allowed: false },
  { name: "rejects an empty public key", keyMode: "empty-key", senderId: TRUSTED_SENDER, signed: true, allowed: false },
  { name: "rejects a malformed public key", keyMode: "bad-key", senderId: TRUSTED_SENDER, signed: true, allowed: false },
  { name: "rejects an invalid envelope", keyMode: "trusted", senderId: TRUSTED_SENDER, signed: true, allowed: false, omitTrace: true },
];

class SecurityBus {
  readonly published: Array<{ subject: string; data: Uint8Array }> = [];
  private readonly callbacks = new Map<string, CallbackEntry>();
  private readonly pendingLegacy = new Set<Promise<void>>();

  publish(subject: string, data: Uint8Array): void {
    this.published.push({ subject, data });
  }

  subscribe(
    subject: string,
    options: SubscriptionOptions = {},
    legacyCallback?: LegacyCallback
  ): Subscription {
    if (options.callback) {
      this.callbacks.set(subject, { callback: options.callback, mode: "options" });
    } else if (legacyCallback) {
      // Audit-only fallback: contract tests below stay RED while production uses arg 3.
      const callback: NatsCallback = (_error, msg) => this.trackLegacy(legacyCallback(msg));
      this.callbacks.set(subject, { callback, mode: "legacy" });
    }
    return fakeSubscription();
  }

  async deliver(subject: string, data: Uint8Array): Promise<void> {
    const entry = this.callbacks.get(subject);
    if (!entry) {
      throw new Error(`no callback registered for ${subject}`);
    }
    entry.callback(null, { subject, data } as unknown as Msg);
    await Promise.allSettled([...this.pendingLegacy]);
    await delay(20);
  }

  callbackMode(subject: string): "options" | "legacy" | "none" {
    return this.callbacks.get(subject)?.mode ?? "none";
  }

  resultCount(): number {
    return this.published.filter((message) => message.subject === SUBJECT_RESULT).length;
  }

  async drain(): Promise<void> {}

  private trackLegacy(result: void | Promise<void>): void {
    if (!(result instanceof Promise)) return;
    const tracked = result.finally(() => this.pendingLegacy.delete(tracked));
    this.pendingLegacy.add(tracked);
  }
}

function fakeSubscription(): Subscription {
  return {
    drain: async () => {},
    unsubscribe: () => {},
  } as unknown as Subscription;
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function keyMapFor(mode: KeyMode): Record<string, string> | undefined {
  if (mode === "unset") return undefined;
  if (mode === "empty-map") return {};
  if (mode === "empty-key") return { [TRUSTED_SENDER]: "" };
  if (mode === "bad-key") return { [TRUSTED_SENDER]: "not-a-public-key" };
  return { [TRUSTED_SENDER]: privateKeyMap.publicKey };
}

async function packetFor(vector: SecurityVector, jobId: string): Promise<Uint8Array> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const envelope: Record<string, unknown> = {
    senderId: vector.senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: { jobId, topic: TOPIC, contextPtr: `redis://ctx:${jobId}` },
  };
  if (!vector.omitTrace) envelope.traceId = `trace-${jobId}`;

  const packet = BusPacket.fromObject(envelope) as unknown as PacketView;
  if (vector.signed) {
    const signer = crypto.createSign("sha256");
    signer.update(encodeUnsignedForSignature(BusPacket, packet));
    packet.signature = signer.sign(privateKeyMap.privateKey);
  }
  if (vector.tamperAfterSigning && packet.jobRequest) {
    packet.jobRequest.jobId = `${jobId}-tampered`;
  }
  return encodeDeterministic(BusPacket, packet);
}

async function createHarness(
  surface: Surface,
  publicKeyMap: Record<string, string> | undefined,
  jobId: string,
  logger: Logger = silentLogger
): Promise<SurfaceHarness> {
  const bus = new SecurityBus();
  let calls = 0;
  if (surface === "startWorker") {
    const subscription = await startWorker({
      nc: bus as unknown as NatsConnection,
      subject: TOPIC,
      senderId: "security-worker",
      publicKeyMap,
      logger,
      handler: async (request: { jobId?: string }) => {
        calls += 1;
        return { jobId: request.jobId, status: "JOB_STATUS_SUCCEEDED" };
      },
    });
    return { bus, handlerCalls: () => calls, close: () => subscription.drain() };
  }

  const store = new InMemoryBlobStore();
  await store.set(`ctx:${jobId}`, Buffer.from("{}", "utf-8"));
  const agent = new Agent({
    store,
    publicKeyMap,
    senderId: "security-agent",
    heartbeatInterval: 60_000,
    logger,
    connectFn: async () => bus as unknown as NatsConnection,
  });
  agent.job<Record<string, never>, { ok: boolean }>(TOPIC, async () => {
    calls += 1;
    return { ok: true };
  });
  await agent.start();
  return { bus, handlerCalls: () => calls, close: () => agent.close() };
}

async function exercise(surface: Surface, vector: SecurityVector, jobId: string) {
  const harness = await createHarness(surface, keyMapFor(vector.keyMode), jobId);
  try {
    await harness.bus.deliver(TOPIC, await packetFor(vector, jobId));
    return {
      handlerCalls: harness.handlerCalls(),
      resultCount: harness.bus.resultCount(),
    };
  } finally {
    await harness.close();
  }
}

for (const surface of ["startWorker", "Agent"] as const) {
  describe(`${surface} inbound security`, () => {
    it("registers a NATS v2 callback inside subscription options", async () => {
      const harness = await createHarness(surface, undefined, `contract-${surface}`);
      try {
        assert.strictEqual(harness.bus.callbackMode(TOPIC), "options");
      } finally {
        await harness.close();
      }
    });

    for (const [index, vector] of vectors.entries()) {
      it(vector.name, async () => {
        const actual = await exercise(surface, vector, `${surface}-${index}`);
        const expectedCount = vector.allowed ? 1 : 0;
        assert.deepStrictEqual(actual, {
          handlerCalls: expectedCount,
          resultCount: expectedCount,
        });
      });
    }

    it("keeps rejected payload and key material out of logs", async () => {
      const entries: string[] = [];
      const logger: Logger = {
        info: () => undefined,
        warn: (message, fields) => entries.push(`${message} ${String(fields?.error ?? "")}`),
        error: (message, fields) => entries.push(`${message} ${String(fields?.error ?? "")}`),
      };
      const secret = `payload-secret-${surface}`;
      const harness = await createHarness(
        surface,
        keyMapFor("trusted"),
        secret,
        logger
      );
      try {
        const invalid = { ...vectors[vectors.length - 1], omitTrace: true };
        await harness.bus.deliver(TOPIC, await packetFor(invalid, secret));
        const output = entries.join("\n");
        assert.ok(output.includes("MalformedPacketError"));
        assert.ok(!output.includes(secret));
        assert.ok(!output.includes(privateKeyMap.publicKey.trim()));
      } finally {
        await harness.close();
      }
    });
  });
}
