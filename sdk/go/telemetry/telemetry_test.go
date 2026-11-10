package telemetry

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const vectorTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

type vectorFile struct {
	SchemaVersion    int                     `json:"schema_version"`
	Contract         string                  `json:"contract"`
	Description      string                  `json:"description"`
	Sources          json.RawMessage         `json:"sources"`
	TraceparentCases []traceparentCase       `json:"traceparent_cases"`
	AttributeMaps    map[string]attributeMap `json:"attribute_maps"`
}

type traceparentCase struct {
	Name     string            `json:"name"`
	Headers  map[string]string `json:"headers"`
	Verdict  string            `json:"verdict"`
	Expected *expectedContext  `json:"expected"`
}

type expectedContext struct {
	TraceID    string `json:"trace_id"`
	ParentID   string `json:"parent_id"`
	Sampled    bool   `json:"sampled"`
	Tracestate string `json:"tracestate"`
}

type attributeMap struct {
	Input      map[string]string `json:"input"`
	SpanName   string            `json:"span_name"`
	SpanKind   string            `json:"span_kind"`
	Attributes map[string]string `json:"attributes"`
}

func loadVectors(t *testing.T) vectorFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "interop", "vectors", "trace-context-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var v vectorFile
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode vectors: %v", err)
	}
	if v.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", v.SchemaVersion)
	}
	return v
}

// specSubjects returns the backticked sys.* subjects of the spec/09 subject table.
func specSubjects(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "09-transport-profile.md"))
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("^\\| `(sys\\.[^`]+)` \\|")
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if m := row.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	if len(out) == 0 || len(slices.Compact(slices.Clone(out))) != len(out) {
		t.Fatalf("spec/09 subject table parse: %v", out)
	}
	return out
}

func newRecorder(t *testing.T) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, exp
}

func setGlobalTracerProvider(t *testing.T, tp trace.TracerProvider) {
	t.Helper()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
}

func setGlobalPropagator(t *testing.T, p propagation.TextMapPropagator) {
	t.Helper()
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(p)
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })
}

func onlySpan(t *testing.T, exp *tracetest.InMemoryExporter) tracetest.SpanStub {
	t.Helper()
	spans := exp.GetSpans()
	exp.Reset()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	return spans[0]
}

func attrMap(t *testing.T, s tracetest.SpanStub) map[string]attribute.Value {
	t.Helper()
	m := make(map[string]attribute.Value, len(s.Attributes))
	for _, kv := range s.Attributes {
		if _, dup := m[string(kv.Key)]; dup {
			t.Fatalf("%s: duplicate attribute %s", s.Name, kv.Key)
		}
		m[string(kv.Key)] = kv.Value
	}
	return m
}

func assertStringAttrs(t *testing.T, s tracetest.SpanStub, want map[string]string) {
	t.Helper()
	got := attrMap(t, s)
	for k, w := range want {
		v, ok := got[k]
		if !ok || v.Type() != attribute.STRING || v.AsString() != w {
			t.Errorf("%s: attribute %s = %v (present %v), want STRING %q", s.Name, k, v.Emit(), ok, w)
		}
	}
	for k, v := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: unexpected attribute %s = %v", s.Name, k, v.Emit())
		}
	}
}

func traceparentOf(sc trace.SpanContext) string {
	return "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-" + sc.TraceFlags().String()
}

func startOp(cfg *Config, ctx context.Context, op string, p Process) (context.Context, *Span) {
	if op == "publish" {
		return cfg.StartPublish(ctx, Publish{Subject: p.Subject, Pool: p.Pool, ReplyInbox: p.ReplyInbox, ConversationID: p.ConversationID})
	}
	return cfg.StartProcess(ctx, p)
}

