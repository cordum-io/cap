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
