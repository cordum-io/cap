import assert from "node:assert/strict";
import { createPublicKey, randomUUID } from "node:crypto";
import * as api from "@opentelemetry/api";
import { W3CTraceContextPropagator } from "@opentelemetry/core";
import { AlwaysOnSampler, BasicTracerProvider, InMemorySpanExporter, SimpleSpanProcessor, type ReadableSpan } from "@opentelemetry/sdk-trace-base";
import { headers, type Msg, type MsgHdrs, type NatsConnection } from "nats";
import { connectNATS } from "../../src/bus";
import { submitJob } from "../../src/client";
import { Agent, InMemoryBlobStore, type AgentOptions } from "../../src/runtime";
import { InMemoryReplayStore } from "../../src/production-replay";
import { SUBJECT_RESULT, SUBJECT_SUBMIT, SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, SUBJECT_WORKER_HANDSHAKE_CHALLENGE } from "../../src/protos";
import * as telemetry from "../../src/telemetry";
import { busPacketType, signedRawPacket } from "../production-admission-support";
import { privateKeyPem } from "../runtime-trust-packet-support";
import { createTrustFixture, FakeTrustRequester, type TrustFixture } from "../worker-trust-runtime-support";
import { NatsServer, reservePort, waitForCondition, withTimeout } from "./nats-server";

const CREATION = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01";
class CountingPropagator extends W3CTraceContextPropagator {
  extracts = 0;
  override extract(context: api.Context, carrier: MsgHdrs, getter: api.TextMapGetter<MsgHdrs>): api.Context {
    this.extracts += 1;
    return super.extract(context, carrier, getter);
  }
}
function tracing() {
  const exporter = new InMemorySpanExporter();
  const provider = new BasicTracerProvider({ sampler: new AlwaysOnSampler(), spanProcessors: [new SimpleSpanProcessor(exporter)] });
  const propagator = new CountingPropagator();
  return { exporter, provider, propagator, config: telemetry.fromOpenTelemetry(api, propagator, provider) };
}
function carrier(value: string): MsgHdrs {
  const h = headers();
  h.set("traceparent", value);
  return h;
}
function findSpan(exporter: InMemorySpanExporter, name: string): ReadableSpan {
  const span = exporter.getFinishedSpans().find((s) => s.name === name);
  assert.ok(span, `missing finished span ${name}`);
  return span;
}