func TestExtractTraceContextVectors(t *testing.T) {
	v := loadVectors(t)
	if len(v.TraceparentCases) == 0 {
		t.Fatal("no traceparent cases")
	}
	cfg := &Config{}
	seen := map[string]bool{}
	valid, invalid := 0, 0
	for _, tc := range v.TraceparentCases {
		if tc.Name == "" || seen[tc.Name] {
			t.Fatalf("empty or duplicate case name %q", tc.Name)
		}
		seen[tc.Name] = true
		h := nats.Header{}
		for k, val := range tc.Headers {
			if k != "traceparent" && k != "tracestate" {
				t.Fatalf("%s: unexpected header key %q", tc.Name, k)
			}
			h[k] = []string{val}
		}
		sc := cfg.Extract(h)
		switch tc.Verdict {
		case "valid":
			valid++
			checkValidCase(t, cfg, tc, sc)
		case "invalid":
			invalid++
			if tc.Expected != nil {
				t.Fatalf("%s: invalid case carries expected", tc.Name)
			}
			if sc.IsValid() {
				t.Errorf("%s: extracted a valid context from an invalid case", tc.Name)
			}
		default:
			t.Fatalf("%s: unknown verdict %q", tc.Name, tc.Verdict)
		}
	}
	t.Logf("vectors: %d cases, %d valid, %d invalid", len(v.TraceparentCases), valid, invalid)
	if valid+invalid != len(v.TraceparentCases) || valid == 0 || invalid == 0 {
		t.Fatalf("verdict counts: %d valid, %d invalid of %d", valid, invalid, len(v.TraceparentCases))
	}
	if cfg.Extract(nats.Header{"Traceparent": {vectorTraceparent}}).IsValid() {
		t.Error("a wrong-case Traceparent key was extracted")
	}
	if cfg.Extract(nats.Header{"traceparent": {}, "tracestate": {}}).IsValid() || cfg.Extract(nil).IsValid() {
		t.Error("an empty or nil header block was extracted")
	}
	repeated := cfg.Extract(nats.Header{"traceparent": {vectorTraceparent, "00-ffffffffffffffffffffffffffffffff-ffffffffffffffff-01"}})
	if repeated.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("a repeated traceparent gave %v, want the first value", repeated)
	}
}

func checkValidCase(t *testing.T, cfg *Config, tc traceparentCase, sc trace.SpanContext) {
	t.Helper()
	e := tc.Expected
	if e == nil {
		t.Fatalf("%s: valid case without expected", tc.Name)
	}
	if !sc.IsValid() || !sc.IsRemote() {
		t.Errorf("%s: valid=%v remote=%v, want a valid remote context", tc.Name, sc.IsValid(), sc.IsRemote())
		return
	}
	if sc.TraceID().String() != e.TraceID || sc.SpanID().String() != e.ParentID {
		t.Errorf("%s: ids %s/%s, want %s/%s", tc.Name, sc.TraceID(), sc.SpanID(), e.TraceID, e.ParentID)
	}
	if sc.IsSampled() != e.Sampled {
		t.Errorf("%s: sampled = %v, want %v", tc.Name, sc.IsSampled(), e.Sampled)
	}
	if got := sc.TraceState().String(); got != e.Tracestate {
		t.Errorf("%s: tracestate = %q, want %q", tc.Name, got, e.Tracestate)
	}
	out := cfg.InjectHeaders(trace.ContextWithRemoteSpanContext(context.Background(), sc), nil)
	if !strings.HasPrefix(out.Get("traceparent"), "00-") || !cfg.Extract(out).Equal(sc) {
		t.Errorf("%s: re-inject gave %v, which does not round-trip", tc.Name, out)
	}
}

func TestSpanAttributeMapsMatchVectors(t *testing.T) {
	v := loadVectors(t)
	if len(v.AttributeMaps) != 2 {
		t.Fatalf("attribute_maps has %d entries, want publish and process", len(v.AttributeMaps))
	}
	kinds := map[string]trace.SpanKind{"PRODUCER": trace.SpanKindProducer, "CONSUMER": trace.SpanKindConsumer}
	tp, exp := newRecorder(t)
	cfg := &Config{TracerProvider: tp}
	for name, m := range v.AttributeMaps {
		var span *Span
		var direct trace.SpanKind
		switch name {
		case "publish":
			_, span = cfg.StartPublish(context.Background(), publishInput(t, m.Input))
			direct = trace.SpanKindProducer
		case "process":
			_, span = cfg.StartProcess(context.Background(), processInput(t, m.Input))
			direct = trace.SpanKindConsumer
		default:
			t.Fatalf("unknown attribute map %q", name)
		}
		span.End()
		s := onlySpan(t, exp)
		want, ok := kinds[m.SpanKind]
		if !ok || s.SpanKind != want || s.SpanKind != direct {
			t.Errorf("%s: kind %v, want %s (%v)", name, s.SpanKind, m.SpanKind, direct)
		}
		if s.Name != m.SpanName {
			t.Errorf("%s: name %q, want %q", name, s.Name, m.SpanName)
		}
		assertStringAttrs(t, s, m.Attributes)
		if s.Status.Code != codes.Unset || len(s.Events) != 0 || len(s.Links) != 0 {
			t.Errorf("%s: status %v, %d events, %d links", name, s.Status, len(s.Events), len(s.Links))
		}
	}
}

