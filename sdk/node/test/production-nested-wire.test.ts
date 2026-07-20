import { expect } from "chai";

import {
  extractProductionSignature,
  verifyProductionPacket,
} from "../src/production-signing";
import {
  busPacketType,
  encodeVarint,
  keyPair,
  sign,
  signedRawPacket,
} from "./production-admission-support";

function readVarint(raw: Buffer, start: number): [number, number] {
  let value = 0;
  let shift = 0;
  for (let offset = start; offset < raw.length && shift < 35; offset += 1) {
    const byte = raw[offset];
    value += (byte & 0x7f) * (2 ** shift);
    if (byte < 0x80) return [value, offset + 1];
    shift += 7;
  }
  throw new Error("invalid test varint");
}

function injectNestedField(unsigned: Buffer, target: number, extra: Buffer): Buffer {
  let offset = 0;
  while (offset < unsigned.length) {
    const start = offset;
    const [tag, valueStart] = readVarint(unsigned, offset);
    const number = Math.floor(tag / 8);
    const wireType = tag & 7;
    if (wireType === 0) {
      [, offset] = readVarint(unsigned, valueStart);
      continue;
    }
    if (wireType !== 2) throw new Error("unsupported test wire type");
    const [length, dataStart] = readVarint(unsigned, valueStart);
    const end = dataStart + length;
    if (number === target) {
      const nested = Buffer.concat([unsigned.subarray(dataStart, end), extra]);
      const replacement = Buffer.concat([
        encodeVarint(tag), encodeVarint(nested.length), nested,
      ]);
      return Buffer.concat([
        unsigned.subarray(0, start), replacement, unsigned.subarray(end),
      ]);
    }
    offset = end;
  }
  throw new Error("target nested field not found");
}

async function signedNestedField(target: number, extra: Buffer): Promise<Buffer> {
  const valid = await signedRawPacket();
  const { unsigned } = extractProductionSignature(valid);
  const malicious = injectNestedField(Buffer.from(unsigned), target, extra);
  const signature = sign(malicious, keyPair.privateKey);
  return Buffer.concat([
    malicious,
    encodeVarint((14 << 3) | 2),
    encodeVarint(signature.length),
    signature,
  ]);
}

describe("CAP-PRODUCTION nested exact-wire grammar", () => {
  const unknown = Buffer.concat([
    encodeVarint((99 << 3) | 2), encodeVarint(7), Buffer.from("unknown"),
  ]);
  for (const [name, fieldNumber] of [["SignatureMetadata", 5], ["JobRequest", 10]] as const) {
    it(`rejects a signed unknown field in ${name}`, async () => {
      const raw = await signedNestedField(fieldNumber, unknown);
      const type = await busPacketType();
      expect(() => verifyProductionPacket(raw, type, {
        audience: "worker-pool-a",
        tenant: "tenant-a",
        sender: "scheduler-1",
        publicKeys: { k1: keyPair.publicKey },
      })).to.throw("unknown nested protobuf field");
    });
  }

  it("rejects a duplicate singular nested field", async () => {
    const duplicateProfile = Buffer.concat([
      encodeVarint((1 << 3) | 2), encodeVarint(6), Buffer.from("future"),
    ]);
    const raw = await signedNestedField(5, duplicateProfile);
    const type = await busPacketType();
    expect(() => verifyProductionPacket(raw, type, {
      audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
    })).to.throw("duplicate nested protobuf field");
  });
});
