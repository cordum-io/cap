package runtime

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultNATSURL  = "nats://127.0.0.1:4222"
	defaultRedisURL = "redis://127.0.0.1:6379/0"
	defaultTimeout  = 5 * time.Second
	defaultMaxBytes = 2 * 1024 * 1024
	defaultSenderID = "cap-runtime"
)

// BlobStore abstracts payload storage.
type BlobStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte) error
	Close() error
}

// RedisBlobStore is the default Redis-backed implementation.
type RedisBlobStore struct {
	client *redis.Client
}

// NewRedisBlobStore creates a Redis-backed blob store.
func NewRedisBlobStore(redisURL string) (*RedisBlobStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	return &RedisBlobStore{client: client}, nil
}

// Get fetches a payload from Redis.
func (r *RedisBlobStore) Get(ctx context.Context, key string) ([]byte, error) {
	return r.client.Get(ctx, key).Bytes()
}

// Set stores a payload in Redis.
func (r *RedisBlobStore) Set(ctx context.Context, key string, data []byte) error {
	return r.client.Set(ctx, key, data, 0).Err()
}

// Close closes the Redis client.
func (r *RedisBlobStore) Close() error {
	return r.client.Close()
}

// InMemoryBlobStore is a test-only store.
type InMemoryBlobStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewInMemoryBlobStore returns a new in-memory blob store.
func NewInMemoryBlobStore() *InMemoryBlobStore {
	return &InMemoryBlobStore{data: make(map[string][]byte)}
}

// Get returns the payload stored under key, or redis.Nil if not found.
func (s *InMemoryBlobStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	if !ok {
		return nil, redis.Nil
	}
	out := make([]byte, len(value))
	copy(out, value)
	return out, nil
}

// Set stores data under the given key.
func (s *InMemoryBlobStore) Set(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]byte, len(data))
	copy(out, data)
	s.data[key] = out
	return nil
}

// Close is a no-op for the in-memory store.
func (s *InMemoryBlobStore) Close() error {
	return nil
}

// Context provides handler context and metadata.
type Context struct {
	Job    *agentv1.JobRequest
	Packet *agentv1.BusPacket
	Logger *slog.Logger
}

// Handler processes a job payload.
type Handler[TIn any, TOut any] func(Context, TIn) (TOut, error)

type handlerSpec struct {
	topic   string
	handler func(Context, any) (any, error)
	decode  func([]byte) (any, error)
	encode  func(any) ([]byte, error)
	retries int
}

// JobOption adjusts per-handler behavior.
type JobOption func(*handlerSpec)

// WithRetries overrides the default retry count for the handler.
func WithRetries(retries int) JobOption {
	return func(spec *handlerSpec) {
		if retries < 0 {
			retries = 0
		}
		spec.retries = retries
	}
}

// NATSConn represents the subset of NATS functionality required.
type NATSConn interface {
	Publish(subject string, data []byte) error
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
}