func publishInput(t *testing.T, in map[string]string) Publish {
	t.Helper()
	var p Publish
	for k, v := range in {
		switch k {
		case "subject":
			p.Subject = v
		case "bus_packet_trace_id":
			p.ConversationID = v
		default:
			t.Fatalf("unknown publish input %q", k)
		}
	}
	return p
}

func processInput(t *testing.T, in map[string]string) Process {
	t.Helper()
	var p Process
	for k, v := range in {
		switch k {
		case "subject":
			// The vector participant consumes its configured pool, job.gpu.
			p.Subject, p.Pool = v, v
		case "queue_group":
			p.QueueGroup = v
		case "admission":
			if v != "cap_production_verified" {
				t.Fatalf("unknown admission %q", v)
			}
			p.ProductionVerified = true
		case "bus_packet_trace_id":
			p.ConversationID = v
		case "signature_metadata_message_id_base64":
			id, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				t.Fatal(err)
			}
			p.MessageID = id
		default:
			t.Fatalf("unknown process input %q", k)
		}
	}
	return p
}

func TestFixedSubjectsMatchSpec09(t *testing.T) {
	want := specSubjects(t)
	got := slices.Sorted(slices.Values(fixedSubjects()))
	if !slices.Equal(got, want) {
		t.Fatalf("fixed subjects %v, want spec/09 %v", got, want)
	}
	t.Logf("spec/09 fixed sys.* subjects: %d", len(want))
}

type destCase struct {
	subject, pool string
	reply, named  bool
}

func TestDestinationNaming(t *testing.T) {
	var cases []destCase
	for _, s := range specSubjects(t) {
		cases = append(cases, destCase{subject: s, named: true})
	}
	cases = append(cases,
		destCase{subject: "sys.worker.handshake"},
		destCase{subject: "sys.worker.handshake.renew"},
		destCase{subject: "sys.job.submit.extra"},
		destCase{subject: "_INBOX.abc"},
		destCase{subject: "job.gpu", pool: "job.gpu", named: true},
		destCase{subject: "job.gpu"},
		destCase{subject: "job.gpu", pool: "job.*"},
		destCase{subject: "job.gpu", pool: "job.cpu"},
		destCase{subject: "job.", pool: "job."},
		destCase{subject: "job.*", pool: "job.*"},
		destCase{subject: "job.>", pool: "job.>"},
		destCase{subject: "job.a b", pool: "job.a b"},
		destCase{subject: "sys.alert", pool: "sys.alert", named: true},
		destCase{subject: "sys.job.submit", reply: true},
		destCase{subject: "_INBOX.abc", reply: true},
	)
	tp, exp := newRecorder(t)
	cfg := &Config{TracerProvider: tp}
	for _, c := range cases {
		for _, op := range []string{"publish", "process"} {
			_, span := startOp(cfg, context.Background(), op, Process{Subject: c.subject, Pool: c.pool, ReplyInbox: c.reply})
			span.End()
			checkDestination(t, onlySpan(t, exp), op, c)
		}
	}
}

