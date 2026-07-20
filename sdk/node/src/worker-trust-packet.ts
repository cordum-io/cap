import { Type } from "protobufjs";

import { encodeDeterministic } from "./codec";
import { loadRoot } from "./protos";
import {
  WORKER_HANDSHAKE_MAX_LIFETIME_MS,
  WORKER_HANDSHAKE_MAX_PACKET_SIZE,
  WORKER_HANDSHAKE_MAX_SESSION_TOKEN_SIZE,
  WORKER_HANDSHAKE_NONCE_SIZE,
  WORKER_HANDSHAKE_PROTOCOL_VERSION,
  WorkerHandshakePacketError,
  dateFromTimestamp,
  timestampNanos,
} from "./worker-trust-contract";
import {
  AuthenticateTrustPacket,
  ChallengeRequestTrustPacket,
  ChallengeTrustPacket,
  ResultTrustPacket,
  WorkerCapabilityHandshake,
  WorkerHandshakeChallenge,
  WorkerTrustPacket,
  WorkerTrustPhase,
} from "./worker-trust-types";
import { rejectUnknownWire } from "./worker-trust-wire";

const PHASE_FIELDS = [
  "workerHandshakeChallengeRequest",
  "workerHandshakeChallenge",
  "workerHandshakeAuthenticate",
  "workerHandshakeResult",
] as const;
const ENVELOPE_FIELDS = [
  "traceId", "senderId", "createdAt", "protocolVersion", "signature", "authToken",
  ...PHASE_FIELDS,
];
const CHALLENGE_FIELDS = [
  "requestId", "challengeId", "traceId", "workerId", "agentId", "tenantId",
  "proofKeyId", "proofAlgorithm", "serverKeyId", "audience", "purpose",
  "clientNonce", "serverNonce", "protocolVersion", "sdkVersion", "issuedAt", "expiresAt",
];
const CAPABILITY_FIELDS = [
  "componentId", "role", "supportedVersions", "capabilities", "sdkVersion",
  "readyTopics", "agentName",
];
const CAPABILITY_KEY = /^[a-z][a-z0-9_.-]{0,63}$/;

export function validateWorkerTrustPacket(packet: WorkerTrustPacket): void {
  assertObject(packet, "packet");
  assertKnown(packet, ENVELOPE_FIELDS, "envelope");
  validateEnvelope(packet);
  const phase = trustPhase(packet);
  if (phase === "challenge_request") validateRequest(packet as ChallengeRequestTrustPacket);
  if (phase === "challenge") validateChallengePacket(packet as ChallengeTrustPacket);
  if (phase === "authenticate") validateAuthenticate(packet as AuthenticateTrustPacket);
  if (phase === "result") validateResult(packet as ResultTrustPacket);
}

export async function marshalWorkerTrustPacket(
  packet: WorkerTrustPacket
): Promise<Uint8Array> {
  validateWorkerTrustPacket(packet);
  const type = await busPacketType();
  const encoded = encodeDeterministic(type, packet);
  if (encoded.length > WORKER_HANDSHAKE_MAX_PACKET_SIZE) {
    fail("size exceeds 65536 bytes");
  }
  return encoded;
}

export async function unmarshalWorkerTrustPacket(
  data: Uint8Array
): Promise<WorkerTrustPacket> {
  if (!(data instanceof Uint8Array) || data.length === 0 ||
      data.length > WORKER_HANDSHAKE_MAX_PACKET_SIZE) {
    fail("size must be 1..65536 bytes");
  }
  const type = await busPacketType();
  rejectUnknownWire(data, type);
  let packet: WorkerTrustPacket;
  try {
    packet = type.decode(data) as unknown as WorkerTrustPacket;
  } catch {
    fail("cannot be decoded");
  }
  validateWorkerTrustPacket(packet);
  const canonical = encodeDeterministic(type, packet);
  if (!Buffer.from(canonical).equals(Buffer.from(data))) {
    fail("wire encoding is not canonical");
  }
  return packet;
}

