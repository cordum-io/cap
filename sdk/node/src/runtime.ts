import {
  connect,
  Events,
  NatsConnection,
  Subscription,
  SubscriptionOptions,
} from "nats";
import { createClient, RedisClientType } from "redis";
import { z, ZodTypeAny } from "zod";
import { loadRoot, SUBJECT_RESULT, DEFAULT_PROTOCOL_VERSION } from "./protos";
import type { Middleware } from "./middleware";
import { heartbeatLoop, heartbeatPayload } from "./heartbeat";
import { MalformedPacketError, InvalidInputError } from "./errors";
import type { Logger } from "./logger";
import type { MetricsHook } from "./metrics";
import { noopMetrics } from "./metrics";
import { handshakePayload, publishHandshake } from "./handshake";
import { encodeOutboundPacket, prepareOutboundPacket } from "./packet-boundary";
import { verifyProductionPacket, ProductionTrustStore, ProductionWireError } from "./production-signing";
import { ReplayStore, ReplayConflictError, ReplayStoreUnavailableError } from "./production-replay";
import { validateIdentityBinding, IdentityMismatchError } from "./production-validation";
import * as crypto from "crypto";
import {
  RuntimeWorkerTrust,
  type RuntimeTrustOptions,
} from "./runtime-worker-trust";

export type { Logger } from "./logger";
export type { RuntimeTrustOptions } from "./runtime-worker-trust";

const DEFAULT_NATS_URL = "nats://127.0.0.1:4222";
const DEFAULT_REDIS_URL = "redis://127.0.0.1:6379/0";
const DEFAULT_TIMEOUT_MS = 5000;
const DEFAULT_MAX_BYTES = 2 * 1024 * 1024;

class OperationTimeoutError extends Error {}

/** Abstraction over payload storage (Redis, in-memory, etc.). */
export interface BlobStore {
  get(key: string): Promise<Buffer | null>;
  set(key: string, data: Buffer): Promise<void>;
  close(): Promise<void>;
}

/** Redis-backed {@link BlobStore} implementation. */
export class RedisBlobStore implements BlobStore {
  private readonly client: RedisClientType;
  private readonly ready: Promise<void>;

  constructor(redisUrl: string) {
    this.client = createClient({ url: redisUrl });
    this.client.on("error", (err) => {
      // Avoid throwing on background redis errors; surface on use instead.
      console.warn("cap.runtime redis error:", err);
    });
    this.ready = this.client.connect().then(() => undefined);
  }

  async get(key: string): Promise<Buffer | null> {
    await this.ready;
    const value = await this.client.get(key);
    return value === null ? null : Buffer.from(value, "utf-8");
  }

  async set(key: string, data: Buffer): Promise<void> {
    await this.ready;
    await this.client.set(key, data.toString("utf-8"));
  }

  async close(): Promise<void> {
    await this.client.quit();
  }
}

/** In-memory {@link BlobStore} for testing without external infrastructure. */
export class InMemoryBlobStore implements BlobStore {
  private readonly data = new Map<string, Buffer>();

  async get(key: string): Promise<Buffer | null> {
    return this.data.get(key) ?? null;
  }

  async set(key: string, data: Buffer): Promise<void> {
    this.data.set(key, data);
  }

  async close(): Promise<void> {
    return;
  }
}

/** Per-request context passed to every job handler. */
export interface Context {
  /** The decoded JobRequest protobuf. */
  job: any;
  /** The full BusPacket envelope. */
  packet: any;
  /** Scoped logger with job/trace metadata. */
  log: Logger;
  jobId: string;
  traceId: string;
}

type Handler<TIn, TOut> = (ctx: Context, data: TIn) => Promise<TOut> | TOut;

interface HandlerSpec {
  topic: string;
  handler: Handler<any, any>;
  inputSchema?: ZodTypeAny;
  outputSchema?: ZodTypeAny;
  retries: number;
}

type AgentState = "idle" | "starting" | "running" | "closing" | "closed";

