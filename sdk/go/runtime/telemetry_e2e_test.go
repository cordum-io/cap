package runtime

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/cordum-io/cap/v2/sdk/go/client"
	"github.com/cordum-io/cap/v2/sdk/go/telemetry"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const telemetryTopic = "job.telemetry.e2e"

type telemetryFixture struct {
	nc          *nats.Conn
	tel         *telemetry.Config
	exporter    *tracetest.InMemoryExporter
	job, result *nats.Subscription
	handled     chan trace.SpanContext
}

func telemetryOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func telemetryEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func telemetryRecorder(t *testing.T) (*telemetry.Config, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { telemetryOK(t, tp.Shutdown(context.Background())) })
	return &telemetry.Config{TracerProvider: tp}, exporter
}

func newTelemetryFixture(t *testing.T, enabled bool, handlerErr error) *telemetryFixture {
	t.Helper()
	tel, exporter := telemetryRecorder(t)
	if !enabled {
		tel = nil
	}
	url := startProductionNATSServer(t)
	nc := connectProductionNATSClient(t, url)
	f := &telemetryFixture{nc: nc, tel: tel, exporter: exporter, handled: make(chan trace.SpanContext, 1)}
	_, err := nc.Subscribe(capsdk.SubjectSubmit, func(msg *nats.Msg) {
		header := msg.Header
		if enabled {
			header = nats.Header{"baggage": {"k=v"}}
			for k, v := range msg.Header {
				header[k] = v
			}
		}
		if err := nc.PublishMsg(&nats.Msg{Subject: telemetryTopic, Data: msg.Data, Header: header}); err != nil {
			t.Error(err)
		}
	})
	telemetryOK(t, err)
	f.job, err = nc.SubscribeSync(telemetryTopic)
	telemetryOK(t, err)
	f.result, err = nc.SubscribeSync(capsdk.SubjectResult)
	telemetryOK(t, err)
	store := NewInMemoryBlobStore()
	telemetryOK(t, store.Set(context.Background(), "ctx:telemetry", []byte("{}")))
	a := &Agent{NATSURL: url, Store: store, SenderID: "telemetry-worker", Logger: silentLogger(), AllowUnsigned: true, HandshakeMode: HandshakeModeOff, Telemetry: tel}
	Register(a, telemetryTopic, func(ctx Context, in struct{}) (struct{}, error) {
		f.handled <- trace.SpanContextFromContext(ctx.TraceContext())
		return in, handlerErr
	})
	telemetryOK(t, a.Start())
	t.Cleanup(func() { telemetryOK(t, a.Close()) })
	flushProductionSubscription(t, a)
	telemetryOK(t, nc.FlushTimeout(5*time.Second))
	return f
}

func (f *telemetryFixture) submit(t *testing.T) {
	t.Helper()
	c := &client.Client{NATS: f.nc, AllowUnsigned: true, Telemetry: f.tel}
	req := &agentv1.JobRequest{JobId: "telemetry", Topic: telemetryTopic, ContextPtr: "redis://ctx:telemetry"}
	telemetryOK(t, c.Submit(context.Background(), req, "trace-telemetry", "telemetry-client", nil))
}

func telemetryMessage(t *testing.T, sub *nats.Subscription) *nats.Msg {
	t.Helper()
	msg, err := sub.NextMsg(5 * time.Second)
	telemetryOK(t, err)
	return msg
}

func telemetrySpans(t *testing.T, exporter *tracetest.InMemoryExporter, count int) map[string]tracetest.SpanStub {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(exporter.GetSpans()) < count && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	spans := exporter.GetSpans()
	telemetryEqual(t, "span count", len(spans), count)
	byName := make(map[string]tracetest.SpanStub)
	for _, span := range spans {
		byName[span.Name] = span
	}
	return byName
}

func telemetryAttribute(t *testing.T, span tracetest.SpanStub, key, want string) {
	t.Helper()
	for _, attr := range span.Attributes {
		if string(attr.Key) == key {
			if want == "" {
				t.Fatalf("unexpected attribute %s", key)
			}
			telemetryEqual(t, key, attr.Value.AsString(), want)
			return
		}
	}
	if want != "" {
		t.Fatalf("missing attribute %s", key)
	}
}

