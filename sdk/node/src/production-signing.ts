import crypto from "crypto";
import { assertClosedNestedWire } from "./production-wire-schema";
import {
  DecodedProductionPacket,
  isProductionRecord,
  ProductionPacketType,
  ProductionSignatureMetadataView,
  ProductionTrustStore,
  validateProductionTrustAuthority,
} from "./production-signing-types";
export type {
  DecodedProductionPacket,
  ProductionPacketType,
  ProductionTrustStore,
} from "./production-signing-types";
export const PRODUCTION_SIGNATURE_DOMAIN = Buffer.from("CAP-PRODUCTION-SIGNATURE-V1\0", "utf8");
export const MAX_PRODUCTION_RAW_BYTES = 1 << 20;
const BUS_PACKET_WIRE_TYPES = new Map<number, number>([
  [1, 2], [2, 2], [3, 2], [4, 0], [5, 2], [6, 2],
  [10, 2], [11, 2], [12, 2], [13, 2], [14, 2], [15, 2],
  [16, 2], [17, 2], [18, 2], [19, 2], [20, 2], [21, 2], [22, 2],
]);
const BUS_PACKET_PAYLOAD_FIELDS = new Set([10, 11, 12, 13, 15, 16, 17, 19, 20, 21, 22]);

export class ProductionWireError extends Error {}

export interface ExtractedProductionWire { unsigned: Uint8Array; signature: Uint8Array }

export function extractProductionSignature(raw: Uint8Array): ExtractedProductionWire {
  if (raw.byteLength > MAX_PRODUCTION_RAW_BYTES) {
    throw new ProductionWireError("production packet exceeds size limit");
  }
  const source = Buffer.from(raw);
  const unsigned: Buffer[] = [];
  let signature: Buffer | undefined;
  const seen = new Set<number>();
  let payloadField: number | undefined;
  let offset = 0;
  while (offset < source.length) {
    const start = offset;
    const tag = readVarint(source, offset);
    offset = tag.end;
    const fieldNumber = Number(tag.value >> 3n);
    const wireType = Number(tag.value & 7n);
    payloadField = validateBusPacketField(
      fieldNumber, wireType, seen, payloadField,
    );
    let valueEnd: number;
    if (fieldNumber === 4) {
      const protocolVersion = readVarint(source, offset);
      valueEnd = protocolVersion.end;
      if (
        protocolVersion.value !== 1n ||
        !source.subarray(offset, valueEnd).equals(Buffer.from([0x01]))
      ) {
        throw new ProductionWireError("invalid protocol version wire");
      }
    } else {
      valueEnd = skipValue(source, offset, wireType);
    }
    if (fieldNumber !== 14) {
      unsigned.push(source.subarray(start, valueEnd));
    } else {
      const length = readVarint(source, offset);
      const end = length.end + Number(length.value);
      if (end !== valueEnd || end === length.end) {
        throw new ProductionWireError("malformed signature");
      }
      signature = source.subarray(length.end, end);
    }
    offset = valueEnd;
  }
  if (!seen.has(4)) throw new ProductionWireError("invalid protocol version wire");
  if (!signature) throw new ProductionWireError("missing signature");
  return { unsigned: Buffer.concat(unsigned), signature };
}

function validateBusPacketField(
  fieldNumber: number,
  wireType: number,
  seen: Set<number>,
  payloadField: number | undefined,
): number | undefined {
  const expected = BUS_PACKET_WIRE_TYPES.get(fieldNumber);
  if (expected === undefined) {
    throw new ProductionWireError("unknown BusPacket field");
  }
  if (wireType !== expected) {
    throw new ProductionWireError("wrong BusPacket field wire type");
  }
  if (seen.has(fieldNumber)) {
    throw new ProductionWireError("duplicate singular BusPacket field");
  }
  seen.add(fieldNumber);
  if (!BUS_PACKET_PAYLOAD_FIELDS.has(fieldNumber)) return payloadField;
  if (payloadField !== undefined) {
    throw new ProductionWireError("multiple BusPacket payload fields");
  }
  return fieldNumber;
}