/** Options for constructing an {@link Agent}. */
export interface AgentOptions {
  natsUrl?: string;
  redisUrl?: string;
  store?: BlobStore;
  publicKeyMap?: Record<string, string>;
  privateKey?: string;
  senderId?: string;
  retries?: number;
  ioTimeoutMs?: number;
  maxContextBytes?: number;
  maxResultBytes?: number;
  connectFn?: (opts: any) => Promise<NatsConnection>;
  logger?: Logger;
  metrics?: MetricsHook;
  heartbeatInterval?: number;
  pool?: string;
  maxParallel?: number;
  workerTrust?: RuntimeTrustOptions;
  /** CAP-PRODUCTION raw-packet admission (task-a13f83fa step-7). Both
   * productionTrust and replayStore must be set to enable; omitting either
   * preserves existing legacy/handshake-mode behavior unchanged. */
  productionTrust?: ProductionTrustStore;
  replayStore?: ReplayStore;
}

/** Per-handler options for {@link Agent.job}. */
export interface JobOptions<TOut = any> {
  outputSchema?: z.ZodType<TOut>;
  retries?: number;
}

type JobRegistrationOptions<TIn, TOut> = JobOptions<TOut> & { inputSchema?: z.ZodType<TIn> };

function pointerForKey(key: string): string {
  return `redis://${key}`;
}

function keyFromPointer(ptr: string): string {
  if (!ptr) {
    throw new InvalidInputError("empty pointer");
  }
  if (!ptr.startsWith("redis://")) {
    throw new InvalidInputError("unsupported pointer scheme");
  }
  const key = ptr.slice("redis://".length);
  if (!key) {
    throw new InvalidInputError("missing pointer key");
  }
  return key;
}

function withContextLogger(base: Logger, meta: { jobId: string; traceId: string; topic: string }): Logger {
  return {
    info(msg: string, fields?: Record<string, any>): void {
      base.info(msg, { ...meta, ...fields });
    },
    warn(msg: string, fields?: Record<string, any>): void {
      base.warn(msg, { ...meta, ...fields });
    },
    error(msg: string, fields?: Record<string, any>): void {
      base.error(msg, { ...meta, ...fields });
    },
  };
}

async function withTimeout<T>(
  promise: Promise<T>,
  ms: number | undefined,
  label: string,
  cancellation?: Promise<never>
): Promise<T> {
  if (!ms || ms <= 0) return cancellation ? Promise.race([promise, cancellation]) : promise;
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new OperationTimeoutError(`${label} timed out`)), ms);
    const succeed = (value: T) => {
      clearTimeout(timer);
      resolve(value);
    };
    const fail = (error: unknown) => {
      clearTimeout(timer);
      reject(error);
    };
    promise.then(succeed, fail);
    cancellation?.then(undefined, fail);
  });
}

/**
 * High-level runtime that manages typed job handlers, blob storage, and NATS subscriptions.
 *
 * Register handlers with {@link Agent.job}, then call {@link Agent.run} to start processing.
 */
export class Agent {
  private readonly natsUrl: string;
  private readonly redisUrl: string;
  private store?: BlobStore;
  private readonly publicKeyMap?: Record<string, string>;
  private readonly privateKey?: string;
  private readonly senderId: string;
  private readonly defaultRetries: number;
  private readonly ioTimeoutMs: number;
  private readonly maxContextBytes?: number;
  private readonly maxResultBytes?: number;
  private readonly connectFn: (opts: any) => Promise<NatsConnection>;
  private readonly logger: Logger;
  private readonly metrics: MetricsHook;
  private readonly heartbeatInterval: number;
  private readonly pool: string;
  private readonly maxParallel: number;
  private readonly workerTrustOptions: RuntimeTrustOptions;
  private readonly productionTrust?: ProductionTrustStore;
  private readonly replayStore?: ReplayStore;
  private readonly handlers = new Map<string, HandlerSpec>();
  private readonly middlewares: Middleware[] = [];
  private nc?: NatsConnection;
  private busPacketType?: any;
  private jobResultType?: any;
  private heartbeatHandle?: { stop: () => void };
  private activeJobCount = 0;
  private readonly subscriptions: Subscription[] = [];
  private readonly inFlight = new Set<Promise<void>>();
  private state: AgentState = "idle";
  private startPromise?: Promise<void>;
  private closePromise?: Promise<void>;
  private trust?: RuntimeWorkerTrust;
  private statusWatchActive = false;
  private statusTask?: Promise<void>;
  private readonly startupCloseSignal: Promise<never>;
  private signalStartupClose!: (error: Error) => void;
  private startupCancelled = false;
  private startupTransportInterrupted = false;

