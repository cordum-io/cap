/**
 * Node side of the CAP cross-language fixture matrix.
 *
 * Run as `node driver.cjs produce|consume` with the request JSON on stdin and
 * the response JSON on stdout. Only the installed `cap-sdk-node` package's
 * public API is used, so a green matrix edge is evidence about the packed
 * tarball rather than about repository source.
 */
"use strict";

const crypto = require("node:crypto");
const cap = require("cap-sdk-node");

const SDK = "node";

function pemPublicKey(base64Der) {
  const lines = base64Der.match(/.{1,64}/g) ?? [];
  return `-----BEGIN PUBLIC KEY-----\n${lines.join("\n")}\n-----END PUBLIC KEY-----\n`;
}

function buildPacket(testCase, corpus, keyId, createdAtUnix, expiresAtUnix) {
  const packet = {
    traceId: testCase.traceId,
    senderId: corpus.senderId,
    // Zero-valued proto3 scalars are deliberately omitted rather than set
    // explicitly: protobufjs encodes an explicit `nanos: 0` on the wire, which
    // Go and Python drop as a default. CAP-PRODUCTION signs exact bytes and
    // dedupes on the signed-body digest, so a non-canonical encoding is a real
    // interop defect, not a cosmetic one.
    createdAt: { seconds: createdAtUnix },
    protocolVersion: 1,
    signatureMetadata: {
      profileVersion: "cap-production-v1",
      algorithm: "ECDSA-P256-SHA256",
      messageId: Buffer.from(testCase.messageId, "base64"),
      audience: corpus.audience,
      expiresAt: { seconds: expiresAtUnix },
      keyId,
    },
    identity: {
      tenantId: testCase.identity.tenantId,
      principalId: testCase.identity.principalId,
      actorId: testCase.identity.actorId,
      delegationId: testCase.identity.delegationId,
    },
  };
  attachPayload(packet, testCase.payload);
  return packet;
}

function attachPayload(packet, payload) {
  switch (payload.kind) {
    case "jobRequest":
      packet.jobRequest = {
        jobId: payload.jobId ?? "",
        topic: payload.topic ?? "",
        tenantId: payload.tenantId ?? "",
        principalId: payload.principalId ?? "",
        ...(payload.contextRef ? { contextRef: resourceRef(payload.contextRef) } : {}),
      };
      return;
    case "jobResult":
      packet.jobResult = {
        jobId: payload.jobId ?? "",
        status: payload.status ?? 0,
        workerId: payload.workerId ?? "",
        executionMs: payload.executionMs ?? 0,
        ...dispatchField(payload),
        ...(payload.resultRef ? { resultRef: resourceRef(payload.resultRef) } : {}),
        ...(payload.artifactRefs
          ? { artifactRefs: payload.artifactRefs.map(resourceRef) }
          : {}),
      };
      return;
    case "jobProgress":
      packet.jobProgress = {
        jobId: payload.jobId ?? "",
        stepId: payload.stepId ?? "",
        percent: payload.percent ?? 0,
        message: payload.message ?? "",
        ...dispatchField(payload),
      };
      return;
    case "heartbeat":
      packet.heartbeat = {
        workerId: payload.workerId ?? "",
        region: payload.region ?? "",
        type: payload.type ?? "",
        activeJobs: payload.activeJobs ?? 0,
        pool: payload.pool ?? "",
      };
      return;
    default:
      throw new Error(`unsupported payload kind ${payload.kind}`);
  }
}

function dispatchField(payload) {
  if (!payload.dispatch) return {};
  return {
    dispatch: {
      dispatchId: payload.dispatch.dispatchId ?? "",
      attempt: payload.dispatch.attempt ?? 0,
      assignedWorkerId: payload.dispatch.assignedWorkerId ?? "",
    },
  };
}

// resourceRef maps a neutral corpus resource object to a ResourceRef. As with
// the packet root, zero-valued proto3 scalars (and an unset nanos on expiresAt)
// are omitted so the encoded bytes stay canonical and identical across SDKs.
function resourceRef(raw) {
  const ref = {
    resolverId: raw.resolverId ?? "",
    uri: raw.uri ?? "",
    sha256: Buffer.from(raw.sha256 ?? "", "base64"),
    mediaType: raw.mediaType ?? "",
    sizeBytes: raw.sizeBytes ?? 0,
    purpose: raw.purpose ?? "",
  };
  if (raw.expiresAtUnix) ref.expiresAt = { seconds: raw.expiresAtUnix };
  return ref;
}

function sha256Hex(bytes) {
  return crypto.createHash("sha256").update(bytes).digest("hex");
}

/** Recompute both digests from the wire, independently of any producer claim. */
function digests(raw, busPacketType) {
  const extracted = cap.extractProductionSignature(raw);
  const unsigned = Buffer.from(extracted.unsigned);
  const decoded = busPacketType.decode(unsigned);
  delete decoded.signature;
  const canonical = Buffer.from(busPacketType.encode(decoded).finish());
  return {
    normalizedDigest: sha256Hex(canonical),
    preimageDigest: sha256Hex(unsigned),
  };
}

async function busPacketType() {
  const root = await cap.loadRoot();
  return root.lookupType("cordum.agent.v1.BusPacket");
}

async function produce(request) {
  const type = await busPacketType();
  const corpus = request.corpus;
  const fixtures = corpus.cases.map((testCase) => {
    const packet = buildPacket(
      testCase, corpus, request.keyId,
      request.createdAtUnix, request.expiresAtUnix,
    );
    const raw = cap.signProductionPacket(packet, type, request.privateKeyPem);
    return {
      case: testCase.name,
      wire: Buffer.from(raw).toString("base64"),
      keyId: request.keyId,
      ...digests(raw, type),
    };
  });
  return { sdk: SDK, fixtures };
}

async function consume(request) {
  const type = await busPacketType();
  return { sdk: SDK, results: request.jobs.map((job) => runJob(request, job, type)) };
}

function runJob(request, job, type) {
  const result = {
    id: job.id, ok: false, normalizedDigest: "", preimageDigest: "", error: "",
  };
  try {
    const raw = Buffer.from(job.wire, "base64");
    const trust = {
      audience: request.audience,
      tenant: request.tenantId,
      sender: request.senderId,
      publicKeys: { [job.keyId]: pemPublicKey(job.publicKeyDer) },
    };
    const packet = cap.verifyProductionPacket(raw, type, trust);
    const errors = cap.validateBusPacket(packet);
    if (errors.length > 0) {
      throw new Error(`validate: ${JSON.stringify(errors)}`);
    }
    Object.assign(result, digests(raw, type), { ok: true });
  } catch (error) {
    result.error = `${error.name}: ${error.message}`;
  }
  return result;
}

async function main() {
  const mode = process.argv[2];
  if (mode !== "produce" && mode !== "consume") {
    process.stderr.write("usage: driver.cjs produce|consume\n");
    process.exit(2);
  }
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  const request = JSON.parse(Buffer.concat(chunks).toString("utf8"));
  const response = mode === "produce" ? await produce(request) : await consume(request);
  process.stdout.write(JSON.stringify(response));
}

main().catch((error) => {
  process.stderr.write(`matrix-driver-node: ${error.stack ?? error}\n`);
  process.exit(1);
});
