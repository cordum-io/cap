import crypto from "crypto";

export const PRODUCTION_SIGNATURE_DOMAIN = Buffer.from(
  "CAP-PRODUCTION-SIGNATURE-V1\0",
  "utf8",
);

export class ProductionWireError extends Error {}

export interface ExtractedProductionWire {
  unsigned: Uint8Array;
  signature: Uint8Array;
}

export function extractProductionSignature(raw: Uint8Array): ExtractedProductionWire {
  const source = Buffer.from(raw);
  const unsigned: Buffer[] = [];
  let signature: Buffer | undefined;
  let offset = 0;
  while (offset < source.length) {
    const start = offset;
    const tag = readVarint(source, offset);
    offset = tag.end;
    const fieldNumber = Number(tag.value >> 3n);
    const wireType = Number(tag.value & 7n);
    const valueEnd = skipValue(source, offset, wireType);
    if (fieldNumber !== 14) {
      unsigned.push(source.subarray(start, valueEnd));
    } else {
      if (signature || wireType !== 2) {
        throw new ProductionWireError("duplicate or malformed signature field");
      }
      const length = readVarint(source, offset);
      const end = length.end + Number(length.value);
      if (end !== valueEnd || end === length.end) {
        throw new ProductionWireError("malformed signature");
      }
      signature = source.subarray(length.end, end);
    }
    offset = valueEnd;
  }
  if (!signature) throw new ProductionWireError("missing signature");
  return { unsigned: Buffer.concat(unsigned), signature };
}

export function productionPreimageDigest(unsigned: Uint8Array): Uint8Array {
  return crypto
    .createHash("sha256")
    .update(PRODUCTION_SIGNATURE_DOMAIN)
    .update(unsigned)
    .digest();
}

export function verifyProductionSignature(raw: Uint8Array, publicKeyPem: string): Uint8Array {
  const extracted = extractProductionSignature(raw);
  const verifier = crypto.createVerify("sha256");
  verifier.update(PRODUCTION_SIGNATURE_DOMAIN);
  verifier.update(extracted.unsigned);
  verifier.end();
  if (!verifier.verify(publicKeyPem, extracted.signature)) {
    throw new ProductionWireError("invalid signature");
  }
  return extracted.unsigned;
}

export const PRODUCTION_PROFILE_VERSION = "cap-production-v1";
export const PRODUCTION_ALGORITHM = "ECDSA-P256-SHA256";

/** Trust configuration for {@link verifyProductionPacket}. Keys are keyed by
 * signature_metadata.key_id (never trusted from the packet itself). */
export interface ProductionTrustStore {
  audience: string;
  publicKeys: Record<string, string>;
  tenant?: string;
  sender?: string;
}

function timestampMillis(value: any): number {
  if (!value) return 0;
  const seconds = typeof value.seconds === "object" ? Number(value.seconds.toString()) : Number(value.seconds ?? 0);
  const nanos = Number(value.nanos ?? 0);
  return seconds * 1000 + Math.floor(nanos / 1e6);
}

function validateMetadataShape(metadata: any): void {
  if (!metadata) {
    throw new ProductionWireError("missing signature metadata");
  }
  const messageId: Uint8Array = metadata.messageId ?? new Uint8Array(0);
  if (
    metadata.profileVersion !== PRODUCTION_PROFILE_VERSION ||
    metadata.algorithm !== PRODUCTION_ALGORITHM ||
    messageId.length !== 16 ||
    !metadata.audience ||
    !metadata.keyId ||
    !metadata.expiresAt
  ) {
    throw new ProductionWireError("invalid signature metadata");
  }
}

/**
 * Verify exact received wire bytes (never a re-serialized object) and
 * return the decoded, verified BusPacket. busPacketType is the protobufjs
 * Type used to decode the (already-extracted) unsigned bytes, matching
 * this runtime's existing reflection-based decode convention.
 */
export function verifyProductionPacket(raw: Uint8Array, busPacketType: { decode: (buf: Uint8Array) => any }, trust: ProductionTrustStore): any {
  const extracted = extractProductionSignature(raw);
  const packet = busPacketType.decode(extracted.unsigned);
  const metadata = packet.signatureMetadata;
  validateMetadataShape(metadata);
  if (trust.audience && metadata.audience !== trust.audience) {
    throw new ProductionWireError("audience mismatch");
  }
  if (timestampMillis(metadata.expiresAt) <= Date.now()) {
    throw new ProductionWireError("signature expired");
  }
  if (trust.tenant && packet.identity?.tenantId !== trust.tenant) {
    throw new ProductionWireError("tenant mismatch");
  }
  if (trust.sender && packet.senderId !== trust.sender) {
    throw new ProductionWireError("sender mismatch");
  }
  const publicKeyPem = trust.publicKeys[metadata.keyId];
  if (!publicKeyPem) {
    throw new ProductionWireError("unknown key id");
  }
  const verifier = crypto.createVerify("sha256");
  verifier.update(PRODUCTION_SIGNATURE_DOMAIN);
  verifier.update(Buffer.from(extracted.unsigned));
  verifier.end();
  if (!verifier.verify(publicKeyPem, Buffer.from(extracted.signature))) {
    throw new ProductionWireError("invalid signature");
  }
  return packet;
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
