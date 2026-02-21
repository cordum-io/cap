import assert from "node:assert";
import * as crypto from "crypto";
import { connect } from "nats";
import {
  progressPayload,
  cancelPayload,
  emitProgress,
  emitCancel,
} from "../src/progress";
import { encodeDeterministic, encodeUnsignedForSignature } from "../src/codec";
import { SUBJECT_PROGRESS, SUBJECT_CANCEL, loadRoot } from "../src/protos";
import { MockNatsConnection } from "../src/testing";

async function waitFor(fn: () => boolean, timeoutMs = 1000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (fn()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error("timeout waiting for condition");
}

describe("progress/cancel helpers", () => {
  it("builds progress payload with correct fields", async () => {
    const packet = await progressPayload("worker-1", "job-1", "step-1", 50, "halfway");

    assert.strictEqual(packet.senderId, "worker-1");
    assert.strictEqual(packet.protocolVersion, 1);
    assert.ok(packet.createdAt);
    assert.strictEqual(packet.jobProgress.jobId, "job-1");
    assert.strictEqual(packet.jobProgress.stepId, "step-1");
    assert.strictEqual(packet.jobProgress.percent, 50);
    assert.strictEqual(packet.jobProgress.message, "halfway");
  });

  it("builds progress payload with zero percent and empty message", async () => {
    const packet = await progressPayload("worker-1", "job-1", "step-1", 0, "");

    assert.strictEqual(packet.jobProgress.jobId, "job-1");
    assert.strictEqual(packet.jobProgress.percent, 0);
    assert.strictEqual(packet.jobProgress.message, "");
  });

  it("builds progress payload with 100 percent", async () => {
    const packet = await progressPayload("worker-1", "job-1", "step-done", 100, "complete");

    assert.strictEqual(packet.jobProgress.percent, 100);
    assert.strictEqual(packet.jobProgress.stepId, "step-done");
    assert.strictEqual(packet.jobProgress.message, "complete");
  });

  it("builds cancel payload with correct fields", async () => {
    const packet = await cancelPayload("worker-1", "job-1", "timeout", "user-7");

    assert.strictEqual(packet.senderId, "worker-1");
    assert.strictEqual(packet.protocolVersion, 1);
    assert.ok(packet.createdAt);
    assert.strictEqual(packet.jobCancel.jobId, "job-1");
    assert.strictEqual(packet.jobCancel.reason, "timeout");
    assert.strictEqual(packet.jobCancel.requestedBy, "user-7");
  });

  it("builds cancel payload with empty reason", async () => {
    const packet = await cancelPayload("worker-1", "job-1", "", "");

    assert.strictEqual(packet.jobCancel.jobId, "job-1");
    assert.strictEqual(packet.jobCancel.reason, "");
    assert.strictEqual(packet.jobCancel.requestedBy, "");
  });

  it("emits progress without signature", async () => {
    const nc = new MockNatsConnection();
    const packet = await progressPayload("worker-emit", "job-1", "step-1", 50, "halfway");

    await emitProgress(nc as any, packet);

    assert.strictEqual(nc.published.length, 1);
    assert.strictEqual(nc.published[0].subject, SUBJECT_PROGRESS);

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(nc.published[0].data) as any;
    assert.strictEqual(decoded.senderId, "worker-emit");
    assert.strictEqual(decoded.jobProgress.jobId, "job-1");
    assert.strictEqual(decoded.signature?.length ?? 0, 0);
  });

  it("emits progress with verifiable signature", async () => {
    const nc = new MockNatsConnection();
    const keyPair = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });

    const packet = await progressPayload("worker-sign", "job-1", "step-1", 75, "almost");
    await emitProgress(nc as any, packet, keyPair.privateKey);

    assert.strictEqual(nc.published.length, 1);
    assert.strictEqual(nc.published[0].subject, SUBJECT_PROGRESS);

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(nc.published[0].data) as any;
    assert.ok(decoded.signature);

    const unsigned = encodeUnsignedForSignature(BusPacket, decoded);
    const verify = crypto.createVerify("sha256");
    verify.update(unsigned);
    assert.strictEqual(verify.verify(keyPair.publicKey, decoded.signature), true);
  });

  it("emits cancel without signature", async () => {
    const nc = new MockNatsConnection();
    const packet = await cancelPayload("worker-emit", "job-1", "timeout", "user-7");

    await emitCancel(nc as any, packet);

    assert.strictEqual(nc.published.length, 1);
    assert.strictEqual(nc.published[0].subject, SUBJECT_CANCEL);

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(nc.published[0].data) as any;
    assert.strictEqual(decoded.senderId, "worker-emit");
    assert.strictEqual(decoded.jobCancel.jobId, "job-1");
    assert.strictEqual(decoded.signature?.length ?? 0, 0);
  });

  it("emits cancel with verifiable signature", async () => {
    const nc = new MockNatsConnection();
    const keyPair = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });

    const packet = await cancelPayload("worker-sign", "job-1", "user-requested", "admin-1");
    await emitCancel(nc as any, packet, keyPair.privateKey);

    assert.strictEqual(nc.published.length, 1);
    assert.strictEqual(nc.published[0].subject, SUBJECT_CANCEL);

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const decoded = BusPacket.decode(nc.published[0].data) as any;
    assert.ok(decoded.signature);

    const unsigned = encodeUnsignedForSignature(BusPacket, decoded);
    const verify = crypto.createVerify("sha256");
    verify.update(unsigned);
    assert.strictEqual(verify.verify(keyPair.publicKey, decoded.signature), true);
  });

  it("produces valid encoded packets for zero/empty values", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

    const progPacket = await progressPayload("worker-zero", "job-1", "", 0, "");
    const progEncoded = encodeDeterministic(BusPacket, progPacket);
    const progDecoded = BusPacket.decode(progEncoded) as any;
    assert.strictEqual(progDecoded.senderId, "worker-zero");
    assert.strictEqual(progDecoded.jobProgress.jobId, "job-1");
    assert.strictEqual(progDecoded.jobProgress.percent, 0);

    const cancPacket = await cancelPayload("worker-zero", "job-1", "", "");
    const cancEncoded = encodeDeterministic(BusPacket, cancPacket);
    const cancDecoded = BusPacket.decode(cancEncoded) as any;
    assert.strictEqual(cancDecoded.senderId, "worker-zero");
    assert.strictEqual(cancDecoded.jobCancel.jobId, "job-1");
  });

  it("integration: publishes progress and cancel on NATS subjects", async function () {
    this.timeout(10_000);

    const natsUrl = process.env.CAP_TEST_NATS_URL ?? "nats://127.0.0.1:4222";
    let nc: any;
    try {
      nc = await connect({
        servers: natsUrl,
        name: "cap-progress-integration",
        maxReconnectAttempts: 0,
        timeout: 1000,
      });
    } catch (err) {
      this.skip();
      return;
    }

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const keyPair = crypto.generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    const senderId = `worker-int-${Date.now()}`;
    let matchedProgress: any;
    let matchedCancel: any;

    const progSub = nc.subscribe(SUBJECT_PROGRESS, {
      callback: (_err: Error | null, msg: any) => {
        const packet = BusPacket.decode(msg.data) as any;
        if (packet?.senderId === senderId) {
          matchedProgress = packet;
        }
      },
    });

    const cancSub = nc.subscribe(SUBJECT_CANCEL, {
      callback: (_err: Error | null, msg: any) => {
        const packet = BusPacket.decode(msg.data) as any;
        if (packet?.senderId === senderId) {
          matchedCancel = packet;
        }
      },
    });

    try {
      const progPacket = await progressPayload(senderId, "job-int", "step-int", 42, "processing");
      await emitProgress(nc, progPacket, keyPair.privateKey);

      await waitFor(() => matchedProgress != null, 3000);

      assert.strictEqual(matchedProgress.senderId, senderId);
      assert.strictEqual(matchedProgress.protocolVersion, 1);
      assert.strictEqual(matchedProgress.jobProgress.jobId, "job-int");
      assert.strictEqual(matchedProgress.jobProgress.stepId, "step-int");
      assert.strictEqual(matchedProgress.jobProgress.percent, 42);
      assert.strictEqual(matchedProgress.jobProgress.message, "processing");
      assert.ok(matchedProgress.signature);

      const unsignedProg = encodeUnsignedForSignature(BusPacket, matchedProgress);
      const verifyProg = crypto.createVerify("sha256");
      verifyProg.update(unsignedProg);
      assert.strictEqual(verifyProg.verify(keyPair.publicKey, matchedProgress.signature), true);

      const cancPacket = await cancelPayload(senderId, "job-int", "user-abort", "admin-int");
      await emitCancel(nc, cancPacket, keyPair.privateKey);

      await waitFor(() => matchedCancel != null, 3000);

      assert.strictEqual(matchedCancel.senderId, senderId);
      assert.strictEqual(matchedCancel.jobCancel.jobId, "job-int");
      assert.strictEqual(matchedCancel.jobCancel.reason, "user-abort");
      assert.strictEqual(matchedCancel.jobCancel.requestedBy, "admin-int");
      assert.ok(matchedCancel.signature);

      const unsignedCanc = encodeUnsignedForSignature(BusPacket, matchedCancel);
      const verifyCanc = crypto.createVerify("sha256");
      verifyCanc.update(unsignedCanc);
      assert.strictEqual(verifyCanc.verify(keyPair.publicKey, matchedCancel.signature), true);
    } finally {
      progSub.unsubscribe();
      cancSub.unsubscribe();
      await nc.drain();
    }
  });
});
