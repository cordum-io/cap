// Package a2abridge is an A2A 1.0 JSON-RPC ingress. It turns authenticated
// SendMessage calls into signed CAP JobRequests on sys.job.submit, as fixed by
// spec/interop/a2a-mapping.md. Results, resume and cancel are not handled here.
package a2abridge

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/cordum-io/cap/v2/sdk/go/client"
)

const (
	defaultMaxTasks  = 10_000
	maxMaxTasks      = 1_000_000
	maxIdentityBytes = 256
	// maxRequestBytes bounds the JSON-RPC body, which a2asrv decodes before any
	// handler runs. It is four times the 262 144-byte turn-input bound.
	maxRequestBytes = 1 << 20
)

// reservedSenders are the Cordum service identities a bridge must never claim.
var reservedSenders = []string{"api-gateway", "workflow-engine", "cordum-scheduler"}

// Identity is the authenticated caller. It comes only from Config.APIKeys.
type Identity struct {
	Tenant    string
	Principal string
}

// InputWriter stores one turn input where the control plane reads it. The key
// is "ctx:<job_id>" and the job carries context_ptr "redis://ctx:<job_id>".
type InputWriter interface {
	PutTurnInput(ctx context.Context, key string, data []byte) error
}

// Config configures an Ingress. New validates every field and fails closed.
type Config struct {
	// APIKeys maps the lowercase-hex SHA-256 of each API key to its identity.
	APIKeys map[string]Identity
	// Topic is the CAP job topic of every published JobRequest.
	Topic string
	// SenderID is the BusPacket sender_id. It must not be a reserved identity.
	SenderID string
	// SigningKey signs every published packet.
	SigningKey *ecdsa.PrivateKey
	// AllowUnsignedDev permits unsigned packets when SigningKey is nil. Dev only.
	AllowUnsignedDev bool
	// Publisher publishes the packet bytes on sys.job.submit.
	Publisher client.Publisher
	// Inputs stores each turn input before its JobRequest is published.
	Inputs InputWriter
	// MaxTasks bounds the in-memory task store. 0 means 10 000.
	MaxTasks int
	// Rand supplies task and context ids. Nil means crypto/rand. It need not be
	// safe for concurrent use.
	Rand io.Reader
	// Now supplies timestamps. Nil means time.Now.
	Now func() time.Time
}

type keyEntry struct {
	hash [32]byte
	id   Identity
}

// Ingress implements a2asrv.RequestHandler for SendMessage and GetTask.
type Ingress struct {
	keys     []keyEntry
	topic    string
	sender   string
	signer   *ecdsa.PrivateKey
	unsigned bool
	pub      client.Publisher
	inputs   InputWriter
	store    *store
	randMu   sync.Mutex
	rand     io.Reader
	now      func() time.Time
}

var _ a2asrv.RequestHandler = (*Ingress)(nil)

// New validates cfg and returns an Ingress.
func New(cfg Config) (*Ingress, error) {
	keys, err := parseKeys(cfg.APIKeys)
	if err != nil {
		return nil, err
	}
	if err := checkTopic(cfg.Topic); err != nil {
		return nil, err
	}
	if err := checkSender(cfg.SenderID); err != nil {
		return nil, err
	}
	if cfg.Publisher == nil {
		return nil, errors.New("a2abridge: Publisher is required")
	}
	if cfg.Inputs == nil {
		return nil, errors.New("a2abridge: Inputs is required")
	}
	if cfg.SigningKey == nil && !cfg.AllowUnsignedDev {
		return nil, errors.New("a2abridge: SigningKey is required unless AllowUnsignedDev is set")
	}
	maxTasks := cfg.MaxTasks
	if maxTasks == 0 {
		maxTasks = defaultMaxTasks
	}
	if maxTasks < 1 || maxTasks > maxMaxTasks {
		return nil, fmt.Errorf("a2abridge: MaxTasks must be 1-%d", maxMaxTasks)
	}
	ing := &Ingress{
		keys: keys, topic: cfg.Topic, sender: cfg.SenderID,
		signer: cfg.SigningKey, unsigned: cfg.AllowUnsignedDev,
		pub: cfg.Publisher, inputs: cfg.Inputs, store: newStore(maxTasks),
		rand: cfg.Rand, now: cfg.Now,
	}
	if ing.rand == nil {
		ing.rand = rand.Reader
	}
	if ing.now == nil {
		ing.now = time.Now
	}
	return ing, nil
}

func parseKeys(in map[string]Identity) ([]keyEntry, error) {
	if len(in) == 0 {
		return nil, errors.New("a2abridge: at least one API key is required")
	}
	keys := make([]keyEntry, 0, len(in))
	for h, id := range in {
		raw, err := hex.DecodeString(h)
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != h {
			return nil, errors.New("a2abridge: each API key hash must be 64 lowercase hex characters")
		}
		if err := checkLabel("tenant", id.Tenant); err != nil {
			return nil, err
		}
		if err := checkLabel("principal", id.Principal); err != nil {
			return nil, err
		}
		var e keyEntry
		copy(e.hash[:], raw)
		e.id = id
		keys = append(keys, e)
	}
	// Map keys are unique and the lowercase check removes case variants, so
	// no two entries share a hash.
	return keys, nil
}

