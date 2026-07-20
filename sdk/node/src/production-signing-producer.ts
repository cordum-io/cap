import * as crypto from "node:crypto";
import type { Field, Type } from "protobufjs";

import {
  DEFAULT_PRODUCTION_MAX_LIFETIME_MS,
  MAX_PRODUCTION_RAW_BYTES,
  PRODUCTION_ALGORITHM,
  PRODUCTION_PROFILE_VERSION,
  PRODUCTION_SIGNATURE_DOMAIN,
  ProductionWireError,
  extractProductionSignature,
} from "./production-signing";
import { assertClosedNestedWire } from "./production-wire-schema";
import { validateBusPacket } from "./validate";

type InputObject = Record<string, unknown>;

/** Produce one exact CAP-PRODUCTION wire packet without mutating the caller. */
export function signProductionPacket(
  packet: unknown,
  packetType: Type,
  privateKey: crypto.KeyLike,
): Buffer {
  assertClosedInput(packet, packetType);
  const clone = packetType.create(packet as InputObject);
  delete (clone as unknown as InputObject).signature;
  validatePacket(clone as unknown as InputObject, packetType);
  const unsigned = Buffer.from(packetType.encode(clone).finish());
  if (unsigned.length >= MAX_PRODUCTION_RAW_BYTES) {
    throw new ProductionWireError("production packet exceeds size limit");
  }
  assertClosedNestedWire(unsigned, packetType);
  const signature = signExactWire(unsigned, privateKey);
  const raw = Buffer.concat([unsigned, field(14, signature)]);
  if (raw.length > MAX_PRODUCTION_RAW_BYTES) {
    throw new ProductionWireError("production packet exceeds size limit");
  }
  extractProductionSignature(raw);
  return raw;
}

function validatePacket(packet: InputObject, packetType: Type): void {
  const typeError = packetType.verify(packet);
  if (typeError) throw new ProductionWireError("invalid production packet");
  if (validateBusPacket(packet).length > 0) {
    throw new ProductionWireError("invalid production packet");
  }
  if (packet.protocolVersion !== 1) {
    throw new ProductionWireError("invalid protocol version wire");
  }
  const metadata = recordField(packet, "signatureMetadata");
  const messageId = metadata.messageId;
  if (
    metadata.profileVersion !== PRODUCTION_PROFILE_VERSION ||
    metadata.algorithm !== PRODUCTION_ALGORITHM ||
    !(messageId instanceof Uint8Array) || messageId.length !== 16 ||
    typeof metadata.audience !== "string" || !metadata.audience.trim() ||
    typeof metadata.keyId !== "string" || !metadata.keyId.trim()
  ) throw new ProductionWireError("invalid signature metadata");
  validateExpiry(recordField(metadata, "expiresAt"));
}

function validateExpiry(timestamp: InputObject): void {
  const sourceSeconds = timestamp.seconds;
  const seconds = typeof sourceSeconds === "object" && sourceSeconds !== null
    ? Number(String(sourceSeconds)) : Number(sourceSeconds ?? 0);
  const nanos = Number(timestamp.nanos ?? 0);
  if (
    !Number.isSafeInteger(seconds) ||
    seconds < -62_135_596_800 || seconds > 253_402_300_799 ||
    !Number.isSafeInteger(nanos) || nanos < 0 || nanos > 999_999_999
  ) throw new ProductionWireError("invalid signature expiry");
  const expiresAt = seconds * 1000 + Math.floor(nanos / 1e6);
  const now = Date.now();
  if (expiresAt <= now || expiresAt > now + DEFAULT_PRODUCTION_MAX_LIFETIME_MS) {
    throw new ProductionWireError("invalid signature expiry");
  }
}

function assertClosedInput(value: unknown, messageType: Type): void {
  if (!isRecord(value)) throw new ProductionWireError("invalid production packet");
  for (const key of Object.keys(value)) {
    const field = messageType.fields[key];
    if (!field) throw new ProductionWireError("unknown production packet field");
    assertClosedField(value[key], field);
  }
}

function assertClosedField(value: unknown, field: Field): void {
  const nestedType = resolvedMessageType(field);
  if (!nestedType || value === undefined || value === null) return;
  if (field.map) {
    if (!isRecord(value)) throw new ProductionWireError("invalid production packet");
    for (const entry of Object.values(value)) assertClosedInput(entry, nestedType);
    return;
  }
  if (field.repeated) {
    if (!Array.isArray(value)) throw new ProductionWireError("invalid production packet");
    for (const entry of value) assertClosedInput(entry, nestedType);
    return;
  }
  assertClosedInput(value, nestedType);
}

function resolvedMessageType(field: Field): Type | undefined {
  const resolved = field.resolvedType;
  if (!resolved || !("fieldsById" in resolved)) return undefined;
  return resolved;
}

function recordField(parent: InputObject, name: string): InputObject {
  const value = parent[name];
  if (!isRecord(value)) throw new ProductionWireError("invalid signature metadata");
  return value;
}

function isRecord(value: unknown): value is InputObject {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    && !(value instanceof Uint8Array);
}

function signExactWire(unsigned: Buffer, privateKey: crypto.KeyLike): Buffer {
  const key = productionPrivateKey(privateKey);
  const signer = crypto.createSign("sha256");
  signer.update(PRODUCTION_SIGNATURE_DOMAIN);
  signer.update(unsigned);
  signer.end();
  return signer.sign(key);
}

function productionPrivateKey(input: crypto.KeyLike): crypto.KeyObject {
  try {
    const key = input instanceof crypto.KeyObject ? input : crypto.createPrivateKey(input);
    const curve = key.asymmetricKeyDetails?.namedCurve;
    if (
      key.type !== "private" || key.asymmetricKeyType !== "ec" ||
      (curve !== "prime256v1" && curve !== "P-256")
    ) throw new Error("wrong key");
    return key;
  } catch {
    throw new ProductionWireError("production signatures require ECDSA P-256");
  }
}

function field(number: number, value: Buffer): Buffer {
  return Buffer.concat([
    encodeVarint(BigInt(number << 3 | 2)),
    encodeVarint(BigInt(value.length)),
    value,
  ]);
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
