import { NatsConnection } from "nats";
import { loadRoot, DEFAULT_PROTOCOL_VERSION, SUBJECT_SUBMIT } from "./protos";
import {
  encodeOutboundPacket,
  type MutableBusPacket,
  prepareOutboundPacket,
} from "./packet-boundary";
import {
  ErrorType, injectHeaders, startPublish,
  type Context as TelemetryContext, type TelemetryConfig,
} from "./telemetry";

/** Options for {@link submitJob}. */
export interface SubmitJobOptions {
  /** Records a publish span and sends its W3C trace context. Undefined sends no headers and records nothing. */
  telemetry?: TelemetryConfig;
  /** Parent context of the publish span (default: the active context). */
  context?: TelemetryContext;
}

/**
 * Publishes a JobRequest onto the CAP submit subject.
 *
 * @param nc - An active NATS connection.
 * @param jobRequest - Plain object conforming to the JobRequest protobuf schema.
 * @param traceId - Distributed trace identifier propagated through the bus.
 * @param senderId - Identity of the sender (used in the BusPacket envelope).
 * @param privateKey - Optional PEM-encoded ECDSA private key for signing.
 * @param sessionToken - Optional authenticated worker session token.
 * @param options - Optional trace configuration and publish parent context.
 */
export async function submitJob(
  nc: NatsConnection,
  jobRequest: Record<string, unknown>,
  traceId: string,
  senderId: string,
  privateKey?: string,
  sessionToken = "",
  options: SubmitJobOptions = {},
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
  if (options.telemetry !== undefined) {
    await publishTraced(nc, options.telemetry, traceId, data, options.context);
    return;
  }
  await nc.publish(SUBJECT_SUBMIT, data);
}

/** Publishes under a producer span and sends that span's headers. */
async function publishTraced(
  nc: NatsConnection, config: TelemetryConfig, traceId: string,
  data: Uint8Array, context: TelemetryContext | undefined,
): Promise<void> {
  const started = startPublish(config, { subject: SUBJECT_SUBMIT, conversationId: traceId }, context);
  try {
    const headers = injectHeaders(config, undefined, started.context);
    try {
      await nc.publish(SUBJECT_SUBMIT, data, { headers });
    } catch (error) {
      started.span.fail(ErrorType.TRANSPORT_PUBLISH_FAILED);
      throw error;
    }
  } finally {
    started.span.end();
  }
}
