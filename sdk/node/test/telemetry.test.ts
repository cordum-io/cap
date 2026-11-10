import assert from "node:assert";
import fs from "fs";
import path from "path";
import * as api from "@opentelemetry/api";
import {
  CompositePropagator,
  W3CBaggagePropagator,
  W3CTraceContextPropagator,
} from "@opentelemetry/core";
import {
  AlwaysOnSampler,
  BasicTracerProvider,
  InMemorySpanExporter,
  SimpleSpanProcessor,
  type ReadableSpan,
} from "@opentelemetry/sdk-trace-base";
import { headers, type MsgHdrs } from "nats";
import { telemetry } from "../src";

interface TraceparentCase {
  name: string;
  headers: Record<string, string>;
  verdict: "valid" | "invalid";
  expected: { trace_id: string; parent_id: string; sampled: boolean; tracestate: string } | null;
}

interface AttributeMap {
  input: Record<string, string>;
  span_name: string;
  span_kind: string;
  attributes: Record<string, string | boolean>;
}

interface Vectors {
  schema_version: number;
  traceparent_cases: TraceparentCase[];
  attribute_maps: { publish: AttributeMap; process: AttributeMap };
}

const VECTORS = JSON.parse(
  fs.readFileSync(
    path.resolve(__dirname, "../../../../spec/interop/vectors/trace-context-v1.json"),
    "utf8",
  ),
) as Vectors;

const TRACE_ID = "4bf92f3577b34da6a3ce929d0e0e4736";
const SPAN_ID = "00f067aa0ba902b7";
const VALID_TRACEPARENT = `00-${TRACE_ID}-${SPAN_ID}-01`;
const STALE_TRACEPARENT = "00-11111111111111111111111111111111-2222222222222222-01";
const SUBMIT = "sys.job.submit";
const FIXED = [
  "sys.job.submit",
  "sys.job.result",
  "sys.heartbeat",
  "sys.alert",
  "sys.job.progress",
  "sys.job.cancel",
  "sys.job.dlq",
  "sys.workflow.event",
  "sys.handshake",
  "sys.worker.handshake.challenge",
  "sys.worker.handshake.authenticate",
];

function vector(name: string): TraceparentCase {
  const found = VECTORS.traceparent_cases.find((c) => c.name === name);
  assert.ok(found, `missing vector ${name}`);
  return found;
}

function hdrs(values: Record<string, string>): MsgHdrs {
  const h = headers();
  for (const [key, value] of Object.entries(values)) h.set(key, value);
  return h;
}

function hex2(flags: number): string {
  return flags.toString(16).padStart(2, "0");
}