func checkDestination(t *testing.T, s tracetest.SpanStub, op string, c destCase) {
	t.Helper()
	got := attrMap(t, s)
	name, hasName := got["messaging.destination.name"]
	tmp, hasTmp := got["messaging.destination.temporary"]
	anon, hasAnon := got["messaging.destination.anonymous"]
	switch {
	case c.reply:
		if s.Name != op || hasName || !hasTmp || !hasAnon || tmp.Type() != attribute.BOOL ||
			!tmp.AsBool() || anon.Type() != attribute.BOOL || !anon.AsBool() {
			t.Errorf("%s %+v: reply inbox span %q attrs %v", op, c, s.Name, s.Attributes)
		}
	case c.named:
		if s.Name != op+" "+c.subject || !hasName || name.Type() != attribute.STRING ||
			name.AsString() != c.subject || hasTmp || hasAnon {
			t.Errorf("%s %+v: named span %q attrs %v", op, c, s.Name, s.Attributes)
		}
	default:
		if s.Name != op || hasName || hasTmp || hasAnon {
			t.Errorf("%s %+v: unnamed span %q attrs %v", op, c, s.Name, s.Attributes)
		}
	}
}

func TestProcessAttributesByAdmission(t *testing.T) {
	id16 := bytes.Repeat([]byte{0xab}, 16)
	base := func(extra map[string]string) map[string]string {
		m := map[string]string{"messaging.system": "nats", "messaging.operation.name": "process", "messaging.operation.type": "process"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name string
		in   Process
		want map[string]string
	}{
		{"legacy", Process{Subject: "job.gpu", Pool: "job.gpu", QueueGroup: "job.gpu", MessageID: id16, ConversationID: "conv-1"},
			base(map[string]string{"messaging.destination.name": "job.gpu", "messaging.consumer.group.name": "job.gpu"})},
		{"production 15-byte id", Process{Subject: "sys.job.result", ProductionVerified: true, MessageID: id16[:15], ConversationID: "conv-1"},
			base(map[string]string{"messaging.destination.name": "sys.job.result", "messaging.message.conversation_id": "conv-1"})},
		{"production 17-byte id", Process{Subject: "sys.job.result", ProductionVerified: true, MessageID: append(slices.Clone(id16), 1)},
			base(map[string]string{"messaging.destination.name": "sys.job.result"})},
		{"production durable", Process{Subject: "sys.job.result", Durable: "cap-results", ProductionVerified: true, MessageID: id16, ConversationID: "conv-1"},
			base(map[string]string{"messaging.destination.name": "sys.job.result", "messaging.destination.subscription.name": "cap-results",
				"messaging.message.id": strings.Repeat("ab", 16), "messaging.message.conversation_id": "conv-1"})},
		{"no queue group", Process{Subject: "sys.heartbeat"},
			base(map[string]string{"messaging.destination.name": "sys.heartbeat"})},
	}
	tp, exp := newRecorder(t)
	cfg := &Config{TracerProvider: tp}
	for _, c := range cases {
		_, span := cfg.StartProcess(context.Background(), c.in)
		span.End()
		s := onlySpan(t, exp)
		t.Run(c.name, func(t *testing.T) { assertStringAttrs(t, s, c.want) })
	}
}

// fixedPropagator writes fixed header values and extracts nothing.
type fixedPropagator map[string]string

func (f fixedPropagator) Inject(_ context.Context, c propagation.TextMapCarrier) {
	for k, v := range f {
		c.Set(k, v)
	}
}

func (f fixedPropagator) Extract(ctx context.Context, _ propagation.TextMapCarrier) context.Context {
	return ctx
}

func (f fixedPropagator) Fields() []string { return slices.Sorted(maps.Keys(f)) }

// spyPropagator records what the carrier exposes, then extracts W3C trace context.
type spyPropagator struct {
	keys []string
	got  map[string]string
}

func (s *spyPropagator) Inject(context.Context, propagation.TextMapCarrier) {}

func (s *spyPropagator) Extract(ctx context.Context, c propagation.TextMapCarrier) context.Context {
	s.keys = c.Keys()
	sort.Strings(s.keys)
	s.got = map[string]string{}
	for _, k := range []string{"baggage", "Baggage", "X-Other", "Traceparent"} {
		s.got[k] = c.Get(k)
	}
	return propagation.TraceContext{}.Extract(ctx, c)
}

func (s *spyPropagator) Fields() []string { return nil }

func TestBaggageIsNeverPropagated(t *testing.T) {
	composite := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	setGlobalPropagator(t, composite)
	tp, _ := newRecorder(t)
	member, err := baggage.NewMember("tenant", "acme")
	if err != nil {
		t.Fatal(err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatal(err)
	}
	ctx, ambient := tp.Tracer("test").Start(baggage.ContextWithBaggage(context.Background(), bag), "ambient")
	defer ambient.End()
	probe := propagation.MapCarrier{}
	composite.Inject(ctx, probe)
	if probe.Get("baggage") == "" {
		t.Fatal("setup: the composite propagator wrote no baggage")
	}
	want := traceparentOf(ambient.SpanContext())
	inbound := nats.Header{"traceparent": {want}, "baggage": {"tenant=evil"}, "X-Other": {"x"}}
	for name, cfg := range map[string]*Config{"default": {}, "composite": {Propagator: composite}} {
		h := cfg.InjectHeaders(ctx, nil)
		if len(h) != 1 || h.Get("traceparent") != want {
			t.Errorf("%s: injected %v, want only traceparent %s", name, h, want)
		}
		if sc := cfg.Extract(inbound); !sc.IsValid() || sc.TraceID() != ambient.SpanContext().TraceID() || sc.SpanID() != ambient.SpanContext().SpanID() {
			t.Errorf("%s: extract with baggage gave %v", name, sc)
		}
	}
	spy := &spyPropagator{}
	if sc := (&Config{Propagator: spy}).Extract(inbound); !sc.IsValid() {
		t.Error("spy propagator: no valid context extracted")
	}
	if !slices.Equal(spy.keys, []string{"traceparent"}) || spy.got["baggage"] != "" || spy.got["Baggage"] != "" ||
		spy.got["X-Other"] != "" || spy.got["Traceparent"] != "" {
		t.Errorf("carrier exposed keys %v and values %v", spy.keys, spy.got)
	}
	setGlobalPropagator(t, fixedPropagator{"traceparent": "global"})
	if got := (&Config{}).InjectHeaders(ctx, nil); len(got) != 1 || got.Get("traceparent") != want {
		t.Errorf("default config used the global propagator: %v", got)
	}
	custom := fixedPropagator{"traceparent": vectorTraceparent, "baggage": "k=v", "X-Other": "x", "Traceparent": "y"}
	if got := (&Config{Propagator: custom}).InjectHeaders(ctx, nil); len(got) != 1 || got.Get("traceparent") != vectorTraceparent {
		t.Errorf("custom propagator: injected %v, want only traceparent", got)
	}
}

func TestNilConfigProducesNoHeadersOrSpans(t *testing.T) {
	globalTP, globalExp := newRecorder(t)
	setGlobalTracerProvider(t, globalTP)
	other, _ := newRecorder(t)
	ctx, ambient := other.Tracer("test").Start(context.Background(), "ambient")
	defer ambient.End()
	var off *Config
	if h := off.InjectHeaders(ctx, nil); h != nil {
		t.Errorf("nil config injected %v", h)
	}
	h := nats.Header{"traceparent": {"stale"}, "x-keep": {"v"}}
	if got := off.InjectHeaders(ctx, h); !reflect.DeepEqual(got, nats.Header{"traceparent": {"stale"}, "x-keep": {"v"}}) ||
		reflect.ValueOf(got).Pointer() != reflect.ValueOf(h).Pointer() {
		t.Errorf("nil config changed the headers: %v", got)
	}
	if off.Extract(nats.Header{"traceparent": {vectorTraceparent}}).IsValid() {
		t.Error("nil config extracted a context")
	}
	creation := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1}, Remote: true})
	pctx, ps := off.StartPublish(ctx, Publish{Subject: "sys.job.submit", ConversationID: "c"})
	cctx, cs := off.StartProcess(ctx, Process{Subject: "sys.job.submit", Creation: creation})
	if pctx != ctx || ps != nil || cctx != ctx || cs != nil {
		t.Errorf("nil config started spans: %v %v", ps, cs)
	}
	ps.Fail(ErrorTypeTransportPublishFailed)
	ps.End()
	cs.Fail(ErrorTypeHandlerFailed)
	cs.End()
	if n := len(globalExp.GetSpans()); n != 0 {
		t.Errorf("nil config recorded %d spans in the global provider", n)
	}
	_, s := (&Config{}).StartPublish(ctx, Publish{Subject: "sys.job.submit"})
	s.End()
	if n := len(globalExp.GetSpans()); n != 1 {
		t.Errorf("a config without a provider recorded %d spans in the global provider, want 1", n)
	}
	var nilCtx context.Context
	cfg := &Config{TracerProvider: other}
	if h := cfg.InjectHeaders(nilCtx, nil); h != nil {
		t.Errorf("nil context injected %v", h)
	}
	_, s1 := cfg.StartPublish(nilCtx, Publish{Subject: "sys.alert"})
	s1.End()
	_, s2 := cfg.StartProcess(nilCtx, Process{Subject: "sys.alert", Creation: creation})
	s2.End()
}

