/** Installed-tarball CAP worker-trust interop client; never prints key/token material. */
import { readFileSync } from "node:fs";
import { createPublicKey, randomBytes, randomUUID } from "node:crypto";

import {
  WORKER_HANDSHAKE_AUDIENCE,
  buildAuthenticate,
  buildChallengeRequest,
  createWorkerTrustConfig,
  marshalWorkerTrustPacket,
  loadRoot,
  signWorkerTrustPacket,
  unmarshalWorkerTrustPacket,
  verifyChallenge,
  verifyResult,
  verifyWorkerTrustPacket,
  type ChallengeRequestTrustPacket,
  type WorkerCapabilityHandshake,
  type WorkerHandshakePurpose,
  type WorkerHandshakeSession,
  type WorkerTrustConfig,
  type WorkerTrustPacket,
} from "cap-sdk-node";
import {
  connect,
  ErrorCode,
  NatsError,
  type NatsConnection,
} from "nats";

const ISSUE: WorkerHandshakePurpose = 1;
const RENEW: WorkerHandshakePurpose = 2;

interface Settings {
  readonly testCase: string;
  readonly natsUrl: string;
  readonly trust: WorkerTrustConfig;
}

interface Result {
  language: "node";
  case: string;
  status: "PASS";
  issue: boolean;
  renew: boolean;
  rotated: boolean;
  mutation_signature_valid: boolean;
  tamper_signature_rejected: boolean;
}

interface MutationProof {
  readonly signatureValid: boolean;
  readonly tamperRejected: boolean;
}

const NO_MUTATION_PROOF: MutationProof = {
  signatureValid: false,
  tamperRejected: false,
};

function required(name: string): string {
  const value = process.env[name]?.trim() ?? "";
  if (!value) throw new Error(`missing required environment: ${name}`);
  return value;
}

function loadSettings(): Settings {
  const schedulerKeyId = required("CAP_HANDSHAKE_SCHEDULER_KEY_ID");
  const trust = createWorkerTrustConfig({
    workerId: required("CAP_HANDSHAKE_WORKER_ID"),
    expectedAgentId: required("CAP_HANDSHAKE_AGENT_ID"),
    tenantId: required("CAP_HANDSHAKE_TENANT_ID"),
    audience: WORKER_HANDSHAKE_AUDIENCE,
    proofKeyId: required("CAP_HANDSHAKE_PROOF_KEY_ID"),
    proofPrivateKey: readFileSync(required("CAP_HANDSHAKE_WORKER_PRIVATE_KEY")),
    expectedSchedulerId: required("CAP_HANDSHAKE_SCHEDULER_ID"),
    schedulerPublicKeys: {
      [schedulerKeyId]: readFileSync(required("CAP_HANDSHAKE_SCHEDULER_PUBLIC_KEY")),
    },
    sdkVersion: required("CAP_HANDSHAKE_SDK_VERSION"),
  });
  return {
    testCase: required("CAP_HANDSHAKE_CASE"),
    natsUrl: required("CAP_HANDSHAKE_NATS_URL"),
    trust,
  };
}

function requestOptions(purpose: WorkerHandshakePurpose, createdAt: Date) {
  return {
    requestId: randomUUID().replace(/-/g, ""),
    traceId: randomUUID().replace(/-/g, ""),
    purpose,
    clientNonce: randomBytes(32),
    createdAt,
  };
}

function capability(trust: WorkerTrustConfig): WorkerCapabilityHandshake {
  return {
    componentId: trust.workerId,
    role: 3,
    supportedVersions: [1],
    capabilities: { progress: true },
    sdkVersion: trust.sdkVersion,
    readyTopics: ["job.interop"],
    agentName: trust.workerId,
  };
}

async function requestPacket(
  connection: NatsConnection,
  subject: string,
  packet: WorkerTrustPacket
): Promise<WorkerTrustPacket> {
  const message = await connection.request(
    subject,
    await marshalWorkerTrustPacket(packet),
    { timeout: 3_000 }
  );
  return unmarshalWorkerTrustPacket(message.data);
}

async function exchange(
  connection: NatsConnection,
  trust: WorkerTrustConfig,
  purpose: WorkerHandshakePurpose,
  token: string
): Promise<WorkerHandshakeSession> {
  const request = await buildChallengeRequest(trust, requestOptions(purpose, new Date()));
  const challenge = await requestPacket(connection, "sys.worker.handshake.challenge", request);
  if (!("workerHandshakeChallenge" in challenge)) throw new Error("challenge response required");
  const verified = await verifyChallenge(trust, request, challenge, new Date());
  const authenticate = await buildAuthenticate(
    trust, verified, capability(trust), token, new Date()
  );
  const result = await requestPacket(connection, "sys.worker.handshake.authenticate", authenticate);
  if (!("workerHandshakeResult" in result)) throw new Error("result response required");
  return verifyResult(trust, verified, authenticate, result, new Date());
}

