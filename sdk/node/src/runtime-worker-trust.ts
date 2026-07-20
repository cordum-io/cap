import type { Type } from "protobufjs";

import type { Logger } from "./logger";
import { verifyInboundPacket } from "./security";
import type { WorkerCapabilityHandshake } from "./worker-trust";
import { NatsWorkerTrustExchange, type WorkerTrustRequester } from "./worker-trust-exchange";
import { WorkerTrustLifecycle } from "./worker-trust-lifecycle";
import {
  resolveRuntimeTrustSettings,
  type RuntimeTrustOptions,
  type RuntimeTrustSettings,
} from "./worker-trust-runtime-config";

const COMPONENT_ROLE_WORKER = 3;
const SDK_VERSION = "cap-node/v2";

function workerCapability(
  senderId: string,
  topics: readonly string[],
  sdkVersion: string
): WorkerCapabilityHandshake {
  const readyTopics = [...topics].sort();
  const capability: WorkerCapabilityHandshake = {
    componentId: senderId,
    role: COMPONENT_ROLE_WORKER,
    capabilities: {},
    readyTopics,
    supportedVersions: [1],
    sdkVersion,
    agentName: senderId,
  };
  Object.freeze(capability.capabilities);
  Object.freeze(capability.readyTopics);
  Object.freeze(capability.supportedVersions);
  return Object.freeze(capability);
}

function pinnedSchedulerMaps(settings: RuntimeTrustSettings): ReadonlyArray<Record<string, string>> {
  const config = settings.config;
  if (!config) return [];
  return Object.values(config.schedulerPublicKeys).map((key) => ({
    [config.expectedSchedulerId]: key.export({ type: "spki", format: "pem" }).toString(),
  }));
}

export class RuntimeWorkerTrust {
  readonly settings: RuntimeTrustSettings;
  readonly capability: WorkerCapabilityHandshake;
  private readonly pins: ReadonlyArray<Record<string, string>>;
  private lifecycle?: WorkerTrustLifecycle;
  private admitting = false;

  constructor(
    options: RuntimeTrustOptions,
    senderId: string,
    topics: readonly string[],
    private readonly logger: Logger
  ) {
    this.settings = resolveRuntimeTrustSettings(options, senderId);
    const sdkVersion = this.settings.config?.sdkVersion ?? SDK_VERSION;
    this.capability = workerCapability(senderId, topics, sdkVersion);
    this.pins = pinnedSchedulerMaps(this.settings);
  }

  get enabled(): boolean {
    return this.settings.mode !== "off";
  }

  get enforcing(): boolean {
    return this.settings.mode === "enforce";
  }

  get sessionToken(): string | undefined {
    return this.lifecycle?.sessionToken;
  }

  get productionSessionActive(): boolean {
    return this.admitting && this.enforcing && Boolean(this.sessionToken);
  }

  outboundSessionToken(): string {
    const token = this.sessionToken;
    if (this.enforcing && !token) throw new Error("authenticated worker session is unavailable");
    return token ?? "";
  }

  async authenticate(requester: WorkerTrustRequester): Promise<void> {
    if (!this.enabled) {
      this.admitting = true;
      return;
    }
    const exchange = new NatsWorkerTrustExchange(
      requester,
      this.settings,
      this.capability
    );
    this.lifecycle = new WorkerTrustLifecycle(exchange, this.settings, this.logger);
    await this.lifecycle.authenticate();
    this.admitting = true;
  }

  startRenewal(onEnforceFailure: (error: Error) => Promise<void>): void {
    this.lifecycle?.startRenewal(onEnforceFailure);
  }

  async reauthenticate(): Promise<void> {
    if (!this.enabled) {
      this.admitting = true;
      return;
    }
    this.admitting = false;
    if (!this.lifecycle) return;
    await this.lifecycle.reauthenticate();
    this.admitting = true;
  }

  stopAdmission(): void {
    this.admitting = false;
  }

  verifyInbound(
    packetType: Type,
    packet: unknown,
    legacyKeys: Readonly<Record<string, string>> | undefined
  ): boolean {
    if (!this.admitting || (this.enforcing && !this.sessionToken)) return false;
    if (!this.enabled) {
      verifyInboundPacket(packetType, packet, legacyKeys);
      return true;
    }
    for (const pins of this.pins) {
      try {
        verifyInboundPacket(packetType, packet, pins);
        return true;
      } catch {
        // Try the next explicitly configured scheduler rotation key.
      }
    }
    return false;
  }

  async close(): Promise<void> {
    this.admitting = false;
    await this.lifecycle?.close();
  }
}

export type { RuntimeTrustOptions } from "./worker-trust-runtime-config";