func TestInjectHeadersReplacesTraceHeaders(t *testing.T) {
	tp, _ := newRecorder(t)
	cfg := &Config{TracerProvider: tp}
	ctx, span := tp.Tracer("test").Start(context.Background(), "ambient")
	defer span.End()
	stale := func() nats.Header {
		return nats.Header{"traceparent": {vectorTraceparent, "00-x"}, "tracestate": {"stale=1"}, "x-keep": {"v"}}
	}
	h := stale()
	got := cfg.InjectHeaders(ctx, h)
	want := nats.Header{"traceparent": {traceparentOf(span.SpanContext())}, "x-keep": {"v"}}
	if !reflect.DeepEqual(got, want) || reflect.ValueOf(got).Pointer() != reflect.ValueOf(h).Pointer() {
		t.Errorf("inject with a span: %v, want %v in the same map", got, want)
	}
	h = stale()
	got = cfg.InjectHeaders(context.Background(), h)
	if !reflect.DeepEqual(got, nats.Header{"x-keep": {"v"}}) || reflect.ValueOf(got).Pointer() != reflect.ValueOf(h).Pointer() {
		t.Errorf("inject without a span: %v, want only x-keep in the same map", got)
	}
	if got := cfg.InjectHeaders(context.Background(), nil); got != nil {
		t.Errorf("inject without a span into nil: %v", got)
	}
}

