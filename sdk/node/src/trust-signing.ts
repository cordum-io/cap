import * as crypto from "node:crypto";
import { Type, Writer } from "protobufjs";

import { encodeDeterministic } from "./codec";
import { loadRoot } from "./protos";
import {
  isStrictDerSignature,
  requireP256Key,
  requireSignature,
  WorkerTrustSigningError,
} from "./trust-signing-crypto";
import {
  WorkerTrustEnvelope,
  WorkerTrustPacket,
  WorkerTrustPhase,
} from "./worker-trust-types";
import { validateWorkerTrustPacket } from "./worker-trust-packet";

export { WorkerTrustSigningError } from "./trust-signing-crypto";
export type {
  AuthenticateTrustPacket,
  ChallengeRequestTrustPacket,
  ChallengeTrustPacket,
  ResultTrustPacket,
  WorkerTrustPacket,
  WorkerTrustPhase,
} from "./worker-trust-types";

export type WorkerTrustKeyMap = Readonly<Record<string, crypto.KeyLike>>;

interface PhaseConfig {
  domain: string;
  field: string;
  payloadType: string;
  tag: number;
}

const PHASE_CONFIG: Record<WorkerTrustPhase, PhaseConfig> = {
  challenge_request: {
    domain: "CAP-WORKER-HANDSHAKE-CHALLENGE-REQUEST-V1",
    field: "workerHandshakeChallengeRequest",
    payloadType: "cordum.agent.v1.WorkerHandshakeChallengeRequest",
    tag: 19,
  },
  challenge: {
    domain: "CAP-WORKER-HANDSHAKE-CHALLENGE-V1",
    field: "workerHandshakeChallenge",
    payloadType: "cordum.agent.v1.WorkerHandshakeChallenge",
    tag: 20,
  },
  authenticate: {
    domain: "CAP-WORKER-HANDSHAKE-AUTHENTICATE-V1",
    field: "workerHandshakeAuthenticate",
    payloadType: "cordum.agent.v1.WorkerHandshakeAuthenticate",
    tag: 21,
  },
  result: {
    domain: "CAP-WORKER-HANDSHAKE-RESULT-V1",
    field: "workerHandshakeResult",
    payloadType: "cordum.agent.v1.WorkerHandshakeResult",
    tag: 22,
  },
};

const BUS_PAYLOAD_FIELDS = [
  "jobRequest",
  "jobResult",
  "heartbeat",
  "alert",
  "jobProgress",
  "jobCancel",
  "handshake",
  ...Object.values(PHASE_CONFIG).map(({ field }) => field),
];

export function workerTrustPhase(packet: WorkerTrustPacket): WorkerTrustPhase {
  const view = packet as unknown as WorkerTrustEnvelope & Record<string, unknown>;
  const present = BUS_PAYLOAD_FIELDS.filter((field) => view[field] != null);
  if (present.length !== 1) {
    throw new WorkerTrustSigningError("exactly one worker trust payload is required");
  }
  const phase = (Object.entries(PHASE_CONFIG) as Array<
    [WorkerTrustPhase, PhaseConfig]
  >).find(([, config]) => config.field === present[0])?.[0];
  if (!phase) {
    throw new WorkerTrustSigningError("a worker trust payload is required");
  }
  return phase;
}

export function workerTrustDomain(phase: WorkerTrustPhase): string {
  return PHASE_CONFIG[phase].domain;
}

export async function encodeWorkerTrustUnsigned(
  packet: WorkerTrustPacket
): Promise<Uint8Array> {
  validateWorkerTrustPacket(packet);
  const root = await loadRoot();
  const phase = validatedTrustPhase(packet);
  const config = PHASE_CONFIG[phase];
  const view = packet as unknown as WorkerTrustEnvelope & Record<string, unknown>;
  const writer = Writer.create();
  writeString(writer, 1, view.traceId);
  writeString(writer, 2, view.senderId);
  writeMessage(
    writer,
    3,
    root.lookupType("google.protobuf.Timestamp"),
    view.createdAt
  );
  writeInt32(writer, 4, view.protocolVersion);
  writeString(writer, 18, view.authToken);
  writeMessage(
    writer,
    config.tag,
    root.lookupType(config.payloadType),
    view[config.field]
  );
  return writer.finish();
}