// Agent coordinates job handling with typed payloads.
type Agent struct {
	NATS       NATSConn
	NATSURL    string
	RedisURL   string
	Store      BlobStore
	PublicKeys map[string]*ecdsa.PublicKey
	PrivateKey *ecdsa.PrivateKey
	// AllowUnsigned explicitly opts into legacy unsigned packet transport.
	AllowUnsigned bool
	SenderID      string
	// AgentName is an optional human-facing display label advertised on the
	// handshake. It is sanitized/bounded before transport and is a DISPLAY
	// label only, never an authentication authority.
	AgentName       string
	Retries         int
	IOTTimeout      time.Duration
	MaxContextBytes int
	MaxResultBytes  int
	Logger          *slog.Logger
	Metrics         MetricsHook

	// Phase-2 worker handshake. Tenant + HandshakeMode opt the agent
	// into the Phase-2 session-token flow on Start(); zero-valued
	// fields preserve the pre-Phase-2 behaviour. See handshake.go.
	Tenant           string
	SDKVersion       string
	HandshakeMode    HandshakeMode
	HandshakeTimeout time.Duration
	HandshakeRetries int
	// WorkerTrust pins the worker proof identity and scheduler verification
	// keys used by warn and enforce modes. It is intentionally distinct from
	// the generic packet PrivateKey/PublicKeys configuration above.
	WorkerTrust capsdk.WorkerTrustConfig
	// Production opts into the CAP-PRODUCTION raw-packet admission layer
	// (exact-wire signature verification + atomic replay defense) ahead of
	// the legacy handshake-mode/object-signature path. Zero-valued (Enabled
	// false) preserves pre-existing behavior exactly — production is never
	// inferred, only explicitly configured. See production_admission.go.
	Production ProductionAdmission

	handlers         map[string]handlerSpec
	middlewares      []Middleware
	trustMode        HandshakeMode
	trustConfig      *capsdk.WorkerTrustConfig
	productionConfig *ProductionAdmission
	session          *sessionState
	renew            *renewer
	capability       *agentv1.Handshake
	ownedNATS        bool
	ownedStore       bool
	subs             []*nats.Subscription
	subsMu           sync.Mutex
	connectNATS      func(string, string, time.Duration) (NATSConn, error)
	openStore        func(string) (BlobStore, error)
	closeOnce        sync.Once
	closeErr         error
	lifecycleMu      sync.Mutex
	started          bool
	closed           bool
}

// Register registers a typed handler for a topic.
func Register[TIn any, TOut any](agent *Agent, topic string, handler Handler[TIn, TOut], opts ...JobOption) {
	if agent == nil {
		return
	}
	spec := handlerSpec{
		topic: topic,
		handler: func(ctx Context, data any) (any, error) {
			return handler(ctx, data.(TIn))
		},
		decode: func(payload []byte) (any, error) {
			var input TIn
			if err := json.Unmarshal(payload, &input); err != nil {
				return nil, err
			}
			return input, nil
		},
		encode: func(value any) ([]byte, error) {
			return json.Marshal(value)
		},
		retries: max(0, agent.Retries),
	}
	for _, opt := range opts {
		opt(&spec)
	}
	agent.registerSpec(spec)
}

func (a *Agent) registerSpec(spec handlerSpec) {
	if a.handlers == nil {
		a.handlers = make(map[string]handlerSpec)
	}
	a.handlers[spec.topic] = spec
}

// Start connects to NATS/Redis if needed and registers subscriptions.
func (a *Agent) Start() error {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if a.closed {
		return errors.New("runtime: agent is closed")
	}
	if a.started {
		return errors.New("runtime: agent already started")
	}
	if len(a.handlers) == 0 {
		return errors.New("runtime: no handlers registered")
	}
	frozenMode, err := a.resolveHandshakeMode()
	if err != nil {
		return err
	}
	a.trustMode = frozenMode
	frozenTrust := cloneRuntimeWorkerTrust(a.WorkerTrust)
	a.trustConfig = &frozenTrust
	a.freezeProductionAdmission()
	if err := a.validateProductionStartup(); err != nil {
		a.trustMode = ""
		a.trustConfig = nil
		a.productionConfig = nil
		return err
	}
	if err := a.validateTrustStartup(); err != nil {
		a.trustMode = ""
		a.trustConfig = nil
		a.productionConfig = nil
		return err
	}
	a.configureRuntimeDefaults()
	if err := a.openRuntimeResources(); err != nil {
		return a.failStart(err)
	}
	if _, err := a.performHandshake(context.Background()); err != nil {
		return a.failStart(err)
	}
	if err := a.subscribeHandlers(); err != nil {
		return a.failStart(err)
	}
	if err := a.publishHandshake(); err != nil {
		return a.failStart(err)
	}
	a.startRenewLoop(context.Background())
	a.started = true
	return nil
}

