import { NatsConnection, Subscription } from "nats";
import { loadRoot, SUBJECT_RESULT, DEFAULT_PROTOCOL_VERSION } from "./protos";
import * as crypto from "crypto";

type Handler = (jobRequest: any) => Promise<any>;

export interface WorkerConfig {
  nc: NatsConnection;
  subject: string;
  queue?: string;
  handler: Handler;
  publicKeyMap?: { [senderId: string]: string }; // senderId -> public key in PEM format
  privateKey?: string; // private key in PEM format for signing outgoing messages
  senderId: string;
}

export async function startWorker(cfg: WorkerConfig): Promise<Subscription> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cortex.agent.v1.BusPacket");
  const JobResult = root.lookupType("cortex.agent.v1.JobResult");

  const sub = cfg.nc.subscribe(cfg.subject, { queue: cfg.queue ?? cfg.subject });
  (async () => {
    for await (const msg of sub) {
      try {
        const packet = BusPacket.decode(msg.data) as any;

        // Signature verification
        if (cfg.publicKeyMap && packet.sender_id && packet.signature && packet.signature.length > 0) {
          const publicKey = cfg.publicKeyMap[packet.sender_id];
          if (!publicKey) {
            console.warn(`Worker: No public key found for sender ${packet.sender_id}. Dropping message.`);
            continue;
          }

          const receivedSignature = packet.signature;
          packet.signature = Buffer.from([]); // Clear signature for verification
          const unsignedData = BusPacket.encode(packet).finish();

          const verify = crypto.createVerify("sha256");
          verify.update(unsignedData);
          if (!verify.verify(publicKey, receivedSignature)) {
            console.warn(`Worker: Invalid signature from sender ${packet.sender_id}. Dropping message.`);
            continue;
          }
        } else if (cfg.publicKeyMap && (!packet.signature || packet.signature.length === 0)) {
            console.warn(`Worker: Message from sender ${packet.sender_id} has no signature but public keys are configured. Dropping message.`);
            continue;
        }

        const jr = packet.job_request;
        if (!jr) continue;
        const resObj = await cfg.handler(jr);
        const jrMsg = JobResult.fromObject(resObj);
        const out = BusPacket.fromObject({
          trace_id: packet.trace_id,
          sender_id: cfg.senderId,
          protocol_version: DEFAULT_PROTOCOL_VERSION,
          created_at: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
          job_result: jrMsg,
        }) as any;

        // Signing outgoing packet
        if (cfg.privateKey) {
            const unsignedOut: any = BusPacket.fromObject(out.toJSON());
            unsignedOut.signature = Buffer.from([]);
            const unsignedOutData = BusPacket.encode(unsignedOut).finish();

            const sign = crypto.createSign("sha256");
            sign.update(unsignedOutData);
            out.signature = sign.sign(cfg.privateKey);
        }

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
