// Package a2abridge is an A2A 1.0 JSON-RPC bridge, as fixed by
// spec/interop/a2a-mapping.md. It turns authenticated SendMessage calls into
// signed CAP JobRequests on sys.job.submit, and Start applies verified results,
// DLQ outcomes and progress back to the tasks. Resume and cancel come later.
package a2abridge

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go/client"
)

const (
	defaultMaxTasks  = 10_000
	maxMaxTasks      = 1_000_000
	maxIdentityBytes = 256
	// maxRequestBytes bounds the JSON-RPC body, which a2asrv decodes before any
	// handler runs. It is four times the 262 144-byte turn-input bound.
	maxRequestBytes = 1 << 20

	// Spec §3 and §4.1 bounds.
	maxIDBytes        = 128
	maxParts          = 16
	maxTextBytes      = 65_536
	maxDataBytes      = 65_536
	maxDataDepth      = 32
	maxTurnInputBytes = 262_144
	maxMediaTypeLen   = 127
)

// mediaTypeRe is the spec §4.1 type/subtype grammar.
var mediaTypeRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]*/[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]*$`)

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
	// Results reads bounded turn-result envelopes; required by Start, not New.
	Results ResultReader
	// MaxTasks bounds the in-memory task store. 0 means 10 000.
	MaxTasks int
	// Rand supplies task and context ids. Nil means crypto/rand. It need not be
	// safe for concurrent use.
	Rand io.Reader
	// Now supplies timestamps. Nil means time.Now.
	Now func() time.Time
	// ResultKeys maps each trusted sender_id of a result, DLQ or progress
	// packet (workers, and cordum-scheduler for sys.job.dlq) to its legacy
	// signing key. A packet must carry a valid signature by its sender's key.
	ResultKeys map[string]*ecdsa.PublicKey
	// ProductionTrust verifies packets under CAP-PRODUCTION instead. It
	// cannot be combined with ResultKeys.
	ProductionTrust ProductionTrustFunc
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
	results  ResultReader
	store    *store
	randMu   sync.Mutex
	rand     io.Reader
	now      func() time.Time
	// resultKeys and trust verify egress packets; see Start.
	resultKeys map[string]*ecdsa.PublicKey
	trust      ProductionTrustFunc
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
	if err := checkResultKeys(cfg); err != nil {
		return nil, err
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
		pub: cfg.Publisher, inputs: cfg.Inputs, results: cfg.Results, store: newStore(maxTasks),
		rand: cfg.Rand, now: cfg.Now, resultKeys: cfg.ResultKeys, trust: cfg.ProductionTrust,
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

// checkResultKeys fails closed on a malformed verification config. A mode is
// not required here, so an ingress-only Config stays valid; Start requires one.
func checkResultKeys(cfg Config) error {
	if cfg.ProductionTrust != nil && len(cfg.ResultKeys) > 0 {
		return errors.New("a2abridge: set ProductionTrust or ResultKeys, not both")
	}
	for sender, key := range cfg.ResultKeys {
		if sender == "" || key == nil || key.X == nil || key.Y == nil {
			return errors.New("a2abridge: each ResultKeys entry needs a sender_id and a public key")
		}
		if _, err := key.ECDH(); err != nil {
			return fmt.Errorf("a2abridge: ResultKeys[%q] is not a valid public key", sender)
		}
	}
	return nil
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

// turnPart is one part in the envelope grammar of spec §4.3.
type turnPart struct {
	Kind      string          `json:"kind"`
	Text      *string         `json:"text,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	MediaType string          `json:"media_type,omitempty"`
}

// turnInput is the JSON object stored at ctx:<job_id> (spec §4).
type turnInput struct {
	TaskID    string     `json:"task_id"`
	ContextID string     `json:"context_id"`
	Turn      int        `json:"turn"`
	MessageID string     `json:"message_id"`
	Parts     []turnPart `json:"parts"`
}

// constantTimeCompare is a variable so tests can count comparisons.
var constantTimeCompare = subtle.ConstantTimeCompare

