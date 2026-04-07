import fs from "fs";
import path from "path";
import assert from "node:assert";
import Long from "long";
import * as crypto from "crypto";
import { loadRoot } from "../src/protos";
import { encodeUnsignedForSignature } from "../src/codec";

function repoRoot(): string {
  return path.resolve(__dirname, "..", "..", "..", "..");
}

function fixturePath(name: string): string {
  return path.join(repoRoot(), "spec", "conformance", "fixtures", name);
}

function asNumber(value: any): number {
  if (Long.isLong(value)) {
    return value.toNumber();
  }
  if (typeof value === "number") {
    return value;
  }
  return Number(value);
}

describe("CAP conformance fixtures", () => {
  const publicKey = fs.readFileSync(fixturePath("public_key.pem"), "utf8");

  function verifySignature(pkt: any, BusPacket: any) {
    assert.ok(pkt.signature);
    const unsigned = encodeUnsignedForSignature(BusPacket, pkt);
    const verify = crypto.createVerify("sha256");
    verify.update(unsigned);
    assert.strictEqual(verify.verify(publicKey, Buffer.from(pkt.signature)), true);
  }

  it("decodes job request fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_job_request.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    assert.strictEqual(pkt.traceId, "trace-job-request");
    assert.strictEqual(pkt.senderId, "client-1");
    assert.strictEqual(pkt.protocolVersion, 1);
    assert.strictEqual(asNumber(pkt.createdAt?.seconds), 1704164645);

    const req = pkt.jobRequest;
    assert.ok(req);
    assert.strictEqual(req.jobId, "job-req-1");
    assert.strictEqual(req.topic, "job.tools");
    assert.strictEqual(req.priority, 1);
    assert.strictEqual(req.contextPtr, "redis://ctx:job-req-1");
    assert.strictEqual(req.env.region, "us-east-1");
    assert.strictEqual(req.env.sandbox, "true");
    assert.strictEqual(req.labels.env, "prod");
    assert.strictEqual(req.labels.team, "platform");
    assert.strictEqual(req.meta.idempotencyKey, "idem-123");
    assert.strictEqual(req.meta.labels.source, "conformance");
    assert.strictEqual(req.compensation.topic, "job.rollback");
    assert.strictEqual(req.compensation.labels.rollback, "true");
  });

  it("decodes job result fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_job_result.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    assert.strictEqual(pkt.traceId, "trace-job-result");
    const res = pkt.jobResult;
    assert.ok(res);
    assert.strictEqual(res.jobId, "job-res-1");
    assert.strictEqual(res.workerId, "worker-1");
    assert.strictEqual(res.status, 10); // JOB_STATUS_FAILED_RETRYABLE
    assert.strictEqual(res.errorCode, "E_TEMP");
    assert.strictEqual(res.artifactPtrs.length, 2);
  });

  it("decodes heartbeat fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_heartbeat.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    assert.strictEqual(pkt.traceId, "trace-heartbeat");
    const hb = pkt.heartbeat;
    assert.ok(hb);
    assert.strictEqual(hb.workerId, "worker-1");
    assert.strictEqual(hb.pool, "job.tools");
    assert.strictEqual(hb.labels.zone, "us-east-1a");
    assert.strictEqual(hb.progressPct, 60);
    assert.strictEqual(hb.authToken, "attest-worker-1");
  });

  it("decodes job progress fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_job_progress.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    const progress = pkt.jobProgress;
    assert.ok(progress);
    assert.strictEqual(progress.jobId, "job-prog-1");
    assert.strictEqual(progress.percent, 50);
    assert.strictEqual(progress.status, 4); // JOB_STATUS_RUNNING
  });

  it("decodes job cancel fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_job_cancel.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    const cancel = pkt.jobCancel;
    assert.ok(cancel);
    assert.strictEqual(cancel.jobId, "job-cancel-1");
    assert.strictEqual(cancel.requestedBy, "user-7");
  });

  it("decodes alert fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_alert.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    const alert = pkt.alert;
    assert.ok(alert);
    assert.strictEqual(alert.level, "WARN");
    assert.strictEqual(alert.component, "scheduler");
  });

  it("decodes handshake fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_handshake.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    assert.strictEqual(pkt.traceId, "trace-handshake");
    assert.strictEqual(pkt.senderId, "worker-1");
    assert.strictEqual(pkt.protocolVersion, 1);

    const hs = pkt.handshake;
    assert.ok(hs);
    assert.strictEqual(hs.componentId, "worker-1");
    assert.strictEqual(hs.role, 3); // COMPONENT_ROLE_WORKER
    assert.deepStrictEqual(hs.supportedVersions, [1]);
    assert.strictEqual(hs.capabilities.signatures, true);
    assert.strictEqual(hs.capabilities.progress, true);
    assert.strictEqual(hs.capabilities.cancel, true);
    assert.strictEqual(hs.capabilities.compensation, false);
    assert.strictEqual(hs.sdkVersion, "2.0.19");
    assert.deepStrictEqual(hs.readyTopics, ["job.tools", "job.tools.bulk"]);
  });

  it("decodes enhanced alert fixture", async () => {
    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const data = fs.readFileSync(fixturePath("buspacket_alert_enhanced.bin"));
    const pkt = BusPacket.decode(data) as any;
    verifySignature(pkt, BusPacket);

    assert.strictEqual(pkt.traceId, "trace-alert-enhanced");

    const alert = pkt.alert;
    assert.ok(alert);
    // Legacy fields
    assert.strictEqual(alert.level, "CRITICAL");
    assert.strictEqual(alert.component, "scheduler");
    assert.strictEqual(alert.code, "SIGNATURE_INVALID");
    // Enhanced fields
    assert.strictEqual(alert.severity, 4); // ALERT_SEVERITY_CRITICAL
    assert.strictEqual(alert.errorCodeEnum, 103); // ERROR_CODE_PROTOCOL_SIGNATURE_INVALID
    assert.strictEqual(alert.sourceComponent, "scheduler-1");
    assert.strictEqual(alert.details.sender, "worker-bad");
    assert.strictEqual(alert.details.subject, "sys.job.result");
    assert.strictEqual(alert.traceId, "trace-offending-packet");
  });
});