  constructor(options: AgentOptions = {}) {
    this.startupCloseSignal = new Promise<never>((_, reject) => {
      this.signalStartupClose = reject;
    });
    void this.startupCloseSignal.catch(() => undefined);
    this.natsUrl = options.natsUrl ?? process.env.NATS_URL ?? DEFAULT_NATS_URL;
    this.redisUrl = options.redisUrl ?? process.env.REDIS_URL ?? DEFAULT_REDIS_URL;
    this.store = options.store;
    this.publicKeyMap = options.publicKeyMap;
    this.privateKey = options.privateKey;
    this.senderId = options.senderId ?? "cap-runtime";
    this.defaultRetries = Math.max(0, options.retries ?? 0);
    this.ioTimeoutMs = options.ioTimeoutMs ?? DEFAULT_TIMEOUT_MS;
    this.maxContextBytes =
      options.maxContextBytes === undefined
        ? DEFAULT_MAX_BYTES
        : options.maxContextBytes > 0
          ? options.maxContextBytes
          : undefined;
    this.maxResultBytes =
      options.maxResultBytes === undefined
        ? DEFAULT_MAX_BYTES
        : options.maxResultBytes > 0
          ? options.maxResultBytes
          : undefined;
    this.connectFn = options.connectFn ?? connect;
    this.logger = options.logger ?? console;
    this.metrics = options.metrics ?? noopMetrics;
    this.heartbeatInterval = options.heartbeatInterval ?? 5000;
    this.pool = options.pool ?? "";
    this.maxParallel = Math.max(1, options.maxParallel ?? 1);
    this.workerTrustOptions = { ...(options.workerTrust ?? {}) };
    this.productionTrust = options.productionTrust;
    this.replayStore = options.replayStore;
  }

  private productionEnabled(): boolean {
    return this.productionTrust !== undefined && this.replayStore !== undefined;
  }

  /** Raw-wire CAP-PRODUCTION admission: verify the exact received bytes
   * (never a re-serialized object), then atomic replay admission, then
   * authoritative identity — all BEFORE any handler runs. Returns null (and
   * logs without raw signature/payload bytes) on any rejection. */
  private decodeProductionPacket(raw: Uint8Array): any | null {
    let packet: any;
    try {
      packet = verifyProductionPacket(raw, this.busPacketType!, this.productionTrust!);
    } catch (err) {
      this.logger.warn("production admission rejected packet", { error: err instanceof ProductionWireError ? err.message : String(err) });
      return null;
    }
    const metadata = packet.signatureMetadata;
    const digest = crypto.createHash("sha256").update(Buffer.from(raw)).digest();
    const expiresAtMs = (() => {
      const ts = metadata.expiresAt;
      const seconds = typeof ts.seconds === "object" ? Number(ts.seconds.toString()) : Number(ts.seconds ?? 0);
      return seconds * 1000 + Math.floor(Number(ts.nanos ?? 0) / 1e6);
    })();
    try {
      this.replayStore!.admit(packet.identity?.tenantId ?? "", metadata.audience, packet.senderId, metadata.messageId, digest, expiresAtMs);
    } catch (err) {
      if (err instanceof ReplayConflictError || err instanceof ReplayStoreUnavailableError) {
        this.logger.warn("production admission rejected packet", { error: err.message });
        return null;
      }
      throw err;
    }
    if (packet.jobRequest && packet.identity) {
      try {
        validateIdentityBinding(packet.jobRequest, packet.identity);
      } catch (err) {
        if (err instanceof IdentityMismatchError) {
          this.logger.warn("production admission rejected packet", { error: err.message });
          return null;
        }
        throw err;
      }
    }
    return packet;
  }

  /** Appends middleware to the agent. Middleware executes in registration order before the handler. */
  use(...mw: Middleware[]): void {
    this.middlewares.push(...mw);
  }

