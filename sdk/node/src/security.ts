import * as crypto from "node:crypto";
import type { Type } from "protobufjs";
import { encodeUnsignedForSignature } from "./codec";
import {
  MalformedPacketError,
  SignatureInvalidError,
  SignatureMissingError,
} from "./errors";
import { validateBusPacket } from "./validate";

interface InboundPacket {
  senderId?: unknown;
  signature?: unknown;
}

/** Validates an inbound envelope and enforces signature policy when configured. */
export function verifyInboundPacket(
  packetType: Type,
  packet: unknown,
  publicKeyMap: Readonly<Record<string, string>> | undefined
): void {
  const validationErrors = validateBusPacket(packet);
  if (validationErrors.length > 0) {
    const details = validationErrors
      .slice(0, 8)
      .map(({ field, message }) => `${field}: ${message}`)
      .join("; ");
    throw new MalformedPacketError(details);
  }
  if (publicKeyMap === undefined) {
    return;
  }

  const view = packet as InboundPacket;
  const senderId = requireSender(view.senderId);
  const signature = requireSignature(view.signature);
  const publicKey = lookupPublicKey(publicKeyMap, senderId);
  if (!verifySignature(packetType, packet, publicKey, signature)) {
    throw new SignatureInvalidError("inbound signature verification failed");
  }
}

function requireSender(value: unknown): string {
  if (typeof value !== "string" || value.trim().length === 0) {
    throw new SignatureInvalidError("authenticated sender identity is required");
  }
  return value;
}

function requireSignature(value: unknown): Uint8Array {
  if (!(value instanceof Uint8Array) || value.length === 0) {
    throw new SignatureMissingError("inbound signature is required");
  }
  return value;
}

function lookupPublicKey(
  publicKeyMap: Readonly<Record<string, string>>,
  senderId: string
): string {
  if (!Object.prototype.hasOwnProperty.call(publicKeyMap, senderId)) {
    throw new SignatureInvalidError("no trusted public key for sender");
  }
  const publicKey = publicKeyMap[senderId];
  if (typeof publicKey !== "string" || publicKey.trim().length === 0) {
    throw new SignatureInvalidError("trusted public key is invalid");
  }
  return publicKey;
}

function verifySignature(
  packetType: Type,
  packet: unknown,
  publicKey: string,
  signature: Uint8Array
): boolean {
  try {
    const verifier = crypto.createVerify("sha256");
    verifier.update(encodeUnsignedForSignature(packetType, packet));
    return verifier.verify(publicKey, signature);
  } catch {
    throw new SignatureInvalidError("trusted public key is invalid");
  }
}
