import { NatsConnection } from "nats";
import { loadRoot, DEFAULT_PROTOCOL_VERSION, SUBJECT_PROGRESS, SUBJECT_CANCEL } from "./protos";
import {
  encodeOutboundPacket,
  type MutableBusPacket,
  prepareOutboundPacket,
} from "./packet-boundary";

interface PacketEnvelope extends MutableBusPacket {
  createdAt: unknown;
  protocolVersion: number;
  senderId: string;
  traceId: string;
}

export interface ProgressPacket extends PacketEnvelope {
  jobProgress: {
    jobId: string;
    message: string;
    percent: number;
    stepId: string;
  };
}

export interface CancelPacket extends PacketEnvelope {
  jobCancel: {
    jobId: string;
    reason: string;
    requestedBy: string;
  };
}

/**
 * Builds a progress packet wrapped in a BusPacket envelope.
 */
export async function progressPayload(
  senderId: string,
  jobId: string,
  stepId: string,
  percent: number,
  message: string,
  sessionToken = ""
): Promise<ProgressPacket> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  const packet = BusPacket.fromObject({
    traceId: jobId,
    senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobProgress: {
      jobId,
      stepId,
      percent,
      message,
    },
  }) as unknown as ProgressPacket;
  return prepareOutboundPacket(packet, sessionToken);
}

/**
 * Builds a cancel packet wrapped in a BusPacket envelope.
 */
export async function cancelPayload(
  senderId: string,
  jobId: string,
  reason: string,
  requestedBy: string,
  sessionToken = ""
): Promise<CancelPacket> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  const packet = BusPacket.fromObject({
    traceId: jobId,
    senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobCancel: {
      jobId,
      reason,
      requestedBy,
    },
  }) as unknown as CancelPacket;
  return prepareOutboundPacket(packet, sessionToken);
}

/**
 * Publishes a progress packet to the CAP progress subject.
 */
export async function emitProgress(
  nc: NatsConnection,
  packet: ProgressPacket,
  privateKey?: string
): Promise<void> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  const data = encodeOutboundPacket(BusPacket, packet, privateKey);
  await nc.publish(SUBJECT_PROGRESS, data);
}

/**
 * Publishes a cancel packet to the CAP cancel subject.
 */
export async function emitCancel(
  nc: NatsConnection,
  packet: CancelPacket,
  privateKey?: string
): Promise<void> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  const data = encodeOutboundPacket(BusPacket, packet, privateKey);
  await nc.publish(SUBJECT_CANCEL, data);
}
