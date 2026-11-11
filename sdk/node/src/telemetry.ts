/**
 * W3C Trace Context and OpenTelemetry messaging spans for CAP over NATS, as
 * defined by spec/interop/trace-context.md.
 *
 * Security notes:
 * - Trace headers are untrusted correlation hints, never authority. Nothing
 *   here reads or writes a BusPacket, job metadata, an audit record, a tenant,
 *   a principal, a key, a policy or an approval.
 * - Baggage is never read or written: the carrier exposes only the exact
 *   lowercase `traceparent` and `tracestate` keys to the propagator.
 * - Call {@link extract} only after CAP-PRODUCTION admission or legacy
 *   validation has accepted the packet.
 * - An undefined {@link TelemetryConfig} is the off switch: no headers, no
 *   extraction and no spans.
 * - The client and runtime do not call this module yet.
 *
 * The types are structural, so this package has no OpenTelemetry dependency: pass
 * the OpenTelemetry JS API module and a W3C propagator to {@link fromOpenTelemetry}.
 */
import { headers as natsHeaders, type MsgHdrs } from "nats";
import {
  SUBJECT_ALERT, SUBJECT_CANCEL, SUBJECT_DLQ, SUBJECT_HANDSHAKE, SUBJECT_HEARTBEAT, SUBJECT_PROGRESS,
  SUBJECT_RESULT, SUBJECT_SUBMIT, SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
  SUBJECT_WORKFLOW_EVENT,
} from "./protos";

export interface Context {
  getValue(key: symbol): unknown;
  setValue(key: symbol, value: unknown): Context;
  deleteValue(key: symbol): Context;
}

export interface TraceState {
  set(key: string, value: string): TraceState;
  unset(key: string): TraceState;
  get(key: string): string | undefined;
  serialize(): string;
}

export interface SpanContext {
  traceId: string;
  spanId: string;
  traceFlags: number;
  isRemote?: boolean;
  traceState?: TraceState;
}

export type Attributes = Record<string, string | boolean>;

export interface Link { context: SpanContext }

export interface SpanOptions {
  kind?: number;
  attributes?: Record<string, unknown>;
  links?: Link[];
}

export interface Span {
  spanContext(): SpanContext;
  setAttribute(key: string, value: string): void;
  setStatus(status: { code: number; message?: string }): void;
  end(): void;
}

export interface Tracer { startSpan(name: string, options?: SpanOptions, context?: Context): Span }

export interface TracerProvider { getTracer(name: string, version?: string): Tracer }

export interface TextMapSetter { set(carrier: MsgHdrs, key: string, value: string): void }

export interface TextMapGetter {
  keys(carrier: MsgHdrs): string[];
  get(carrier: MsgHdrs, key: string): string | undefined;
}

export interface Propagator {
  inject(context: Context, carrier: MsgHdrs, setter: TextMapSetter): void;
  extract(context: Context, carrier: MsgHdrs, getter: TextMapGetter): Context;
  fields(): string[];
}

export interface OpenTelemetryApi {
  readonly trace: TracerProvider & {
    setSpan(context: Context, span: Span): Context;
    setSpanContext(context: Context, spanContext: SpanContext): Context;
    getSpanContext(context: Context): SpanContext | undefined;
    isSpanContextValid(spanContext: SpanContext): boolean;
  };
  readonly context: { active(): Context };
  readonly ROOT_CONTEXT: Context;
}

export interface TelemetryConfig {
  readonly api: OpenTelemetryApi;
  readonly tracer: Tracer;
  readonly propagator: Propagator;
}

/** The closed `error.type` values of the contract. */
export const ErrorType = Object.freeze({
  PROTOCOL_VERSION_MISMATCH: "PROTOCOL_VERSION_MISMATCH",
  PROTOCOL_MALFORMED_PACKET: "PROTOCOL_MALFORMED_PACKET",
  PROTOCOL_UNKNOWN_PAYLOAD: "PROTOCOL_UNKNOWN_PAYLOAD",
  PROTOCOL_SIGNATURE_INVALID: "PROTOCOL_SIGNATURE_INVALID",
  PROTOCOL_SIGNATURE_MISSING: "PROTOCOL_SIGNATURE_MISSING",
  ADMISSION_REJECTED: "ADMISSION_REJECTED",
  HANDLER_FAILED: "HANDLER_FAILED",
  TRANSPORT_PUBLISH_FAILED: "TRANSPORT_PUBLISH_FAILED",
  OTHER: "_OTHER",
} as const);
export type ErrorType = (typeof ErrorType)[keyof typeof ErrorType];

