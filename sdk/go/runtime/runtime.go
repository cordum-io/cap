package runtime

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
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
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultNATSURL   = "nats://127.0.0.1:4222"
	defaultRedisURL  = "redis://127.0.0.1:6379/0"
	defaultTimeout   = 5 * time.Second
	defaultMaxBytes  = 2 * 1024 * 1024
	defaultSenderID  = "cap-runtime"
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

func (s *InMemoryBlobStore) Set(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]byte, len(data))
	copy(out, data)
	s.data[key] = out
	return nil
}

func (s *InMemoryBlobStore) Close() error {
	return nil
}

// Context provides handler context and metadata.
type Context struct {
	Job    *agentv1.JobRequest
	Packet *agentv1.BusPacket
	Logger *log.Logger
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
	NATS            NATSConn
	NATSURL         string
	RedisURL        string
	Store           BlobStore
	PublicKeys      map[string]*ecdsa.PublicKey
	PrivateKey      *ecdsa.PrivateKey
	SenderID        string
	Retries         int
	IOTTimeout      time.Duration
	MaxContextBytes int
	MaxResultBytes  int
	Logger          *log.Logger

	handlers map[string]handlerSpec
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
	if len(a.handlers) == 0 {
		return errors.New("runtime: no handlers registered")
	}
	if a.SenderID == "" {
		a.SenderID = defaultSenderID
	}
	if a.Logger == nil {
		a.Logger = log.New(os.Stdout, "cap-runtime ", log.LstdFlags)
	}
	if a.IOTTimeout == 0 {
		a.IOTTimeout = defaultTimeout
	}
	if a.MaxContextBytes == 0 {
		a.MaxContextBytes = defaultMaxBytes
	}
	if a.MaxResultBytes == 0 {
		a.MaxResultBytes = defaultMaxBytes
	}
	if a.NATS == nil {
		url := strings.TrimSpace(a.NATSURL)
		if url == "" {
			url = strings.TrimSpace(os.Getenv("NATS_URL"))
		}
		if url == "" {
			url = defaultNATSURL
		}
		connectTimeout := a.IOTTimeout
		if connectTimeout <= 0 {
			connectTimeout = defaultTimeout
		}
		nc, err := nats.Connect(url, nats.Name(a.SenderID), nats.Timeout(connectTimeout))
		if err != nil {
			return err
		}
		a.NATS = nc
	}
	if a.Store == nil {
		url := strings.TrimSpace(a.RedisURL)
		if url == "" {
			url = strings.TrimSpace(os.Getenv("REDIS_URL"))
		}
		if url == "" {
			url = defaultRedisURL
		}
		store, err := NewRedisBlobStore(url)
		if err != nil {
			return err
		}
		a.Store = store
	}

	for _, spec := range a.handlers {
		handler := spec
		_, err := a.NATS.QueueSubscribe(handler.topic, handler.topic, func(msg *nats.Msg) {
			a.handleMessage(msg, handler)
		})
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", handler.topic, err)
		}
	}
	return nil
}

// Close drains NATS and closes the blob store if possible.
func (a *Agent) Close() error {
	if conn, ok := a.NATS.(*nats.Conn); ok {
		_ = conn.Drain()
	}
	if a.Store != nil {
		return a.Store.Close()
	}
	return nil
}

func (a *Agent) handleMessage(msg *nats.Msg, spec handlerSpec) {
	start := time.Now()
	var packet agentv1.BusPacket
	if err := proto.Unmarshal(msg.Data, &packet); err != nil {
		a.Logger.Printf("runtime: decode failed: %v", err)
		return
	}

	if a.PublicKeys != nil {
		pub, ok := a.PublicKeys[packet.GetSenderId()]
		if !ok {
			a.Logger.Printf("runtime: no public key for sender %s", packet.GetSenderId())
			return
		}
		if len(packet.GetSignature()) == 0 {
			a.Logger.Printf("runtime: missing signature for sender %s", packet.GetSenderId())
			return
		}
		if err := capsdk.VerifyPacketSignature(&packet, pub); err != nil {
			a.Logger.Printf("runtime: invalid signature from sender %s: %v", packet.GetSenderId(), err)
			return
		}
	}

	req := packet.GetJobRequest()
	if req == nil || req.GetJobId() == "" {
		return
	}

	ctxLogger := log.New(a.Logger.Writer(),
		fmt.Sprintf("job_id=%s trace_id=%s topic=%s ", req.GetJobId(), packet.GetTraceId(), req.GetTopic()),
		a.Logger.Flags(),
	)
	ctx := Context{Job: req, Packet: &packet, Logger: ctxLogger}

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
		a.publishFailure(ctx, req, fmt.Sprintf("input validation failed: %v", err), 0)
		return
	}

	var output any
	var failure error
	for attempt := 0; attempt <= spec.retries; attempt++ {
		output, failure = spec.handler(ctx, input)
		if failure == nil {
			break
		}
		ctx.Logger.Printf("runtime: handler failed (attempt %d/%d): %v", attempt+1, spec.retries+1, failure)
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
	if a.PrivateKey != nil {
		if err := capsdk.SignPacket(packet, a.PrivateKey); err != nil {
			ctx.Logger.Printf("runtime: failed signing packet: %v", err)
			return
		}
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		ctx.Logger.Printf("runtime: failed encoding packet: %v", err)
		return
	}
	if err := a.NATS.Publish(capsdk.SubjectResult, data); err != nil {
		ctx.Logger.Printf("runtime: publish failed: %v", err)
		return
	}
	if flusher, ok := a.NATS.(interface{ FlushTimeout(time.Duration) error }); ok && a.IOTTimeout > 0 {
		_ = flusher.FlushTimeout(a.IOTTimeout)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
