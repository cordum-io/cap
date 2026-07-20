import assert from "node:assert";
import { generateKeyPairSync } from "node:crypto";
import type {
  Msg,
  NatsConnection,
  Subscription,
  SubscriptionOptions,
} from "nats";
import { submitJob } from "../src/client";
import { handshakePayload, publishHandshake } from "../src/handshake";
import { emitHeartbeat, heartbeatPayload } from "../src/heartbeat";
import { cancelPayload, progressPayload } from "../src/progress";
import { loadRoot } from "../src/protos";
import { verifyInboundPacket } from "../src/security";
import { validateBusPacket } from "../src/validate";
import { startWorker } from "../src/worker";

const SESSION_TOKEN = "session-node-boundary";
const keyPair = generateKeyPairSync("ec", {
  namedCurve: "prime256v1",
  privateKeyEncoding: { type: "pkcs8", format: "pem" },
  publicKeyEncoding: { type: "spki", format: "pem" },
});

interface PacketView {
  authToken?: string;
  heartbeat?: { workerId?: string };
  handshake?: { componentId?: string };
  jobResult?: { workerId?: string };
  protocolVersion?: number;
  senderId?: string;
  signature?: Uint8Array;
  traceId?: string;
}

class MockNats {
  readonly published: Array<{ subject: string; data: Uint8Array }> = [];
  private callback?: NonNullable<SubscriptionOptions["callback"]>;

  publish(subject: string, data: Uint8Array): void {
    this.published.push({ subject, data });
  }

  subscribe(
    _subject: string,
    options?: SubscriptionOptions
  ): Subscription {
    this.callback = options?.callback;
    return { unsubscribe: () => undefined } as unknown as Subscription;
  }

  deliver(data: Uint8Array): void {
    assert.ok(this.callback, "worker subscription callback missing");
    this.callback(null, { data, subject: "job.boundary" } as Msg);
  }
}

function nats(mock: MockNats): NatsConnection {
  return mock as unknown as NatsConnection;
}

async function waitForPublish(mock: MockNats): Promise<void> {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    if (mock.published.length > 0) return;
    await new Promise((resolve) => setTimeout(resolve, 2));
  }
  assert.fail("timed out waiting for publish");
}

describe("generic packet validation", () => {
  it("uses a fresh trace for each legacy handshake", async () => {
    const first = await handshakePayload("worker");
    const second = await handshakePayload("worker");
    assert.notStrictEqual(first.traceId, "worker");
    assert.notStrictEqual(first.traceId, second.traceId);
    assert.strictEqual(first.handshake.sdkVersion, "cap-node/v2");
  });

  it("binds exact protocol and nested producer identities", () => {
    const base = {
      traceId: "trace",
      senderId: "worker",
      protocolVersion: 1,
      createdAt: { seconds: 1, nanos: 0 },
    };
    assert.deepStrictEqual(validateBusPacket({ ...base, heartbeat: { workerId: "other" } })
      .map(({ field }) => field), ["heartbeat.worker_id"]);
    assert.deepStrictEqual(validateBusPacket({ ...base, handshake: { componentId: "other" } })
      .map(({ field }) => field), ["handshake.component_id"]);
    assert.deepStrictEqual(validateBusPacket({ ...base, jobResult: {
      jobId: "job", status: 1, workerId: "other",
    } }).map(({ field }) => field), ["job_result.worker_id"]);
    assert.deepStrictEqual(validateBusPacket({ ...base, alert: {
      message: "unsafe", sourceComponent: "other",
    } }).map(({ field }) => field), ["alert.source_component"]);
    assert.deepStrictEqual(validateBusPacket({ ...base, alert: {
      message: "unsafe", details: { policy: "deny" },
    } }).map(({ field }) => field), ["alert.source_component"]);
    assert.deepStrictEqual(validateBusPacket({ ...base, alert: {
      level: "warning", message: "legacy", component: "worker", code: "legacy",
    } }), []);
    assert.deepStrictEqual(validateBusPacket({ ...base, protocolVersion: 2,
      heartbeat: { workerId: "worker" } }).map(({ field }) => field), ["protocol_version"]);
  });

});