func TestErrorTypeOnlyOnFailureAndClosed(t *testing.T) {
	contract := map[ErrorType]string{
		ErrorTypeProtocolVersionMismatch: "PROTOCOL_VERSION_MISMATCH", ErrorTypeProtocolMalformedPacket: "PROTOCOL_MALFORMED_PACKET",
		ErrorTypeProtocolUnknownPayload: "PROTOCOL_UNKNOWN_PAYLOAD", ErrorTypeProtocolSignatureInvalid: "PROTOCOL_SIGNATURE_INVALID",
		ErrorTypeProtocolSignatureMissing: "PROTOCOL_SIGNATURE_MISSING", ErrorTypeAdmissionRejected: "ADMISSION_REJECTED",
		ErrorTypeHandlerFailed: "HANDLER_FAILED", ErrorTypeTransportPublishFailed: "TRANSPORT_PUBLISH_FAILED", ErrorTypeOther: "_OTHER",
	}
	for c, s := range contract {
		if string(c) != s {
			t.Errorf("constant %q, want %q", c, s)
		}
	}
	if len(contract) != 9 {
		t.Fatalf("%d distinct constants, want 9", len(contract))
	}
	want := map[string]map[ErrorType]ErrorType{"publish": {}, "process": {}}
	for c := range contract {
		want["publish"][c], want["process"][c] = ErrorTypeOther, c
	}
	want["publish"][ErrorTypeTransportPublishFailed] = ErrorTypeTransportPublishFailed
	want["process"][ErrorTypeTransportPublishFailed] = ErrorTypeOther
	for _, m := range want {
		m["boom: secret"], m[""] = ErrorTypeOther, ErrorTypeOther
	}
	tp, exp := newRecorder(t)
	cfg := &Config{TracerProvider: tp}
	in := Process{Subject: "sys.job.result", ConversationID: "c", ProductionVerified: true}
	for op, cases := range want {
		_, okSpan := startOp(cfg, context.Background(), op, in)
		okSpan.End()
		success := onlySpan(t, exp)
		okAttrs := attrMap(t, success)
		if _, has := okAttrs["error.type"]; has || success.Status.Code != codes.Unset || len(success.Events) != 0 {
			t.Errorf("%s success: attrs %v status %v", op, success.Attributes, success.Status)
		}
		for given, wantType := range cases {
			_, span := startOp(cfg, context.Background(), op, in)
			span.Fail(given)
			span.End()
			s := onlySpan(t, exp)
			got := attrMap(t, s)
			et, has := got["error.type"]
			delete(got, "error.type")
			if !has || et.Type() != attribute.STRING || et.AsString() != string(wantType) || !reflect.DeepEqual(got, okAttrs) ||
				s.Status.Code != codes.Error || s.Status.Description != string(wantType) || len(s.Events) != 0 {
				t.Errorf("%s Fail(%q): error.type %v (present %v), status %v, %d events, attrs %v; want %q",
					op, given, et.Emit(), has, s.Status, len(s.Events), s.Attributes, wantType)
			}
		}
	}
}