describe("telemetry (spec/interop/trace-context.md)", () => {
  let exporter: InMemorySpanExporter;
  let provider: BasicTracerProvider;
  let config: telemetry.TelemetryConfig;

  function newProvider(spans: InMemorySpanExporter): BasicTracerProvider {
    return new BasicTracerProvider({
      sampler: new AlwaysOnSampler(),
      spanProcessors: [new SimpleSpanProcessor(spans)],
    });
  }

  function only(): ReadableSpan {
    const spans = exporter.getFinishedSpans();
    assert.strictEqual(spans.length, 1);
    return spans[0];
  }

  beforeEach(() => {
    exporter = new InMemorySpanExporter();
    provider = newProvider(exporter);
    config = telemetry.fromOpenTelemetry(api, new W3CTraceContextPropagator(), provider);
  });

  it("extracts every trace-context-v1 traceparent vector with its expected verdict", () => {
    assert.strictEqual(VECTORS.schema_version, 1);
    assert.strictEqual(VECTORS.traceparent_cases.length, 32);
    let valid = 0;
    for (const c of VECTORS.traceparent_cases) {
      const sc = telemetry.extract(config, hdrs(c.headers));
      if (c.verdict === "invalid") {
        assert.strictEqual(sc, undefined, c.name);
        assert.strictEqual(c.expected, null, c.name);
        continue;
      }
      valid += 1;
      assert.ok(sc, c.name);
      assert.ok(c.expected, c.name);
      assert.deepStrictEqual(
        {
          trace_id: sc.traceId,
          parent_id: sc.spanId,
          sampled: (sc.traceFlags & 1) === 1,
          tracestate: sc.traceState?.serialize() ?? "",
          isRemote: sc.isRemote,
        },
        { ...c.expected, isRemote: true },
        c.name,
      );
    }
    assert.strictEqual(valid, 12);
  });

  it("records the publish attribute map exactly", () => {
    const m = VECTORS.attribute_maps.publish;
    const started = telemetry.startPublish(config, {
      subject: m.input.subject,
      conversationId: m.input.bus_packet_trace_id,
    });
    started.span.end();
    const s = only();
    assert.strictEqual(s.name, m.span_name);
    assert.strictEqual(api.SpanKind[s.kind], m.span_kind);
    assert.strictEqual(s.kind, api.SpanKind.PRODUCER);
    assert.deepStrictEqual({ ...s.attributes }, m.attributes);
    assert.deepStrictEqual(s.links, []);
    assert.deepStrictEqual(s.events, []);
    assert.strictEqual(s.status.code, api.SpanStatusCode.UNSET);
    assert.strictEqual(s.instrumentationScope.name, "cap.telemetry");
  });

  it("records the process attribute map exactly", () => {
    const m = VECTORS.attribute_maps.process;
    const messageId = Uint8Array.from(
      Buffer.from(m.input.signature_metadata_message_id_base64, "base64"),
    );
    assert.strictEqual(messageId.length, 16);
    const started = telemetry.startProcess(config, {
      subject: m.input.subject,
      pool: m.input.subject,
      queueGroup: m.input.queue_group,
      productionVerified: m.input.admission === "cap_production_verified",
      messageId,
      conversationId: m.input.bus_packet_trace_id,
    });
    started.span.end();
    const s = only();
    assert.strictEqual(s.name, m.span_name);
    assert.strictEqual(api.SpanKind[s.kind], m.span_kind);
    assert.strictEqual(s.kind, api.SpanKind.CONSUMER);
    assert.deepStrictEqual({ ...s.attributes }, m.attributes);
    assert.deepStrictEqual(s.links, []);
    assert.deepStrictEqual(s.events, []);
    assert.strictEqual(s.parentSpanContext, undefined);
  });

  it("injects only traceparent for a fresh span and extracts it back; a child forwards tracestate", () => {
    const pub = telemetry.startPublish(config, { subject: SUBMIT });
    const sc = api.trace.getSpanContext(pub.context ?? api.ROOT_CONTEXT);
    assert.ok(sc);
    const h = telemetry.injectHeaders(config, undefined, pub.context);
    assert.ok(h);
    assert.deepStrictEqual(h.keys(), ["traceparent"]);
    assert.strictEqual(h.get("traceparent"), `00-${sc.traceId}-${sc.spanId}-${hex2(sc.traceFlags)}`);
    const back = telemetry.extract(config, h);
    assert.ok(back);
    assert.deepStrictEqual(
      [back.traceId, back.spanId, back.traceFlags, back.isRemote],
      [sc.traceId, sc.spanId, sc.traceFlags, true],
    );
    pub.span.end();

    const creation = telemetry.extract(config, hdrs(vector("valid_sampled_with_tracestate").headers));
    assert.ok(creation);
    const proc = telemetry.startProcess(config, { subject: SUBMIT, creation });
    const out = telemetry.injectHeaders(config, undefined, proc.context);
    assert.ok(out);
    assert.deepStrictEqual(out.keys().sort(), ["traceparent", "tracestate"]);
    assert.strictEqual(out.get("tracestate"), "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE");
    assert.strictEqual(out.get("traceparent").split("-")[1], TRACE_ID);
    proc.span.end();
  });

  it("never injects or extracts baggage, even through a W3C plus Baggage composite", () => {
    const composite = new CompositePropagator({
      propagators: [new W3CTraceContextPropagator(), new W3CBaggagePropagator()],
    });
    let extracted: api.Context | undefined;
    const spy: telemetry.Propagator = {
      inject: (ctx, carrier, setter) => composite.inject(ctx, carrier, setter),
      extract: (ctx, carrier, getter) => {
        extracted = composite.extract(ctx, carrier, getter);
        return extracted;
      },
      fields: () => composite.fields(),
    };
    const withBaggage = telemetry.fromOpenTelemetry(api, spy, provider);
    const pub = telemetry.startPublish(withBaggage, { subject: SUBMIT });
    const ctx = api.propagation.setBaggage(
      pub.context ?? api.ROOT_CONTEXT,
      api.propagation.createBaggage({ tenant: { value: "acme" } }),
    );
    const h = telemetry.injectHeaders(withBaggage, undefined, ctx);
    assert.ok(h);
    assert.deepStrictEqual(h.keys(), ["traceparent"]);
    pub.span.end();

    const inbound = hdrs({ traceparent: VALID_TRACEPARENT, baggage: "tenant=acme" });
    const sc = telemetry.extract(withBaggage, inbound);
    assert.ok(sc);
    assert.strictEqual(sc.traceId, TRACE_ID);
    assert.ok(extracted);
    assert.strictEqual(api.propagation.getBaggage(extracted), undefined);
  });

  it("deletes stale trace headers, keeps others, and returns the same headers object", () => {
    const pub = telemetry.startPublish(config, { subject: SUBMIT });
    const sc = api.trace.getSpanContext(pub.context ?? api.ROOT_CONTEXT);
    assert.ok(sc);
    const h = hdrs({ traceparent: STALE_TRACEPARENT, tracestate: "old=1", "x-keep": "1" });
    assert.strictEqual(telemetry.injectHeaders(config, h, pub.context), h);
    assert.deepStrictEqual(h.keys().sort(), ["traceparent", "x-keep"]);
    assert.strictEqual(h.get("traceparent"), `00-${sc.traceId}-${sc.spanId}-01`);
    assert.strictEqual(h.get("x-keep"), "1");
    pub.span.end();

    const noSpan = hdrs({ traceparent: STALE_TRACEPARENT, tracestate: "old=1", "x-keep": "1" });
    assert.strictEqual(telemetry.injectHeaders(config, noSpan, api.ROOT_CONTEXT), noSpan);
    assert.deepStrictEqual(noSpan.keys(), ["x-keep"]);
    assert.strictEqual(telemetry.injectHeaders(config, undefined, api.ROOT_CONTEXT), undefined);
  });

  it("is a no-op with an undefined config: no headers, no extraction, no spans", () => {
    const h = hdrs({ traceparent: STALE_TRACEPARENT, "x-keep": "1" });
    assert.strictEqual(telemetry.injectHeaders(undefined, h), h);
    assert.deepStrictEqual(h.keys(), ["traceparent", "x-keep"]);
    assert.strictEqual(h.get("traceparent"), STALE_TRACEPARENT);
    assert.strictEqual(telemetry.injectHeaders(undefined, undefined), undefined);
    assert.strictEqual(telemetry.extract(undefined, hdrs({ traceparent: VALID_TRACEPARENT })), undefined);
    assert.strictEqual(telemetry.extract(config, undefined), undefined);

    const given = api.trace.setSpanContext(api.ROOT_CONTEXT, {
      traceId: TRACE_ID,
      spanId: SPAN_ID,
      traceFlags: 1,
    });
    const pub = telemetry.startPublish(undefined, { subject: SUBMIT }, given);
    const proc = telemetry.startProcess(undefined, { subject: SUBMIT, productionVerified: true }, given);
    assert.strictEqual(pub.context, given);
    assert.strictEqual(proc.context, given);
    assert.strictEqual(telemetry.startPublish(undefined, { subject: SUBMIT }).context, undefined);
    assert.strictEqual(telemetry.startProcess(undefined, { subject: SUBMIT }).context, undefined);
    assert.strictEqual(pub.span, proc.span);
    assert.strictEqual(Object.isFrozen(pub.span), true);
    pub.span.fail(telemetry.ErrorType.TRANSPORT_PUBLISH_FAILED);
    pub.span.end();
    proc.span.fail(telemetry.ErrorType.HANDLER_FAILED);
    proc.span.end();
    assert.strictEqual(exporter.getFinishedSpans().length, 0);
  });

  it("extracts from an empty context, never the ambient one, and only exact-case keys", () => {
    const local = provider.getTracer("ambient").startSpan("local");
    const remote = api.trace.setSpanContext(api.ROOT_CONTEXT, {
      traceId: TRACE_ID,
      spanId: SPAN_ID,
      traceFlags: 1,
      isRemote: true,
    });
    for (const ambient of [api.trace.setSpan(api.ROOT_CONTEXT, local), remote]) {
      const scoped = telemetry.fromOpenTelemetry(
        { ...api, context: { active: () => ambient } },
        new W3CTraceContextPropagator(),
        provider,
      );
      assert.strictEqual(telemetry.extract(scoped, headers()), undefined);
      assert.strictEqual(telemetry.extract(scoped, hdrs({ Traceparent: VALID_TRACEPARENT })), undefined);
    }
    local.end();
  });

  it("limits the carrier to exact traceparent and tracestate for get, set and keys", () => {
    const seen: Array<string | undefined> = [];
    let keys: string[] = [];
    const spy: telemetry.Propagator = {
      inject: (_ctx, carrier, setter) => {
        setter.set(carrier, "baggage", "tenant=acme");
        setter.set(carrier, "Traceparent", VALID_TRACEPARENT);
        setter.set(carrier, "x-other", "1");
        setter.set(carrier, "traceparent", VALID_TRACEPARENT);
      },
      extract: (ctx, carrier, getter) => {
        keys = getter.keys(carrier);
        seen.push(getter.get(carrier, "baggage"), getter.get(carrier, "Traceparent"));
        seen.push(getter.get(carrier, "tracestate"), getter.get(carrier, "traceparent"));
        return ctx;
      },
      fields: () => ["traceparent", "tracestate"],
    };
    const scoped = telemetry.fromOpenTelemetry(api, spy, provider);
    const out = telemetry.injectHeaders(scoped, undefined, api.ROOT_CONTEXT);
    assert.ok(out);
    assert.deepStrictEqual(out.keys(), ["traceparent"]);

    const inbound = hdrs({ baggage: "tenant=acme", Traceparent: "x", "x-other": "1", traceparent: VALID_TRACEPARENT });
    assert.strictEqual(telemetry.extract(scoped, inbound), undefined);
    assert.deepStrictEqual(keys, ["traceparent"]);
    assert.deepStrictEqual(seen, [undefined, undefined, undefined, VALID_TRACEPARENT]);
  });

  it("rejects a valid span context that the propagator did not mark remote", () => {
    const localOnly: telemetry.Propagator = {
      inject: () => undefined,
      extract: (ctx) => api.trace.setSpanContext(ctx, { traceId: TRACE_ID, spanId: SPAN_ID, traceFlags: 1 }),
      fields: () => [],
    };
    const scoped = telemetry.fromOpenTelemetry(api, localOnly, provider);
    assert.strictEqual(telemetry.extract(scoped, hdrs({ traceparent: VALID_TRACEPARENT })), undefined);
  });

  it("links the creation context and parents on it only without an ambient span", () => {
    const creation = telemetry.extract(config, hdrs(vector("valid_sampled_with_tracestate").headers));
    assert.ok(creation);
    const linked = (s: ReadableSpan) => s.links.map((l) => [l.context.traceId, l.context.spanId]);

    const rootLevel = telemetry.startProcess(config, { subject: SUBMIT, creation });
    rootLevel.span.end();
    const first = only();
    assert.strictEqual(first.parentSpanContext?.spanId, creation.spanId);
    assert.strictEqual(first.spanContext().traceId, creation.traceId);
    assert.deepStrictEqual(linked(first), [[creation.traceId, creation.spanId]]);
    assert.ok(rootLevel.context);
    assert.strictEqual(api.trace.getSpanContext(rootLevel.context)?.spanId, first.spanContext().spanId);

    exporter.reset();
    const ambient = provider.getTracer("ambient").startSpan("ambient");
    const base = api.trace.setSpan(api.ROOT_CONTEXT, ambient);
    const nested = telemetry.startProcess(config, { subject: SUBMIT, creation }, base);
    nested.span.end();
    const second = only();
    assert.strictEqual(second.parentSpanContext?.spanId, ambient.spanContext().spanId);
    assert.strictEqual(second.spanContext().traceId, ambient.spanContext().traceId);
    assert.deepStrictEqual(linked(second), [[creation.traceId, creation.spanId]]);
    assert.ok(nested.context);
    assert.strictEqual(api.trace.getSpanContext(nested.context)?.spanId, second.spanContext().spanId);
    ambient.end();
  });

  it("starts a root span with no links for an invalid or absent creation context", () => {
    const invalid = { traceId: "0".repeat(32), spanId: "0".repeat(16), traceFlags: 1, isRemote: true };
    telemetry.startProcess(config, { subject: SUBMIT, creation: invalid }).span.end();
    telemetry.startProcess(config, { subject: SUBMIT }).span.end();
    const spans = exporter.getFinishedSpans();
    assert.strictEqual(spans.length, 2);
    for (const s of spans) {
      assert.strictEqual(s.parentSpanContext, undefined);
      assert.deepStrictEqual(s.links, []);
    }
  });

  it("names only fixed subjects and configured literal pools; inboxes are temporary and anonymous", () => {
    assert.strictEqual(telemetry.FIXED_SUBJECTS.size, 11);
    assert.deepStrictEqual([...telemetry.FIXED_SUBJECTS].sort(), [...FIXED].sort());
    const cases: Array<[telemetry.Process, string, Record<string, string | boolean>]> = [
      [{ subject: "job.gpu", pool: "job.gpu" }, "process job.gpu", { "messaging.destination.name": "job.gpu" }],
      [{ subject: "job.gpu" }, "process", {}],
      [{ subject: "job.cpu", pool: "job.gpu" }, "process", {}],
      [{ subject: "job.*", pool: "job.*" }, "process", {}],
      [{ subject: "job.>", pool: "job.>" }, "process", {}],
      [{ subject: "job.a b", pool: "job.a b" }, "process", {}],
      [{ subject: "job.", pool: "job." }, "process", {}],
      [{ subject: "sys.worker.handshake" }, "process", {}],
      [{ subject: "orders.created" }, "process", {}],
      [
        { subject: "_INBOX.abc", replyInbox: true },
        "process",
        { "messaging.destination.temporary": true, "messaging.destination.anonymous": true },
      ],
      [
        { subject: "sys.job.result", replyInbox: true },
        "process",
        { "messaging.destination.temporary": true, "messaging.destination.anonymous": true },
      ],
    ];
    for (const subject of FIXED) {
      cases.push([{ subject }, `process ${subject}`, { "messaging.destination.name": subject }]);
    }
    const base = { "messaging.system": "nats", "messaging.operation.name": "process", "messaging.operation.type": "process" };
    for (const [input, name, extra] of cases) {
      exporter.reset();
      telemetry.startProcess(config, input).span.end();
      const s = only();
      assert.strictEqual(s.name, name, input.subject);
      assert.deepStrictEqual({ ...s.attributes }, { ...base, ...extra }, input.subject);
    }

    exporter.reset();
    telemetry.startPublish(config, { subject: "sys.worker.handshake" }).span.end();
    telemetry.startPublish(config, { subject: "job.gpu", pool: "job.gpu" }).span.end();
    telemetry.startPublish(config, { subject: "_INBOX.x", replyInbox: true }).span.end();
    const [legacy, pool, inbox] = exporter.getFinishedSpans();
    assert.deepStrictEqual([legacy.name, pool.name, inbox.name], ["publish", "publish job.gpu", "publish"]);
    assert.deepStrictEqual({ ...inbox.attributes }, {
      "messaging.system": "nats",
      "messaging.operation.name": "publish",
      "messaging.operation.type": "send",
      "messaging.destination.temporary": true,
      "messaging.destination.anonymous": true,
    });
  });

  it("sets message and conversation ids only for production-verified packets", () => {
    const id16 = Uint8Array.from({ length: 16 }, (_v, i) => i * 17);
    const input = { subject: "job.gpu", pool: "job.gpu", messageId: id16, conversationId: "conv-1" };
    const base = {
      "messaging.system": "nats",
      "messaging.operation.name": "process",
      "messaging.operation.type": "process",
      "messaging.destination.name": "job.gpu",
    };
    const attrsOf = (input: telemetry.Process) => {
      exporter.reset();
      telemetry.startProcess(config, input).span.end();
      return { ...only().attributes };
    };
    assert.deepStrictEqual(attrsOf(input), base);
    assert.deepStrictEqual(attrsOf({ ...input, productionVerified: false }), base);
    assert.deepStrictEqual(attrsOf({ ...input, productionVerified: true }), {
      ...base,
      "messaging.message.id": "00112233445566778899aabbccddeeff",
      "messaging.message.conversation_id": "conv-1",
    });
    assert.deepStrictEqual(attrsOf({ ...input, productionVerified: true, messageId: id16.subarray(0, 15) }), {
      ...base,
      "messaging.message.conversation_id": "conv-1",
    });
    assert.deepStrictEqual(attrsOf({ ...input, productionVerified: true, conversationId: "" }), {
      ...base,
      "messaging.message.id": "00112233445566778899aabbccddeeff",
    });
    assert.deepStrictEqual(attrsOf({ ...input, durable: "cap-workers" }), {
      ...base,
      "messaging.destination.subscription.name": "cap-workers",
    });

    exporter.reset();
    telemetry.startPublish(config, { subject: SUBMIT, conversationId: "" }).span.end();
    assert.deepStrictEqual({ ...only().attributes }, {
      "messaging.system": "nats",
      "messaging.operation.name": "publish",
      "messaging.operation.type": "send",
      "messaging.destination.name": SUBMIT,
    });
  });

  it("maps fail() to the closed error.type set with ERROR status and no events", () => {
    const E = telemetry.ErrorType;
    const cases: Array<["publish" | "process", string, string]> = [
      ["publish", E.TRANSPORT_PUBLISH_FAILED, "TRANSPORT_PUBLISH_FAILED"],
      ["publish", E.OTHER, "_OTHER"],
      ["publish", E.HANDLER_FAILED, "_OTHER"],
      ["process", "boom: secret detail", "_OTHER"],
      ["process", E.ADMISSION_REJECTED, "ADMISSION_REJECTED"],
      ["process", E.HANDLER_FAILED, "HANDLER_FAILED"],
      ["process", E.PROTOCOL_SIGNATURE_INVALID, "PROTOCOL_SIGNATURE_INVALID"],
      ["process", E.TRANSPORT_PUBLISH_FAILED, "_OTHER"],
    ];
    for (const [kind, input, want] of cases) {
      exporter.reset();
      const started =
        kind === "publish"
          ? telemetry.startPublish(config, { subject: SUBMIT })
          : telemetry.startProcess(config, { subject: SUBMIT });
      started.span.fail(input);
      started.span.end();
      const s = only();
      assert.strictEqual(s.attributes["error.type"], want, `${kind} ${input}`);
      assert.deepStrictEqual(s.status, { code: api.SpanStatusCode.ERROR, message: want });
      assert.strictEqual(s.events.length, 0);
    }
    assert.deepStrictEqual(Object.values(E).sort(), [
      "ADMISSION_REJECTED",
      "HANDLER_FAILED",
      "PROTOCOL_MALFORMED_PACKET",
      "PROTOCOL_SIGNATURE_INVALID",
      "PROTOCOL_SIGNATURE_MISSING",
      "PROTOCOL_UNKNOWN_PAYLOAD",
      "PROTOCOL_VERSION_MISMATCH",
      "TRANSPORT_PUBLISH_FAILED",
      "_OTHER",
    ]);
  });

  it("uses the global tracer provider when none is given", () => {
    const globalSpans = new InMemorySpanExporter();
    try {
      assert.strictEqual(api.trace.setGlobalTracerProvider(newProvider(globalSpans)), true);
      const global = telemetry.fromOpenTelemetry(api, new W3CTraceContextPropagator());
      telemetry.startPublish(global, { subject: SUBMIT }).span.end();
      const spans = globalSpans.getFinishedSpans();
      assert.strictEqual(spans.length, 1);
      assert.strictEqual(spans[0].instrumentationScope.name, "cap.telemetry");
      assert.strictEqual(exporter.getFinishedSpans().length, 0);
    } finally {
      api.trace.disable();
    }
  });

  it("fails closed at construction when a required method is missing", () => {
    const w3c = new W3CTraceContextPropagator();
    assert.throws(
      () => telemetry.fromOpenTelemetry(api, undefined as unknown as telemetry.Propagator, provider),
      { name: "TypeError", message: "propagator must provide inject()" },
    );
    const noExtract = { inject: () => undefined, fields: () => [] } as unknown as telemetry.Propagator;
    assert.throws(() => telemetry.fromOpenTelemetry(api, noExtract, provider), {
      name: "TypeError",
      message: "propagator must provide extract()",
    });
    const noRoot = { ...api, ROOT_CONTEXT: undefined } as unknown as telemetry.OpenTelemetryApi;
    assert.throws(() => telemetry.fromOpenTelemetry(noRoot, w3c, provider), {
      name: "TypeError",
      message: "api.ROOT_CONTEXT must provide getValue()",
    });
    const noProvider = {} as unknown as telemetry.TracerProvider;
    assert.throws(() => telemetry.fromOpenTelemetry(api, w3c, noProvider), {
      name: "TypeError",
      message: "tracerProvider must provide getTracer()",
    });
  });
});
