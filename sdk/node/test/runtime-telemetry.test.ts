import assert from "node:assert/strict";
import * as api from "@opentelemetry/api";
import { W3CTraceContextPropagator } from "@opentelemetry/core";
import { AlwaysOnSampler, BasicTracerProvider, InMemorySpanExporter, SimpleSpanProcessor, type ReadableSpan } from "@opentelemetry/sdk-trace-base";
import { headers, type MsgHdrs, type NatsConnection } from "nats";
import type { Type } from "protobufjs";
import { Agent, InMemoryBlobStore, type AgentOptions, type Context } from "../src/runtime";
import { submitJob } from "../src/client";
import { InMemoryReplayStore } from "../src/production-replay";
import { SUBJECT_RESULT, SUBJECT_SUBMIT } from "../src/protos";
import * as telemetry from "../src/telemetry";
import { busPacketType, keyPair, signedRawPacket } from "./production-admission-support";

const CREATION = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01";
const SILENT = { info() {}, warn() {}, error() {} };
type PublishArgs = [string, Uint8Array, { headers?: MsgHdrs }?];
class FakeNats {
  calls: PublishArgs[] = [];
  failure?: Error;
  publish(...args: PublishArgs): void {
    this.calls.push(args);
    if (this.failure) throw this.failure;
  }
  connection(): NatsConnection { return this as unknown as NatsConnection; }
}
class CountingPropagator extends W3CTraceContextPropagator {
  extracts = 0;
  override extract(context: api.Context, carrier: MsgHdrs, getter: api.TextMapGetter<MsgHdrs>): api.Context {
    this.extracts += 1;
    return super.extract(context, carrier, getter);
  }
}
function tracing() {
  const exporter = new InMemorySpanExporter();
  const provider = new BasicTracerProvider({
    sampler: new AlwaysOnSampler(), spanProcessors: [new SimpleSpanProcessor(exporter)],
  });
  const propagator = new CountingPropagator();
  return { exporter, provider, propagator, config: telemetry.fromOpenTelemetry(api, propagator, provider) };
}
function carrier(value = CREATION): MsgHdrs {
  const result = headers();
  result.set("traceparent", value);
  return result;
}
interface RuntimeHarness {
  busPacketType: Type;
  jobResultType: Type;
  nc: FakeNats;
  trust: object;
  onMessage(message: { data: Uint8Array; subject: string; headers?: MsgHdrs }, spec: {
    topic: string; retries: number; handler(ctx: Context): Promise<object>;
    inputSchema?: { parse(value: unknown): unknown };
  }): Promise<void>;
}
async function runtime(config?: telemetry.TelemetryConfig, options: AgentOptions = {}) {
  const store = new InMemoryBlobStore();
  const agent = new Agent({ senderId: "worker-a", store, logger: SILENT, telemetry: config, ...options });
  const harness = agent as unknown as RuntimeHarness;
  const type = await busPacketType();
  harness.busPacketType = type;
  harness.jobResultType = type.root.lookupType("cordum.agent.v1.JobResult");
  harness.nc = new FakeNats();
  harness.trust = { verifyInbound: () => true, outboundSessionToken: () => "" };
  await store.set("ctx:job-1", Buffer.from("{}"));
  const data = type.encode(type.fromObject({
    traceId: "trace-job-1", senderId: "scheduler-1", protocolVersion: 1,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: { jobId: "job-1", topic: "job.test", contextPtr: "redis://ctx:job-1" },
  })).finish();
  const message = { data, subject: "job.test", headers: carrier() };
  const seen: Array<telemetry.Context | undefined> = [];
  const spec: Parameters<RuntimeHarness["onMessage"]>[1] = { topic: "job.test", retries: 0, async handler(ctx: Context) {
    seen.push(ctx.otelContext);
    return { ok: true };
  } };
  return { harness, store, type, message, spec, seen };
}
function span(exporter: InMemorySpanExporter, name: string): ReadableSpan {
  const found = exporter.getFinishedSpans().find((value) => value.name === name);
  assert.ok(found, `missing finished span ${name}`);
  return found;
}
function assertResult(harness: RuntimeHarness, process: ReadableSpan, publish: ReadableSpan): void {
  assert.equal(publish.parentSpanContext?.spanId, process.spanContext().spanId);
  const output = harness.nc.calls[0][2]?.headers;
  assert.ok(output);
  assert.equal(output.get("traceparent"), `00-${publish.spanContext().traceId}-${publish.spanContext().spanId}-01`);
  assert.deepEqual(output.keys(), ["traceparent"]);
}