func TestProcessLinkAndParent(t *testing.T) {
	tp, exp := newRecorder(t)
	other, _ := newRecorder(t)
	cfg := &Config{TracerProvider: tp}
	creation := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x4b, 0xf9, 1}, SpanID: trace.SpanID{0x00, 0xf0, 1}, TraceFlags: trace.FlagsSampled, Remote: true,
	})
	partial := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{0x4b, 0xf9, 1}, Remote: true})
	actx, ambient := other.Tracer("test").Start(context.Background(), "ambient")
	defer ambient.End()
	cases := []struct {
		name       string
		ctx        context.Context
		creation   trace.SpanContext
		wantParent trace.SpanContext
		wantLink   bool
	}{
		{"creation, no ambient", context.Background(), creation, creation, true},
		{"creation, ambient", actx, creation, ambient.SpanContext(), true},
		{"no creation, no ambient", context.Background(), trace.SpanContext{}, trace.SpanContext{}, false},
		{"partial creation, no ambient", context.Background(), partial, trace.SpanContext{}, false},
		{"no creation, ambient", actx, trace.SpanContext{}, ambient.SpanContext(), false},
	}
	for _, c := range cases {
		_, span := cfg.StartProcess(c.ctx, Process{Subject: "sys.job.submit", Creation: c.creation})
		span.End()
		s := onlySpan(t, exp)
		if !s.Parent.Equal(c.wantParent) || s.Parent.IsRemote() != c.wantParent.IsRemote() {
			t.Errorf("%s: parent %v, want %v", c.name, s.Parent, c.wantParent)
		}
		if c.wantParent.IsValid() && s.SpanContext.TraceID() != c.wantParent.TraceID() {
			t.Errorf("%s: trace id %s, want %s", c.name, s.SpanContext.TraceID(), c.wantParent.TraceID())
		}
		linked := len(s.Links) == 1 && s.Links[0].SpanContext.Equal(c.creation) && len(s.Links[0].Attributes) == 0
		if linked != c.wantLink || (!c.wantLink && len(s.Links) != 0) {
			t.Errorf("%s: links %v, want link=%v", c.name, s.Links, c.wantLink)
		}
	}
}

func TestPublishSpanIsInjectedCreationContext(t *testing.T) {
	tp, exp := newRecorder(t)
	other, _ := newRecorder(t)
	cfg := &Config{TracerProvider: tp}
	actx, ambient := other.Tracer("test").Start(context.Background(), "ambient")
	defer ambient.End()
	pctx, span := cfg.StartPublish(actx, Publish{Subject: "sys.job.result", ConversationID: "c"})
	h := cfg.InjectHeaders(pctx, nil)
	span.End()
	s := onlySpan(t, exp)
	if !s.Parent.Equal(ambient.SpanContext()) {
		t.Errorf("publish parent %v, want the ambient span %v", s.Parent, ambient.SpanContext())
	}
	want := "00-" + s.SpanContext.TraceID().String() + "-" + s.SpanContext.SpanID().String() + "-01"
	if len(h) != 1 || h.Get("traceparent") != want {
		t.Errorf("injected %v, want only traceparent %s", h, want)
	}
	if got := cfg.Extract(h); got.SpanID() != s.SpanContext.SpanID() || !got.IsRemote() {
		t.Errorf("extracted creation context %v, want the publish span", got)
	}
}
