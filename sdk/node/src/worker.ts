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
  const BusPacket = root.lookupType("coretex.agent.v1.BusPacket");
  const JobResult = root.lookupType("coretex.agent.v1.JobResult");

  const onMessage = async (msg: any) => {
    try {
      const packet = BusPacket.decode(msg.data) as any;

      // Signature verification
      if (cfg.publicKeyMap && packet.senderId && packet.signature && packet.signature.length > 0) {
        const publicKey = cfg.publicKeyMap[packet.senderId];
        if (!publicKey) {
          console.warn(`Worker: No public key found for sender ${packet.senderId}. Dropping message.`);
          return;
        }

        const receivedSignature = packet.signature;
        packet.signature = Buffer.from([]); // Clear signature for verification
        const unsignedData = BusPacket.encode(packet).finish();

        const verify = crypto.createVerify("sha256");
        verify.update(unsignedData);
        if (!verify.verify(publicKey, receivedSignature)) {
          console.warn(`Worker: Invalid signature from sender ${packet.senderId}. Dropping message.`);
          return;
        }
      } else if (cfg.publicKeyMap && (!packet.signature || packet.signature.length === 0)) {
          console.warn(`Worker: Message from sender ${packet.senderId} has no signature but public keys are configured. Dropping message.`);
          return;
      }

      const jr = packet.jobRequest;
      if (!jr) return;
      const resObj = await cfg.handler(jr);
      const jrMsg = JobResult.fromObject(resObj);
      const out = BusPacket.fromObject({
        traceId: packet.traceId,
        senderId: cfg.senderId,
        protocolVersion: DEFAULT_PROTOCOL_VERSION,
        createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
        jobResult: jrMsg,
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
  };

  const sub: any = (cfg.nc as any).subscribe(cfg.subject, { queue: cfg.queue ?? cfg.subject }, onMessage);
  if (sub && (sub as any)[Symbol.asyncIterator]) {
    (async () => {
      for await (const msg of sub as any) {
        await onMessage(msg);
      }
    })();
  }
  return sub;
}