describe("generic outbound builder boundary", () => {
  it("attaches a session token to every public packet builder", async () => {
    const mock = new MockNats();
    const packets: PacketView[] = [
      await handshakePayload(
        "worker", {}, "worker", [], "agent", "cap-node/v2", SESSION_TOKEN
      ),
      await heartbeatPayload("worker", "pool", 0, 1, 0, "", "", SESSION_TOKEN),
      await progressPayload("worker", "job", "step", 10, "running", SESSION_TOKEN),
      await cancelPayload("worker", "job", "cancel", "operator", SESSION_TOKEN),
    ];
    await submitJob(nats(mock), { jobId: "job", topic: "job.boundary" },
      "trace", "client", undefined, SESSION_TOKEN);
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    packets.push(BusPacket.decode(mock.published[0].data) as unknown as PacketView);
    for (const packet of packets) {
      assert.strictEqual(packet.authToken, SESSION_TOKEN);
      assert.deepStrictEqual(validateBusPacket(packet), []);
    }
  });

});

describe("generic outbound signing boundary", () => {
  it("validates before signing or publishing", async () => {
    const mock = new MockNats();
    const packet = await handshakePayload("worker");
    assert.ok(packet.handshake);
    packet.handshake.componentId = "impersonated";
    await assert.rejects(
      () => publishHandshake(nats(mock), packet, keyPair.privateKey),
      /handshake\.component_id/
    );
    assert.strictEqual(packet.signature?.length ?? 0, 0);
    assert.strictEqual(mock.published.length, 0);
  });

  it("rejects noncanonical session tokens", async () => {
    await assert.rejects(
      () => heartbeatPayload("worker", "pool", 0, 1, 0, "", "", "bad token"),
      /auth_token/
    );
  });

  it("revalidates a mutated session token before signing", async () => {
    const mock = new MockNats();
    const packet = await heartbeatPayload("worker", "pool", 0, 1, 0);
    packet.authToken = "mutated token";
    await assert.rejects(
      () => emitHeartbeat(nats(mock), packet, keyPair.privateKey),
      /auth_token/
    );
    assert.strictEqual(packet.signature?.length ?? 0, 0);
    assert.strictEqual(mock.published.length, 0);
  });
});

describe("authenticated outbound signing", () => {
  it("signs the installed session token", async () => {
    const mock = new MockNats();
    const packet = await heartbeatPayload(
      "worker", "pool", 0, 1, 0, "", "", SESSION_TOKEN
    );
    await emitHeartbeat(nats(mock), packet, keyPair.privateKey);
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(mock.published[0].data) as unknown as PacketView;
    verifyInboundPacket(BusPacket, decoded, { worker: keyPair.publicKey });
    decoded.authToken = "tampered-session";
    assert.throws(
      () => verifyInboundPacket(BusPacket, decoded, { worker: keyPair.publicKey }),
      /signature verification failed/
    );
  });
});

describe("low-level worker packet boundary", () => {
  it("attaches the session token to low-level worker results", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const mock = new MockNats();
    await startWorker({
      nc: nats(mock), subject: "job.boundary", senderId: "worker",
      sessionToken: SESSION_TOKEN,
      handler: async () => ({ jobId: "job", status: 2, workerId: "worker" }),
    });
    const request = BusPacket.fromObject({
      traceId: "trace", senderId: "client", protocolVersion: 1,
      createdAt: { seconds: 1, nanos: 0 },
      jobRequest: { jobId: "job", topic: "job.boundary" },
    });
    mock.deliver(BusPacket.encode(request).finish());
    await waitForPublish(mock);
    const result = BusPacket.decode(mock.published[0].data) as unknown as PacketView;
    assert.strictEqual(result.authToken, SESSION_TOKEN);
    assert.deepStrictEqual(validateBusPacket(result), []);
  });
});
