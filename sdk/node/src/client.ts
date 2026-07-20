import { NatsConnection } from "nats";
import { loadRoot, DEFAULT_PROTOCOL_VERSION, SUBJECT_SUBMIT } from "./protos";
import {
  encodeOutboundPacket,
  type MutableBusPacket,
  prepareOutboundPacket,
} from "./packet-boundary";

/**
 * Publishes a JobRequest onto the CAP submit subject.
 *
 * @param nc - An active NATS connection.
 * @param jobRequest - Plain object conforming to the JobRequest protobuf schema.
 * @param traceId - Distributed trace identifier propagated through the bus.
 * @param senderId - Identity of the sender (used in the BusPacket envelope).
 * @param privateKey - Optional PEM-encoded ECDSA private key for signing.
 * @param sessionToken - Optional authenticated worker session token.
 */
export async function submitJob(
  nc: NatsConnection,
  jobRequest: Record<string, unknown>,
  traceId: string,
  senderId: string,
  privateKey?: string,
  sessionToken = ""
): Promise<void> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const JobRequest = root.lookupType("cordum.agent.v1.JobRequest");

  const jrMsg = JobRequest.fromObject(jobRequest);
  const payload = BusPacket.fromObject({
    traceId: traceId,
    senderId: senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: jrMsg,
  }) as unknown as MutableBusPacket;
  prepareOutboundPacket(payload, sessionToken);
  const data = encodeOutboundPacket(BusPacket, payload, privateKey);
  await nc.publish(SUBJECT_SUBMIT, data);
}