func (a *Agent) publishHandshake() error {
	traceID, err := randomHex(16)
	if err != nil {
		return fmt.Errorf("build handshake: %w", err)
	}

	packet := &agentv1.BusPacket{
		TraceId:         traceID,
		SenderId:        a.SenderID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload: &agentv1.BusPacket_Handshake{
			Handshake: a.capabilityHandshake(),
		},
	}
	// Attach the session token on the legacy handshake packet too —
	// this is the generic protocol-negotiation handshake (not the
	// Phase-2 trust handshake), and a session token on it lets the
	// scheduler tie this broadcast to a trusted agent identity when
	// enforce mode is active.
	a.withSessionToken(packet)
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return fmt.Errorf("validate handshake: %w", err)
	}
	if a.PrivateKey != nil {
		if err := capsdk.SignPacket(packet, a.PrivateKey); err != nil {
			return fmt.Errorf("sign handshake: %w", err)
		}
	}

	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return fmt.Errorf("marshal handshake: %w", err)
	}
	if err := a.NATS.Publish(capsdk.SubjectHandshake, data); err != nil {
		return fmt.Errorf("publish handshake: %w", err)
	}
	return nil
}

// Close drains NATS and closes the blob store if possible.
func (a *Agent) Close() error {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	a.closeOnce.Do(func() {
		a.closed = true
		a.started = false
		a.stopRenewLoop()
		a.unsubscribeAll()
		if conn, ok := a.NATS.(*nats.Conn); ok {
			if err := conn.Drain(); err != nil {
				slog.Warn("nats drain failed during close", "error", err)
			}
		}
		if a.Store != nil {
			a.closeErr = a.Store.Close()
		}
		a.clearSession()
	})
	return a.closeErr
}

func (a *Agent) handleMessage(msg *nats.Msg, spec handlerSpec) {
	start := time.Now()
	var packet *agentv1.BusPacket
	if a.productionAdmissionEnabled() {
		if !a.productionSessionActive() {
			a.Logger.Warn("production admission rejected packet", "error", "authenticated session is not active")
			return
		}
		// CAP-PRODUCTION: verify the exact received wire bytes (never a
		// re-serialized object) and admit atomically for replay BEFORE any
		// protobuf handler runs. A rejection here never reaches Unmarshal-
		// based dispatch below.
		verified, err := a.admitProductionPacketForAudience(msg.Data, msg.Subject)
		if err != nil {
			a.Logger.Warn("production admission rejected packet", "error", err)
			return
		}
		if verified == nil {
			return // identical replay was admitted previously; drop idempotently
		}
		packet = verified
	} else {
		packet = &agentv1.BusPacket{}
		if err := proto.Unmarshal(msg.Data, packet); err != nil {
			a.Logger.Error("decode failed", "error", capsdk.NewMalformedPacketError(err.Error()))
			return
		}
		if err := a.validateInboundPacket(packet); err != nil {
			a.Logger.Warn("invalid packet", "error", err)
			return
		}
	}
	if packet.GetProtocolVersion() != capsdk.DefaultProtocolVersion {
		a.Logger.Warn("invalid packet", "error", "unsupported protocol_version")
		return
	}

	req := packet.GetJobRequest()
	if req == nil || req.GetJobId() == "" {
		return
	}

	a.Metrics.OnJobReceived(req.GetJobId(), req.GetTopic())

	ctxLogger := a.Logger.With(
		"job_id", req.GetJobId(),
		"trace_id", packet.GetTraceId(),
		"topic", req.GetTopic(),
	)
	ctx := Context{Job: req, Packet: packet, Logger: ctxLogger}

	payload, err := a.fetchContext(req.GetContextPtr())
	if err != nil {
		a.publishFailure(ctx, req, err.Error(), 0)
		return
	}
	if a.MaxContextBytes > 0 && len(payload) > a.MaxContextBytes {
		a.publishFailure(ctx, req, "context exceeds max size", 0)
		return
	}

	input, err := spec.decode(payload)
	if err != nil {
		a.publishFailure(ctx, req, capsdk.NewInvalidInputError(err.Error()).Error(), 0)
		return
	}

	wrapped := applyMiddleware(spec.handler, a.middlewares)

	var output any
	var failure error
	for attempt := 0; attempt <= spec.retries; attempt++ {
		output, failure = wrapped(ctx, input)
		if failure == nil {
			break
		}
		ctx.Logger.Warn("handler failed", "attempt", attempt+1, "max_attempts", spec.retries+1, "error", failure)
	}

	execMs := time.Since(start).Milliseconds()
	if failure != nil {
		a.publishFailure(ctx, req, failure.Error(), execMs)
		return
	}

	resultPayload, err := spec.encode(output)
	if err != nil {
		a.publishFailure(ctx, req, fmt.Sprintf("result encode failed: %v", err), execMs)
		return
	}
	if a.MaxResultBytes > 0 && len(resultPayload) > a.MaxResultBytes {
		a.publishFailure(ctx, req, "result exceeds max size", execMs)
		return
	}
	resultKey := fmt.Sprintf("res:%s", req.GetJobId())
	if err := a.storeResult(resultKey, resultPayload); err != nil {
		a.publishFailure(ctx, req, fmt.Sprintf("result write failed: %v", err), execMs)
		return
	}
	resultPtr := pointerForKey(resultKey)

	result := &agentv1.JobResult{
		JobId:       req.GetJobId(),
		Status:      agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
		ResultPtr:   resultPtr,
		WorkerId:    a.SenderID,
		ExecutionMs: execMs,
	}
	a.Metrics.OnJobCompleted(req.GetJobId(), execMs, "SUCCEEDED")
	a.publishResult(ctx, result)
}

