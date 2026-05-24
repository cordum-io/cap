import { NatsConnection } from "nats";
import * as crypto from "crypto";
import { encodeDeterministic, encodeUnsignedForSignature } from "./codec";
import { DEFAULT_PROTOCOL_VERSION, loadRoot, SUBJECT_HANDSHAKE, sanitizeAgentName } from "./protos";

const COMPONENT_ROLE_WORKER = 3;

export type HandshakePacket = any;

/**
 * Builds a worker handshake packet.
 */
export async function handshakePayload(
  componentId: string,
  capabilities: Record<string, boolean> = {},
  senderId = componentId,
  readyTopics: string[] = [],
  agentName = ""
): Promise<HandshakePacket> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const normalizedAgentName = sanitizeAgentName(agentName);

  return BusPacket.fromObject({
    senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    handshake: {
      componentId,
      role: COMPONENT_ROLE_WORKER,
      supportedVersions: [DEFAULT_PROTOCOL_VERSION],
      capabilities,
      readyTopics,
      ...(normalizedAgentName ? { agentName: normalizedAgentName } : {}),
    },
  });
}

/**
 * Publishes a worker handshake packet to the CAP handshake subject.
 */
export async function publishHandshake(
  nc: NatsConnection,
  packet: HandshakePacket,
  privateKey?: string
): Promise<void> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  if (privateKey) {
    const unsignedData = encodeUnsignedForSignature(BusPacket, packet);
    const sign = crypto.createSign("sha256");
    sign.update(unsignedData);
    packet.signature = sign.sign(privateKey);
  }

  const data = encodeDeterministic(BusPacket, packet);
  await nc.publish(SUBJECT_HANDSHAKE, data);
}
