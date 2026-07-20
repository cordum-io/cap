import { NatsConnection } from "nats";
import { randomUUID } from "node:crypto";
import { DEFAULT_PROTOCOL_VERSION, loadRoot, SUBJECT_HANDSHAKE, sanitizeAgentName } from "./protos";
import {
  encodeOutboundPacket,
  type MutableBusPacket,
  prepareOutboundPacket,
} from "./packet-boundary";

const COMPONENT_ROLE_WORKER = 3;

export interface HandshakePacket extends MutableBusPacket {
  createdAt: unknown;
  handshake: {
    agentName?: string;
    capabilities: Record<string, boolean>;
    componentId: string;
    readyTopics: string[];
    role: number;
    sdkVersion: string;
    supportedVersions: number[];
  };
  protocolVersion: number;
  senderId: string;
  traceId: string;
}

/**
 * Builds a worker handshake packet.
 */
export async function handshakePayload(
  componentId: string,
  capabilities: Record<string, boolean> = {},
  senderId = componentId,
  readyTopics: string[] = [],
  agentName = "",
  sdkVersion = "cap-node/v2",
  sessionToken = ""
): Promise<HandshakePacket> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const normalizedAgentName = sanitizeAgentName(agentName);

  const packet = BusPacket.fromObject({
    traceId: randomUUID().replace(/-/g, ""),
    senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    handshake: {
      componentId,
      role: COMPONENT_ROLE_WORKER,
      supportedVersions: [DEFAULT_PROTOCOL_VERSION],
      capabilities,
      readyTopics,
      sdkVersion,
      ...(normalizedAgentName ? { agentName: normalizedAgentName } : {}),
    },
  }) as unknown as HandshakePacket;
  return prepareOutboundPacket(packet, sessionToken);
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

  const data = encodeOutboundPacket(BusPacket, packet, privateKey);
  await nc.publish(SUBJECT_HANDSHAKE, data);
}