export function productionPreimageDigest(unsigned: Uint8Array): Uint8Array {
  return crypto.createHash("sha256").update(PRODUCTION_SIGNATURE_DOMAIN)
    .update(unsigned).digest();
}

export function verifyProductionSignature(raw: Uint8Array, publicKeyPem: string): Uint8Array {
  const extracted = extractProductionSignature(raw);
  const publicKey = productionPublicKey(publicKeyPem);
  const verifier = crypto.createVerify("sha256");
  verifier.update(PRODUCTION_SIGNATURE_DOMAIN);
  verifier.update(extracted.unsigned);
  verifier.end();
  if (!verifier.verify(publicKey, extracted.signature)) {
    throw new ProductionWireError("invalid signature");
  }
  return extracted.unsigned;
}

export const PRODUCTION_PROFILE_VERSION = "cap-production-v1";
export const PRODUCTION_ALGORITHM = "ECDSA-P256-SHA256";
export const DEFAULT_PRODUCTION_MAX_LIFETIME_MS = 5 * 60_000;
export const MAX_PRODUCTION_CLOCK_SKEW_MS = 60_000;
function validateTrustAuthority(trust: ProductionTrustStore): void {
  try {
    validateProductionTrustAuthority(trust);
  } catch {
    throw new ProductionWireError("production trust authority required");
  }
}
function timestampMillis(value: unknown): number {
  if (!isProductionRecord(value)) throw new ProductionWireError("invalid signature expiry");
  const secondsValue = value.seconds;
  const seconds = typeof secondsValue === "object" && secondsValue !== null
    ? Number(String(secondsValue)) : Number(secondsValue ?? 0);
  const nanos = Number(value.nanos ?? 0);
  if (
    !Number.isFinite(seconds) || !Number.isInteger(seconds) ||
    seconds < -62_135_596_800 || seconds > 253_402_300_799 ||
    !Number.isFinite(nanos) || !Number.isInteger(nanos) ||
    nanos < 0 || nanos > 999_999_999
  ) {
    throw new ProductionWireError("invalid signature expiry");
  }
  return seconds * 1000 + Math.floor(nanos / 1e6);
}

function validateMetadataShape(
  metadata: unknown,
): asserts metadata is ProductionSignatureMetadataView {
  if (!isProductionRecord(metadata)) {
    throw new ProductionWireError("missing signature metadata");
  }
  const messageId = metadata.messageId;
  if (
    metadata.profileVersion !== PRODUCTION_PROFILE_VERSION ||
    metadata.algorithm !== PRODUCTION_ALGORITHM ||
    !(messageId instanceof Uint8Array) || messageId.length !== 16 ||
    typeof metadata.audience !== "string" || !metadata.audience ||
    typeof metadata.keyId !== "string" || !metadata.keyId ||
    !isProductionRecord(metadata.expiresAt)
  ) {
    throw new ProductionWireError("invalid signature metadata");
  }
}

function decodeClosedPacket(
  extracted: ExtractedProductionWire,
  packetType: ProductionPacketType,
): Record<string, unknown> {
  try {
    assertClosedNestedWire(extracted.unsigned, packetType);
  } catch (error) {
    const message = error instanceof Error ? error.message : "invalid nested protobuf wire";
    throw new ProductionWireError(message);
  }
  try {
    const decoded = packetType.decode(extracted.unsigned);
    if (!isProductionRecord(decoded)) throw new Error("not an object");
    return decoded;
  } catch {
    throw new ProductionWireError("malformed production protobuf");
  }
}

function validateLifetime(
  metadata: ProductionSignatureMetadataView,
  trust: ProductionTrustStore,
): void {
  const now = trust.nowMs?.() ?? Date.now();
  const clockSkew = trust.clockSkewMs ?? 0;
  const maxLifetime = trust.maxLifetimeMs ?? DEFAULT_PRODUCTION_MAX_LIFETIME_MS;
  if (
    !Number.isSafeInteger(now) ||
    !Number.isSafeInteger(maxLifetime) || maxLifetime <= 0 ||
    maxLifetime > DEFAULT_PRODUCTION_MAX_LIFETIME_MS ||
    !Number.isSafeInteger(clockSkew) || clockSkew < 0 ||
    clockSkew > MAX_PRODUCTION_CLOCK_SKEW_MS || clockSkew > maxLifetime
  ) throw new ProductionWireError("invalid production lifetime bounds");
  const expiresAt = timestampMillis(metadata.expiresAt);
  if (expiresAt <= now - clockSkew) throw new ProductionWireError("signature expired");
  if (expiresAt > now + maxLifetime + clockSkew)
    throw new ProductionWireError("signature lifetime exceeds bound");
}

