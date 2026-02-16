import { connect, NatsConnection } from "nats";
import { createClient, RedisClientType } from "redis";
import { z, ZodTypeAny } from "zod";
import { encodeDeterministic, encodeUnsignedForSignature } from "./codec";
import { loadRoot, SUBJECT_RESULT, DEFAULT_PROTOCOL_VERSION } from "./protos";
import type { Middleware } from "./middleware";
import * as crypto from "crypto";

export interface Logger {
  info: (...args: any[]) => void;
  warn: (...args: any[]) => void;
  error: (...args: any[]) => void;
}

const DEFAULT_NATS_URL = "nats://127.0.0.1:4222";
const DEFAULT_REDIS_URL = "redis://127.0.0.1:6379/0";
const DEFAULT_TIMEOUT_MS = 5000;
const DEFAULT_MAX_BYTES = 2 * 1024 * 1024;

export interface BlobStore {
  get(key: string): Promise<Buffer | null>;
  set(key: string, data: Buffer): Promise<void>;
  close(): Promise<void>;
}

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

export interface Context {
  job: any;
  packet: any;
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
}

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
    throw new Error("empty pointer");
  }
  if (!ptr.startsWith("redis://")) {
    throw new Error("unsupported pointer scheme");
  }
  const key = ptr.slice("redis://".length);
  if (!key) {
    throw new Error("missing pointer key");
  }
  return key;
}

function withContextLogger(base: Logger, meta: { jobId: string; traceId: string; topic: string }): Logger {
  const prefix = `job_id=${meta.jobId} trace_id=${meta.traceId} topic=${meta.topic}`;
  return {
    info: (...args: any[]) => base.info(prefix, ...args),
    warn: (...args: any[]) => base.warn(prefix, ...args),
    error: (...args: any[]) => base.error(prefix, ...args),
  };
}

async function withTimeout<T>(promise: Promise<T>, ms: number | undefined, label: string): Promise<T> {
  if (!ms || ms <= 0) {
    return promise;
  }
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`${label} timed out`)), ms);
    promise
      .then((value) => {
        clearTimeout(timer);
        resolve(value);
      })
      .catch((err) => {
        clearTimeout(timer);
        reject(err);
      });
  });
}

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
  private readonly handlers = new Map<string, HandlerSpec>();
  private readonly middlewares: Middleware[] = [];
  private nc?: NatsConnection;
  private busPacketType?: any;
  private jobResultType?: any;

  constructor(options: AgentOptions = {}) {
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
    this.nc = await withTimeout(
      this.connectFn({ servers: this.natsUrl, name: this.senderId }),
      this.ioTimeoutMs,
      "nats connect"
    );
    if (!this.store) {
      this.store = new RedisBlobStore(this.redisUrl);
    }

    const root = await loadRoot();
    this.busPacketType = root.lookupType("cordum.agent.v1.BusPacket");
    this.jobResultType = root.lookupType("cordum.agent.v1.JobResult");

    for (const spec of this.handlers.values()) {
      (this.nc as any).subscribe(
        spec.topic,
        { queue: spec.topic },
        (...args: any[]) => {
          let err: Error | null = null;
          let msg: any;
          if (args.length >= 2) {
            err = args[0] as Error | null;
            msg = args[1];
          } else {
            msg = args[0];
          }
          if (err) {
            this.logger.error("runtime: subscribe error:", err);
            return;
          }
          if (!msg) {
            return;
          }
          void this.onMessage(msg, spec);
        }
      );
    }
  }

  async close(): Promise<void> {
    if (this.nc) {
      await this.nc.drain();
    }
    if (this.store) {
      await this.store.close();
    }
  }

  async run(): Promise<void> {
    await this.start();
    // Keep the process alive
    return new Promise(() => undefined);
  }

  private async onMessage(msg: any, spec: HandlerSpec): Promise<void> {
    if (!this.store) {
      this.logger.error("runtime: blob store not initialized");
      return;
    }
    if (!this.busPacketType) {
      this.logger.error("runtime: protobuf types not initialized");
      return;
    }

    let packet: any;
    try {
      packet = this.busPacketType.decode(msg.data);
    } catch (err) {
      this.logger.error("runtime: decode failed:", err);
      return;
    }

    if (this.publicKeyMap) {
      const senderId = packet.senderId;
      const signature = packet.signature;
      if (!senderId || !this.publicKeyMap[senderId]) {
        this.logger.warn(`runtime: no public key for sender ${senderId}`);
        return;
      }
      if (!signature || signature.length === 0) {
        this.logger.warn(`runtime: missing signature for sender ${senderId}`);
        return;
      }
      const unsignedData = encodeUnsignedForSignature(this.busPacketType, packet);
      const verify = crypto.createVerify("sha256");
      verify.update(unsignedData);
      if (!verify.verify(this.publicKeyMap[senderId], signature)) {
        this.logger.warn(`runtime: invalid signature from sender ${senderId}`);
        return;
      }
    }

    const req = packet.jobRequest;
    if (!req || !req.jobId) {
      return;
    }

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
        ctx.log.warn(`runtime: handler failed (attempt ${attempt + 1}/${spec.retries + 1}):`, err);
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

      await this.publishResult(ctx, req, resultPtr, elapsedMs);
    } catch (err) {
      await this.publishFailure(ctx, req, `result write failed: ${err}`, elapsedMs);
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
      ctx.log.error("runtime: NATS not initialized");
      return;
    }
    if (!this.busPacketType || !this.jobResultType) {
      ctx.log.error("runtime: protobuf types not initialized");
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

    if (this.privateKey) {
      const unsignedOut = encodeUnsignedForSignature(this.busPacketType, out);
      const sign = crypto.createSign("sha256");
      sign.update(unsignedOut);
      out.signature = sign.sign(this.privateKey);
    }

    const data = encodeDeterministic(this.busPacketType, out);
    await (this.nc.publish(SUBJECT_RESULT, data) as any);
  }
}