func checkLabel(name, v string) error {
	if v == "" || len(v) > maxIdentityBytes || !utf8.ValidString(v) {
		return fmt.Errorf("a2abridge: %s must be 1-%d bytes of valid UTF-8", name, maxIdentityBytes)
	}
	if strings.IndexFunc(v, unicode.IsControl) >= 0 {
		return fmt.Errorf("a2abridge: %s must not contain control characters", name)
	}
	return nil
}

func checkTopic(t string) error {
	if t == "" || strings.ContainsAny(t, "*>") || strings.IndexFunc(t, notPrintable) >= 0 {
		return errors.New("a2abridge: Topic must be non-empty with no wildcard, whitespace or control characters")
	}
	return nil
}

func checkSender(s string) error {
	if s == "" || strings.IndexFunc(s, notPrintable) >= 0 {
		return errors.New("a2abridge: SenderID must be non-empty with no whitespace or control characters")
	}
	for _, r := range reservedSenders {
		if strings.EqualFold(s, r) {
			return fmt.Errorf("a2abridge: SenderID %q is a reserved Cordum service identity", r)
		}
	}
	return nil
}

func notPrintable(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }

// Handler serves the ingress as A2A JSON-RPC, with a bounded request body.
func (i *Ingress) Handler() http.Handler {
	return http.MaxBytesHandler(a2asrv.NewJSONRPCHandler(i), maxRequestBytes)
}

// authenticate fails closed until the key check is implemented.
func (i *Ingress) authenticate(context.Context) (Identity, error) {
	return Identity{}, a2a.ErrUnauthenticated
}

// SendMessage accepts the first turn of a new task.
func (i *Ingress) SendMessage(ctx context.Context, _ *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	if _, err := i.authenticate(ctx); err != nil {
		return nil, err
	}
	return nil, a2a.ErrUnsupportedOperation
}

// GetTask returns a task owned by the caller.
func (i *Ingress) GetTask(ctx context.Context, _ *a2a.GetTaskRequest) (*a2a.Task, error) {
	if _, err := i.authenticate(ctx); err != nil {
		return nil, err
	}
	return nil, a2a.ErrUnsupportedOperation
}

// deny authenticates the caller and then returns err.
func (i *Ingress) deny(ctx context.Context, err error) error {
	if _, authErr := i.authenticate(ctx); authErr != nil {
		return authErr
	}
	return err
}

func (i *Ingress) denySeq(ctx context.Context, err error) iter.Seq2[a2a.Event, error] {
	err = i.deny(ctx, err)
	return func(yield func(a2a.Event, error) bool) { yield(nil, err) }
}

// ListTasks is not supported.
func (i *Ingress) ListTasks(ctx context.Context, _ *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	return nil, i.deny(ctx, a2a.ErrUnsupportedOperation)
}

// CancelTask is not supported by the ingress; the egress slice owns it.
func (i *Ingress) CancelTask(ctx context.Context, _ *a2a.CancelTaskRequest) (*a2a.Task, error) {
	return nil, i.deny(ctx, a2a.ErrUnsupportedOperation)
}

// SubscribeToTask is not supported (no streaming in v1).
func (i *Ingress) SubscribeToTask(ctx context.Context, _ *a2a.SubscribeToTaskRequest) iter.Seq2[a2a.Event, error] {
	return i.denySeq(ctx, a2a.ErrUnsupportedOperation)
}

// SendStreamingMessage is not supported (no streaming in v1).
func (i *Ingress) SendStreamingMessage(ctx context.Context, _ *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error] {
	return i.denySeq(ctx, a2a.ErrUnsupportedOperation)
}

// GetTaskPushConfig is not supported.
func (i *Ingress) GetTaskPushConfig(ctx context.Context, _ *a2a.GetTaskPushConfigRequest) (*a2a.PushConfig, error) {
	return nil, i.deny(ctx, a2a.ErrPushNotificationNotSupported)
}

// ListTaskPushConfigs is not supported.
func (i *Ingress) ListTaskPushConfigs(ctx context.Context, _ *a2a.ListTaskPushConfigRequest) (*a2a.ListTaskPushConfigResponse, error) {
	return nil, i.deny(ctx, a2a.ErrPushNotificationNotSupported)
}

// CreateTaskPushConfig is not supported.
func (i *Ingress) CreateTaskPushConfig(ctx context.Context, _ *a2a.PushConfig) (*a2a.PushConfig, error) {
	return nil, i.deny(ctx, a2a.ErrPushNotificationNotSupported)
}

// DeleteTaskPushConfig is not supported.
func (i *Ingress) DeleteTaskPushConfig(ctx context.Context, _ *a2a.DeleteTaskPushConfigRequest) error {
	return i.deny(ctx, a2a.ErrPushNotificationNotSupported)
}

// GetExtendedAgentCard is not supported.
func (i *Ingress) GetExtendedAgentCard(ctx context.Context, _ *a2a.GetExtendedAgentCardRequest) (*a2a.AgentCard, error) {
	return nil, i.deny(ctx, a2a.ErrUnsupportedOperation)
}
