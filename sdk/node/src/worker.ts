import { NatsConnection, Subscription } from "nats";
import { loadRoot, SUBJECT_RESULT, DEFAULT_PROTOCOL_VERSION } from "./protos";

type Handler = (jobRequest: any) => Promise<any>;

export interface WorkerConfig {
  nc: NatsConnection;
  subject: string;
  queue?: string;
  handler: Handler;
}

export async function startWorker(cfg: WorkerConfig): Promise<Subscription> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cortex.agent.v1.BusPacket");
  const JobResult = root.lookupType("cortex.agent.v1.JobResult");

  const sub = cfg.nc.subscribe(cfg.subject, { queue: cfg.queue ?? cfg.subject });
  (async () => {
    for await (const msg of sub) {
      try {
        const packet = BusPacket.decode(msg.data);
        const jr = packet["job_request"];
        if (!jr) continue;
        const resObj = await cfg.handler(jr);
        const jrMsg = JobResult.fromObject(resObj);
        const out = BusPacket.fromObject({
          trace_id: packet["trace_id"],
          sender_id: cfg.nc.info?.client_id?.toString() ?? "cap-worker",
          protocol_version: DEFAULT_PROTOCOL_VERSION,
          created_at: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
          job_result: jrMsg,
        });
        const data = BusPacket.encode(out).finish();
        await cfg.nc.publish(SUBJECT_RESULT, data);
      } catch (err) {
        // log and continue
        console.error(err);
      }
    }
  })();
  return sub;
}
