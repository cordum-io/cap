import * as crypto from "node:crypto";

export const WORKER_HANDSHAKE_AUDIENCE = "cordum-scheduler";
export const WORKER_HANDSHAKE_PROTOCOL_VERSION = 1;
export const WORKER_HANDSHAKE_NONCE_SIZE = 32;
export const WORKER_HANDSHAKE_MAX_PACKET_SIZE = 64 * 1024;
export const WORKER_HANDSHAKE_MAX_IDENTITY_LENGTH = 256;
export const WORKER_HANDSHAKE_MAX_SESSION_TOKEN_SIZE = 16 * 1024;
export const WORKER_HANDSHAKE_MAX_SKEW_MS = 60_000;
export const WORKER_HANDSHAKE_MAX_LIFETIME_MS = 60_000;
const NANOS_PER_MILLISECOND = 1_000_000n;
const NANOS_PER_SECOND = 1_000_000_000n;
const MIN_PROTO_SECONDS = -62_135_596_800n;
const MAX_PROTO_SECONDS = 253_402_300_799n;

export const WorkerTrustModes = Object.freeze({
  OFF: "off",
  WARN: "warn",
  ENFORCE: "enforce",
} as const);

export type WorkerTrustMode =
  (typeof WorkerTrustModes)[keyof typeof WorkerTrustModes];
export type WorkerHandshakePurpose = 1 | 2;

export interface ProtoTimestamp {
  seconds: number | string | { toString(): string };
  nanos?: number;
}

export interface WorkerTrustConfigInput {
  workerId: string;
  expectedAgentId: string;
  tenantId: string;
  audience: string;
  proofKeyId: string;
  proofPrivateKey: crypto.KeyLike;
  expectedSchedulerId: string;
  schedulerPublicKeys: Readonly<Record<string, crypto.KeyLike>>;
  sdkVersion: string;
}

export interface WorkerTrustConfig {
  readonly workerId: string;
  readonly expectedAgentId: string;
  readonly tenantId: string;
  readonly audience: string;
  readonly proofKeyId: string;
  readonly proofPrivateKey: crypto.KeyObject;
  readonly expectedSchedulerId: string;
  readonly schedulerPublicKeys: Readonly<Record<string, crypto.KeyObject>>;
  readonly sdkVersion: string;
}

export interface WorkerHandshakeRequestOptions {
  readonly requestId: string;
  readonly traceId: string;
  readonly purpose: WorkerHandshakePurpose;
  readonly clientNonce: Uint8Array;
  readonly createdAt: Date;
}

export interface WorkerHandshakeSession {
  readonly token: string;
  readonly issuedAt: Date;
  readonly expiresAt: Date;
}

export class WorkerTrustError extends Error {}
export class WorkerTrustModeError extends WorkerTrustError {}
export class WorkerTrustConfigError extends WorkerTrustError {}
export class WorkerHandshakePacketError extends WorkerTrustError {}
export class WorkerHandshakeBindingError extends WorkerTrustError {}
export class WorkerHandshakeExpiredError extends WorkerTrustError {}

export class WorkerHandshakeRejectionError extends WorkerTrustError {
  readonly reason: number;

  constructor(reason: number) {
    super("worker handshake rejected");
    this.reason = reason;
  }
}

export function parseWorkerTrustMode(raw: unknown): WorkerTrustMode {
  const normalized = typeof raw === "string" ? raw.trim().toLowerCase() : "";
  if (Object.values(WorkerTrustModes).includes(normalized as WorkerTrustMode)) {
    return normalized as WorkerTrustMode;
  }
  throw new WorkerTrustModeError(`invalid worker trust mode: ${String(raw)}`);
}

export function createWorkerTrustConfig(
  input: WorkerTrustConfigInput
): WorkerTrustConfig {
  if (!input || typeof input !== "object") {
    throw new WorkerTrustConfigError("worker trust configuration is required");
  }
  const schedulerPublicKeys: Record<string, crypto.KeyObject> = {};
  for (const [keyId, value] of Object.entries(input.schedulerPublicKeys ?? {})) {
    requireIdentity(keyId, "scheduler key ID");
    schedulerPublicKeys[keyId] = requireP256Key(value, "public");
  }
  const config: WorkerTrustConfig = {
    ...input,
    proofPrivateKey: requireP256Key(input.proofPrivateKey, "private"),
    schedulerPublicKeys: Object.freeze(schedulerPublicKeys),
  };
  validateWorkerTrustConfig(config);
  return Object.freeze(config);
}

