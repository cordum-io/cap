/** Fail-closed CAP-PRODUCTION ResourceRef validation. */

export const MAX_RESOURCE_IDENTIFIER_BYTES = 128;
export const MAX_RESOURCE_AUTHORITY_BYTES = 255;
export const MAX_RESOURCE_URI_BYTES = 2048;
export const MAX_RESOURCE_MEDIA_TYPE_BYTES = 127;
export const MAX_RESOURCE_PURPOSE_BYTES = 128;
export const MAX_RESOURCE_SIZE_BYTES = 1_073_741_824;
export const MAX_LEGACY_REDIS_KEY_BYTES = 1024;

export type ResourceInteger = number | string | bigint | { toString(): string };

export interface ResourceTimestampInput {
  seconds?: ResourceInteger;
  nanos?: ResourceInteger;
}

export interface ResourceRefInput {
  resolverId?: string;
  uri?: string;
  sha256?: Uint8Array;
  mediaType?: string;
  sizeBytes?: ResourceInteger;
  expiresAt?: ResourceTimestampInput;
  purpose?: string;
}

export class ResourceRefValidationError extends Error {
  constructor(message: string) {
    super(`invalid production resource reference: ${message}`);
    this.name = "ResourceRefValidationError";
  }
}

const ID_PATTERN = /^[a-z0-9][a-z0-9._-]*$/;
const PURPOSE_PATTERN = /^[a-z0-9][a-z0-9._:-]*$/;
const MEDIA_PATTERN = /^[a-z0-9][a-z0-9!#$&^_.+-]*\/[a-z0-9][a-z0-9!#$&^_.+-]*$/;
const URI_PATTERN = /^([a-z][a-z0-9+.-]{0,31}):\/\/(.+)$/;
const LEGACY_KEY_PATTERN = /^[A-Za-z0-9][A-Za-z0-9:._@\-\[\]]*$/;

export function validateResourceRef(
  ref: ResourceRefInput,
  installedResolvers: readonly string[],
  now: Date = new Date(),
): void {
  if (ref === null || typeof ref !== "object") {
    throw new ResourceRefValidationError("missing reference");
  }
  validateInstalledResolvers(installedResolvers);
  validateIdentifiers(ref, installedResolvers);
  if (!(ref.sha256 instanceof Uint8Array) || ref.sha256.byteLength !== 32) {
    throw new ResourceRefValidationError("SHA-256 must contain exactly 32 bytes");
  }
  const size = parseInteger(ref.sizeBytes, "declared size");
  if (size <= 0n || size > BigInt(MAX_RESOURCE_SIZE_BYTES)) {
    throw new ResourceRefValidationError("declared size is outside production bounds");
  }
  validateUri(ref.uri ?? "");
  validateExpiry(ref.expiresAt, now);
}

function validateInstalledResolvers(installed: readonly string[]): void {
  if (!Array.isArray(installed) || installed.length === 0
      || installed.some((value) => !validIdentifier(value, MAX_RESOURCE_IDENTIFIER_BYTES, ID_PATTERN))) {
    throw new ResourceRefValidationError("resolver configuration is empty or noncanonical");
  }
}

function validateIdentifiers(ref: ResourceRefInput, installed: readonly string[]): void {
  if (!validIdentifier(ref.resolverId, MAX_RESOURCE_IDENTIFIER_BYTES, ID_PATTERN)) {
    throw new ResourceRefValidationError("resolver ID is not canonical");
  }
  if (!installed.includes(ref.resolverId)) {
    throw new ResourceRefValidationError("resolver is not installed");
  }
  if (!validIdentifier(ref.mediaType, MAX_RESOURCE_MEDIA_TYPE_BYTES, MEDIA_PATTERN)) {
    throw new ResourceRefValidationError("media type is not canonical");
  }
  if (!validIdentifier(ref.purpose, MAX_RESOURCE_PURPOSE_BYTES, PURPOSE_PATTERN)) {
    throw new ResourceRefValidationError("purpose is not canonical");
  }
}

function validIdentifier(value: string | undefined, limit: number, pattern: RegExp): value is string {
  return value !== undefined
    && value.length > 0
    && Buffer.byteLength(value, "utf8") <= limit
    && value.trim() === value
    && pattern.test(value);
}

function validateUri(uri: string): void {
  if (!uri || Buffer.byteLength(uri, "utf8") > MAX_RESOURCE_URI_BYTES || uri.trim() !== uri) {
    throw new ResourceRefValidationError("resource URI is empty, untrimmed, or too long");
  }
  if (!isPrintableAscii(uri)) {
    throw new ResourceRefValidationError("resource URI must contain printable ASCII only");
  }
  const match = URI_PATTERN.exec(uri);
  if (match === null) {
    throw new ResourceRefValidationError("resource URI scheme is not canonical");
  }
  const authorityAndPath = match[2];
  if (/[@?#\\%]/.test(authorityAndPath)) {
    throw new ResourceRefValidationError("resource URI contains credentials, metadata, or escapes");
  }
  validatePath(authorityAndPath);
}

function validatePath(authorityAndPath: string): void {
  const slash = authorityAndPath.indexOf("/");
  const authority = slash < 0 ? authorityAndPath : authorityAndPath.slice(0, slash);
  if (!authority || Buffer.byteLength(authority, "ascii") > MAX_RESOURCE_AUTHORITY_BYTES) {
    throw new ResourceRefValidationError("resource URI authority is empty or too long");
  }
  if (slash < 0) {
    return;
  }
  const segments = authorityAndPath.slice(slash + 1).split("/");
  if (segments.some((segment) => !segment || segment === "." || segment.includes(".."))) {
    throw new ResourceRefValidationError("resource URI path is not normalized");
  }
}

function validateExpiry(expiresAt: ResourceTimestampInput | undefined, now: Date): void {
  if (!expiresAt || Number.isNaN(now.getTime())) {
    throw new ResourceRefValidationError("resource expiry is required and validation time must be valid");
  }
  const seconds = parseInteger(expiresAt.seconds, "expiry seconds");
  const nanos = parseInteger(expiresAt.nanos ?? 0, "expiry nanos");
  if (seconds < -62_135_596_800n || seconds > 253_402_300_799n || nanos < 0n || nanos >= 1_000_000_000n) {
    throw new ResourceRefValidationError("resource expiry timestamp is invalid");
  }
  const expiryNanos = seconds * 1_000_000_000n + nanos;
  const nowNanos = BigInt(now.getTime()) * 1_000_000n;
  if (expiryNanos <= nowNanos) {
    throw new ResourceRefValidationError("resource reference is expired");
  }
}

function parseInteger(value: ResourceInteger | undefined, label: string): bigint {
  if (value === undefined || value === null) {
    throw new ResourceRefValidationError(`${label} is missing`);
  }
  if (typeof value === "number") {
    if (!Number.isSafeInteger(value)) {
      throw new ResourceRefValidationError(`${label} must be a safe integer`);
    }
    return BigInt(value);
  }
  let text: string;
  try {
    const candidate: unknown = value.toString();
    if (typeof candidate !== "string") {
      throw new ResourceRefValidationError(`${label} cannot be read`);
    }
    text = candidate;
  } catch {
    throw new ResourceRefValidationError(`${label} cannot be read`);
  }
  if (!/^-?(0|[1-9][0-9]*)$/.test(text)) {
    throw new ResourceRefValidationError(`${label} is not canonical`);
  }
  return BigInt(text);
}

function isPrintableAscii(value: string): boolean {
  for (const byte of Buffer.from(value, "utf8")) {
    if (byte < 0x21 || byte > 0x7e) {
      return false;
    }
  }
  return true;
}

export function canonicalLegacyRedisKey(pointer: string): Uint8Array {
  if (pointer.trim() !== pointer
      || Buffer.byteLength(pointer, "utf8") > "redis://".length + MAX_LEGACY_REDIS_KEY_BYTES) {
    throw new ResourceRefValidationError("legacy Redis pointer is untrimmed or too long");
  }
  if (!pointer.startsWith("redis://")) {
    throw new ResourceRefValidationError("legacy Redis pointer has the wrong scheme");
  }
  const key = pointer.slice("redis://".length);
  const at = key.indexOf("@");
  const colon = key.indexOf(":");
  const hasUserinfo = at >= 0 && (colon < 0 || at < colon);
  if (!LEGACY_KEY_PATTERN.test(key) || key.includes("..") || hasUserinfo) {
    throw new ResourceRefValidationError("legacy Redis key is empty or ambiguous");
  }
  return Buffer.from(key, "ascii");
}

export function validateResourceRefCompatibility(
  legacyPointer: string,
  ref: ResourceRefInput | undefined,
): void {
  if (!legacyPointer || ref === undefined) {
    return;
  }
  if (ref.resolverId !== "redis") {
    throw new ResourceRefValidationError("legacy Redis pointer uses a different resolver");
  }
  const legacyKey = Buffer.from(canonicalLegacyRedisKey(legacyPointer));
  let structuredKey: Buffer;
  try {
    structuredKey = Buffer.from(canonicalLegacyRedisKey(ref.uri ?? ""));
  } catch (error: unknown) {
    throw new ResourceRefValidationError("legacy structured URI is ambiguous");
  }
  if (!legacyKey.equals(structuredKey)) {
    throw new ResourceRefValidationError("legacy and structured references differ");
  }
}