export function trustPhase(packet: WorkerTrustPacket): WorkerTrustPhase {
  const present = PHASE_FIELDS.filter((field) =>
    Boolean((packet as unknown as Record<string, unknown>)[field])
  );
  if (present.length !== 1) fail("must contain exactly one trust phase");
  const index = PHASE_FIELDS.indexOf(present[0]);
  return (["challenge_request", "challenge", "authenticate", "result"] as const)[index];
}

function validateEnvelope(packet: WorkerTrustPacket): void {
  packetText(packet.traceId, "trace ID");
  packetText(packet.senderId, "sender ID");
  validateTimestampObject(packet.createdAt, "created_at");
  if (packet.protocolVersion !== WORKER_HANDSHAKE_PROTOCOL_VERSION) {
    fail("protocol version must be exactly v1");
  }
  if (!(packet.signature instanceof Uint8Array) || packet.signature.length === 0) {
    fail("signature is required");
  }
}

function validateRequest(packet: ChallengeRequestTrustPacket): void {
  const request = packet.workerHandshakeChallengeRequest;
  assertKnown(request, [
    "requestId", "traceId", "workerId", "proofKeyId", "proofAlgorithm", "audience",
    "purpose", "clientNonce", "protocolVersion", "sdkVersion",
  ], "challenge request");
  [request.requestId, request.traceId, request.workerId, request.proofKeyId,
    request.audience, request.sdkVersion].forEach((value) => packetText(value, "request identity"));
  if (packet.traceId !== request.traceId || packet.senderId !== request.workerId) {
    fail("challenge request trace or sender binding is invalid");
  }
  validateProof(request.protocolVersion, request.proofAlgorithm, request.purpose);
  nonce(request.clientNonce, "client nonce");
  if (packet.authToken) fail("challenge request must not carry a session token");
}

function validateChallengePacket(packet: ChallengeTrustPacket): void {
  validateChallenge(packet, packet.workerHandshakeChallenge);
  if (packet.authToken) fail("challenge must not carry a session token");
}

function validateChallenge(packet: WorkerTrustPacket, challenge: WorkerHandshakeChallenge): void {
  assertKnown(challenge, CHALLENGE_FIELDS, "challenge");
  [challenge.requestId, challenge.challengeId, challenge.traceId, challenge.workerId,
    challenge.agentId, challenge.tenantId, challenge.proofKeyId, challenge.serverKeyId,
    challenge.audience, challenge.sdkVersion].forEach((value) => packetText(value, "challenge identity"));
  if (packet.traceId !== challenge.traceId) fail("challenge trace binding is invalid");
  validateProof(challenge.protocolVersion, challenge.proofAlgorithm, challenge.purpose);
  nonce(challenge.clientNonce, "client nonce");
  nonce(challenge.serverNonce, "server nonce");
  const issued = validateTimestampObject(challenge.issuedAt, "challenge issued_at");
  const expires = validateTimestampObject(challenge.expiresAt, "challenge expires_at");
  if (expires <= issued ||
      expires - issued > BigInt(WORKER_HANDSHAKE_MAX_LIFETIME_MS) * 1_000_000n) {
    fail("challenge lifetime is invalid");
  }
}

function validateAuthenticate(packet: AuthenticateTrustPacket): void {
  const authenticate = packet.workerHandshakeAuthenticate;
  assertKnown(authenticate, ["challenge", "capabilityHandshake"], "authenticate");
  validateChallenge(packet, authenticate.challenge);
  if (packet.senderId !== authenticate.challenge.workerId) {
    fail("authenticate sender binding is invalid");
  }
  validateCapability(authenticate.capabilityHandshake, authenticate.challenge);
  validateSessionForPurpose(authenticate.challenge.purpose, packet.authToken ?? "");
}