describe("Node trace propagation over real NATS", function () {
  this.timeout(45_000);
  let server: NatsServer;
  let client: NatsConnection;
  let cleanup: Array<() => Promise<void> | void>;
  let errors: unknown[];

  before(async () => {
    server = new NatsServer(await reservePort());
    await server.start();
    client = await connectNATS({ url: server.url });
  });
  beforeEach(() => { cleanup = []; errors = []; });
  afterEach(async () => {
    try {
      for (const close of cleanup.reverse()) {
        try { await close(); } catch (error) { errors.push(error); }
      }
      assert.deepEqual(errors, []);
    } finally { api.trace.disable(); }
  });
  after(async () => {
    try { if (client) await client.drain(); }
    finally { if (server) await server.stop(false); }
  });

  function collect(subject: string): Msg[] {
    const messages: Msg[] = [];
    const sub = client.subscribe(subject, { callback(error, message) {
      if (error) errors.push(error);
      else messages.push(message);
    } });
    cleanup.push(() => sub.unsubscribe());
    return messages;
  }
  async function relaySubmits(topic: string, extra: Record<string, string> = {}): Promise<Msg[]> {
    const jobs = collect(topic);
    const sub = client.subscribe(SUBJECT_SUBMIT, { callback(error, msg) {
      if (error) { errors.push(error); return; }
      let h = msg.headers;
      for (const [key, value] of Object.entries(extra)) {
        h ??= headers();
        h.set(key, value);
      }
      if (h) client.publish(topic, msg.data, { headers: h });
      else client.publish(topic, msg.data);
    } });
    cleanup.push(() => sub.unsubscribe());
    await client.flush();
    return jobs;
  }
  async function startAgent(topic: string, config?: telemetry.TelemetryConfig, options: AgentOptions = {}) {
    const store = new InMemoryBlobStore();
    const seen: Array<api.SpanContext | undefined> = [];
    const handled: string[] = [];
    const agent = new Agent({
      natsUrl: server.url, senderId: "worker-a", store, heartbeatInterval: 60_000, telemetry: config,
      logger: { info() {}, warn() {}, error(message, fields) { errors.push({ message, fields }); } }, ...options,
    });
    cleanup.push(() => agent.close());
    agent.job(topic, async (ctx) => {
      seen.push(ctx.otelContext ? api.trace.getSpanContext(ctx.otelContext) : undefined);
      handled.push(ctx.jobId);
      return { ok: true };
    });
    for (const id of ["job-1", "job-2"]) await store.set(`ctx:${id}`, Buffer.from("{}"));
    await withTimeout(agent.start(), 8_000, "Agent.start timed out");
    return { seen, handled };
  }
  async function settled(exporter: InMemorySpanExporter, count: number, results: Msg[], resultCount: number): Promise<void> {
    await waitForCondition(() => exporter.getFinishedSpans().length >= count && results.length >= resultCount,
      5_000, "spans or results did not settle");
    assert.equal(exporter.getFinishedSpans().length, count);
    assert.equal(results.length, resultCount);
  }
  async function scheduler(fixture: TrustFixture): Promise<void> {
    const connection = await connectNATS({ url: server.url, name: "telemetry-scheduler" });
    cleanup.push(() => connection.drain());
    const requester = new FakeTrustRequester(fixture);
    for (const subject of [SUBJECT_WORKER_HANDSHAKE_CHALLENGE, SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE]) {
      const sub = connection.subscribe(subject, { callback(error, msg) {
        if (error) { errors.push(error); return; }
        void requester.request(subject, msg.data, { timeout: 1_000 }).then((response) => {
          if (!msg.respond(response.data)) throw new Error("handshake response failed");
        }).catch((cause: unknown) => errors.push(cause));
      } });
      cleanup.push(() => sub.unsubscribe());
    }
    await connection.flush();
  }
  function productionOptions(fixture: TrustFixture): AgentOptions {
    return {
      senderId: fixture.config.workerId, privateKey: privateKeyPem(fixture),
      productionTrust: {
        audience: "job.test", tenant: fixture.config.tenantId, sender: fixture.config.expectedSchedulerId,
        publicKeys: { "production-key": createPublicKey(fixture.schedulerPrivateKey).export({ type: "spki", format: "pem" }).toString() },
      },
      replayStore: new InMemoryReplayStore(),
      workerTrust: { mode: "enforce", config: fixture.config, timeoutMs: 1_000, retries: 2, renewMinIntervalMs: 30_000 },
    };
  }

  it("carries one trace from submit through process to result", async () => {
    const t = tracing();
    const topic = `job.telemetry.${randomUUID()}`;
    const jobs = await relaySubmits(topic, { baggage: "k=v" });
    const results = collect(SUBJECT_RESULT);
    const r = await startAgent(topic, t.config);
    await submitJob(client, { jobId: "job-1", topic, contextPtr: "redis://ctx:job-1" }, "trace-job-1", "client", undefined, "", { telemetry: t.config });
    await settled(t.exporter, 3, results, 1);
    const submit = findSpan(t.exporter, "publish sys.job.submit");
    const process = findSpan(t.exporter, `process ${topic}`);
    const publish = findSpan(t.exporter, "publish sys.job.result");
    assert.deepEqual([submit.kind, process.kind, publish.kind], [api.SpanKind.PRODUCER, api.SpanKind.CONSUMER, api.SpanKind.PRODUCER]);
    assert.equal(jobs.length, 1);
    assert.equal(jobs[0].headers?.get("traceparent").split("-")[1], submit.spanContext().traceId);
    assert.deepEqual(r.seen, [process.spanContext()]);
    assert.equal(process.parentSpanContext?.spanId, submit.spanContext().spanId);
    assert.equal(process.parentSpanContext?.isRemote, true);
    assert.deepEqual(process.links.map(({ context }) => [context.traceId, context.spanId, context.traceFlags, context.isRemote]),
      [[submit.spanContext().traceId, submit.spanContext().spanId, submit.spanContext().traceFlags, true]]);
    assert.equal(process.links[0].context.traceState?.serialize(), submit.spanContext().traceState?.serialize());
    assert.equal(publish.spanContext().traceId, submit.spanContext().traceId);
    assert.equal(publish.parentSpanContext?.spanId, process.spanContext().spanId);
    assert.equal(results[0].headers?.get("traceparent"), `00-${submit.spanContext().traceId}-${publish.spanContext().spanId}-01`);
    assert.deepEqual(results[0].headers?.keys(), ["traceparent"]);
    assert.equal(publish.attributes["messaging.message.conversation_id"], undefined);
    assert.equal(submit.attributes["messaging.message.conversation_id"], "trace-job-1");
  });

  it("runs the handler under a new root for an invalid traceparent", async () => {
    const t = tracing();
    const topic = `job.telemetry.${randomUUID()}`;
    const results = collect(SUBJECT_RESULT);
    const r = await startAgent(topic, t.config);
    const type = await busPacketType();
    for (const [index, value] of ["00-00000000000000000000000000000000-0000000000000000-01", "garbage"].entries()) {
      const id = `job-${index + 1}`;
      const data = type.encode(type.fromObject({
        traceId: `trace-${id}`, senderId: "client", protocolVersion: 1,
        createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
        jobRequest: { jobId: id, topic, contextPtr: `redis://ctx:${id}` },
      })).finish();
      client.publish(topic, data, { headers: carrier(value) });
    }
    await settled(t.exporter, 4, results, 2);
    const processes = t.exporter.getFinishedSpans().filter((s) => s.kind === api.SpanKind.CONSUMER);
    assert.equal(processes.length, 2);
    for (const process of processes) {
      assert.equal(process.parentSpanContext, undefined);
      assert.deepEqual(process.links, []);
    }
    assert.deepEqual(r.seen.map((s) => s?.spanId).sort(), processes.map((s) => s.spanContext().spanId).sort());
    assert.equal(new Set(processes.map((s) => s.spanContext().traceId)).size, 2);
  });

  it("sends no headers and records nothing without telemetry", async () => {
    const t = tracing();
    api.trace.disable();
    assert.equal(api.trace.setGlobalTracerProvider(t.provider), true);
    const topic = `job.telemetry.${randomUUID()}`;
    const jobs = await relaySubmits(topic);
    const results = collect(SUBJECT_RESULT);
    const r = await startAgent(topic);
    await submitJob(client, { jobId: "job-1", topic, contextPtr: "redis://ctx:job-1" }, "trace-job-1", "client");
    await settled(t.exporter, 0, results, 1);
    assert.equal(jobs.length, 1);
    assert.equal(jobs[0].headers, undefined);
    assert.equal(results[0].headers, undefined);
    assert.deepEqual(r.seen, [undefined]);
  });

  it("starts no span and never extracts for a packet rejected at production admission", async () => {
    const t = tracing();
    const fixture = createTrustFixture("worker-telemetry");
    await scheduler(fixture);
    const results = collect(SUBJECT_RESULT);
    const r = await startAgent("job.test", t.config, productionOptions(fixture));
    for (const n of [1, 2]) {
      const data = await signedRawPacket({
        audience: "job.test", keyId: "production-key", messageId: Buffer.alloc(16, n),
        privateKeyPem: fixture.schedulerPrivateKey.export({ type: "pkcs8", format: "pem" }).toString(),
        tenantId: fixture.config.tenantId, senderId: fixture.config.expectedSchedulerId, contextPtr: "redis://ctx:job-1",
      });
      if (n === 1) data[data.length - 1] ^= 0xff;
      client.publish("job.test", data, { headers: carrier(CREATION) });
    }
    await settled(t.exporter, 2, results, 1);
    assert.deepEqual(r.handled, ["job-1"]);
    assert.equal(t.propagator.extracts, 1);
    const process = findSpan(t.exporter, "process job.test");
    assert.equal(process.attributes["messaging.message.id"], Buffer.alloc(16, 2).toString("hex"));
    assert.equal(process.attributes["messaging.message.conversation_id"], "trace-1");
    assert.equal(process.parentSpanContext?.traceId, CREATION.split("-")[1]);
  });
});
