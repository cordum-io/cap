import { NatsConnection } from "nats";
import { loadRoot, DEFAULT_PROTOCOL_VERSION, SUBJECT_SUBMIT } from "./protos";
import { encodeDeterministic, encodeUnsignedForSignature } from "./codec";
import * as crypto from "crypto";

export async function submitJob(
  nc: NatsConnection,
  jobRequest: Record<string, unknown>,
  traceId: string,
  senderId: string,
  privateKey?: string
) {
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
  });

  if (privateKey) {
    const unsignedData = encodeUnsignedForSignature(BusPacket, payload);
    const sign = crypto.createSign("sha256");
    sign.update(unsignedData);
    const signature = sign.sign(privateKey);

    (payload as any).signature = signature;
  }

  const data = encodeDeterministic(BusPacket, payload);
  await nc.publish(SUBJECT_SUBMIT, data);
}