async function assertOtherFailures(): Promise<void> {
  for (const failure of ["fetch", "size", "decode", "validation", "write"]) {
    const t = tracing();
    const r = await runtime(t.config, failure === "size" ? { maxContextBytes: 1 } : {});
    const reject = () => { throw new Error(`${failure} failed`); };
    if (failure === "fetch") r.store.get = async () => reject();
    if (failure === "decode") await r.store.set("ctx:job-1", Buffer.from("not json"));
    if (failure === "validation") r.spec.inputSchema = { parse: reject };
    if (failure === "write") r.store.set = async () => reject();
    await r.harness.onMessage(r.message, r.spec);
    assert.equal(t.exporter.getFinishedSpans().length, 2, failure);
    const process = span(t.exporter, "process job.test");
    assert.equal(process.attributes["error.type"], "_OTHER", failure);
    assert.equal(process.status.code, api.SpanStatusCode.ERROR, failure);
    assertResult(r.harness, process, span(t.exporter, "publish sys.job.result"));
  }
}

async function assertPropagatorFailures(): Promise<void> {
  const t = tracing();
  const config = telemetry.fromOpenTelemetry(api, {
    fields: () => t.propagator.fields(),
    extract: (context, value, getter) => t.propagator.extract(context, value, getter),
    inject: () => { throw new Error("broken propagator"); },
  }, t.provider);
  const r = await runtime(config);
  await assert.rejects(submitJob(r.harness.nc.connection(), { jobId: "j", topic: "job.test" }, "trace-j", "client", undefined, "", { telemetry: config }), /broken propagator/);
  await assert.rejects(r.harness.onMessage(r.message, r.spec), /broken propagator/);
  assert.deepEqual(r.harness.nc.calls, []);
  assert.equal(t.exporter.getFinishedSpans().length, 4);
  assert.equal(span(t.exporter, "publish sys.job.submit").attributes["error.type"], undefined);
  const brokenExtract = telemetry.fromOpenTelemetry(api, {
    ...config.propagator, extract: () => { throw new Error("broken extraction"); },
  }, t.provider);
  const rejected = await runtime(brokenExtract);
  await assert.rejects(rejected.harness.onMessage(rejected.message, rejected.spec), /broken extraction/);
  assert.deepEqual(rejected.seen, []);
  assert.equal(t.exporter.getFinishedSpans().length, 4);
}

