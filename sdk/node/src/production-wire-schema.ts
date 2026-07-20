import type { Field, MapField, Type } from "protobufjs";

type MessageType = Pick<Type, "fieldsById">;

interface VarintResult {
  value: bigint;
  end: number;
}

interface WireValue {
  end: number;
  payload?: Buffer;
}

const SCALAR_WIRE_TYPES: Readonly<Record<string, number>> = Object.freeze({
  bool: 0, enum: 0, int32: 0, int64: 0, sint32: 0, sint64: 0,
  uint32: 0, uint64: 0, double: 1, fixed64: 1, sfixed64: 1,
  bytes: 2, string: 2, fixed32: 5, float: 5, sfixed32: 5,
});

export function assertClosedNestedWire(
  raw: Uint8Array,
  messageType: MessageType,
): void {
  walkMessage(Buffer.from(raw), messageType, false);
}

function walkMessage(
  raw: Buffer,
  messageType: MessageType,
  nested: boolean,
): void {
  const seen = new Set<number>();
  const seenOneofs = new Set<string>();
  let offset = 0;
  while (offset < raw.length) {
    const tag = readVarint(raw, offset);
    offset = tag.end;
    const fieldNumber = Number(tag.value >> 3n);
    const wireType = Number(tag.value & 7n);
    const field = messageType.fieldsById[fieldNumber];
    if (!field) throw new Error("unknown nested protobuf field");
    validateOccurrence(field, fieldNumber, wireType, seen, seenOneofs, nested);
    const value = readWireValue(raw, offset, wireType);
    validateNestedValue(field, value.payload);
    offset = value.end;
  }
}

function validateOccurrence(
  field: Field,
  fieldNumber: number,
  wireType: number,
  seen: Set<number>,
  seenOneofs: Set<string>,
  nested: boolean,
): void {
  if (!allowedWireTypes(field).includes(wireType)) {
    throw new Error("invalid nested protobuf wire");
  }
  if (nested && !field.repeated && !field.map && seen.has(fieldNumber)) {
    throw new Error("duplicate nested protobuf field");
  }
  seen.add(fieldNumber);
  const oneof = field.partOf?.name;
  if (nested && oneof && seenOneofs.has(oneof)) {
    throw new Error("multiple nested protobuf oneof fields");
  }
  if (oneof) seenOneofs.add(oneof);
}

function validateNestedValue(field: Field, payload?: Buffer): void {
  if (!payload) return;
  if (field.map) {
    walkMapEntry(payload, field);
    return;
  }
  const nestedType = resolvedMessageType(field);
  if (nestedType) walkMessage(payload, nestedType, true);
}

function walkMapEntry(raw: Buffer, field: Field): void {
  const seen = new Set<number>();
  let offset = 0;
  while (offset < raw.length) {
    const tag = readVarint(raw, offset);
    offset = tag.end;
    const number = Number(tag.value >> 3n);
    const wireType = Number(tag.value & 7n);
    if ((number !== 1 && number !== 2) || seen.has(number)) {
      throw new Error("unknown nested protobuf field");
    }
    const expected = number === 1
      ? scalarWireType((field as unknown as MapField).keyType)
      : mapValueWireType(field);
    if (wireType !== expected) throw new Error("invalid nested protobuf wire");
    seen.add(number);
    const value = readWireValue(raw, offset, wireType);
    if (number === 2 && value.payload) {
      const nestedType = resolvedMessageType(field);
      if (nestedType) walkMessage(value.payload, nestedType, true);
    }
    offset = value.end;
  }
}

function allowedWireTypes(field: Field): number[] {
  if (field.map || resolvedMessageType(field)) return [2];
  const base = field.resolvedType
    ? 0
    : scalarWireType(field.type);
  if (field.repeated && field.packed !== false && [0, 1, 5].includes(base)) {
    return [base, 2];
  }
  return [base];
}

function mapValueWireType(field: Field): number {
  if (resolvedMessageType(field)) return 2;
  if (field.resolvedType) return 0;
  return scalarWireType(field.type);
}

function scalarWireType(type: string): number {
  const wireType = SCALAR_WIRE_TYPES[type];
  if (wireType === undefined) throw new Error("unknown nested protobuf schema");
  return wireType;
}

function resolvedMessageType(field: Field): MessageType | undefined {
  const resolved = field.resolvedType;
  if (!resolved || !("fieldsById" in resolved)) return undefined;
  return resolved;
}

function readWireValue(raw: Buffer, offset: number, wireType: number): WireValue {
  if (wireType === 0) return { end: readVarint(raw, offset).end };
  if (wireType === 1) return boundedValue(raw, offset + 8);
  if (wireType === 5) return boundedValue(raw, offset + 4);
  if (wireType !== 2) throw new Error("invalid nested protobuf wire");
  const length = readVarint(raw, offset);
  if (length.value > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new Error("invalid nested protobuf wire");
  }
  const end = length.end + Number(length.value);
  if (end > raw.length) throw new Error("invalid nested protobuf wire");
  return { end, payload: raw.subarray(length.end, end) };
}

function boundedValue(raw: Buffer, end: number): WireValue {
  if (end > raw.length) throw new Error("invalid nested protobuf wire");
  return { end };
}

function readVarint(raw: Buffer, start: number): VarintResult {
  let value = 0n;
  for (let index = 0; index < 10; index += 1) {
    const offset = start + index;
    if (offset >= raw.length) throw new Error("invalid nested protobuf wire");
    const byte = raw[offset];
    value |= BigInt(byte & 0x7f) << BigInt(index * 7);
    if (byte < 0x80) {
      if (!encodeVarint(value).equals(raw.subarray(start, offset + 1))) {
        throw new Error("invalid nested protobuf wire");
      }
      return { value, end: offset + 1 };
    }
  }
  throw new Error("invalid nested protobuf wire");
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
