// Package telemetry adapts the OpenTelemetry trace API to the CAP contract for
// W3C Trace Context over NATS, spec/interop/trace-context.md. It uses the
// OpenTelemetry API only; the caller supplies the tracer provider.
//
// Trust and ordering:
//   - Trace headers sit outside the signed packet bytes. Their values are
//     untrusted correlation hints, even on a CAP-PRODUCTION verified packet.
//   - Call [Config.Extract] only after the packet has passed CAP-PRODUCTION
//     admission or legacy validation, and never for a rejected packet.
//   - Trace data never selects or influences tenant, principal or sender
//     identity, a key, policy, a safety decision, approval, admission or
//     routing.
//
// Behavior:
//   - Only the lowercase traceparent and tracestate headers are read or
//     written. Baggage is never written or read, whatever propagator is
//     configured.
//   - A nil *Config is the off switch: it writes no headers, extracts nothing
//     and starts no spans.
//   - There is no span for a rejected packet; the contract makes it optional.
//   - A legacy packet's trace_id is never recorded: a process span carries
//     messaging.message.conversation_id only for a CAP-PRODUCTION verified
//     packet.
//   - The CAP Go client and runtime do not call this package yet.
package telemetry

import (
	"context"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName        = "github.com/cordum-io/cap/v2/sdk/go/telemetry"
	headerTraceparent = "traceparent"
	headerTracestate  = "tracestate"
	jobPrefix         = "job."
)

// Config selects the OpenTelemetry tracer provider and propagator. It is
// read-only after construction. A nil *Config is the off switch: it writes no
// headers, extracts nothing and starts no spans.
type Config struct {
	// TracerProvider records the spans. Nil means otel.GetTracerProvider().
	TracerProvider trace.TracerProvider
	// Propagator encodes the span context. Nil means propagation.TraceContext{},
	// never the global propagator, which may include Baggage. Whatever the
	// propagator, it sees only the traceparent and tracestate headers.
	Propagator propagation.TextMapPropagator
}

// ErrorType is a value of the contract's closed error.type enum.
type ErrorType string

// The contract's error.type values. Publish spans allow
// ErrorTypeTransportPublishFailed and ErrorTypeOther; process spans allow the
// other seven and ErrorTypeOther.
const (
	ErrorTypeProtocolVersionMismatch  ErrorType = "PROTOCOL_VERSION_MISMATCH"
	ErrorTypeProtocolMalformedPacket  ErrorType = "PROTOCOL_MALFORMED_PACKET"
	ErrorTypeProtocolUnknownPayload   ErrorType = "PROTOCOL_UNKNOWN_PAYLOAD"
	ErrorTypeProtocolSignatureInvalid ErrorType = "PROTOCOL_SIGNATURE_INVALID"
	ErrorTypeProtocolSignatureMissing ErrorType = "PROTOCOL_SIGNATURE_MISSING"
	ErrorTypeAdmissionRejected        ErrorType = "ADMISSION_REJECTED"
	ErrorTypeHandlerFailed            ErrorType = "HANDLER_FAILED"
	ErrorTypeTransportPublishFailed   ErrorType = "TRANSPORT_PUBLISH_FAILED"
	ErrorTypeOther                    ErrorType = "_OTHER"
)

// Publish describes one published packet.
type Publish struct {
	// Subject is the actual NATS subject.
	Subject string
	// Pool is the configured job.<pool> this participant publishes to. The
	// span names Subject only when it is a fixed spec/09 sys.* subject or
	// equals Pool.
	Pool string
	// ReplyInbox marks a broker-generated reply inbox subject.
	ReplyInbox bool
	// ConversationID is the published packet's BusPacket.trace_id. Leave it
	// empty when that trace_id was copied from an unauthenticated packet:
	// spec/10 Safe Observability requires such values to be bounded and
	// sanitized first, and this package defines no sanitizer.
	ConversationID string
}