  job<TIn, TOut>(
    topic: string,
    inputSchema: z.ZodType<TIn>,
    handler: Handler<TIn, TOut>,
    options?: JobOptions<TOut>
  ): void;
  job<TIn, TOut>(
    topic: string,
    handler: Handler<TIn, TOut>,
    options?: JobRegistrationOptions<TIn, TOut>
  ): void;
  job<TIn, TOut>(
    topic: string,
    arg2: z.ZodType<TIn> | Handler<TIn, TOut>,
    arg3?: Handler<TIn, TOut> | JobRegistrationOptions<TIn, TOut>,
    arg4: JobOptions<TOut> = {}
  ): void {
    let inputSchema: z.ZodType<TIn> | undefined;
    let handler: Handler<TIn, TOut>;
    let options: JobRegistrationOptions<TIn, TOut>;

    if (typeof arg2 === "function") {
      handler = arg2;
      options = (arg3 as JobRegistrationOptions<TIn, TOut>) ?? {};
      inputSchema = options.inputSchema;
    } else {
      inputSchema = arg2;
      handler = arg3 as Handler<TIn, TOut>;
      options = arg4 as JobRegistrationOptions<TIn, TOut>;
    }

    if (!handler) {
      throw new Error("handler is required");
    }
    const retries = options.retries === undefined ? this.defaultRetries : Math.max(0, options.retries);
    this.handlers.set(topic, {
      topic,
      handler,
      inputSchema,
      outputSchema: options.outputSchema as ZodTypeAny | undefined,
      retries,
    });
  }

  async start(): Promise<void> {
    if (this.handlers.size === 0) {
      throw new Error("no handlers registered");
    }
    if (this.state !== "idle") {
      throw new Error(`Agent already started or closed (state: ${this.state})`);
    }
    this.trust = new RuntimeWorkerTrust(
      this.workerTrustOptions,
      this.senderId,
      [...this.handlers.keys()],
      this.logger
    );
    this.state = "starting";
    this.startPromise = this.startInternal();
    return this.startPromise;
  }

  async close(): Promise<void> {
    if (!this.closePromise) {
      this.cancelStartup();
      this.closePromise = this.closeInternal();
    }
    return this.closePromise;
  }

  get sessionToken(): string | undefined {
    return this.trust?.sessionToken;
  }

  private async startInternal(): Promise<void> {
    try {
      await this.initializeRuntime();
      if (!this.nc || !this.trust) throw new Error("runtime trust is unavailable");
      this.requireStartupContinuity();
      this.startStatusWatcher();
      await this.trust.authenticate(this.nc);
      this.requireStartupContinuity();
      this.subscribeHandlers(this.nc);
      await this.publishStartupHandshake();
      this.requireStartupContinuity();
      this.startHeartbeat();
      this.state = "running";
      this.trust.startRenewal((error) => this.onTrustFailure(error));
    } catch (error) {
      try {
        await this.closeResources();
      } catch (cleanupError) {
        this.logger.error("Agent start cleanup failed", {
          error: String(cleanupError),
        });
      }
      this.state = "closed";
      throw error;
    }
  }

  private async initializeRuntime(): Promise<void> {
    const connection = await this.connectRuntime();
    this.nc = connection;
    if (!this.store) {
      this.store = new RedisBlobStore(this.redisUrl);
    }
    const root = await loadRoot();
    this.busPacketType = root.lookupType("cordum.agent.v1.BusPacket");
    this.jobResultType = root.lookupType("cordum.agent.v1.JobResult");
  }

  private subscribeHandlers(connection: NatsConnection): void {
    for (const spec of this.handlers.values()) {
      this.subscriptions.push(this.subscribe(connection, spec));
    }
  }

  private async connectRuntime(): Promise<NatsConnection> {
    const pending = this.connectFn({ servers: this.natsUrl, name: this.senderId });
    try {
      return await withTimeout(
        pending,
        this.ioTimeoutMs,
        "nats connect",
        this.startupCloseSignal
      );
    } catch (error) {
      if (error instanceof OperationTimeoutError || this.startupCancelled) {
        void pending.then(
          (connection) => this.closeLateConnection(connection),
          () => undefined
        );
      }
      throw error;
    }
  }

  private async closeLateConnection(connection: NatsConnection): Promise<void> {
    try {
      await connection.close();
    } catch (error) {
      this.logger.warn("late NATS connection cleanup failed", { error: String(error) });
    }
  }

