import {
  createWorkerTrustConfig,
  parseWorkerTrustMode,
  type WorkerTrustConfig,
  type WorkerTrustMode,
} from "./worker-trust-contract";

export const ENV_WORKER_TRUST_MODE = "CORDUM_SDK_HANDSHAKE";
export const DEFAULT_TRUST_TIMEOUT_MS = 10_000;
export const DEFAULT_TRUST_RETRIES = 3;
export const DEFAULT_RENEW_MIN_INTERVAL_MS = 30_000;

const MAX_TRUST_TIMEOUT_MS = 60_000;
const MAX_TRUST_RETRIES = 10;

export interface RuntimeTrustOptions {
  mode?: WorkerTrustMode | string;
  config?: WorkerTrustConfig;
  timeoutMs?: number;
  retries?: number;
  renewMinIntervalMs?: number;
}

export interface RuntimeTrustSettings {
  readonly mode: WorkerTrustMode;
  readonly config?: WorkerTrustConfig;
  readonly timeoutMs: number;
  readonly retries: number;
  readonly renewMinIntervalMs: number;
}

function legacyDefault(options: RuntimeTrustOptions, environmentMode: string): boolean {
  return environmentMode.trim() === "" && Object.values(options).every((value) => value === undefined);
}

function requireBoundedNumber(value: number, max: number, name: string): number {
  if (!Number.isFinite(value) || value <= 0 || value > max) {
    throw new Error(`${name} is outside allowed bounds`);
  }
  return value;
}

function requireRetries(value: number): number {
  if (!Number.isInteger(value) || value < 1 || value > MAX_TRUST_RETRIES) {
    throw new Error("worker trust retries are outside allowed bounds");
  }
  return value;
}

function resolveEnabled(
  options: RuntimeTrustOptions,
  mode: WorkerTrustMode,
  senderId: string
): RuntimeTrustSettings {
  if (!options.config) {
    throw new Error("worker trust configuration is required");
  }
  const config = createWorkerTrustConfig(options.config);
  if (config.workerId !== senderId) {
    throw new Error("sender does not match worker trust identity");
  }
  return Object.freeze({
    mode,
    config,
    timeoutMs: requireBoundedNumber(
      options.timeoutMs ?? DEFAULT_TRUST_TIMEOUT_MS,
      MAX_TRUST_TIMEOUT_MS,
      "worker trust timeout"
    ),
    retries: requireRetries(options.retries ?? DEFAULT_TRUST_RETRIES),
    renewMinIntervalMs: requireBoundedNumber(
      options.renewMinIntervalMs ?? DEFAULT_RENEW_MIN_INTERVAL_MS,
      MAX_TRUST_TIMEOUT_MS,
      "worker trust renewal interval"
    ),
  });
}

export function resolveRuntimeTrustSettings(
  options: RuntimeTrustOptions,
  senderId: string,
  environmentMode = process.env[ENV_WORKER_TRUST_MODE] ?? ""
): RuntimeTrustSettings {
  if (legacyDefault(options, environmentMode)) {
    return Object.freeze({
      mode: "off",
      timeoutMs: DEFAULT_TRUST_TIMEOUT_MS,
      retries: DEFAULT_TRUST_RETRIES,
      renewMinIntervalMs: DEFAULT_RENEW_MIN_INTERVAL_MS,
    });
  }
  const mode = parseWorkerTrustMode(options.mode ?? environmentMode);
  if (mode === "off") {
    if (
      options.config ||
      options.timeoutMs !== undefined ||
      options.retries !== undefined ||
      options.renewMinIntervalMs !== undefined
    ) {
      throw new Error("worker trust mode off conflicts with trust configuration");
    }
    return Object.freeze({
      mode,
      timeoutMs: DEFAULT_TRUST_TIMEOUT_MS,
      retries: DEFAULT_TRUST_RETRIES,
      renewMinIntervalMs: requireBoundedNumber(
        options.renewMinIntervalMs ?? DEFAULT_RENEW_MIN_INTERVAL_MS,
        MAX_TRUST_TIMEOUT_MS,
        "worker trust renewal interval"
      ),
    });
  }
  return resolveEnabled(options, mode, senderId);
}
