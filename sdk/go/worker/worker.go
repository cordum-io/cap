package worker

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler processes a JobRequest and returns a JobResult.
type Handler func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error)

// NATSConn is an interface that represents a NATS connection.
type NATSConn interface {
	Publish(subject string, data []byte) error
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
}

// WorkerMiddleware wraps a Handler, returning a new Handler.
// Middleware is applied in FIFO order: the first registered middleware is the outermost.
type WorkerMiddleware func(next Handler) Handler

// HeartbeatOption mutates an outgoing heartbeat before it is encoded.
type HeartbeatOption func(*agentv1.Heartbeat)

// WithAuthToken sets the optional worker attestation token on the heartbeat.
func WithAuthToken(token string) HeartbeatOption {
	return func(hb *agentv1.Heartbeat) {
		if hb == nil {
			return
		}
		if token = strings.TrimSpace(token); token != "" {
			hb.AuthToken = token
		}
	}
}

// WithAgentName sets the optional human-facing agent display label on the
// heartbeat. The value is sanitized and bounded via capsdk.SanitizeAgentName.
// This is a DISPLAY label only — it is NOT an authentication authority, and
// consumers must prefer authenticated identity records over it. Blank labels
// are ignored.
func WithAgentName(name string) HeartbeatOption {
	return func(hb *agentv1.Heartbeat) {
		if hb == nil {
			return
		}
		if clean := capsdk.SanitizeAgentName(name); clean != "" {
			hb.AgentName = clean
		}
	}
}

// Worker subscribes to a pool subject and handles jobs.
type Worker struct {
	NATS        NATSConn
	Subject     string
	Handler     Handler
	PublicKeys  map[string]*ecdsa.PublicKey
	PrivateKey  *ecdsa.PrivateKey
	SenderID    string
	Logger      *slog.Logger
	Metrics     capsdk.MetricsHook
	middlewares []WorkerMiddleware
}

// Use appends middleware to the Worker. Middleware executes in registration order
// before the handler.
func (w *Worker) Use(mw ...WorkerMiddleware) {
	w.middlewares = append(w.middlewares, mw...)
}

// Start begins consuming and handling JobRequests. It returns after the subscription is created.
func (w *Worker) Start() error {
	if err := w.validateStart(); err != nil {
		return err
	}
	if w.Logger == nil {
		w.Logger = slog.Default()
	}
	if w.Metrics == nil {
		w.Metrics = capsdk.NoopMetrics
	}
	_, err := w.NATS.QueueSubscribe(w.Subject, w.Subject, w.handleMessage)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	return nil
}

// HeartbeatPayload returns a protobuf-encoded heartbeat envelope.
func HeartbeatPayload(workerID, pool string, activeJobs, maxParallel int, cpuLoad float32, opts ...HeartbeatOption) ([]byte, error) {
	return HeartbeatPayloadWithProgress(workerID, pool, activeJobs, maxParallel, cpuLoad, 0, 0, "", opts...)
}

// HeartbeatPayloadWithMemory returns a heartbeat payload including memory utilization.
func HeartbeatPayloadWithMemory(workerID, pool string, activeJobs, maxParallel int, cpuLoad, memoryLoad float32, opts ...HeartbeatOption) ([]byte, error) {
	return HeartbeatPayloadWithProgress(workerID, pool, activeJobs, maxParallel, cpuLoad, memoryLoad, 0, "", opts...)
}

// HeartbeatPayloadWithProgress returns a heartbeat payload including optional progress checkpoints.
func HeartbeatPayloadWithProgress(workerID, pool string, activeJobs, maxParallel int, cpuLoad, memoryLoad float32, progressPct int32, lastMemo string, opts ...HeartbeatOption) ([]byte, error) {
	heartbeat := &agentv1.Heartbeat{
		WorkerId:        workerID,
		Pool:            pool,
		ActiveJobs:      int32(activeJobs),
		MaxParallelJobs: int32(maxParallel),
		CpuLoad:         cpuLoad,
		MemoryLoad:      memoryLoad,
		ProgressPct:     progressPct,
		LastMemo:        lastMemo,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(heartbeat)
		}
	}
	// ValidateBusPacket requires trace_id and created_at on every envelope.
	// Use workerID as the heartbeat correlation id (it is already SenderId);
	// stricter bus consumers reject helper-built heartbeats if either is empty.
	hb := &agentv1.BusPacket{
		TraceId:         workerID,
		SenderId:        workerID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload: &agentv1.BusPacket_Heartbeat{
			Heartbeat: heartbeat,
		},
	}
	return marshalValidatedEnvelope(hb, nil)
}

// ProgressPayload returns a protobuf-encoded progress envelope.
func ProgressPayload(senderID, jobID, stepID string, percent int32, message string) ([]byte, error) {
	pkt := &agentv1.BusPacket{
		TraceId:         jobID,
		SenderId:        senderID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload: &agentv1.BusPacket_JobProgress{
			JobProgress: &agentv1.JobProgress{
				JobId:   jobID,
				StepId:  stepID,
				Percent: percent,
				Message: message,
			},
		},
	}
	return marshalValidatedEnvelope(pkt, nil)
}

// CancelPayload returns a protobuf-encoded cancel envelope.
func CancelPayload(senderID, jobID, reason, requestedBy string) ([]byte, error) {
	pkt := &agentv1.BusPacket{
		TraceId:         jobID,
		SenderId:        senderID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload: &agentv1.BusPacket_JobCancel{
			JobCancel: &agentv1.JobCancel{
				JobId:       jobID,
				Reason:      reason,
				RequestedBy: requestedBy,
			},
		},
	}
	return marshalValidatedEnvelope(pkt, nil)
}

// EmitProgress publishes a progress packet once.
func EmitProgress(nc *nats.Conn, payload []byte) error {
	return nc.Publish(capsdk.SubjectProgress, payload)
}

// EmitCancel publishes a cancel packet once.
func EmitCancel(nc *nats.Conn, payload []byte) error {
	return nc.Publish(capsdk.SubjectCancel, payload)
}

// EmitHeartbeat publishes a heartbeat once. Call repeatedly on a ticker.
func EmitHeartbeat(nc *nats.Conn, payload []byte) error {
	return nc.Publish(capsdk.SubjectHeartbeat, payload)
}

// HeartbeatLoop emits heartbeats until ctx is done.
func HeartbeatLoop(ctx context.Context, nc *nats.Conn, payloadFn func() ([]byte, error)) {
	ticker := time.NewTicker(capsdk.DefaultHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload, err := payloadFn()
			if err == nil {
				if hbErr := EmitHeartbeat(nc, payload); hbErr != nil {
					slog.Warn("worker: heartbeat emit failed", "error", hbErr)
				}
			}
		}
	}
}