export function validateWorkerTrustConfig(config: WorkerTrustConfig): void {
  if (!config || typeof config !== "object") {
    throw new WorkerTrustConfigError("worker trust configuration is required");
  }
  const identities = [
    config.workerId,
    config.expectedAgentId,
    config.tenantId,
    config.proofKeyId,
    config.expectedSchedulerId,
    config.sdkVersion,
  ];
  identities.forEach((value) => requireIdentity(value, "worker trust identity"));
  if (config.audience !== WORKER_HANDSHAKE_AUDIENCE) {
    throw new WorkerTrustConfigError("worker trust audience is invalid");
  }
  requireP256Key(config.proofPrivateKey, "private");
  const entries = Object.entries(config.schedulerPublicKeys ?? {});
  if (entries.length === 0) {
    throw new WorkerTrustConfigError("scheduler trust keys are required");
  }
  for (const [keyId, key] of entries) {
    requireIdentity(keyId, "scheduler key ID");
    requireP256Key(key, "public");
  }
}

export function requireP256Key(
  value: crypto.KeyLike,
  expectedType: "private" | "public"
): crypto.KeyObject {
  let key: crypto.KeyObject;
  try {
    key = value instanceof crypto.KeyObject
      ? value
      : expectedType === "private"
        ? crypto.createPrivateKey(value)
        : crypto.createPublicKey(value);
  } catch {
    throw new WorkerTrustConfigError("worker trust key is invalid");
  }
  const curve = key.asymmetricKeyDetails?.namedCurve;
  if (key.type !== expectedType || key.asymmetricKeyType !== "ec" ||
      (curve !== "prime256v1" && curve !== "P-256")) {
    throw new WorkerTrustConfigError(`worker trust key must be ${expectedType} P-256`);
  }
  return key;
}

export function requireIdentity(value: unknown, field: string): string {
  if (typeof value !== "string" || value.length === 0 ||
      value !== value.trim() || value.length > WORKER_HANDSHAKE_MAX_IDENTITY_LENGTH) {
    throw new WorkerTrustConfigError(`${field} is invalid`);
  }
  return value;
}

export function timestampFromDate(value: Date): ProtoTimestamp {
  if (!(value instanceof Date) || !Number.isFinite(value.getTime())) {
    throw new WorkerHandshakePacketError("timestamp is invalid");
  }
  const milliseconds = value.getTime();
  const seconds = Math.floor(milliseconds / 1000);
  return { seconds, nanos: (milliseconds - seconds * 1000) * 1_000_000 };
}

export function dateFromTimestamp(value: ProtoTimestamp | undefined): Date {
  const milliseconds = timestampNanos(value) / NANOS_PER_MILLISECOND;
  const result = new Date(Number(milliseconds));
  if (!Number.isFinite(result.getTime())) {
    throw new WorkerHandshakePacketError("timestamp is invalid");
  }
  return result;
}

export function timestampNanos(value: ProtoTimestamp | undefined): bigint {
  if (!value || typeof value !== "object") {
    throw new WorkerHandshakePacketError("timestamp is invalid");
  }
  const secondsText = value.seconds?.toString();
  const nanos = value.nanos ?? 0;
  if (!/^-?(0|[1-9]\d*)$/.test(secondsText) || !Number.isInteger(nanos) ||
      nanos < 0 || nanos >= 1_000_000_000) {
    throw new WorkerHandshakePacketError("timestamp is invalid");
  }
  const seconds = BigInt(secondsText);
  if (seconds < MIN_PROTO_SECONDS || seconds > MAX_PROTO_SECONDS) {
    throw new WorkerHandshakePacketError("timestamp is outside protobuf range");
  }
  return seconds * NANOS_PER_SECOND + BigInt(nanos);
}

export function dateNanos(value: Date): bigint {
  if (!(value instanceof Date) || !Number.isFinite(value.getTime())) {
    throw new WorkerHandshakePacketError("timestamp is invalid");
  }
  return BigInt(value.getTime()) * NANOS_PER_MILLISECOND;
}