  private subscribe(connection: NatsConnection, spec: HandlerSpec): Subscription {
    const options: SubscriptionOptions = {
      queue: spec.topic,
      callback: (error, msg) => {
        if (error) {
          this.logger.error("subscribe error", { error: String(error) });
          return;
        }
        this.trackMessage(this.onMessage(msg, spec));
      },
    };
    return connection.subscribe(spec.topic, options);
  }

  private trackMessage(work: Promise<void>): void {
    const tracked = work
      .catch((error: unknown) => {
        this.logger.error("message processing failed", { error: String(error) });
      })
      .finally(() => this.inFlight.delete(tracked));
    this.inFlight.add(tracked);
  }

  private async publishStartupHandshake(): Promise<void> {
    if (!this.nc) {
      throw new Error("NATS not initialized");
    }
    try {
      const readyTopics = Array.from(this.handlers.keys()).sort();
      const capability = this.trust?.capability;
      const advertisedCapabilities = this.trust?.enabled
        ? capability?.capabilities ?? {}
        : Object.fromEntries(readyTopics.map((topic) => [topic, true]));
      const packet = await handshakePayload(
        this.senderId,
        advertisedCapabilities,
        this.senderId,
        readyTopics,
        this.senderId,
        capability?.sdkVersion ?? "cap-node/v2",
        this.trust?.outboundSessionToken() ?? ""
      );
      await publishHandshake(this.nc, packet, this.privateKey);
    } catch (error) {
      if (this.trust?.enabled) throw error;
      this.logger.warn("handshake publish failed", {
        senderId: this.senderId,
        error: String(error),
      });
    }
  }

  private startHeartbeat(): void {
    if (!this.nc) {
      throw new Error("NATS not initialized");
    }
    this.heartbeatHandle = heartbeatLoop(
      this.nc,
      () => heartbeatPayload(
        this.senderId,
        this.pool,
        this.activeJobCount,
        this.maxParallel,
        0,
        "",
        this.senderId,
        this.trust?.outboundSessionToken() ?? ""
      ),
      {
        interval: this.heartbeatInterval,
        privateKey: this.privateKey,
        metrics: this.metrics,
        logger: this.logger,
      }
    );
  }

  private startStatusWatcher(): void {
    if (!this.nc || !this.trust?.enabled || this.statusTask) return;
    this.statusWatchActive = true;
    this.statusTask = this.watchConnectionStatus(this.nc).finally(() => {
      this.statusTask = undefined;
    });
  }

  private async watchConnectionStatus(connection: NatsConnection): Promise<void> {
    try {
      for await (const status of connection.status()) {
        if (!this.statusWatchActive) return;
        if (status.type === Events.Disconnect) {
          if (this.state === "starting") this.startupTransportInterrupted = true;
          this.trust?.stopAdmission();
          this.stopHeartbeat();
        } else if (status.type === Events.Reconnect) {
          await this.reauthenticateAfterReconnect();
        }
      }
    } catch (error) {
      if (this.statusWatchActive) {
        this.logger.error("NATS status watcher failed", { errorType: this.errorType(error) });
      }
    }
  }

  private async reauthenticateAfterReconnect(): Promise<void> {
    if (this.state !== "running" || !this.nc || !this.trust?.enabled) return;
    this.trust.stopAdmission();
    this.stopHeartbeat();
    await this.drainHandlerSubscriptions();
    try {
      await this.trust.reauthenticate();
      if (this.state !== "running") return;
      this.subscribeHandlers(this.nc);
      await this.publishStartupHandshake();
      this.startHeartbeat();
      this.trust.startRenewal((error) => this.onTrustFailure(error));
    } catch (error) {
      if (this.trust.enforcing) await this.onTrustFailure(this.asError(error));
      else this.logger.warn("worker trust reconnect failed", { errorType: this.errorType(error) });
    }
  }

  private async onTrustFailure(error: Error): Promise<void> {
    this.trust?.stopAdmission();
    this.stopHeartbeat();
    await this.drainHandlerSubscriptions();
    this.logger.error("authenticated session renewal failed; admissions stopped", {
      errorType: error.name,
    });
  }

  private stopHeartbeat(): void {
    this.heartbeatHandle?.stop();
    this.heartbeatHandle = undefined;
  }