async function mutate(
  testCase: string,
  packet: ChallengeRequestTrustPacket,
  trust: WorkerTrustConfig
): Promise<ChallengeRequestTrustPacket> {
  switch (testCase) {
    case "wrong_audience":
      packet.workerHandshakeChallengeRequest.audience = "other-scheduler";
      return signWorkerTrustPacket(packet, { [trust.proofKeyId]: trust.proofPrivateKey });
    case "missing_identity":
      packet.senderId = "";
      break;
    case "missing_trace":
      packet.traceId = "";
      break;
    case "unsupported_version":
      packet.protocolVersion = 2;
      break;
    case "tamper":
      packet.workerHandshakeChallengeRequest.audience = "tampered-after-signing";
      break;
    case "skew":
      break;
    default:
      throw new Error(`unsupported negative case: ${testCase}`);
  }
  return packet;
}

async function proveMutationSignature(
  testCase: string,
  packet: ChallengeRequestTrustPacket,
  trust: WorkerTrustConfig
): Promise<MutationProof> {
  if (testCase !== "wrong_audience" && testCase !== "tamper") {
    return NO_MUTATION_PROOF;
  }
  const keys = { [trust.proofKeyId]: createPublicKey(trust.proofPrivateKey) };
  const signatureValid = await verifyWorkerTrustPacket(packet, keys);
  if (testCase === "wrong_audience") {
    if (!signatureValid) throw new Error("re-signed mutation has an invalid signature");
    return { signatureValid: true, tamperRejected: false };
  }
  if (testCase === "tamper") {
    if (signatureValid) throw new Error("tampered packet retained a valid signature");
    return { signatureValid: false, tamperRejected: true };
  }
  throw new Error(`mutation proof is not implemented for ${testCase}`);
}

async function expectRejected(
  connection: NatsConnection,
  packet: ChallengeRequestTrustPacket
): Promise<void> {
  try {
    await connection.request(
      "sys.worker.handshake.challenge",
      await rawEncode(packet),
      { timeout: 750 }
    );
  } catch (error) {
    if (error instanceof NatsError && error.code === ErrorCode.Timeout) return;
    throw error;
  }
  throw new Error("negative request received a reply");
}

async function exerciseNegative(
  connection: NatsConnection,
  settings: Settings
): Promise<MutationProof> {
  const createdAt = new Date(Date.now() + (settings.testCase === "skew" ? 61_000 : 0));
  let request = await buildChallengeRequest(settings.trust, requestOptions(ISSUE, createdAt));
  if (settings.testCase === "replay") {
    await requestPacket(connection, "sys.worker.handshake.challenge", request);
    await expectRejected(connection, request);
    return NO_MUTATION_PROOF;
  }
  if (settings.testCase !== "impersonation") {
    request = await mutate(settings.testCase, request, settings.trust);
  }
  const proof = await proveMutationSignature(settings.testCase, request, settings.trust);
  await expectRejected(connection, request);
  return proof;
}

async function rawEncode(packet: ChallengeRequestTrustPacket): Promise<Uint8Array> {
  const root = await loadRoot();
  return root.lookupType("cordum.agent.v1.BusPacket").encode(packet).finish();
}

async function run(): Promise<Result> {
  const settings = loadSettings();
  const connection = await connect({ servers: settings.natsUrl, name: "cap-node-handshake-interop" });
  const result: Result = {
    language: "node", case: settings.testCase, status: "PASS",
    issue: false, renew: false, rotated: false,
    mutation_signature_valid: false, tamper_signature_rejected: false,
  };
  try {
    if (settings.testCase !== "valid") {
      const proof = await exerciseNegative(connection, settings);
      result.mutation_signature_valid = proof.signatureValid;
      result.tamper_signature_rejected = proof.tamperRejected;
      return result;
    }
    const issued = await exchange(connection, settings.trust, ISSUE, "");
    const renewed = await exchange(connection, settings.trust, RENEW, issued.token);
    result.issue = true;
    result.renew = true;
    result.rotated = Boolean(issued.token && renewed.token && issued.token !== renewed.token);
    if (!result.rotated) throw new Error("session did not rotate");
    return result;
  } finally {
    await connection.drain();
  }
}

run().then(
  (result) => process.stdout.write(`${JSON.stringify(result)}\n`),
  (error: unknown) => {
    const type = error instanceof Error ? error.name : "UnknownError";
    process.stderr.write(`handshake interop client failed: ${type}\n`);
    process.exitCode = 1;
  }
);
