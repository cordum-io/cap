import * as crypto from "node:crypto";

import { signWorkerTrustPacket, verifyWorkerTrustPacket } from "./trust-signing";
import { cloneCapability, cloneChallenge, sameBytes } from "./worker-trust-clone";
import {
  WORKER_HANDSHAKE_NONCE_SIZE,
  WORKER_HANDSHAKE_PROTOCOL_VERSION,
  WorkerHandshakeBindingError,
  WorkerHandshakeExpiredError,
  WorkerHandshakePacketError,
  WorkerHandshakeRejectionError,
  WorkerHandshakeRequestOptions,
  WorkerHandshakeSession,
  WorkerTrustConfig,
  dateNanos,
  dateFromTimestamp,
  timestampFromDate,
  timestampNanos,
  validateWorkerTrustConfig,
} from "./worker-trust-contract";
import {
  AuthenticateTrustPacket,
  ChallengeRequestTrustPacket,
  ChallengeTrustPacket,
  ResultTrustPacket,
  WorkerCapabilityHandshake,
  WorkerHandshakeChallenge,
} from "./worker-trust-types";
import {
  marshalWorkerTrustPacket,
  unmarshalWorkerTrustPacket,
  validateWorkerTrustPacket,
} from "./worker-trust-packet";
import { requireFreshTimestamp, requireLiveChallenge } from "./worker-trust-time";

const VERIFIED = new WeakMap<
  VerifiedWorkerHandshakeChallenge,
  readonly [ChallengeRequestTrustPacket, ChallengeTrustPacket]
>();

export class VerifiedWorkerHandshakeChallenge {
  readonly #response: ChallengeTrustPacket;

  constructor(
    request: ChallengeRequestTrustPacket,
    response: ChallengeTrustPacket,
    secret: symbol
  ) {
    if (secret !== verifiedSecret) throw new WorkerHandshakeBindingError("verified challenge is required");
    this.#response = response;
    VERIFIED.set(this, [request, response]);
    Object.freeze(this);
  }

