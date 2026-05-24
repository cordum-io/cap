import { NatsConnection } from "nats";
import * as crypto from "crypto";
import { encodeDeterministic, encodeUnsignedForSignature } from "./codec";
import { loadRoot, DEFAULT_PROTOCOL_VERSION, SUBJECT_HEARTBEAT, sanitizeAgentName } from "./protos";
import type { Logger } from "./logger";
import type { MetricsHook } from "./metrics";

export type HeartbeatPacket = any;
export interface HeartbeatLoopOptions {
  interval?: number;
  privateKey?: string;
  metrics?: MetricsHook;
  logger?: Logger;
}

/**
 * Builds a heartbeat packet with CPU load only.
 */
export async function heartbeatPayload(
  workerId: string,
  pool: string,
  activeJobs: number,
  maxParallel: number,
  cpuLoad: number,
  authToken = "",
  agentName = ""
): Promise<HeartbeatPacket> {
  return heartbeatPayloadWithProgress(workerId, pool, activeJobs, maxParallel, cpuLoad, 0, 0, "", authToken, agentName);
}

/**
 * Builds a heartbeat packet including memory load.
 */
export async function heartbeatPayloadWithMemory(
  workerId: string,
  pool: string,
  activeJobs: number,
  maxParallel: number,
  cpuLoad: number,
  memoryLoad: number,
  authToken = "",
  agentName = ""
): Promise<HeartbeatPacket> {
  return heartbeatPayloadWithProgress(workerId, pool, activeJobs, maxParallel, cpuLoad, memoryLoad, 0, "", authToken, agentName);
}

/**
 * Builds a heartbeat packet including optional progress checkpoint fields.
 */
export async function heartbeatPayloadWithProgress(
  workerId: string,
  pool: string,
  activeJobs: number,
  maxParallel: number,
  cpuLoad: number,
  memoryLoad = 0,
  progressPct = 0,
  lastMemo = "",
  authToken = "",
  agentName = ""
): Promise<HeartbeatPacket> {
  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
  const normalizedAuthToken = authToken.trim();
  const normalizedAgentName = sanitizeAgentName(agentName);

  return BusPacket.fromObject({
    senderId: workerId,
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    heartbeat: {
      workerId,
      pool,
      activeJobs,
      maxParallelJobs: maxParallel,
      cpuLoad,
      memoryLoad,
      progressPct,
      lastMemo,
      ...(normalizedAuthToken ? { authToken: normalizedAuthToken } : {}),
      ...(normalizedAgentName ? { agentName: normalizedAgentName } : {}),
    },
  });
}

/**
 * Publishes a heartbeat packet to the CAP heartbeat subject.
 */
export async function emitHeartbeat(
  nc: NatsConnection,
  packet: HeartbeatPacket,
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
  await nc.publish(SUBJECT_HEARTBEAT, data);
}

/**
 * Starts periodic heartbeat emission. Call `stop()` to end the loop.
 */
export function heartbeatLoop(
  nc: NatsConnection,
  payloadFn: () => Promise<HeartbeatPacket>,
  opts: HeartbeatLoopOptions = {}
): { stop: () => void } {
  const interval = opts.interval ?? 5000;
  const logger = opts.logger ?? console;
  let stopped = false;
  let tickInFlight: Promise<void> | null = null;

  const tick = async (): Promise<void> => {
    if (stopped) {
      return;
    }
    try {
      const packet = await payloadFn();
      await emitHeartbeat(nc, packet, opts.privateKey);
      const workerId = packet?.heartbeat?.workerId ?? packet?.senderId ?? "";
      if (workerId) {
        opts.metrics?.onHeartbeatSent(workerId);
      }
    } catch (err) {
      logger.warn("heartbeat emission failed", { error: String(err) });
    }
  };

  const timer = setInterval(() => {
    tickInFlight = tick().finally(() => {
      tickInFlight = null;
    });
  }, Math.max(1, interval));
  timer.unref?.();

  return {
    stop: (): void => {
      if (stopped) {
        return;
      }
      stopped = true;
      clearInterval(timer);
      // Best-effort cleanup; no await in sync stop API.
      void tickInFlight;
    },
  };
}