// Process describes one admitted packet.
type Process struct {
	// Subject is the actual NATS subject.
	Subject string
	// Pool is the configured job.<pool> this participant consumes.
	Pool string
	// ReplyInbox marks a broker-generated reply inbox subject.
	ReplyInbox bool
	// QueueGroup is the NATS queue group, or empty.
	QueueGroup string
	// Durable is the JetStream durable consumer name, or empty.
	Durable string
	// ProductionVerified reports that the packet passed CAP-PRODUCTION
	// admission. Only then are MessageID and ConversationID recorded.
	ProductionVerified bool
	// MessageID is the 16-byte SignatureMetadata.message_id. Any other length
	// is not recorded.
	MessageID []byte
	// ConversationID is the packet's BusPacket.trace_id.
	ConversationID string
	// Creation is the context returned by Extract. The zero value means none.
	Creation trace.SpanContext
}

// Span is a contract span. Only Fail and End are exposed, so no caller can add
// a forbidden attribute or event. A nil *Span is a no-op.
type Span struct {
	span    trace.Span
	process bool
}

// carrier exposes exactly the traceparent and tracestate keys of a nats.Header,
// by exact case, to a propagator. Every other key reads as absent and is never
// written, which keeps baggage out under any propagator.
type carrier struct{ h nats.Header }

func traceKey(key string) bool { return key == headerTraceparent || key == headerTracestate }

func (c *carrier) Get(key string) string {
	if v := c.h[key]; traceKey(key) && len(v) > 0 {
		return v[0]
	}
	return ""
}

func (c *carrier) Set(key, value string) {
	if !traceKey(key) {
		return
	}
	if c.h == nil {
		c.h = nats.Header{}
	}
	c.h[key] = []string{value}
}

func (c *carrier) Keys() []string {
	var keys []string
	for _, k := range [...]string{headerTraceparent, headerTracestate} {
		if len(c.h[k]) > 0 {
			keys = append(keys, k)
		}
	}
	return keys
}

func (c *Config) propagator() propagation.TextMapPropagator {
	if c.Propagator != nil {
		return c.Propagator
	}
	return propagation.TraceContext{}
}

// InjectHeaders writes the span context of ctx into h as the traceparent and
// tracestate headers and returns h. It first deletes any existing trace
// headers, so a received context is never forwarded and a new traceparent is
// never paired with a stale tracestate. A nil h is allocated only when a header
// is written. A nil *Config returns h unchanged.
func (c *Config) InjectHeaders(ctx context.Context, h nats.Header) nats.Header {
	if c == nil {
		return h
	}
	if ctx == nil {
		ctx = context.Background()
	}
	delete(h, headerTraceparent)
	delete(h, headerTracestate)
	car := &carrier{h: h}
	c.propagator().Inject(ctx, car)
	return car.h
}

// Extract returns the valid remote creation context carried by h, or the zero
// value. It never consults the caller's context, so an ambient span cannot pass
// as a creation context. Call it only after admission or validation succeeded.
// A nil *Config returns the zero value.
func (c *Config) Extract(h nats.Header) trace.SpanContext {
	if c == nil {
		return trace.SpanContext{}
	}
	ctx := c.propagator().Extract(context.Background(), &carrier{h: h})
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() || !sc.IsRemote() {
		return trace.SpanContext{}
	}
	return sc
}

// StartPublish starts the PRODUCER publish span. Inject the returned context
// with InjectHeaders: the publish span is the creation context. A nil *Config
// returns ctx and a nil *Span.
func (c *Config) StartPublish(ctx context.Context, p Publish) (context.Context, *Span) {
	if c == nil {
		return ctx, nil
	}
	name, attrs := destination("publish", p.Subject, p.Pool, p.ReplyInbox)
	attrs = append(attrs,
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.operation.name", "publish"),
		attribute.String("messaging.operation.type", "send"))
	if p.ConversationID != "" {
		attrs = append(attrs, attribute.String("messaging.message.conversation_id", p.ConversationID))
	}
	return c.start(ctx, name, false, trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(attrs...))
}