function verifyExactSignature(extracted: ExtractedProductionWire, publicKeyPem: string): void {
  const verifier = crypto.createVerify("sha256");
  verifier.update(PRODUCTION_SIGNATURE_DOMAIN);
  verifier.update(Buffer.from(extracted.unsigned));
  verifier.end();
  if (!verifier.verify(productionPublicKey(publicKeyPem), Buffer.from(extracted.signature)))
    throw new ProductionWireError("invalid signature");
}

export function verifyProductionPacket(
  raw: Uint8Array,
  busPacketType: ProductionPacketType,
  trust: ProductionTrustStore,
): DecodedProductionPacket {
  validateTrustAuthority(trust);
  const extracted = extractProductionSignature(raw);
  const packet = decodeClosedPacket(extracted, busPacketType);
  const metadata = packet.signatureMetadata;
  validateMetadataShape(metadata);
  if (metadata.audience !== trust.audience) throw new ProductionWireError("audience mismatch");
  validateLifetime(metadata, trust);
  if (!isProductionRecord(packet.identity) || packet.identity.tenantId !== trust.tenant)
    throw new ProductionWireError("tenant mismatch");
  if (packet.senderId !== trust.sender) throw new ProductionWireError("sender mismatch");
  if (!Object.prototype.hasOwnProperty.call(trust.publicKeys, metadata.keyId)) {
    throw new ProductionWireError("unknown key id");
  }
  const publicKeyPem = trust.publicKeys[metadata.keyId];
  if (typeof publicKeyPem !== "string" || !publicKeyPem) {
    throw new ProductionWireError("unknown key id");
  }
  verifyExactSignature(extracted, publicKeyPem);
  return packet as unknown as DecodedProductionPacket;
}

function productionPublicKey(publicKeyPem: string): crypto.KeyObject {
  try {
    const key = crypto.createPublicKey(publicKeyPem);
    const curve = key.asymmetricKeyDetails?.namedCurve;
    if (key.asymmetricKeyType !== "ec" || (curve !== "prime256v1" && curve !== "P-256")) {
      throw new Error("wrong curve");
    }
    return key;
  } catch {
    throw new ProductionWireError("production signatures require ECDSA P-256");
  }
}

interface VarintResult {
  value: bigint;
  end: number;
}

function readVarint(raw: Buffer, start: number): VarintResult {
  let value = 0n;
  for (let index = 0; index < 10; index += 1) {
    const offset = start + index;
    if (offset >= raw.length) throw new ProductionWireError("truncated varint");
    const byte = raw[offset];
    value |= BigInt(byte & 0x7f) << BigInt(index * 7);
    if (byte < 0x80) {
      const encoded = encodeVarint(value);
      if (!encoded.equals(raw.subarray(start, offset + 1))) {
        throw new ProductionWireError("non-minimal varint");
      }
      return { value, end: offset + 1 };
    }
  }
  throw new ProductionWireError("oversize varint");
}

function encodeVarint(input: bigint): Buffer {
  const bytes: number[] = [];
  let value = input;
  while (value >= 0x80n) {
    bytes.push(Number(value & 0x7fn) | 0x80);
    value >>= 7n;
  }
  bytes.push(Number(value));
  return Buffer.from(bytes);
}

function skipValue(raw: Buffer, offset: number, wireType: number): number {
  let end: number;
  if (wireType === 0) return readVarint(raw, offset).end;
  if (wireType === 1) end = offset + 8;
  else if (wireType === 2) {
    const length = readVarint(raw, offset);
    end = length.end + Number(length.value);
  } else if (wireType === 5) end = offset + 4;
  else throw new ProductionWireError("unsupported wire type");
  if (end > raw.length) throw new ProductionWireError("truncated field");
  return end;
}