func (a *Agent) fetchContext(ptr string) ([]byte, error) {
	if a.Store == nil {
		return nil, errors.New("blob store not initialized")
	}
	key, err := keyFromPointer(ptr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := a.ioContext()
	defer cancel()
	data, err := a.Store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.New("context not found")
		}
		return nil, err
	}
	return data, nil
}

func (a *Agent) storeResult(key string, data []byte) error {
	if a.Store == nil {
		return errors.New("blob store not initialized")
	}
	ctx, cancel := a.ioContext()
	defer cancel()
	return a.Store.Set(ctx, key, data)
}

func (a *Agent) publishFailure(ctx Context, req *agentv1.JobRequest, errorMsg string, execMs int64) {
	a.Metrics.OnJobFailed(req.GetJobId(), errorMsg)
	result := &agentv1.JobResult{
		JobId:        req.GetJobId(),
		Status:       agentv1.JobStatus_JOB_STATUS_FAILED,
		ErrorMessage: errorMsg,
		WorkerId:     a.SenderID,
		ExecutionMs:  execMs,
	}
	a.publishResult(ctx, result)
}

func (a *Agent) publishResult(ctx Context, result *agentv1.JobResult) {
	packet := &agentv1.BusPacket{
		TraceId:         ctx.Packet.GetTraceId(),
		SenderId:        a.SenderID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload: &agentv1.BusPacket_JobResult{
			JobResult: result,
		},
	}
	// Phase-2: attach the session token BEFORE signing so the
	// signature covers the token bytes. A stripped/tampered token
	// would invalidate the packet signature — belt-and-suspenders
	// with the scheduler-side token verification.
	token, _ := a.SessionToken()
	if a.activeHandshakeMode() != HandshakeModeOff && token == "" {
		ctx.Logger.Error("cannot publish privileged result without a live worker session")
		return
	}
	attachSessionToken(packet, token)
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		ctx.Logger.Error("invalid outbound packet", "error", err)
		return
	}
	if a.PrivateKey != nil {
		if err := capsdk.SignPacket(packet, a.PrivateKey); err != nil {
			ctx.Logger.Error("failed signing packet", "error", err)
			return
		}
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		ctx.Logger.Error("failed encoding packet", "error", err)
		return
	}
	if err := a.NATS.Publish(capsdk.SubjectResult, data); err != nil {
		ctx.Logger.Error("publish failed", "error", err)
		return
	}
	if flusher, ok := a.NATS.(interface{ FlushTimeout(time.Duration) error }); ok && a.IOTTimeout > 0 {
		if err := flusher.FlushTimeout(a.IOTTimeout); err != nil {
			ctx.Logger.Warn("flush timeout after result publish", "error", err)
		}
	}
}