  private async drainHandlerSubscriptions(): Promise<void> {
    const subscriptions = this.subscriptions.splice(0);
    await Promise.all(subscriptions.map((subscription) => this.drainSubscription(subscription)));
  }

  private asError(error: unknown): Error {
    return error instanceof Error ? error : new Error(String(error));
  }

  private errorType(error: unknown): string {
    return this.asError(error).name;
  }

  private async closeInternal(): Promise<void> {
    if (this.state === "starting" && this.startPromise) {
      this.state = "closing";
      await this.trust?.close();
      try {
        await this.startPromise;
      } catch {
        return;
      }
    }
    if (this.state === "closed") {
      return;
    }
    this.state = "closing";
    try {
      await this.closeResources();
    } finally {
      this.state = "closed";
    }
  }

  private cancelStartup(): void {
    if (this.state !== "starting" || this.startupCancelled) return;
    this.startupCancelled = true;
    this.signalStartupClose(new Error("Agent start cancelled by close"));
  }

  private requireStartupContinuity(): void {
    if (this.state !== "starting") throw new Error("Agent start cancelled by close");
    if (this.startupTransportInterrupted) {
      throw new Error("worker trust transport interrupted during startup");
    }
  }

  private async closeResources(): Promise<void> {
    this.statusWatchActive = false;
    this.stopHeartbeat();
    await this.drainHandlerSubscriptions();
    while (this.inFlight.size > 0) {
      await Promise.all([...this.inFlight]);
    }
    await this.trust?.close();
    const connection = this.nc;
    try {
      if (connection) {
        try {
          await withTimeout(connection.drain(), this.ioTimeoutMs, "nats drain");
        } catch (error) {
          this.logger.warn("connection drain failed", { error: String(error) });
          await connection.close();
        }
      }
    } finally {
      this.nc = undefined;
      const store = this.store;
      this.store = undefined;
      if (store) {
        await store.close();
      }
    }
  }

  private async drainSubscription(subscription: Subscription): Promise<void> {
    try {
      await withTimeout(subscription.drain(), this.ioTimeoutMs, "subscription drain");
    } catch (error) {
      this.logger.warn("subscription drain failed", { error: String(error) });
      try {
        subscription.unsubscribe();
      } catch (unsubscribeError) {
        this.logger.warn("subscription unsubscribe failed", {
          error: String(unsubscribeError),
        });
      }
    }
  }

  async run(): Promise<void> {
    await this.start();
    // Keep the process alive
    return new Promise(() => undefined);
  }