  message(): WorkerHandshakeChallenge {
    return cloneChallenge(this.#response.workerHandshakeChallenge);
  }
}

const verifiedSecret = Symbol("verified-worker-handshake-challenge");

export async function buildChallengeRequest(
  config: WorkerTrustConfig,
  options: WorkerHandshakeRequestOptions
): Promise<ChallengeRequestTrustPacket> {
  validateWorkerTrustConfig(config);
  validateOptions(options);
  const packet: ChallengeRequestTrustPacket = {
    traceId: options.traceId,
    senderId: config.workerId,
    createdAt: timestampFromDate(options.createdAt),
    protocolVersion: WORKER_HANDSHAKE_PROTOCOL_VERSION,
    signature: new Uint8Array(),
    workerHandshakeChallengeRequest: {
      requestId: options.requestId,
      traceId: options.traceId,
      workerId: config.workerId,
      proofKeyId: config.proofKeyId,
      proofAlgorithm: 1,
      audience: config.audience,
      purpose: options.purpose,
      clientNonce: new Uint8Array(options.clientNonce),
      protocolVersion: WORKER_HANDSHAKE_PROTOCOL_VERSION,
      sdkVersion: config.sdkVersion,
    },
  };
  const signed = await signWorkerTrustPacket(packet, { [config.proofKeyId]: config.proofPrivateKey });
  validateWorkerTrustPacket(signed);
  return signed;
}

export async function verifyChallenge(
  config: WorkerTrustConfig,
  request: ChallengeRequestTrustPacket,
  response: ChallengeTrustPacket | Promise<ChallengeTrustPacket>,
  now: Date
): Promise<VerifiedWorkerHandshakeChallenge> {
  validateWorkerTrustConfig(config);
  validateWorkerTrustPacket(request);
  bindRequestConfig(config, request);
  await requireValidSignature(request, {
    [config.proofKeyId]: crypto.createPublicKey(config.proofPrivateKey),
  }, "request");
  requireFreshTimestamp(request.createdAt, now, "request created_at");
  const resolved = await response;
  await verifySchedulerPacket(config, resolved, now);
  correlateChallenge(config, request, resolved.workerHandshakeChallenge);
  requireLiveChallenge(resolved.workerHandshakeChallenge, now);
  const requestCopy = await clonePacket(request) as ChallengeRequestTrustPacket;
  const responseCopy = await clonePacket(resolved) as ChallengeTrustPacket;
  return new VerifiedWorkerHandshakeChallenge(requestCopy, responseCopy, verifiedSecret);
}

export async function buildAuthenticate(
  config: WorkerTrustConfig,
  verified: VerifiedWorkerHandshakeChallenge,
  capability: WorkerCapabilityHandshake,
  currentSessionToken: string,
  createdAt: Date
): Promise<AuthenticateTrustPacket> {
  validateWorkerTrustConfig(config);
  const [, response] = verifiedPackets(verified);
  const challenge = cloneChallenge(response.workerHandshakeChallenge);
  bindChallengeConfig(config, challenge);
  const packet: AuthenticateTrustPacket = {
    traceId: challenge.traceId,
    senderId: config.workerId,
    createdAt: timestampFromDate(createdAt),
    protocolVersion: 1,
    signature: new Uint8Array(),
    authToken: currentSessionToken,
    workerHandshakeAuthenticate: {
      challenge,
      capabilityHandshake: cloneCapability(capability),
    },
  };
  const signed = await signWorkerTrustPacket(packet, { [config.proofKeyId]: config.proofPrivateKey });
  validateWorkerTrustPacket(signed);
  return signed;
}

export async function verifyResult(
  config: WorkerTrustConfig,
  verified: VerifiedWorkerHandshakeChallenge,
  authenticate: AuthenticateTrustPacket,
  response: ResultTrustPacket | Promise<ResultTrustPacket>,
  now: Date
): Promise<WorkerHandshakeSession> {
  validateWorkerTrustConfig(config);
  const [, challengeResponse] = verifiedPackets(verified);
  validateWorkerTrustPacket(authenticate);
  await requireValidSignature(authenticate, {
    [config.proofKeyId]: crypto.createPublicKey(config.proofPrivateKey),
  }, "authenticate");
  requireSameChallenge(authenticate.workerHandshakeAuthenticate.challenge,
    challengeResponse.workerHandshakeChallenge);
  const resolved = await response;
  await verifySchedulerPacket(config, resolved, now);
  requireSameChallenge(resolved.workerHandshakeResult.challenge,
    challengeResponse.workerHandshakeChallenge);
  const result = resolved.workerHandshakeResult;
  requireLiveChallenge(result.challenge, now);
  requireFreshTimestamp(result.issuedAt, now, "result issued_at");
  if (!result.accepted) throw new WorkerHandshakeRejectionError(result.rejectionReason);
  const expiresAt = dateFromTimestamp(result.tokenExpiresAt);
  if (timestampNanos(result.tokenExpiresAt) <= dateNanos(now)) {
    throw new WorkerHandshakeExpiredError("result token is expired");
  }
  if (result.challenge.purpose === 2 && resolved.authToken === authenticate.authToken) {
    throw new WorkerHandshakeBindingError("renewal must rotate session token");
  }
  return Object.freeze({
    token: resolved.authToken ?? "",
    issuedAt: dateFromTimestamp(result.issuedAt),
    expiresAt,
  });
}

function verifiedPackets(
  value: VerifiedWorkerHandshakeChallenge
): readonly [ChallengeRequestTrustPacket, ChallengeTrustPacket] {
  const packets = VERIFIED.get(value);
  if (!packets) {
    throw new WorkerHandshakeBindingError("verified challenge is required");
  }
  return packets;
}

async function verifySchedulerPacket(
  config: WorkerTrustConfig,
  packet: ChallengeTrustPacket | ResultTrustPacket,
  now: Date
): Promise<void> {
  validateWorkerTrustPacket(packet);
  if (packet.senderId !== config.expectedSchedulerId) {
    throw new WorkerHandshakeBindingError("scheduler identity changed");
  }
  requireFreshTimestamp(packet.createdAt, now, "scheduler packet created_at");
  await requireValidSignature(packet, config.schedulerPublicKeys, "scheduler packet");
}

async function requireValidSignature(
  packet: ChallengeRequestTrustPacket | ChallengeTrustPacket | AuthenticateTrustPacket | ResultTrustPacket,
  keys: Readonly<Record<string, crypto.KeyLike>>,
  label: string
): Promise<void> {
  try {
    if (await verifyWorkerTrustPacket(packet, keys)) return;
  } catch {
    throw new WorkerHandshakePacketError(`${label} signature is invalid`);
  }
  throw new WorkerHandshakePacketError(`${label} signature is invalid`);
}

function correlateChallenge(
  config: WorkerTrustConfig,
  packet: ChallengeRequestTrustPacket,
  challenge: WorkerHandshakeChallenge
): void {
  const request = packet.workerHandshakeChallengeRequest;
  const matches = request.requestId === challenge.requestId && request.traceId === challenge.traceId &&
    request.workerId === challenge.workerId && request.proofKeyId === challenge.proofKeyId &&
    request.proofAlgorithm === challenge.proofAlgorithm && request.audience === challenge.audience &&
    request.purpose === challenge.purpose && sameBytes(request.clientNonce, challenge.clientNonce) &&
    request.protocolVersion === challenge.protocolVersion && request.sdkVersion === challenge.sdkVersion;
  if (!matches || challenge.agentId !== config.expectedAgentId ||
      challenge.tenantId !== config.tenantId) {
    throw new WorkerHandshakeBindingError("challenge correlation or identity changed");
  }
}

function bindRequestConfig(
  config: WorkerTrustConfig,
  packet: ChallengeRequestTrustPacket
): void {
  const request = packet.workerHandshakeChallengeRequest;
  if (request.workerId !== config.workerId || request.proofKeyId !== config.proofKeyId ||
      request.audience !== config.audience || request.sdkVersion !== config.sdkVersion) {
    throw new WorkerHandshakeBindingError("request changed configured worker identity");
  }
}

function bindChallengeConfig(config: WorkerTrustConfig, challenge: WorkerHandshakeChallenge): void {
  if (challenge.workerId !== config.workerId || challenge.agentId !== config.expectedAgentId ||
      challenge.tenantId !== config.tenantId || challenge.proofKeyId !== config.proofKeyId ||
      challenge.audience !== config.audience || challenge.sdkVersion !== config.sdkVersion) {
    throw new WorkerHandshakeBindingError("challenge identity changed");
  }
}

function requireSameChallenge(actual: WorkerHandshakeChallenge, expected: WorkerHandshakeChallenge): void {
  const scalarKeys: Array<keyof WorkerHandshakeChallenge> = [
    "requestId", "challengeId", "traceId", "workerId", "agentId", "tenantId", "proofKeyId",
    "proofAlgorithm", "serverKeyId", "audience", "purpose", "protocolVersion", "sdkVersion",
  ];
  const scalarMatch = scalarKeys.every((key) => actual[key] === expected[key]);
  const timeMatch = timestampNanos(actual.issuedAt) === timestampNanos(expected.issuedAt) &&
    timestampNanos(actual.expiresAt) === timestampNanos(expected.expiresAt);
  if (!scalarMatch || !timeMatch || !sameBytes(actual.clientNonce, expected.clientNonce) ||
      !sameBytes(actual.serverNonce, expected.serverNonce)) {
    throw new WorkerHandshakeBindingError("result challenge correlation changed");
  }
}

function validateOptions(options: WorkerHandshakeRequestOptions): void {
  if (!options || !options.requestId || options.requestId !== options.requestId.trim() ||
      !options.traceId || options.traceId !== options.traceId.trim()) {
    throw new WorkerHandshakePacketError("request correlation is invalid");
  }
  if (options.purpose !== 1 && options.purpose !== 2) {
    throw new WorkerHandshakePacketError("request purpose is invalid");
  }
  if (!(options.clientNonce instanceof Uint8Array) ||
      options.clientNonce.length !== WORKER_HANDSHAKE_NONCE_SIZE) {
    throw new WorkerHandshakePacketError("client nonce must be exactly 32 bytes");
  }
  timestampFromDate(options.createdAt);
}

async function clonePacket(
  packet: ChallengeRequestTrustPacket | ChallengeTrustPacket
): Promise<ChallengeRequestTrustPacket | ChallengeTrustPacket> {
  return await unmarshalWorkerTrustPacket(await marshalWorkerTrustPacket(packet)) as
    ChallengeRequestTrustPacket | ChallengeTrustPacket;
}