func TestTelemetryRoundTripOverNATS(t *testing.T) {
	f := newTelemetryFixture(t, true, nil)
	f.submit(t)
	job, result := telemetryMessage(t, f.job), telemetryMessage(t, f.result)
	spans := telemetrySpans(t, f.exporter, 3)
	submit, process, publish := spans["publish sys.job.submit"], spans["process "+telemetryTopic], spans["publish sys.job.result"]
	telemetryEqual(t, "submit kind", submit.SpanKind, trace.SpanKindProducer)
	telemetryEqual(t, "process kind", process.SpanKind, trace.SpanKindConsumer)
	telemetryEqual(t, "result kind", publish.SpanKind, trace.SpanKindProducer)
	creation, output := f.tel.Extract(job.Header), f.tel.Extract(result.Header)
	telemetryEqual(t, "job trace", creation.TraceID(), submit.SpanContext.TraceID())
	telemetryEqual(t, "job span", creation.SpanID(), submit.SpanContext.SpanID())
	select {
	case handled := <-f.handled:
		telemetryEqual(t, "handler trace", handled.TraceID(), submit.SpanContext.TraceID())
		telemetryEqual(t, "handler span", handled.SpanID(), process.SpanContext.SpanID())
	case <-time.After(5 * time.Second):
		t.Fatal("handler timeout")
	}
	telemetryEqual(t, "process parent", process.Parent.SpanID(), submit.SpanContext.SpanID())
	telemetryEqual(t, "remote parent", process.Parent.IsRemote(), true)
	telemetryEqual(t, "links", len(process.Links), 1)
	telemetryEqual(t, "link", process.Links[0].SpanContext.SpanID(), submit.SpanContext.SpanID())
	telemetryEqual(t, "result trace", output.TraceID(), submit.SpanContext.TraceID())
	telemetryEqual(t, "result span", output.SpanID(), publish.SpanContext.SpanID())
	telemetryEqual(t, "result parent", publish.Parent.SpanID(), process.SpanContext.SpanID())
	telemetryEqual(t, "result header keys", len(result.Header), 1)
	telemetryAttribute(t, submit, "messaging.message.conversation_id", "trace-telemetry")
	telemetryAttribute(t, process, "messaging.message.conversation_id", "")
	telemetryAttribute(t, publish, "messaging.message.conversation_id", "")
}

func TestTelemetryHandlerFailureMarksProcessSpan(t *testing.T) {
	f := newTelemetryFixture(t, true, errors.New("handler failed"))
	f.submit(t)
	result := telemetryMessage(t, f.result)
	var packet agentv1.BusPacket
	telemetryOK(t, proto.Unmarshal(result.Data, &packet))
	telemetryEqual(t, "result status", packet.GetJobResult().GetStatus(), agentv1.JobStatus_JOB_STATUS_FAILED)
	telemetryEqual(t, "result trace valid", f.tel.Extract(result.Header).IsValid(), true)
	process := telemetrySpans(t, f.exporter, 3)["process "+telemetryTopic]
	telemetryEqual(t, "process status", process.Status.Code, codes.Error)
	telemetryAttribute(t, process, "error.type", string(telemetry.ErrorTypeHandlerFailed))
}

func TestTelemetryInvalidTraceparentStartsNewRoot(t *testing.T) {
	for _, header := range []string{"00-00000000000000000000000000000000-0000000000000000-01", "garbage"} {
		t.Run(header, func(t *testing.T) {
			f := newTelemetryFixture(t, true, nil)
			packet := &agentv1.BusPacket{TraceId: "trace-telemetry", SenderId: "telemetry-client", CreatedAt: timestamppb.Now(), ProtocolVersion: capsdk.DefaultProtocolVersion,
				Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{JobId: "telemetry", Topic: telemetryTopic, ContextPtr: "redis://ctx:telemetry"}}}
			data, err := capsdk.MarshalDeterministic(packet)
			telemetryOK(t, err)
			telemetryOK(t, f.nc.PublishMsg(&nats.Msg{Subject: telemetryTopic, Data: data, Header: nats.Header{"traceparent": {header}}}))
			telemetryMessage(t, f.result)
			process := telemetrySpans(t, f.exporter, 2)["process "+telemetryTopic]
			telemetryEqual(t, "parent invalid", process.Parent.IsValid(), false)
			telemetryEqual(t, "new root valid", process.SpanContext.IsValid(), true)
			telemetryEqual(t, "links", len(process.Links), 0)
			select {
			case handled := <-f.handled:
				telemetryEqual(t, "handler trace", handled.TraceID(), process.SpanContext.TraceID())
			case <-time.After(5 * time.Second):
				t.Fatal("handler timeout")
			}
		})
	}
}