  private async onMessage(msg: any, spec: HandlerSpec): Promise<void> {
    if (!this.store) {
      this.logger.error("blob store not initialized");
      return;
    }
    if (!this.busPacketType) {
      this.logger.error("protobuf types not initialized");
      return;
    }

    let packet: any;
    if (this.productionEnabled()) {
      // CAP-PRODUCTION: verify the exact received wire bytes (never a
      // re-serialized object) and admit atomically for replay BEFORE any
      // handler runs. A rejection here never reaches decode-based dispatch.
      packet = this.decodeProductionPacket(msg.data);
      if (!packet) return;
    } else {
      try {
        packet = this.busPacketType.decode(msg.data);
      } catch (err) {
        this.logger.error("decode failed", { error: String(new MalformedPacketError(err instanceof Error ? err.message : String(err))) });
        return;
      }

      try {
        if (!this.trust?.verifyInbound(this.busPacketType, packet, this.publicKeyMap)) {
          throw new Error("worker trust admission rejected packet");
        }
      } catch (error) {
        this.logger.warn("Agent rejected inbound packet", { error: String(error) });
        return;
      }
    }

    const req = packet.jobRequest;
    if (!req || !req.jobId) {
      return;
    }

    this.activeJobCount += 1;
    try {
      this.metrics.onJobReceived(req.jobId, req.topic);

      const ctx: Context = {
        job: req,
        packet,
        log: withContextLogger(this.logger, { jobId: req.jobId, traceId: packet.traceId, topic: req.topic }),
        jobId: req.jobId,
        traceId: packet.traceId,
      };

      let payload: Buffer | null = null;
      try {
        const key = keyFromPointer(req.contextPtr);
        payload = await withTimeout(this.store.get(key), this.ioTimeoutMs, "context fetch");
        if (!payload) {
          throw new Error("context not found");
        }
        if (this.maxContextBytes && payload.length > this.maxContextBytes) {
          throw new Error("context exceeds max size");
        }
      } catch (err) {
        await this.publishFailure(ctx, req, err instanceof Error ? err.message : String(err), 0);
        return;
      }

      let raw: any;
      try {
        raw = JSON.parse(payload.toString("utf-8"));
      } catch (err) {
        await this.publishFailure(ctx, req, `context decode failed: ${err}`, 0);
        return;
      }

      let inputData: any;
      try {
        inputData = spec.inputSchema ? spec.inputSchema.parse(raw) : raw;
      } catch (err) {
        await this.publishFailure(ctx, req, `input validation failed: ${err}`, 0);
        return;
      }

      // Build middleware chain: outermost first, terminal calls handler.
      const terminal = () => spec.handler(ctx, inputData);
      let chain = terminal;
      for (let i = this.middlewares.length - 1; i >= 0; i--) {
        const mw = this.middlewares[i];
        const next = chain;
        chain = () => mw(ctx, next);
      }

      const start = Date.now();
      let output: any;
      let error: string | null = null;
      for (let attempt = 0; attempt <= spec.retries; attempt += 1) {
        try {
          output = await chain();
          output = spec.outputSchema ? spec.outputSchema.parse(output) : output;
          error = null;
          break;
        } catch (err) {
          error = err instanceof Error ? err.message : String(err);
          ctx.log.warn("handler failed", { attempt: attempt + 1, maxAttempts: spec.retries + 1, error: String(err) });
        }
      }

      const elapsedMs = Date.now() - start;
      if (error) {
        await this.publishFailure(ctx, req, error, elapsedMs);
        return;
      }

      let resultPayload: Buffer;
      try {
        resultPayload = this.serializeOutput(output);
        if (this.maxResultBytes && resultPayload.length > this.maxResultBytes) {
          throw new Error("result exceeds max size");
        }
        const resultKey = `res:${req.jobId}`;
        await withTimeout(this.store.set(resultKey, resultPayload), this.ioTimeoutMs, "result write");
        const resultPtr = pointerForKey(resultKey);

        this.metrics.onJobCompleted(req.jobId, elapsedMs, "SUCCEEDED");
        await this.publishResult(ctx, req, resultPtr, elapsedMs);
      } catch (err) {
        await this.publishFailure(ctx, req, `result write failed: ${err}`, elapsedMs);
      }
    } finally {
      this.activeJobCount = Math.max(0, this.activeJobCount - 1);
    }
  }

  private serializeOutput(data: any): Buffer {
    if (data === undefined) {
      throw new Error("output is undefined");
    }
    if (Buffer.isBuffer(data)) {
      return data;
    }
    return Buffer.from(JSON.stringify(data), "utf-8");
  }

  private async publishFailure(ctx: Context, req: any, error: string, executionMs: number): Promise<void> {
    this.metrics.onJobFailed(req.jobId, error);
    await this.publishResult(ctx, req, "", executionMs, {
      status: "JOB_STATUS_FAILED",
      errorMessage: error,
    });
  }

  private async publishResult(
    ctx: Context,
    req: any,
    resultPtr: string,
    executionMs: number,
    overrides: Record<string, any> = {}
  ): Promise<void> {
    if (!this.nc) {
      ctx.log.error("NATS not initialized");
      return;
    }
    if (!this.busPacketType || !this.jobResultType) {
      ctx.log.error("protobuf types not initialized");
      return;
    }

    const jrMsg = this.jobResultType.fromObject({
      jobId: req.jobId,
      status: "JOB_STATUS_SUCCEEDED",
      resultPtr,
      workerId: this.senderId,
      executionMs,
      ...overrides,
    });

    const out = this.busPacketType.fromObject({
      traceId: ctx.packet.traceId,
      senderId: this.senderId,
      protocolVersion: DEFAULT_PROTOCOL_VERSION,
      createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
      jobResult: jrMsg,
    }) as any;
    prepareOutboundPacket(out, this.trust?.outboundSessionToken() ?? "");
    const data = encodeOutboundPacket(this.busPacketType, out, this.privateKey);
    await (this.nc.publish(SUBJECT_RESULT, data) as any);
  }
}
