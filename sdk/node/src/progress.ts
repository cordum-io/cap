import { NatsConnection } from "nats";
import * as crypto from "crypto";
import { encodeDeterministic, encodeUnsignedForSignature } from "./codec";
import { loadRoot, DEFAULT_PROTOCOL_VERSION, SUBJECT_PROGRESS, SUBJECT_CANCEL } from "./protos";

/**
 * Builds a progress packet wrapped in a BusPacket envelope.
 */
export async function progressPayload(
  senderId: string,
  jobId: string,
  stepId: string,
  percent: number,
  message: string
): Promise<any> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  return BusPacket.fromObject({
    senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobProgress: {
      jobId,
      stepId,
      percent,
      message,
    },
  });
}

/**
 * Builds a cancel packet wrapped in a BusPacket envelope.
 */
export async function cancelPayload(
  senderId: string,
  jobId: string,
  reason: string,
  requestedBy: string
): Promise<any> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  return BusPacket.fromObject({
    senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobCancel: {
      jobId,
      reason,
      requestedBy,
    },
  });
}

/**
 * Publishes a progress packet to the CAP progress subject.
 */
export async function emitProgress(
  nc: NatsConnection,
  packet: any,
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
  await nc.publish(SUBJECT_PROGRESS, data);
}

/**
 * Publishes a cancel packet to the CAP cancel subject.
 */
export async function emitCancel(
  nc: NatsConnection,
  packet: any,
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
  await nc.publish(SUBJECT_CANCEL, data);
}