export async function workerTrustTranscript(
  packet: WorkerTrustPacket
): Promise<Uint8Array> {
  const domain = Buffer.from(workerTrustDomain(workerTrustPhase(packet)), "ascii");
  const unsigned = await encodeWorkerTrustUnsigned(packet);
  return Buffer.concat([domain, Buffer.from([0]), unsigned]);
}

export async function workerTrustDigest(
  packet: WorkerTrustPacket
): Promise<Uint8Array> {
  return crypto.createHash("sha256").update(await workerTrustTranscript(packet)).digest();
}

export async function signWorkerTrustPacket<T extends WorkerTrustPacket>(
  packet: T,
  privateKeys: WorkerTrustKeyMap
): Promise<T> {
  const candidate = { ...packet, signature: Uint8Array.of(1) } as T;
  validateWorkerTrustPacket(candidate);
  const keyId = workerTrustKeyId(candidate);
  const key = requireP256Key(lookupKey(privateKeys, keyId), "private");
  const signature = crypto.sign("sha256", await workerTrustTranscript(candidate), {
    key,
    dsaEncoding: "der",
  });
  return { ...packet, signature: new Uint8Array(signature) } as T;
}

export async function verifyWorkerTrustPacket(
  packet: WorkerTrustPacket,
  publicKeys: WorkerTrustKeyMap
): Promise<boolean> {
  validateWorkerTrustPacket(packet);
  const signature = requireSignature(packet.signature);
  if (!isStrictDerSignature(signature)) {
    return false;
  }
  const keyId = workerTrustKeyId(packet);
  const key = requireP256Key(lookupKey(publicKeys, keyId), "public");
  try {
    return crypto.verify(
      "sha256",
      await workerTrustTranscript(packet),
      { key, dsaEncoding: "der" },
      signature
    );
  } catch {
    return false;
  }
}

function workerTrustKeyId(packet: WorkerTrustPacket): string {
  const phase = workerTrustPhase(packet);
  const challenge = trustProofRecord(packet, phase);
  const value =
    phase === "challenge" || phase === "result"
      ? challenge.serverKeyId
      : challenge.proofKeyId;
  if (typeof value !== "string" || value.length === 0) {
    throw new WorkerTrustSigningError("worker trust key id is required");
  }
  return value;
}

function validatedTrustPhase(packet: WorkerTrustPacket): WorkerTrustPhase {
  const phase = workerTrustPhase(packet);
  if (trustProofRecord(packet, phase).proofAlgorithm !== 1) {
    throw new WorkerTrustSigningError(
      "worker trust proof algorithm must be ECDSA_P256_SHA256"
    );
  }
  return phase;
}

function trustProofRecord(
  packet: WorkerTrustPacket,
  phase: WorkerTrustPhase
): Record<string, unknown> {
  const payload = requireRecord(
    (packet as unknown as Record<string, unknown>)[PHASE_CONFIG[phase].field],
    "worker trust payload"
  );
  return phase === "authenticate" || phase === "result"
    ? requireRecord(payload.challenge, "worker trust challenge")
    : payload;
}

function writeString(writer: Writer, tag: number, value: unknown): void {
  if (value === undefined || value === "") return;
  if (typeof value !== "string") {
    throw new WorkerTrustSigningError(`field ${tag} must be a string`);
  }
  writer.uint32((tag << 3) | 2).string(value);
}

function writeInt32(writer: Writer, tag: number, value: unknown): void {
  if (value === undefined || value === 0) return;
  if (typeof value !== "number" || !Number.isInteger(value)) {
    throw new WorkerTrustSigningError(`field ${tag} must be an int32`);
  }
  writer.uint32(tag << 3).int32(value);
}

function writeMessage(
  writer: Writer,
  tag: number,
  type: Type,
  value: unknown
): void {
  if (value === undefined || value === null) return;
  requireRecord(value, `field ${tag}`);
  writer.uint32((tag << 3) | 2).bytes(encodeDeterministic(type, value));
}

function requireRecord(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new WorkerTrustSigningError(`${label} is required`);
  }
  return value as Record<string, unknown>;
}

function lookupKey(keys: WorkerTrustKeyMap, keyId: string): crypto.KeyLike {
  if (!Object.prototype.hasOwnProperty.call(keys, keyId)) {
    throw new WorkerTrustSigningError("unknown worker trust key id");
  }
  return keys[keyId];
}