const FIXED_SUBJECT_LIST: readonly string[] = Object.freeze([
  SUBJECT_SUBMIT, SUBJECT_RESULT, SUBJECT_HEARTBEAT, SUBJECT_ALERT, SUBJECT_PROGRESS, SUBJECT_CANCEL, SUBJECT_DLQ,
  SUBJECT_WORKFLOW_EVENT, SUBJECT_HANDSHAKE, SUBJECT_WORKER_HANDSHAKE_CHALLENGE, SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
]);

/** The fixed spec/09 `sys.*` subjects that span names may carry. */
export const FIXED_SUBJECTS: ReadonlySet<string> = new Set(FIXED_SUBJECT_LIST);

export interface Publish {
  subject: string;
  /** The configured `job.<pool>` subject this participant publishes to. */
  pool?: string;
  replyInbox?: boolean;
  /** `BusPacket.trace_id`; pass "" when it was copied from an unauthenticated packet. */
  conversationId?: string;
}

export interface Process {
  subject: string;
  /** The configured `job.<pool>` subject this participant consumes from. */
  pool?: string;
  replyInbox?: boolean;
  queueGroup?: string;
  durable?: string;
  /** True only after CAP-PRODUCTION admission accepted the packet. */
  productionVerified?: boolean;
  /** The 16-byte `SignatureMetadata.message_id`. */
  messageId?: Uint8Array;
  conversationId?: string;
  /** The creation context returned by {@link extract}. */
  creation?: SpanContext;
}

export interface MessagingSpan {
  /** Records `error.type` (outside the span's closed set it becomes `_OTHER`) and ERROR status. */
  fail(errorType: ErrorType | string): void;
  end(): void;
}

export interface StartedSpan { context: Context | undefined; span: MessagingSpan }

const TRACER_NAME = "cap.telemetry";
const SPAN_KIND_PRODUCER = 3;
const SPAN_KIND_CONSUMER = 4;
const STATUS_ERROR = 2;
const TRACE_KEYS: readonly string[] = Object.freeze(["traceparent", "tracestate"]);
const PUBLISH_ERRORS: ReadonlySet<string> = new Set([ErrorType.TRANSPORT_PUBLISH_FAILED, ErrorType.OTHER]);
const PROCESS_ERRORS: ReadonlySet<string> = new Set(
  Object.values(ErrorType).filter((v) => v !== ErrorType.TRANSPORT_PUBLISH_FAILED),
);
const NOOP_SPAN: MessagingSpan = Object.freeze({ fail: () => undefined, end: () => undefined });

const TRACE_CARRIER: TextMapGetter & TextMapSetter = Object.freeze({
  keys: (carrier: MsgHdrs) => TRACE_KEYS.filter((key) => carrier.has(key)),
  get: (carrier: MsgHdrs, key: string) => (TRACE_KEYS.includes(key) && carrier.has(key) ? carrier.get(key) : undefined),
  set: (carrier: MsgHdrs, key: string, value: string) => {
    if (TRACE_KEYS.includes(key)) carrier.set(key, value);
  },
});

class RecordingSpan implements MessagingSpan {
  constructor(private readonly span: Span, private readonly allowed: ReadonlySet<string>) {}

  fail(errorType: ErrorType | string): void {
    const value = this.allowed.has(errorType) ? errorType : ErrorType.OTHER;
    this.span.setAttribute("error.type", value);
    this.span.setStatus({ code: STATUS_ERROR, message: value });
  }

  end(): void {
    this.span.end();
  }
}

function requireMethods(target: unknown, label: string, methods: readonly string[]): void {
  for (const method of methods) {
    const value = target == null ? undefined : (target as Record<string, unknown>)[method];
    if (typeof value !== "function") throw new TypeError(`${label} must provide ${method}()`);
  }
}

/**
 * Builds a config from the OpenTelemetry JS API module (`import * as api`), a
 * W3C trace-context propagator (never the global one, which may carry baggage)
 * and an optional tracer provider (default: the global provider). Throws a
 * TypeError when a required method is missing.
 */
export function fromOpenTelemetry(
  api: OpenTelemetryApi,
  propagator: Propagator,
  tracerProvider?: TracerProvider,
): TelemetryConfig {
  const traceMethods = ["getTracer", "setSpan", "setSpanContext", "getSpanContext", "isSpanContextValid"];
  requireMethods(api == null ? undefined : api.trace, "api.trace", traceMethods);
  requireMethods(api.context, "api.context", ["active"]);
  requireMethods(api.ROOT_CONTEXT, "api.ROOT_CONTEXT", ["getValue", "setValue", "deleteValue"]);
  requireMethods(propagator, "propagator", ["inject", "extract", "fields"]);
  if (tracerProvider !== undefined) requireMethods(tracerProvider, "tracerProvider", ["getTracer"]);
  const tracer = (tracerProvider ?? api.trace).getTracer(TRACER_NAME);
  requireMethods(tracer, "tracer", ["startSpan"]);
  return Object.freeze({ api, tracer, propagator });
}

