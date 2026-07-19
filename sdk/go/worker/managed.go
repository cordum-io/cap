package worker

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultNATSURL        = "nats://127.0.0.1:4222"
	defaultConnectTimeout = 5 * time.Second
	defaultMaxParallel    = int32(1)
)

// ManagedConfig controls ManagedWorker behavior.
type ManagedConfig struct {
	Pool            string
	Subjects        []string
	Queue           string
	NatsURL         string
	MaxParallelJobs int32
	Capabilities    []string
	Labels          map[string]string
	Type            string
	WorkerID        string
	// AgentName is an optional human-facing display label (e.g. "Claude Code —
	// Billing Bot") advertised on heartbeats and handshakes. It is sanitized and
	// bounded before transport and is a DISPLAY label only, never an
	// authentication authority.
	AgentName      string
	HeartbeatEvery time.Duration
	PublicKeys     map[string]*ecdsa.PublicKey
	PrivateKey     *ecdsa.PrivateKey
	// AllowUnsigned explicitly opts into legacy unsigned packet transport.
	AllowUnsigned bool
	// NATSTLSConfig is an optional TLS configuration for the NATS connection.
	// When nil, NewManagedWorker attempts to build one from NATS_TLS_* env vars.
	NATSTLSConfig *tls.Config
	// Logger overrides the default stdout logger. When nil, a standard log.Logger is used.
	Logger *log.Logger
	// Metrics receives job lifecycle events for observability (Prometheus, OTel, etc.).
	// When nil, no metrics callbacks are fired.
	Metrics capsdk.MetricsHook
	// WorkerTrustMode zero retains legacy off behavior. Unknown nonzero values
	// fail; warn and enforce both require complete pinned trust.
	WorkerTrustMode capsdk.WorkerTrustMode
	// WorkerTrust is separate from generic packet signing keys because proof-key
	// possession is bound to the authoritative worker identity.
	WorkerTrust *capsdk.WorkerTrustConfig
	// WorkerTrustTimeout bounds each challenge and authenticate request.
	WorkerTrustTimeout time.Duration
}

// ManagedWorker is a batteries-included CAP worker that handles NATS
// connection management, heartbeats, handshakes, panic recovery, and
// parallel job concurrency control.
type ManagedWorker struct {
	cfg      ManagedConfig
	conn     *nats.Conn
	subjects []string
	admitted []string
	queue    string
	workerID string
	pool     string

	sem    chan struct{}
	active int32
	wg     sync.WaitGroup

	subs []*nats.Subscription

	cancelMu      sync.Mutex
	cancel        context.CancelFunc
	heartbeatDone chan struct{}
	logger        *log.Logger
	metrics       capsdk.MetricsHook
	trust         *managedTrustState
	capability    *agentv1.Handshake

	consecutiveHBFailures int32
	subsMu                sync.Mutex
	runMu                 sync.Mutex
	runStarted            bool
}

// NewManagedWorker builds a worker with a NATS connection.
func NewManagedWorker(cfg ManagedConfig) (*ManagedWorker, error) {
	if cfg.WorkerTrustMode == 0 {
		cfg.WorkerTrustMode = capsdk.WorkerTrustModeOff
	}
	cfg = cloneManagedTrustConfig(cfg)
	resolved, err := resolveManagedConfig(cfg)
	if err != nil {
		return nil, err
	}
	conn, err := connectManagedNATS(cfg, resolved)
	if err != nil {
		return nil, err
	}
	w := &ManagedWorker{
		cfg: cfg, conn: conn, subjects: resolved.subjects, admitted: resolved.admitted,
		queue: resolved.queue, workerID: resolved.workerID, pool: resolved.pool,
		logger: resolved.logger, metrics: cfg.Metrics, trust: newManagedTrustState(cfg),
	}
	w.capability = buildManagedCapability(cfg, resolved.workerID, resolved.admitted)
	w.sem = make(chan struct{}, resolved.maxParallel)
	w.cfg.MaxParallelJobs = resolved.maxParallel
	return w, nil
}

// Run subscribes to configured subjects and processes jobs until ctx is canceled.
func (w *ManagedWorker) Run(ctx context.Context, handler func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error)) error {
	if handler == nil {
		return errors.New("handler required")
	}
	if w.conn == nil {
		return errors.New("nats connection unavailable")
	}
	if err := w.beginRun(); err != nil {
		return err
	}
	if err := w.performInitialTrust(ctx); err != nil {
		return err
	}

	for _, subject := range w.admitted {
		queue := w.queue
		if queue == "" {
			queue = subject
		}
		sub, err := w.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
			w.dispatch(ctx, msg, handler)
		})
		if err != nil {
			w.unsubscribeAll()
			return fmt.Errorf("subscribe %s: %w", subject, err)
		}
		w.subsAppend(sub)
	}

	w.publishHandshake()
	w.startHeartbeat(ctx)
	defer w.stopHeartbeat()
	w.startTrustRenewal(ctx)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-w.trustFailure():
		w.unsubscribeAll()
		return err
	}
}

// Close cancels heartbeats, waits for in-flight handlers to finish, then
// drains the NATS connection.
func (w *ManagedWorker) Close() error {
	w.stopTrustRenewal()
	w.stopHeartbeat()

	w.wg.Wait()

	var err error
	if w.conn != nil {
		err = w.conn.Drain()
	}
	if w.trust != nil {
		w.trust.clearSession()
	}
	return err
}

func (w *ManagedWorker) publishHandshake() {
	packet := &agentv1.BusPacket{
		TraceId:         w.managedTraceID("handshake"),
		SenderId:        w.workerID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		AuthToken:       w.sessionToken(),
		Payload: &agentv1.BusPacket_Handshake{
			Handshake: w.capabilityClone(),
		},
	}
	data, err := marshalValidatedEnvelope(packet, w.cfg.PrivateKey)
	if err != nil {
		w.logger.Printf("worker: marshal handshake failed: %v", err)
		return
	}
	if err := w.conn.Publish(capsdk.SubjectHandshake, data); err != nil {
		w.logger.Printf("worker: publish handshake failed: %v", err)
	}
}

func (w *ManagedWorker) subsAppend(sub *nats.Subscription) {
	if sub == nil {
		return
	}
	w.subsMu.Lock()
	defer w.subsMu.Unlock()
	w.subs = append(w.subs, sub)
}

func (w *ManagedWorker) unsubscribeAll() {
	w.subsMu.Lock()
	subscriptions := append([]*nats.Subscription(nil), w.subs...)
	w.subs = nil
	w.subsMu.Unlock()
	for _, subscription := range subscriptions {
		if subscription != nil {
			_ = subscription.Unsubscribe()
		}
	}
}

func trimSubjects(subjects []string) []string {
	if len(subjects) == 0 {
		return nil
	}
	out := make([]string, 0, len(subjects))
	seen := map[string]struct{}{}
	for _, subject := range subjects {
		if s := strings.TrimSpace(subject); s != "" {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

func resolveWorkerID(explicit, workerType string) string {
	workerID := strings.TrimSpace(explicit)
	if workerID == "" {
		workerID = strings.TrimSpace(os.Getenv("WORKER_ID"))
	}
	if workerID != "" {
		return workerID
	}

	workerType = strings.TrimSpace(workerType)
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		if workerType != "" {
			return workerType
		}
		return "cap-worker"
	}
	if workerType == "" {
		return host
	}
	return workerType + "-" + host
}
