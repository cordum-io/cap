import * as crypto from "node:crypto";
import type { Type } from "protobufjs";
import { encodeDeterministic, encodeUnsignedForSignature } from "./codec";
import { MalformedPacketError } from "./errors";
import { validateBusPacket } from "./validate";

const MAX_SESSION_TOKEN_SIZE = 16 * 1024;

export interface MutableBusPacket {
  authToken?: string;
  signature?: Uint8Array;
}

function invalidToken(token: string): boolean {
  if (token.length === 0 || token.length > MAX_SESSION_TOKEN_SIZE) return true;
  for (const character of token) {
    const code = character.charCodeAt(0);
    if (code < 0x21 || code > 0x7e) return true;
  }
  return false;
}

function assertSessionToken(token: unknown): void {
  if (token === undefined || token === "") return;
  if (typeof token !== "string" || invalidToken(token)) {
    throw new MalformedPacketError(
      "auth_token: must be printable ASCII and at most 16384 bytes"
    );
  }
}

function validateAttachedToken(packet: unknown): void {
  if (typeof packet !== "object" || packet === null) return;
  const message = packet as Record<string, unknown>;
  assertSessionToken(message.authToken);
  assertSessionToken(message.auth_token);
}

export function attachSessionToken(
  packet: MutableBusPacket,
  sessionToken?: string
): void {
  if (sessionToken === undefined || sessionToken === "") return;
  assertSessionToken(sessionToken);
  packet.authToken = sessionToken;
}

export function validateOutboundPacket(packet: unknown): void {
  validateAttachedToken(packet);
  const errors = validateBusPacket(packet);
  if (errors.length === 0) return;
  const details = errors.slice(0, 8)
    .map(({ field, message }) => `${field}: ${message}`)
    .join("; ");
  throw new MalformedPacketError(details);
}

export function prepareOutboundPacket<T extends MutableBusPacket>(
  packet: T,
  sessionToken?: string
): T {
  attachSessionToken(packet, sessionToken);
  validateOutboundPacket(packet);
  return packet;
}

export function encodeOutboundPacket(
  packetType: Type,
  packet: MutableBusPacket,
  privateKey?: string
): Uint8Array {
  validateOutboundPacket(packet);
  if (privateKey) {
    const signer = crypto.createSign("sha256");
    signer.update(encodeUnsignedForSignature(packetType, packet));
    packet.signature = signer.sign(privateKey);
  }
  return encodeDeterministic(packetType, packet);
}
