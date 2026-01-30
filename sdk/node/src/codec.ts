import Long from "long";
import { Enum, Type } from "protobufjs";

const TO_OBJECT_OPTIONS = {
  defaults: false,
  enums: Number,
  longs: Long,
  bytes: Buffer,
  arrays: true,
  objects: true,
  oneofs: false,
};

function isBinary(value: unknown): boolean {
  return Buffer.isBuffer(value) || value instanceof Uint8Array;
}

function isLong(value: unknown): boolean {
  return Long.isLong(value);
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value) && !isBinary(value) && !isLong(value);
}

export function canonicalize(value: any): any {
  if (value === null || value === undefined) {
    return value;
  }
  if (isBinary(value) || isLong(value)) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => canonicalize(item));
  }
  if (isPlainObject(value)) {
    const out: Record<string, unknown> = {};
    for (const key of Object.keys(value).sort()) {
      const v = (value as Record<string, unknown>)[key];
      if (v === undefined) {
        continue;
      }
      out[key] = canonicalize(v);
    }
    return out;
  }
  return value;
}

function toPlainObject(type: Type, message: any): any {
  if (message && typeof message === "object" && (message as any).$type) {
    return type.toObject(message, TO_OBJECT_OPTIONS);
  }
  return message;
}

function isDefaultScalar(field: any, value: any): boolean {
  if (value === null || value === undefined) {
    return true;
  }
  if (field.resolvedType && field.resolvedType instanceof Enum) {
    return value === 0;
  }
  if (Long.isLong(value)) {
    return value.isZero();
  }
  switch (field.type) {
    case "string":
      return value === "";
    case "bool":
      return value === false;
    case "bytes": {
      if (typeof value === "string") {
        return value.length === 0;
      }
      if (Buffer.isBuffer(value) || value instanceof Uint8Array) {
        return value.length === 0;
      }
      return value === 0;
    }
    case "double":
    case "float":
    case "int32":
    case "uint32":
    case "sint32":
    case "fixed32":
    case "sfixed32":
    case "int64":
    case "uint64":
    case "sint64":
    case "fixed64":
    case "sfixed64":
      return value === 0;
    default:
      return false;
  }
}

function stripDefaults(type: Type, obj: any): any {
  if (!obj || typeof obj !== "object") {
    return obj;
  }
  const out: Record<string, unknown> = {};
  for (const field of type.fieldsArray) {
    const name = field.name;
    if (!Object.prototype.hasOwnProperty.call(obj, name)) {
      continue;
    }
    const value = obj[name];
    if (value === undefined || value === null) {
      continue;
    }
    if (field.map) {
      if (typeof value !== "object") {
        continue;
      }
      const mapOut: Record<string, unknown> = {};
      const keys = Object.keys(value).sort();
      for (const key of keys) {
        mapOut[key] = value[key];
      }
      if (Object.keys(mapOut).length > 0) {
        out[name] = mapOut;
      }
      continue;
    }
    if (field.repeated) {
      if (!Array.isArray(value) || value.length === 0) {
        continue;
      }
      if (field.resolvedType && field.resolvedType instanceof Type) {
        out[name] = value.map((entry: any) => stripDefaults(field.resolvedType as Type, entry));
      } else {
        out[name] = value;
      }
      continue;
    }
    if (field.resolvedType && field.resolvedType instanceof Type) {
      const nested = stripDefaults(field.resolvedType as Type, value);
      if (nested && Object.keys(nested).length > 0) {
        out[name] = nested;
      }
      continue;
    }
    if (isDefaultScalar(field, value)) {
      continue;
    }
    out[name] = value;
  }
  return out;
}

export function encodeDeterministic(type: Type, message: any): Uint8Array {
  const obj = toPlainObject(type, message);
  const normalized = stripDefaults(type, obj);
  return type.encode(normalized).finish();
}

export function encodeUnsignedForSignature(type: Type, packet: any): Uint8Array {
  const obj = toPlainObject(type, packet);
  if (obj && typeof obj === "object") {
    delete obj.signature;
  }
  const normalized = stripDefaults(type, obj);
  return type.encode(normalized).finish();
}
