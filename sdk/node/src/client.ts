import { NatsConnection } from "nats";
import { loadRoot, DEFAULT_PROTOCOL_VERSION, SUBJECT_SUBMIT } from "./protos";

export async function submitJob(
  nc: NatsConnection,
  jobRequest: Record<string, unknown>,
  traceId: string,
  senderId: string
) {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cortex.agent.v1.BusPacket");
  const JobRequest = root.lookupType("cortex.agent.v1.JobRequest");

  const jrMsg = JobRequest.fromObject(jobRequest);
  const payload = BusPacket.fromObject({
    trace_id: traceId,
    sender_id: senderId,
    protocol_version: DEFAULT_PROTOCOL_VERSION,
    created_at: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    job_request: jrMsg,
  });
  const data = BusPacket.encode(payload).finish();
  await nc.publish(SUBJECT_SUBMIT, data);
}
