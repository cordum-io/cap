import * as crypto from "node:crypto";

export class WorkerTrustSigningError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "WorkerTrustSigningError";
  }
}

export function requireP256Key(
  keyLike: crypto.KeyLike,
  usage: "private" | "public"
): crypto.KeyObject {
  let key: crypto.KeyObject;
  try {
    if (keyLike instanceof crypto.KeyObject) {
      if (usage === "private" && keyLike.type !== "private") {
        throw new Error("private key required");
      }
      key = usage === "public" && keyLike.type !== "public"
        ? crypto.createPublicKey(keyLike)
        : keyLike;
    } else {
      key = usage === "private"
        ? crypto.createPrivateKey(keyLike)
        : crypto.createPublicKey(keyLike);
    }
  } catch {
    throw new WorkerTrustSigningError(`worker trust ${usage} key is invalid`);
  }
  if (
    key.asymmetricKeyType !== "ec" ||
    key.asymmetricKeyDetails?.namedCurve !== "prime256v1"
  ) {
    throw new WorkerTrustSigningError(`worker trust ${usage} key must use P-256`);
  }
  return key;
}

export function requireSignature(value: unknown): Uint8Array {
  if (!(value instanceof Uint8Array) || value.length === 0) {
    throw new WorkerTrustSigningError("worker trust signature is required");
  }
  return value;
}

export function isStrictDerSignature(signature: Uint8Array): boolean {
  if (
    signature.length < 8 ||
    signature.length > 72 ||
    signature[0] !== 0x30 ||
    signature[1] !== signature.length - 2
  ) {
    return false;
  }
  const rEnd = derIntegerEnd(signature, 2);
  if (rEnd < 0) return false;
  return derIntegerEnd(signature, rEnd) === signature.length;
}

function derIntegerEnd(signature: Uint8Array, offset: number): number {
  if (offset + 2 > signature.length || signature[offset] !== 0x02) return -1;
  const length = signature[offset + 1];
  const start = offset + 2;
  const end = start + length;
  if (length < 1 || length > 33 || end > signature.length) return -1;
  const first = signature[start];
  if ((first & 0x80) !== 0) return -1;
  if (length > 1 && first === 0 && (signature[start + 1] & 0x80) === 0) {
    return -1;
  }
  return end;
}