function validateCapability(
  capability: WorkerCapabilityHandshake,
  challenge: WorkerHandshakeChallenge
): void {
  assertKnown(capability, CAPABILITY_FIELDS, "capability handshake");
  if (capability.componentId !== challenge.workerId || capability.role !== 3 ||
      capability.sdkVersion !== challenge.sdkVersion) {
    fail("capability identity does not match challenge");
  }
  if (capability.supportedVersions?.length !== 1 || capability.supportedVersions[0] !== 1) {
    fail("capability must support exactly protocol v1");
  }
  for (const [key, enabled] of Object.entries(capability.capabilities ?? {})) {
    if (!CAPABILITY_KEY.test(key) || enabled !== true) fail("capability entry is invalid");
  }
  const topics = capability.readyTopics ?? [];
  if (topics.some((topic) => !topic?.trim()) || new Set(topics).size !== topics.length) {
    fail("ready topic is empty or duplicated");
  }
}

function validateResult(packet: ResultTrustPacket): void {
  const result = packet.workerHandshakeResult;
  assertKnown(result, [
    "challenge", "accepted", "rejectionReason", "tokenExpiresAt", "issuedAt",
  ], "result");
  validateChallenge(packet, result.challenge);
  const issued = validateTimestampObject(result.issuedAt, "result issued_at");
  if (result.accepted) {
    const expires = validateTimestampObject(result.tokenExpiresAt, "token expires_at");
    if (!validToken(packet.authToken ?? "") || expires <= issued || result.rejectionReason !== 0) {
      fail("accepted result token metadata is invalid");
    }
  } else if (packet.authToken || result.tokenExpiresAt ||
             !Number.isInteger(result.rejectionReason) || result.rejectionReason < 1 ||
             result.rejectionReason > 9) {
    fail("rejected result token metadata is invalid");
  }
}

function validateProof(version: number, algorithm: number, purpose: number): void {
  if (version !== 1) fail("nested protocol version must be exactly v1");
  if (algorithm !== 1) fail("proof algorithm must be ECDSA P-256 SHA-256");
  if (purpose !== 1 && purpose !== 2) fail("handshake purpose is invalid");
}

function validateSessionForPurpose(purpose: number, token: string): void {
  if (purpose === 1 && token !== "") fail("initial authentication must not carry a token");
  if (purpose === 2 && !validToken(token)) fail("renewal requires a valid token");
}

function validToken(token: string): boolean {
  if (typeof token !== "string" || token.length === 0 ||
      token.length > WORKER_HANDSHAKE_MAX_SESSION_TOKEN_SIZE) return false;
  for (const character of token) {
    const code = character.charCodeAt(0);
    if (code < 0x21 || code > 0x7e) return false;
  }
  return true;
}

function nonce(value: Uint8Array, field: string): void {
  if (!(value instanceof Uint8Array) || value.length !== WORKER_HANDSHAKE_NONCE_SIZE) {
    fail(`${field} must be exactly 32 bytes`);
  }
}

function validateTimestampObject(value: unknown, field: string): bigint {
  assertKnown(value, ["seconds", "nanos"], field);
  return timestampNanos(value as Parameters<typeof dateFromTimestamp>[0]);
}

function packetText(value: unknown, field: string): string {
  if (typeof value !== "string" || value.length === 0 || value !== value.trim() || value.length > 256) {
    fail(`${field} is invalid`);
  }
  return value;
}

function assertObject(value: unknown, field: string): asserts value is Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) fail(`${field} is required`);
}

function assertKnown(value: unknown, fields: readonly string[], label: string): void {
  assertObject(value, label);
  const unknown = Object.keys(value).filter((key) => !fields.includes(key));
  if (unknown.length > 0) fail(`${label} contains unknown field ${unknown[0]}`);
}

async function busPacketType(): Promise<Type> {
  return (await loadRoot()).lookupType("cordum.agent.v1.BusPacket");
}

function fail(message: string): never {
  throw new WorkerHandshakePacketError(`worker trust packet ${message}`);
}