/**
 * Replaces `traceparent`/`tracestate` with the given (default: active) context.
 * Returns undefined when `headers` was undefined and nothing was written.
 */
export function injectHeaders(
  config: TelemetryConfig | undefined, headers?: MsgHdrs, context?: Context,
): MsgHdrs | undefined {
  if (config === undefined) return headers;
  const carrier = headers ?? natsHeaders();
  for (const key of TRACE_KEYS) carrier.delete(key);
  config.propagator.inject(context ?? config.api.context.active(), carrier, TRACE_CARRIER);
  return headers === undefined && carrier.keys().length === 0 ? undefined : carrier;
}

/** Returns the valid remote creation context in `headers`, or undefined. */
export function extract(config: TelemetryConfig | undefined, headers?: MsgHdrs): SpanContext | undefined {
  if (config === undefined || headers === undefined) return undefined;
  const { api } = config;
  const spanContext = api.trace.getSpanContext(config.propagator.extract(api.ROOT_CONTEXT, headers, TRACE_CARRIER));
  if (spanContext === undefined || spanContext.isRemote !== true) return undefined;
  return api.trace.isSpanContextValid(spanContext) ? spanContext : undefined;
}

function isConfiguredPool(subject: string, pool: string | undefined): boolean {
  return pool !== undefined && pool.length > 4 && subject === pool && pool.startsWith("job.") && !/[*>\s]/.test(pool);
}

function destination(op: string, subject: string, pool?: string, replyInbox?: boolean): [string, Attributes] {
  if (replyInbox === true) {
    return [op, { "messaging.destination.temporary": true, "messaging.destination.anonymous": true }];
  }
  if (FIXED_SUBJECT_LIST.includes(subject) || isConfiguredPool(subject, pool)) {
    return [`${op} ${subject}`, { "messaging.destination.name": subject }];
  }
  return [op, {}];
}

function messaging(op: string, type: string, input: Publish | Process): [string, Attributes] {
  const [name, target] = destination(op, input.subject, input.pool, input.replyInbox);
  const base = { "messaging.system": "nats", "messaging.operation.name": op, "messaging.operation.type": type };
  return [name, { ...base, ...target }];
}

/** Starts the PRODUCER publish span; inject the returned context into the headers. */
export function startPublish(config: TelemetryConfig | undefined, publish: Publish, context?: Context): StartedSpan {
  if (config === undefined) return { context, span: NOOP_SPAN };
  const { api } = config;
  const parent = context ?? api.context.active();
  const [name, attributes] = messaging("publish", "send", publish);
  if (publish.conversationId) attributes["messaging.message.conversation_id"] = publish.conversationId;
  const span = config.tracer.startSpan(name, { kind: SPAN_KIND_PRODUCER, attributes }, parent);
  return { context: api.trace.setSpan(parent, span), span: new RecordingSpan(span, PUBLISH_ERRORS) };
}

function processSpan(input: Process): [string, Attributes] {
  const [name, attributes] = messaging("process", "process", input);
  if (input.queueGroup) attributes["messaging.consumer.group.name"] = input.queueGroup;
  if (input.durable) attributes["messaging.destination.subscription.name"] = input.durable;
  if (input.productionVerified === true) {
    const id = input.messageId;
    if (id !== undefined && id.length === 16) attributes["messaging.message.id"] = Buffer.from(id).toString("hex");
    if (input.conversationId) attributes["messaging.message.conversation_id"] = input.conversationId;
  }
  return [name, attributes];
}

/**
 * Starts the CONSUMER process span for an admitted packet. A valid creation
 * context is linked, and is the parent only when the context holds no valid span.
 */
export function startProcess(config: TelemetryConfig | undefined, input: Process, context?: Context): StartedSpan {
  if (config === undefined) return { context, span: NOOP_SPAN };
  const { api } = config;
  const base = context ?? api.context.active();
  const creation = input.creation;
  const linked = creation !== undefined && api.trace.isSpanContextValid(creation);
  const ambient = api.trace.getSpanContext(base);
  const hasAmbient = ambient !== undefined && api.trace.isSpanContextValid(ambient);
  const parent = linked && !hasAmbient ? api.trace.setSpanContext(base, creation) : base;
  const [name, attributes] = processSpan(input);
  const options: SpanOptions = { kind: SPAN_KIND_CONSUMER, attributes };
  if (linked) options.links = [{ context: creation }];
  const span = config.tracer.startSpan(name, options, parent);
  return { context: api.trace.setSpan(base, span), span: new RecordingSpan(span, PROCESS_ERRORS) };
}