// StartProcess starts the CONSUMER process span for an admitted packet. A
// valid Creation becomes a link, and also the parent when ctx carries no
// active span. A nil *Config returns ctx and a nil *Span.
func (c *Config) StartProcess(ctx context.Context, p Process) (context.Context, *Span) {
	if c == nil {
		return ctx, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	name, attrs := destination("process", p.Subject, p.Pool, p.ReplyInbox)
	attrs = append(attrs,
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.operation.name", "process"),
		attribute.String("messaging.operation.type", "process"))
	if p.QueueGroup != "" {
		attrs = append(attrs, attribute.String("messaging.consumer.group.name", p.QueueGroup))
	}
	if p.Durable != "" {
		attrs = append(attrs, attribute.String("messaging.destination.subscription.name", p.Durable))
	}
	if p.ProductionVerified && len(p.MessageID) == 16 {
		attrs = append(attrs, attribute.String("messaging.message.id", hex.EncodeToString(p.MessageID)))
	}
	if p.ProductionVerified && p.ConversationID != "" {
		attrs = append(attrs, attribute.String("messaging.message.conversation_id", p.ConversationID))
	}
	opts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(attrs...)}
	if p.Creation.IsValid() {
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: p.Creation}))
		if !trace.SpanContextFromContext(ctx).IsValid() {
			ctx = trace.ContextWithRemoteSpanContext(ctx, p.Creation)
		}
	}
	return c.start(ctx, name, true, opts...)
}

func (c *Config) start(ctx context.Context, name string, process bool, opts ...trace.SpanStartOption) (context.Context, *Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	tp := c.TracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	ctx, span := tp.Tracer(tracerName).Start(ctx, name, opts...)
	return ctx, &Span{span: span, process: process}
}

// fixedSubjects returns the fixed sys.* subjects of the spec/09 subject table.
func fixedSubjects() []string {
	return []string{
		"sys.job.submit", "sys.job.result", "sys.heartbeat", "sys.alert",
		"sys.job.progress", "sys.job.cancel", "sys.job.dlq", "sys.workflow.event",
		"sys.handshake", "sys.worker.handshake.challenge", "sys.worker.handshake.authenticate",
	}
}

// destination returns the span name and destination attributes for op. Only a
// fixed spec/09 subject or the configured literal job.<pool> is named, so
// publisher-chosen subjects never reach telemetry.
func destination(op, subject, pool string, replyInbox bool) (string, []attribute.KeyValue) {
	switch {
	case replyInbox:
		return op, []attribute.KeyValue{
			attribute.Bool("messaging.destination.temporary", true),
			attribute.Bool("messaging.destination.anonymous", true),
		}
	case slices.Contains(fixedSubjects(), subject) || configuredPool(subject, pool):
		return op + " " + subject, []attribute.KeyValue{attribute.String("messaging.destination.name", subject)}
	default:
		return op, nil
	}
}

func configuredPool(subject, pool string) bool {
	return pool != "" && subject == pool && strings.HasPrefix(pool, jobPrefix) &&
		len(pool) > len(jobPrefix) && !strings.ContainsAny(pool, "*> \t\r\n")
}

// Fail marks the span failed with t. A value outside the span's allowed set,
// including free text, is recorded as ErrorTypeOther. Fail sets only error.type
// and an ERROR status whose description is that value; it records no event.
func (s *Span) Fail(t ErrorType) {
	if s == nil || s.span == nil {
		return
	}
	if !s.allows(t) {
		t = ErrorTypeOther
	}
	s.span.SetAttributes(attribute.String("error.type", string(t)))
	s.span.SetStatus(codes.Error, string(t))
}

func (s *Span) allows(t ErrorType) bool {
	switch t {
	case ErrorTypeOther:
		return true
	case ErrorTypeTransportPublishFailed:
		return !s.process
	case ErrorTypeProtocolVersionMismatch, ErrorTypeProtocolMalformedPacket, ErrorTypeProtocolUnknownPayload,
		ErrorTypeProtocolSignatureInvalid, ErrorTypeProtocolSignatureMissing, ErrorTypeAdmissionRejected,
		ErrorTypeHandlerFailed:
		return s.process
	}
	return false
}

// End ends the span.
func (s *Span) End() {
	if s == nil || s.span == nil {
		return
	}
	s.span.End()
}