func TestTelemetryNilConfigSendsNoHeaders(t *testing.T) {
	tel, exporter := telemetryRecorder(t)
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(tel.TracerProvider)
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	f := newTelemetryFixture(t, false, nil)
	f.submit(t)
	telemetryEqual(t, "job nil headers", telemetryMessage(t, f.job).Header == nil, true)
	telemetryEqual(t, "result nil headers", telemetryMessage(t, f.result).Header == nil, true)
	select {
	case handled := <-f.handled:
		telemetryEqual(t, "handler invalid", handled.IsValid(), false)
	case <-time.After(5 * time.Second):
		t.Fatal("handler timeout")
	}
	telemetrySpans(t, exporter, 0)
}

type countingTracePropagator struct {
	propagation.TraceContext
	calls atomic.Int32
}

func (p *countingTracePropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	p.calls.Add(1)
	return p.TraceContext.Extract(ctx, carrier)
}

func TestTelemetryProductionRejectionCreatesNoSpan(t *testing.T) {
	tel, exporter := telemetryRecorder(t)
	propagator := &countingTracePropagator{}
	tel.Propagator = propagator
	f := newTelemetryProductionFixture(t, tel)
	bad := tamperProductionPacket(t, f.signedPacket("job-tampered", 2), "job-tampered")
	for _, raw := range [][]byte{bad, f.signedPacket("job-barrier", 999)} {
		telemetryOK(t, f.client.PublishMsg(&nats.Msg{Subject: productionNATSTestSubject, Data: raw,
			Header: nats.Header{"traceparent": {"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"}}}))
	}
	assertHandledJobs(t, f.awaitBarrier(), "job-barrier")
	spans := telemetrySpans(t, exporter, 2)
	telemetryEqual(t, "Extract calls", propagator.calls.Load(), int32(1))
	process := spans["process "+productionNATSTestSubject]
	telemetryEqual(t, "process kind", process.SpanKind, trace.SpanKindConsumer)
	telemetryEqual(t, "parent trace", process.Parent.TraceID().String(), "4bf92f3577b34da6a3ce929d0e0e4736")
	telemetryAttribute(t, process, "messaging.message.id", hex.EncodeToString([]byte("0000000000000999")))
	telemetryAttribute(t, process, "messaging.message.conversation_id", "trace-job-barrier")
	telemetryAttribute(t, spans["publish sys.job.result"], "messaging.message.conversation_id", "trace-job-barrier")
}

func TestContextTraceContextNeverNil(t *testing.T) {
	ctx := Context{}.TraceContext()
	telemetryEqual(t, "context nonnil", ctx != nil, true)
	telemetryEqual(t, "span invalid", trace.SpanContextFromContext(ctx).IsValid(), false)
}

func newTelemetryProductionFixture(t *testing.T, tel *telemetry.Config) *productionNATSFixture {
	t.Helper()
	url := startProductionNATSServer(t)
	nc := connectProductionNATSClient(t, url)
	key, proof := productionNATSTestKey(t), productionNATSTestKey(t)
	trust := capsdk.WorkerTrustConfig{WorkerID: "worker-production-e2e", ExpectedAgentID: "agent-production-e2e", TenantID: "tenant-a",
		Audience: capsdk.WorkerHandshakeAudience, ProofKeyID: "proof-production-e2e", ProofPrivateKey: proof, ExpectedSchedulerID: "scheduler-1",
		SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"scheduler-runtime-key": &key.PublicKey}, SDKVersion: "cap-go/runtime-production-e2e"}
	installProductionTrustResponder(t, nc, trust, key)
	f := &productionNATSFixture{t: t, key: key, client: nc, store: NewInMemoryBlobStore(), handled: make(chan string, 8)}
	a := &Agent{NATSURL: url, Store: f.store, SenderID: trust.WorkerID, Logger: silentLogger(), PrivateKey: proof,
		Telemetry: tel, HandshakeMode: HandshakeModeEnforce, WorkerTrust: trust, HandshakeTimeout: 2 * time.Second, HandshakeRetries: 1,
		Production: ProductionAdmission{Enabled: true, Replay: capsdk.NewInMemoryReplayStore(), Trust: capsdk.ProductionTrustStore{
			Audience: productionNATSTestSubject, Tenant: "tenant-a", Sender: "scheduler-1", MaxLifetime: 5 * time.Minute,
			PublicKeys: map[string]*ecdsa.PublicKey{"scheduler-key": &key.PublicKey}}}}
	Register(a, productionNATSTestSubject, f.recordHandler)
	telemetryOK(t, a.Start())
	t.Cleanup(func() { telemetryOK(t, a.Close()) })
	if token, expiry := a.SessionToken(); token == "" || !expiry.After(time.Now()) {
		t.Fatal("no live production session")
	}
	flushProductionSubscription(t, a)
	return f
}
