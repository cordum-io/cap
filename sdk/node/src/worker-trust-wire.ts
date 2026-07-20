import { Field, Reader, Type } from "protobufjs";

import { WorkerHandshakePacketError } from "./worker-trust-contract";

const VARINT_TYPES = new Set([
  "int32", "uint32", "sint32", "int64", "uint64", "sint64",
  "bool", "enum",
]);
const FIXED64_TYPES = new Set(["fixed64", "sfixed64", "double"]);
const FIXED32_TYPES = new Set(["fixed32", "sfixed32", "float"]);

export function rejectUnknownWire(data: Uint8Array, type: Type): void {
  try {
    const reader = Reader.create(data);
    scanMessage(reader, reader.len, type);
    if (reader.pos !== reader.len) {
      fail("trailing bytes");
    }
  } catch (error) {
    if (error instanceof WorkerHandshakePacketError) {
      throw error;
    }
    fail("malformed protobuf");
  }
}

function scanMessage(reader: Reader, end: number, type: Type): void {
  const seen = new Set<number>();
  const seenOneofs = new Set<string>();
  while (reader.pos < end) {
    const tag = reader.uint32();
    const field = type.fieldsById[tag >>> 3];
    if (!field) {
      fail(`unknown field ${tag >>> 3} in ${type.fullName}`);
    }
    rejectDuplicate(field, seen, seenOneofs);
    scanField(reader, end, tag & 7, field);
  }
  if (reader.pos !== end) {
    fail(`invalid length in ${type.fullName}`);
  }
}

function rejectDuplicate(
  field: Field,
  seen: Set<number>,
  seenOneofs: Set<string>
): void {
  if (!field.repeated && !field.map && seen.has(field.id)) {
    fail(`duplicate singular field ${field.fullName}`);
  }
  seen.add(field.id);
  const oneof = field.partOf?.name;
  if (oneof && seenOneofs.has(oneof)) {
    fail(`duplicate oneof ${oneof}`);
  }
  if (oneof) {
    seenOneofs.add(oneof);
  }
}

function scanField(reader: Reader, parentEnd: number, wire: number, field: Field): void {
  const expected = expectedWire(field);
  const packed = field.repeated && field.packed && wire === 2 && expected !== 2;
  if (wire !== expected && !packed) {
    fail(`invalid wire type for ${field.fullName}`);
  }
  if (wire !== 2) {
    reader.skipType(wire);
    return;
  }
  const length = reader.uint32();
  const end = reader.pos + length;
  if (end > parentEnd) {
    fail(`invalid length for ${field.fullName}`);
  }
  if (field.map) {
    scanMapEntry(reader, end, field);
  } else if (field.resolvedType instanceof Type) {
    scanMessage(reader, end, field.resolvedType);
  } else {
    reader.pos = end;
  }
}

function scanMapEntry(reader: Reader, end: number, field: Field): void {
  const seen = new Set<number>();
  while (reader.pos < end) {
    const tag = reader.uint32();
    const id = tag >>> 3;
    if ((id !== 1 && id !== 2) || seen.has(id)) {
      fail(`unknown or duplicate map field in ${field.fullName}`);
    }
    seen.add(id);
    reader.skipType(tag & 7);
  }
  if (reader.pos !== end) {
    fail(`invalid map entry in ${field.fullName}`);
  }
}

function expectedWire(field: Field): number {
  if (field.map || field.resolvedType instanceof Type ||
      field.type === "string" || field.type === "bytes") {
    return 2;
  }
  if (FIXED64_TYPES.has(field.type)) return 1;
  if (FIXED32_TYPES.has(field.type)) return 5;
  if (VARINT_TYPES.has(field.type) || field.resolvedType) return 0;
  fail(`unsupported protobuf field ${field.fullName}`);
}

function fail(message: string): never {
  throw new WorkerHandshakePacketError(`worker trust packet ${message}`);
}