describe("Node runtime telemetry", () => {
  afterEach(() => api.trace.disable());

  it("starts no span and never extracts for a packet rejected at admission", async () => {
    const t = tracing();
    const r = await runtime(t.config, {
      productionTrust: { audience: "job.test", tenant: "tenant-a", sender: "scheduler-1", publicKeys: { k1: keyPair.publicKey } },
      replayStore: new InMemoryReplayStore(),
    });
    r.harness.trust = { productionSessionActive: true, outboundSessionToken: () => "session-a", settings: { config: {
      proofKeyId: "worker-key", proofPrivateKey: keyPair.privateKey, tenantId: "tenant-a", expectedSchedulerId: "scheduler-1",
    } } };
    const valid = await signedRawPacket({ audience: "job.test", contextPtr: "redis://ctx:job-1" });
    const tampered = Buffer.from(valid);
    tampered[tampered.length - 1] ^= 0xff;
    await r.harness.onMessage({ ...r.message, data: tampered }, r.spec);
    assert.equal(t.propagator.extracts, 0);
    assert.deepEqual(t.exporter.getFinishedSpans(), []);
    assert.deepEqual(r.harness.nc.calls, []);
    await r.harness.onMessage({ ...r.message, data: valid }, r.spec);
    assert.equal(t.propagator.extracts, 1);
    assert.equal(t.exporter.getFinishedSpans().length, 2);
    const process = span(t.exporter, "process job.test");
    const publish = span(t.exporter, "publish sys.job.result");
    assert.equal(process.attributes["messaging.message.id"], Buffer.from("0123456789abcdef").toString("hex"));
    assert.equal(process.attributes["messaging.message.conversation_id"], "trace-1");
    assert.equal(process.attributes["messaging.consumer.group.name"], "job.test");
    assert.equal(process.parentSpanContext?.traceId, CREATION.split("-")[1]);
    assert.ok(r.seen[0]);
    assert.deepEqual(api.trace.getSpanContext(r.seen[0]), process.spanContext());
    assert.equal(publish.attributes["messaging.message.conversation_id"], "trace-1");
    assertResult(r.harness, process, publish);
  });

  it("marks the process span HANDLER_FAILED and omits a legacy conversation id", async () => {
    const t = tracing();
    const r = await runtime(t.config);
    r.spec.handler = async () => { throw new Error("boom"); };
    await r.harness.onMessage(r.message, r.spec);
    const process = span(t.exporter, "process job.test");
    const publish = span(t.exporter, "publish sys.job.result");
    assert.equal(process.attributes["error.type"], "HANDLER_FAILED");
    assert.equal(process.status.code, api.SpanStatusCode.ERROR);
    for (const s of [process, publish]) assert.equal(s.attributes["messaging.message.conversation_id"], undefined);
    const packet = r.type.toObject(r.type.decode(r.harness.nc.calls[0][1]), { enums: String });
    assert.equal(packet.jobResult.status, "JOB_STATUS_FAILED");
    assert.equal(packet.jobResult.errorMessage, "boom");
    assertResult(r.harness, process, publish);
    await assertOtherFailures();
  });

  it("sends no headers and records nothing without telemetry", async () => {
    const t = tracing();
    api.trace.disable();
    assert.equal(api.trace.setGlobalTracerProvider(t.provider), true);
    try {
      const r = await runtime();
      Object.defineProperty(r.message, "headers", { get() { throw new Error("must not read headers"); } });
      await r.harness.onMessage(r.message, r.spec);
      await submitJob(r.harness.nc.connection(), { jobId: "j", topic: "job.test" }, "trace-j", "client");
      assert.deepEqual(r.harness.nc.calls.map((args) => [args[0], args.length]), [[SUBJECT_RESULT, 2], [SUBJECT_SUBMIT, 2]]);
      assert.deepEqual(r.seen, [undefined]);
      assert.deepEqual(t.exporter.getFinishedSpans(), []);
    } finally { api.trace.disable(); }
  });

  it("injects the submit publish span into the submit headers", async () => {
    const t = tracing();
    const nc = new FakeNats();
    await submitJob(nc.connection(), { jobId: "j", topic: "job.test" }, "trace-j", "client", undefined, "", { telemetry: t.config });
    assert.equal(t.exporter.getFinishedSpans().length, 1);
    const publish = span(t.exporter, "publish sys.job.submit");
    assert.equal(publish.attributes["messaging.message.conversation_id"], "trace-j");
    const output = nc.calls[0][2]?.headers;
    assert.ok(output);
    assert.deepEqual(output.keys(), ["traceparent"]);
    const sc = publish.spanContext();
    assert.equal(output.get("traceparent"), `00-${sc.traceId}-${sc.spanId}-${sc.traceFlags.toString(16).padStart(2, "0")}`);
  });

  it("never parents the process span on the active context", async () => {
    const t = tracing();
    const ambient = t.provider.getTracer("test").startSpan("ambient");
    const ambientContext = api.trace.setSpan(api.ROOT_CONTEXT, ambient);
    const config = telemetry.fromOpenTelemetry({
      trace: api.trace, context: { active: () => ambientContext }, ROOT_CONTEXT: api.ROOT_CONTEXT,
    }, t.propagator, t.provider);
    try {
      const r = await runtime(config);
      await r.harness.onMessage(r.message, r.spec);
      const process = span(t.exporter, "process job.test");
      assert.equal(process.parentSpanContext?.spanId, "00f067aa0ba902b7");
      assert.equal(process.parentSpanContext?.isRemote, true);
      assertResult(r.harness, process, span(t.exporter, "publish sys.job.result"));
      await submitJob(r.harness.nc.connection(), { jobId: "j", topic: "job.test" }, "trace-j", "client", undefined, "", { telemetry: config });
      assert.equal(span(t.exporter, "publish sys.job.submit").parentSpanContext?.spanId, ambient.spanContext().spanId);
    } finally { ambient.end(); }
  });

  it("marks failed publishes TRANSPORT_PUBLISH_FAILED and rethrows", async () => {
    const t = tracing();
    const r = await runtime(t.config);
    r.harness.nc.failure = new Error("nats down");
    await assert.rejects(submitJob(r.harness.nc.connection(), { jobId: "j", topic: "job.test" }, "trace-j", "client", undefined, "", { telemetry: t.config }), /nats down/);
    await assert.rejects(r.harness.onMessage({ ...r.message, headers: undefined }, r.spec), /nats down/);
    const publishes = t.exporter.getFinishedSpans().filter((s) => s.kind === api.SpanKind.PRODUCER);
    assert.deepEqual(publishes.map((s) => [s.name, s.attributes["error.type"]]), [
      ["publish sys.job.submit", "TRANSPORT_PUBLISH_FAILED"],
      ["publish sys.job.result", "TRANSPORT_PUBLISH_FAILED"],
      ["publish sys.job.result", "TRANSPORT_PUBLISH_FAILED"],
    ]);
    const process = span(t.exporter, "process job.test");
    assert.equal(process.attributes["error.type"], "_OTHER");
    assert.equal(process.parentSpanContext, undefined);
    await assertPropagatorFailures();
  });
});
