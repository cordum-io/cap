import type {
  Msg,
  NatsConnection,
  Subscription,
  SubscriptionOptions,
} from "nats";
import type { Type } from "protobufjs";
import { loadRoot, SUBJECT_RESULT, DEFAULT_PROTOCOL_VERSION } from "./protos";
import { encodeDeterministic, encodeUnsignedForSignature } from "./codec";
import type { Logger } from "./logger";
import type { MetricsHook } from "./metrics";
import { noopMetrics } from "./metrics";
import * as crypto from "crypto";
import { verifyInboundPacket } from "./security";

/** Callback that processes a decoded JobRequest and returns a JobResult. */
type Handler = (jobRequest: any) => Promise<any>;

/** Middleware wraps a worker Handler, returning a new Handler. */
export type WorkerMiddleware = (next: Handler) => Handler;

/** Configuration for {@link startWorker}. */
export interface WorkerConfig {
  nc: NatsConnection;
  subject: string;
  queue?: string;
  handler: Handler;
  publicKeyMap?: { [senderId: string]: string }; // senderId -> public key in PEM format
  privateKey?: string; // private key in PEM format for signing outgoing messages
  senderId: string;
  /** Optional middleware applied in FIFO order before the handler. */
  middlewares?: WorkerMiddleware[];
  logger?: Logger;
  metrics?: MetricsHook;
}

interface JobRequestView {
  jobId?: string;
  topic?: string;
}

interface PacketView {
  traceId?: string;
  jobRequest?: JobRequestView;
}

interface SignablePacket {
  signature?: Uint8Array;
}

interface WorkerTypes {
  busPacket: Type;
  jobResult: Type;
}

/**
 * Subscribes to a NATS subject and dispatches incoming JobRequests to the handler.
 *
 * Results are wrapped in a signed BusPacket and published to the result subject.
 *
 * @param cfg - Worker configuration including NATS connection and handler.
 * @returns The underlying NATS subscription.
 */
export async function startWorker(cfg: WorkerConfig): Promise<Subscription> {
  const root = await loadRoot();
  const types: WorkerTypes = {
    busPacket: root.lookupType("cordum.agent.v1.BusPacket"),
    jobResult: root.lookupType("cordum.agent.v1.JobResult"),
  };
  const logger: Logger = cfg.logger ?? console;
  const metrics: MetricsHook = cfg.metrics ?? noopMetrics;
  const callback: NonNullable<SubscriptionOptions["callback"]> = (error, msg) => {
    if (error) {
      logger.error("worker subscription error", { error: String(error) });
      return;
    }
    void processWorkerMessage(cfg, types, logger, metrics, msg).catch((processingError) => {
      logger.error("worker message processing failed", {
        error: String(processingError),
      });
    });
  };
  return cfg.nc.subscribe(cfg.subject, {
    queue: cfg.queue ?? cfg.subject,
    callback,
  });
}

async function processWorkerMessage(
  cfg: WorkerConfig,
  types: WorkerTypes,
  logger: Logger,
  metrics: MetricsHook,
  msg: Msg
): Promise<void> {
  const packet = types.busPacket.decode(msg.data) as unknown as PacketView;
  try {
    verifyInboundPacket(types.busPacket, packet, cfg.publicKeyMap);
  } catch (error) {
    logger.warn("worker rejected inbound packet", { error: String(error) });
    return;
  }
  const request = packet.jobRequest;
  if (!request) return;
  metrics.onJobReceived(request.jobId ?? "", request.topic ?? "");
  const startedAt = Date.now();
  const result = await executeHandler(cfg, request);
  const elapsedMs = Date.now() - startedAt;
  recordResultMetrics(metrics, result, elapsedMs);
  await publishWorkerResult(cfg, types, packet, result);
}

async function executeHandler(
  cfg: WorkerConfig,
  request: JobRequestView
): Promise<Record<string, unknown>> {
  let wrappedHandler = cfg.handler;
  const middlewares = cfg.middlewares ?? [];
  for (let index = middlewares.length - 1; index >= 0; index -= 1) {
    wrappedHandler = middlewares[index](wrappedHandler);
  }
  let value: unknown;
  try {
    value = await wrappedHandler(request);
  } catch (error) {
    value = failureResult(request.jobId, error instanceof Error ? error.message : String(error));
  }
  const result = isRecord(value)
    ? value
    : failureResult(request.jobId, "handler returned null");
  if (typeof result.jobId !== "string" || result.jobId.length === 0) {
    result.jobId = request.jobId ?? "";
  }
  if (typeof result.workerId !== "string" || result.workerId.length === 0) {
    result.workerId = cfg.senderId;
  }
  return result;
}

function failureResult(jobId: string | undefined, errorMessage: string): Record<string, unknown> {
  return { jobId: jobId ?? "", status: "JOB_STATUS_FAILED", errorMessage };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function recordResultMetrics(
  metrics: MetricsHook,
  result: Record<string, unknown>,
  elapsedMs: number
): void {
  const jobId = String(result.jobId ?? "");
  if (result.status === "JOB_STATUS_FAILED") {
    metrics.onJobFailed(jobId, String(result.errorMessage ?? ""));
    return;
  }
  metrics.onJobCompleted(
    jobId,
    elapsedMs,
    String(result.status ?? "JOB_STATUS_SUCCEEDED")
  );
}

async function publishWorkerResult(
  cfg: WorkerConfig,
  types: WorkerTypes,
  packet: PacketView,
  result: Record<string, unknown>
): Promise<void> {
  const resultMessage = types.jobResult.fromObject(result);
  const outgoing = types.busPacket.fromObject({
    traceId: packet.traceId,
    senderId: cfg.senderId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobResult: resultMessage,
  }) as unknown as SignablePacket;
  if (cfg.privateKey) {
    const signer = crypto.createSign("sha256");
    signer.update(encodeUnsignedForSignature(types.busPacket, outgoing));
    outgoing.signature = signer.sign(cfg.privateKey);
  }
  const data = encodeDeterministic(types.busPacket, outgoing);
  await cfg.nc.publish(SUBJECT_RESULT, data);
}
