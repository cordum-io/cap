// Package telemetry is the CAP Go SDK adapter for spec/interop/trace-context.md.
package telemetry

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Config selects the OpenTelemetry tracer provider and propagator. A nil
// *Config is the off switch: it writes no headers, extracts nothing and starts
// no spans.
type Config struct {
	// TracerProvider records the spans. Nil means otel.GetTracerProvider().
	TracerProvider trace.TracerProvider
	// Propagator encodes the span context. Nil means propagation.TraceContext{}.
	Propagator propagation.TextMapPropagator
}

// ErrorType is a value of the contract's closed error.type enum.
type ErrorType string

// The contract's error.type values.
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
	Subject        string
	Pool           string
	ReplyInbox     bool
	ConversationID string
}

// Process describes one admitted packet.
type Process struct {
	Subject            string
	Pool               string
	ReplyInbox         bool
	QueueGroup         string
	Durable            string
	ProductionVerified bool
	MessageID          []byte
	ConversationID     string
	Creation           trace.SpanContext
}

// Span is a contract span. It exposes only Fail and End.
type Span struct{}

// InjectHeaders writes the trace context of ctx into h.
func (c *Config) InjectHeaders(ctx context.Context, h nats.Header) nats.Header {
	return h
}

// Extract reads the creation context from h.
func (c *Config) Extract(h nats.Header) trace.SpanContext {
	return trace.SpanContext{}
}

// StartPublish starts the PRODUCER publish span.
func (c *Config) StartPublish(ctx context.Context, p Publish) (context.Context, *Span) {
	return ctx, nil
}

// StartProcess starts the CONSUMER process span.
func (c *Config) StartProcess(ctx context.Context, p Process) (context.Context, *Span) {
	return ctx, nil
}

// fixedSubjects returns the fixed sys.* subjects of the spec/09 subject table.
func fixedSubjects() []string { return nil }

// Fail records a failure on the span.
func (s *Span) Fail(t ErrorType) {}

// End ends the span.
func (s *Span) End() {}