// idempotencyKey is the length-prefixed SHA-256 key of spec §3.1.
func idempotencyKey(tenant, principal, taskRef, messageID string) string {
	h := sha256.New()
	h.Write([]byte("cap-a2a-turn-v1"))
	for _, field := range [...]string{tenant, principal, taskRef, messageID} {
		h.Write(binary.BigEndian.AppendUint32(nil, uint32(len(field))))
		h.Write([]byte(field))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// authenticate maps exactly one "Bearer <key>" Authorization value to its
// configured identity. It compares the key's hash with every configured hash
// in constant time and never reports which check failed.
func (i *Ingress) authenticate(ctx context.Context) (Identity, error) {
	cc, ok := a2asrv.CallContextFrom(ctx)
	if !ok || cc == nil {
		return Identity{}, a2a.ErrUnauthenticated
	}
	vals, _ := cc.ServiceParams().Get("authorization")
	if len(vals) != 1 {
		return Identity{}, a2a.ErrUnauthenticated
	}
	scheme, key, found := strings.Cut(vals[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || key == "" {
		return Identity{}, a2a.ErrUnauthenticated
	}
	sum := sha256.Sum256([]byte(key))
	match := -1
	for idx := range i.keys {
		eq := constantTimeCompare(sum[:], i.keys[idx].hash[:])
		match = subtle.ConstantTimeSelect(eq, idx, match)
	}
	if match < 0 {
		return Identity{}, a2a.ErrUnauthenticated
	}
	return i.keys[match].id, nil
}

// SendMessage accepts the first turn of a new task. Identity comes only from
// the API key; request tenant, metadata, parts and headers never set it.
func (i *Ingress) SendMessage(ctx context.Context, req *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	id, err := i.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, invalid("request is required")
	}
	if req.Config != nil && req.Config.PushConfig != nil {
		return nil, a2a.ErrPushNotificationNotSupported // spec §6
	}
	parts, err := validateMessage(req.Message)
	if err != nil {
		return nil, err
	}
	if req.Message.TaskID != "" {
		// Only turn 1 is accepted here; resume belongs to the egress slice.
		if i.store.exists(id, string(req.Message.TaskID)) {
			return nil, a2a.ErrUnsupportedOperation
		}
		return nil, a2a.ErrTaskNotFound
	}
	return i.startTask(ctx, id, req.Message, parts)
}

// startTask reserves the task, writes its input and only then publishes. Any
// failure after the reservation releases it, so a retry publishes again.
func (i *Ingress) startTask(ctx context.Context, id Identity, msg *a2a.Message, parts []turnPart) (*a2a.Task, error) {
	partsJSON, err := json.Marshal(parts)
	if err != nil {
		return nil, a2a.ErrInternalError
	}
	key := idempotencyKey(id.Tenant, id.Principal, "", msg.ID)
	rec := taskRecord{jobID: "a2a-" + key, messageID: msg.ID, turn: 1,
		state: a2a.TaskStateSubmitted, created: i.now().UTC(), parts: parts}
	if rec.id, rec.contextID, err = i.newIDs(msg.ContextID); err != nil {
		return nil, a2a.ErrInternalError
	}
	input, err := json.Marshal(turnInput{TaskID: rec.id, ContextID: rec.contextID, Turn: rec.turn,
		MessageID: rec.messageID, Parts: parts})
	if err != nil {
		return nil, a2a.ErrInternalError
	}
	if len(input) > maxTurnInputBytes {
		return nil, invalid("turn input exceeds 262144 bytes")
	}
	snap, created, err := i.store.reserve(id, key, sha256.Sum256(partsJSON), rec)
	if errors.Is(err, a2a.ErrInvalidParams) {
		return nil, invalid("messageId was already used with different parts")
	}
	if err != nil {
		return nil, a2a.ErrInternalError
	}
	if !created {
		return snap.task(), nil
	}
	if err := i.publish(ctx, id, key, rec, input); err != nil {
		i.store.release(key)
		return nil, a2a.ErrInternalError
	}
	return snap.task(), nil
}

// newIDs draws the task id and, when the client sent none, the context id.
func (i *Ingress) newIDs(clientContext string) (string, string, error) {
	var buf [32]byte
	i.randMu.Lock()
	_, err := io.ReadFull(i.rand, buf[:])
	i.randMu.Unlock()
	if err != nil {
		return "", "", err
	}
	contextID := clientContext
	if contextID == "" {
		contextID = "a2a-ctx-" + hex.EncodeToString(buf[16:])
	}
	return "a2a-task-" + hex.EncodeToString(buf[:16]), contextID, nil
}

// publish writes the turn input and then publishes the JobRequest.
func (i *Ingress) publish(ctx context.Context, id Identity, key string, rec taskRecord, input []byte) error {
	if err := i.inputs.PutTurnInput(ctx, "ctx:"+rec.jobID, input); err != nil {
		return err
	}
	req := &agentv1.JobRequest{
		JobId: rec.jobID, Topic: i.topic, ContextPtr: "redis://ctx:" + rec.jobID,
		WorkflowId: rec.id, StepIndex: 1, TenantId: id.Tenant, PrincipalId: id.Principal,
		Meta: &agentv1.JobMetadata{TenantId: id.Tenant, ActorId: id.Principal, IdempotencyKey: key},
	}
	if i.signer == nil && i.unsigned {
		return client.SubmitUnsigned(ctx, i.pub, req, rec.id, i.sender)
	}
	return client.Submit(ctx, i.pub, req, rec.id, i.sender, i.signer)
}

// GetTask returns a task owned by the caller. A foreign, unknown or malformed
// id gets the same TASK_NOT_FOUND from the same single lookup.
func (i *Ingress) GetTask(ctx context.Context, req *a2a.GetTaskRequest) (*a2a.Task, error) {
	id, err := i.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, a2a.ErrTaskNotFound
	}
	rec, ok := i.store.get(id, string(req.ID))
	if !ok {
		return nil, a2a.ErrTaskNotFound
	}
	return rec.task(), nil
}

// task renders a record as the A2A Task the caller sees.
func (r taskRecord) task() *a2a.Task {
	created := r.created
	parts := make(a2a.ContentParts, 0, len(r.parts))
	for _, p := range r.parts {
		part := &a2a.Part{MediaType: p.MediaType}
		if p.Text != nil {
			part.Content = a2a.Text(*p.Text)
		} else {
			var v any
			_ = json.Unmarshal(p.Data, &v) // our own json.Marshal output
			part.Content = a2a.Data{Value: v}
		}
		parts = append(parts, part)
	}
	msg := &a2a.Message{ID: r.messageID, ContextID: r.contextID, TaskID: a2a.TaskID(r.id),
		Role: a2a.MessageRoleUser, Parts: parts}
	return &a2a.Task{ID: a2a.TaskID(r.id), ContextID: r.contextID,
		Status:  a2a.TaskStatus{State: r.state, Timestamp: &created, Message: r.statusMsg},
		History: append([]*a2a.Message{msg}, r.history...), Artifacts: r.artifacts}
}

func invalid(msg string) error { return a2a.NewError(a2a.ErrInvalidParams, msg) }

// validateMessage applies the spec §3 and §4.1 bounds and converts the parts
// to the envelope grammar. It runs before any store access or publish.
func validateMessage(m *a2a.Message) ([]turnPart, error) {
	switch {
	case m == nil:
		return nil, invalid("message is required")
	case m.Role != a2a.MessageRoleUser:
		return nil, invalid("message role must be ROLE_USER")
	case !validID(m.ID):
		return nil, invalid("messageId must be 1-128 characters of A-Za-z0-9._:-")
	case m.ContextID != "" && !validID(m.ContextID):
		return nil, invalid("contextId must be 1-128 characters of A-Za-z0-9._:-")
	case len(m.Parts) < 1 || len(m.Parts) > maxParts:
		return nil, invalid("message must have 1-16 parts")
	}
	parts := make([]turnPart, 0, len(m.Parts))
	for _, p := range m.Parts {
		tp, err := convertPart(p)
		if err != nil {
			return nil, err
		}
		parts = append(parts, tp)
	}
	return parts, nil
}

func convertPart(p *a2a.Part) (turnPart, error) {
	if p == nil {
		return turnPart{}, invalid("part is required")
	}
	tp := turnPart{MediaType: p.MediaType}
	switch c := p.Content.(type) {
	case a2a.Text:
		if len(c) > maxTextBytes {
			return turnPart{}, invalid("text part exceeds 65536 bytes")
		}
		text := string(c)
		tp.Kind, tp.Text = "text", &text
	case a2a.Data:
		raw, err := json.Marshal(c.Value)
		switch {
		case err != nil:
			return turnPart{}, invalid("data part must be JSON")
		case len(raw) > maxDataBytes:
			return turnPart{}, invalid("data part exceeds 65536 bytes")
		case jsonDepth(raw) > maxDataDepth:
			return turnPart{}, invalid("data part nests deeper than 32")
		}
		tp.Kind, tp.Data = "data", raw
	case a2a.URL, a2a.Raw:
		return turnPart{}, a2a.ErrUnsupportedContentType
	default:
		return turnPart{}, invalid("part content is required")
	}
	if tp.MediaType != "" && (len(tp.MediaType) > maxMediaTypeLen || !mediaTypeRe.MatchString(tp.MediaType)) {
		return turnPart{}, invalid("part mediaType must be type/subtype, at most 127 characters")
	}
	return tp, nil
}

// jsonDepth returns the deepest object/array nesting of compact, valid JSON.
func jsonDepth(raw []byte) int {
	depth, deepest, inString, escaped := 0, 0, false, false
	for _, b := range raw {
		switch {
		case escaped:
			escaped = false
		case inString:
			escaped = b == '\\'
			inString = b != '"'
		case b == '"':
			inString = true
		case b == '{' || b == '[':
			depth++
			deepest = max(deepest, depth)
		case b == '}' || b == ']':
			depth--
		}
	}
	return deepest
}

func validID(s string) bool {
	if s == "" || len(s) > maxIDBytes {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("._:-", c) >= 0) {
			return false
		}
	}
	return true
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