func (a *Agent) ioContext() (context.Context, context.CancelFunc) {
	if a.IOTTimeout < 0 {
		return context.Background(), func() {}
	}
	if a.IOTTimeout == 0 {
		return context.WithTimeout(context.Background(), defaultTimeout)
	}
	return context.WithTimeout(context.Background(), a.IOTTimeout)
}

func pointerForKey(key string) string {
	return "redis://" + key
}

func keyFromPointer(ptr string) (string, error) {
	if ptr == "" {
		return "", errors.New("empty pointer")
	}
	if !strings.HasPrefix(ptr, "redis://") {
		return "", errors.New("unsupported pointer scheme")
	}
	key := strings.TrimPrefix(ptr, "redis://")
	if key == "" {
		return "", errors.New("missing pointer key")
	}
	return key, nil
}

// NewRedisBlobStoreWithTLS creates a Redis-backed blob store that applies
// REDIS_TLS_* environment variables (CA cert, client cert/key, server name).
// This is required when Redis uses TLS with a custom CA (e.g. self-signed
// certs). Without this, the rediss:// scheme provides only a basic TLS config
// that trusts system CAs, which fails with custom CAs.
func NewRedisBlobStoreWithTLS(redisURL string) (*RedisBlobStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	tlsCfg, tlsErr := capsdk.RedisTLSConfigFromEnv()
	if tlsErr != nil {
		return nil, fmt.Errorf("redis tls config: %w", tlsErr)
	}
	if tlsCfg != nil {
		opts.TLSConfig = tlsCfg
	}
	client := redis.NewClient(opts)
	return &RedisBlobStore{client: client}, nil
}

// NewRedisBlobStoreWithPing creates a Redis-backed blob store with TLS support
// and immediately verifies the connection with a PING. This surfaces auth
// and TLS failures (NOAUTH, certificate errors) at startup.
func NewRedisBlobStoreWithPing(redisURL string) (*RedisBlobStore, error) {
	store, err := NewRedisBlobStoreWithTLS(redisURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := store.client.Ping(ctx).Err(); err != nil {
		if closeErr := store.Close(); closeErr != nil {
			slog.Warn("redis store close failed after ping error", "error", closeErr)
		}
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return store, nil
}

// PingRedis verifies connectivity and auth to a Redis instance.
// It applies REDIS_TLS_* env vars when the URL uses the rediss:// scheme.
func PingRedis(redisURL string) error {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	tlsCfg, tlsErr := capsdk.RedisTLSConfigFromEnv()
	if tlsErr != nil {
		return fmt.Errorf("redis tls config: %w", tlsErr)
	}
	if tlsCfg != nil {
		opts.TLSConfig = tlsCfg
	}
	client := redis.NewClient(opts)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	return client.Ping(ctx).Err()
}

// ValidateRedisURL returns true if the URL contains valid auth credentials.
// Uses proper URL parsing instead of substring matching to avoid false
// positives from @ characters in non-auth positions (paths, query params).
func ValidateRedisURL(redisURL string) bool {
	u, err := url.Parse(redisURL)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "redis" && scheme != "rediss" {
		return false
	}
	if u.User == nil {
		return false
	}
	_, hasPassword := u.User.Password()
	return u.User.Username() != "" || hasPassword
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
