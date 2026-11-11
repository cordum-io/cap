package a2abridge

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

const (
	testTopic  = "job.a2a.agent"
	testSender = "a2a-bridge"
)

var (
	fixedTime  = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	taskIDRe   = regexp.MustCompile(`^a2a-task-[0-9a-f]{32}$`)
	contextRe  = regexp.MustCompile(`^a2a-ctx-[0-9a-f]{32}$`)
	testIdents = map[string]Identity{
		"key-alice":   {Tenant: "tenant-a", Principal: "alice"},
		"key-bob":     {Tenant: "tenant-a", Principal: "bob"},
		"key-mallory": {Tenant: "tenant-b", Principal: "mallory"},
	}
)

// ---- fixtures ----

type publishCall struct {
	subject string
	data    []byte
	ok      bool
}

type fakePublisher struct {
	mu        sync.Mutex
	calls     []publishCall
	failNext  int
	onPublish func(subject string, data []byte)
}

func (p *fakePublisher) Publish(subject string, data []byte) error {
	p.mu.Lock()
	fail := p.failNext > 0
	if fail {
		p.failNext--
	}
	p.calls = append(p.calls, publishCall{subject: subject, data: bytes.Clone(data), ok: !fail})
	hook := p.onPublish
	p.mu.Unlock()
	if hook != nil {
		hook(subject, data)
	}
	if fail {
		return errors.New("nats: connection closed secret-detail")
	}
	return nil
}

func (p *fakePublisher) attempts() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

// count is the number of successful publishes.
func (p *fakePublisher) count() int {
	return len(p.packets(nil))
}

func (p *fakePublisher) packets(t *testing.T) []*agentv1.BusPacket {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []*agentv1.BusPacket
	for _, c := range p.calls {
		if !c.ok {
			continue
		}
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(c.data, &pkt); err != nil && t != nil {
			t.Fatalf("decode packet: %v", err)
		}
		if c.subject != capsdk.SubjectSubmit && t != nil {
			t.Fatalf("published on %q, want %q", c.subject, capsdk.SubjectSubmit)
		}
		out = append(out, &pkt)
	}
	return out
}

type fakeInputs struct {
	mu     sync.Mutex
	data   map[string][]byte
	writes int
	fail   error
}

func (f *fakeInputs) PutTurnInput(_ context.Context, key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.fail != nil {
		err := f.fail
		f.fail = nil
		return err
	}
	f.data[key] = bytes.Clone(data)
	return nil
}

func (f *fakeInputs) get(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	return v, ok
}

func (f *fakeInputs) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}

type harness struct {
	ing *Ingress
	pub *fakePublisher
	in  *fakeInputs
	key *ecdsa.PrivateKey
}

func hashKey(k string) string {
	s := sha256.Sum256([]byte(k))
	return hex.EncodeToString(s[:])
}

func testConfig(t *testing.T) (Config, *fakePublisher, *fakeInputs, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]Identity{}
	for k, id := range testIdents {
		keys[hashKey(k)] = id
	}
	pub := &fakePublisher{}
	in := &fakeInputs{data: map[string][]byte{}}
	return Config{
		APIKeys: keys, Topic: testTopic, SenderID: testSender, SigningKey: key,
		Publisher: pub, Inputs: in, Rand: mathrand.NewChaCha8([32]byte{7}),
		Now: func() time.Time { return fixedTime },
	}, pub, in, key
}

func newHarness(t *testing.T, mutate ...func(*Config)) *harness {
	t.Helper()
	cfg, pub, in, key := testConfig(t)
	for _, m := range mutate {
		m(&cfg)
	}
	ing, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &harness{ing: ing, pub: pub, in: in, key: key}
}

func callerCtx(auth ...string) context.Context {
	params := map[string][]string{}
	if len(auth) > 0 {
		params["Authorization"] = auth
	}
	ctx, _ := a2asrv.NewCallContext(context.Background(), a2asrv.NewServiceParams(params))
	return ctx
}

func as(key string) context.Context { return callerCtx("Bearer " + key) }

func textMsg(id string, texts ...string) *a2a.Message {
	parts := make([]*a2a.Part, 0, len(texts))
	for _, s := range texts {
		parts = append(parts, a2a.NewTextPart(s))
	}
	return &a2a.Message{ID: id, Role: a2a.MessageRoleUser, Parts: parts}
}

func (h *harness) send(ctx context.Context, msg *a2a.Message) (*a2a.Task, error) {
	res, err := h.ing.SendMessage(ctx, &a2a.SendMessageRequest{Message: msg})
	if err != nil {
		return nil, err
	}
	task, ok := res.(*a2a.Task)
	if !ok || task == nil {
		return nil, fmt.Errorf("SendMessage returned %T, want *a2a.Task", res)
	}
	return task, nil
}

func (h *harness) mustSend(t *testing.T, ctx context.Context, msg *a2a.Message) *a2a.Task {
	t.Helper()
	task, err := h.send(ctx, msg)
	if err != nil {
		t.Fatalf("SendMessage(%s): %v", msg.ID, err)
	}
	return task
}

// refKey is an independent implementation of the spec §3.1 formula.
func refKey(fields ...string) string {
	var b bytes.Buffer
	b.WriteString("cap-a2a-turn-v1")
	for _, f := range fields {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(f)))
		b.Write(n[:])
		b.WriteString(f)
	}
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:])
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
}

func (h *harness) wantNothingStored(t *testing.T) {
	t.Helper()
	if n := h.pub.attempts(); n != 0 {
		t.Fatalf("publish attempts = %d, want 0", n)
	}
	if n := h.in.writeCount(); n != 0 {
		t.Fatalf("input writes = %d, want 0", n)
	}
	if n := h.ing.store.size(); n != 0 {
		t.Fatalf("store entries = %d, want 0", n)
	}
}

// ---- wire helpers ----

type rpcError struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []map[string]any `json:"data"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

func (e *rpcError) reason() string {
	for _, d := range e.Data {
		if d["@type"] == "type.googleapis.com/google.rpc.ErrorInfo" {
			s, _ := d["reason"].(string)
			return s
		}
	}
	return ""
}

func postRaw(t *testing.T, url string, auth []string, body []byte) (int, rpcResponse) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, a := range auth {
		req.Header.Add("Authorization", a)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func rpc(t *testing.T, url string, auth []string, method, params string) (int, rpcResponse) {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":` + params + `}`
	return postRaw(t, url, auth, []byte(body))
}

const wireSend = `{"message":{"messageId":"m-1","role":"ROLE_USER","parts":[{"text":"hi"}]}}`

// ---- DoD-1: authentication ----

func TestAuthRejectsMissingUnknownMalformedKeys(t *testing.T) {
	cases := map[string]context.Context{
		"no call context":  context.Background(),
		"no header":        callerCtx(),
		"bearer only":      callerCtx("Bearer"),
		"empty bearer key": callerCtx("Bearer "),
		"unknown key":      callerCtx("Bearer key-eve"),
		"two values":       callerCtx("Bearer key-alice", "Bearer key-alice"),
		"basic scheme":     callerCtx("Basic x"),
		"hash as key":      callerCtx("Bearer " + hashKey("key-alice")),
		"no scheme":        callerCtx("key-alice"),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.send(ctx, textMsg("m-1", "hi"))
			wantErr(t, err, a2a.ErrUnauthenticated)
			_, err = h.ing.GetTask(ctx, &a2a.GetTaskRequest{ID: "a2a-task-00000000000000000000000000000000"})
			wantErr(t, err, a2a.ErrUnauthenticated)
			h.wantNothingStored(t)
		})
	}
	t.Run("valid key and case-insensitive scheme", func(t *testing.T) {
		h := newHarness(t)
		h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
		h.mustSend(t, callerCtx("bearer key-bob"), textMsg("m-1", "hi"))
		if n := h.pub.count(); n != 2 {
			t.Fatalf("publishes = %d, want 2", n)
		}
	})
}

func TestAuthWireCodeIsUnauthenticated(t *testing.T) {
	h := newHarness(t)
	srv := httptest.NewServer(h.ing.Handler())
	defer srv.Close()
	for _, auth := range [][]string{nil, {"Bearer key-eve"}, {"Basic x"}} {
		status, resp := rpc(t, srv.URL, auth, "SendMessage", wireSend)
		if status != http.StatusOK || resp.Error == nil || resp.Error.Code != -31401 {
			t.Fatalf("auth %v: status %d error %+v, want HTTP 200 code -31401", auth, status, resp.Error)
		}
	}
	h.wantNothingStored(t)
	_, resp := rpc(t, srv.URL, []string{"Bearer key-alice"}, "SendMessage", wireSend)
	var res struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if resp.Error != nil || json.Unmarshal(resp.Result, &res) != nil || !taskIDRe.MatchString(res.Task.ID) {
		t.Fatalf("valid wire SendMessage: error %+v result %s", resp.Error, resp.Result)
	}
	if n := h.pub.count(); n != 1 {
		t.Fatalf("publishes = %d, want 1", n)
	}
}

func TestAuthComparesEveryKey(t *testing.T) {
	h := newHarness(t)
	var calls int
	orig := constantTimeCompare
	constantTimeCompare = func(a, b []byte) int { calls++; return orig(a, b) }
	t.Cleanup(func() { constantTimeCompare = orig })
	for _, key := range []string{"key-alice", "key-bob", "key-mallory", "key-eve"} {
		calls = 0
		_, _ = h.ing.GetTask(as(key), &a2a.GetTaskRequest{ID: "a2a-task-x"})
		if calls != len(testIdents) {
			t.Fatalf("%s: %d constant-time comparisons, want %d", key, calls, len(testIdents))
		}
	}
}

func TestHandlerBoundsRequestBody(t *testing.T) {
	h := newHarness(t)
	srv := httptest.NewServer(h.ing.Handler())
	defer srv.Close()
	big := `{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{"messageId":"m-1","role":"ROLE_USER","parts":[{"text":"` +
		strings.Repeat("a", maxRequestBytes) + `"}]}}}`
	_, resp := postRaw(t, srv.URL, []string{"Bearer key-alice"}, []byte(big))
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("oversized body: error %+v, want code -32700", resp.Error)
	}
	h.wantNothingStored(t)
}

// ---- DoD-2: identity ----

func TestIdentityComesOnlyFromKeyMap(t *testing.T) {
	h := newHarness(t)
	ctx, cc := a2asrv.NewCallContext(context.Background(), a2asrv.NewServiceParams(map[string][]string{
		"Authorization": {"Bearer key-alice"},
		"X-Tenant-Id":   {"tenant-evil"},
		"Traceparent":   {"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
		"Baggage":       {"tenant_id=tenant-evil,principal_id=root"},
	}))
	cc.User = a2asrv.NewAuthenticatedUser("mallory", nil)
	msg := &a2a.Message{
		ID: "m-1", Role: a2a.MessageRoleUser, ContextID: "client-ctx-1",
		Metadata: map[string]any{"tenant_id": "tenant-evil"},
		Parts: []*a2a.Part{
			a2a.NewTextPart("hi"),
			{Content: a2a.Data{Value: map[string]any{"tenant_id": "tenant-evil"}}, MediaType: "application/json"},
		},
	}
	res, err := h.ing.SendMessage(ctx, &a2a.SendMessageRequest{
		Tenant: "tenant-evil", Message: msg, Metadata: map[string]any{"tenant_id": "tenant-evil"},
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	task := res.(*a2a.Task)
	if !taskIDRe.MatchString(string(task.ID)) || task.ContextID != "client-ctx-1" {
		t.Fatalf("task id %q context %q", task.ID, task.ContextID)
	}
	if task.Status.State != a2a.TaskStateSubmitted || task.Status.Timestamp == nil || !task.Status.Timestamp.Equal(fixedTime) {
		t.Fatalf("status = %+v", task.Status)
	}
	pkts := h.pub.packets(t)
	if len(pkts) != 1 {
		t.Fatalf("publishes = %d, want 1", len(pkts))
	}
	pkt := pkts[0]
	if err := capsdk.VerifyPacketSignature(pkt, &h.key.PublicKey); err != nil {
		t.Fatalf("signature: %v", err)
	}
	if pkt.GetSenderId() != testSender || pkt.GetTraceId() != string(task.ID) {
		t.Fatalf("sender %q trace %q", pkt.GetSenderId(), pkt.GetTraceId())
	}
	key := refKey("tenant-a", "alice", "", "m-1")
	jobID := "a2a-" + key
	jr := pkt.GetJobRequest()
	got := []string{jr.GetJobId(), jr.GetWorkflowId(), jr.GetTopic(), jr.GetContextPtr(), jr.GetParentJobId(),
		jr.GetTenantId(), jr.GetPrincipalId(), jr.GetMeta().GetTenantId(), jr.GetMeta().GetActorId(), jr.GetMeta().GetIdempotencyKey()}
	want := []string{jobID, string(task.ID), testTopic, "redis://ctx:" + jobID, "",
		"tenant-a", "alice", "tenant-a", "alice", key}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("JobRequest fields\n got %q\nwant %q", got, want)
	}
	if jr.GetStepIndex() != 1 || len(jr.GetEnv()) != 0 || len(jr.GetLabels()) != 0 || len(jr.GetMeta().GetLabels()) != 0 {
		t.Fatalf("step_index %d env %v labels %v meta.labels %v", jr.GetStepIndex(), jr.GetEnv(), jr.GetLabels(), jr.GetMeta().GetLabels())
	}
	input, ok := h.in.get("ctx:" + jobID)
	wantInput := `{"task_id":"` + string(task.ID) + `","context_id":"client-ctx-1","turn":1,"message_id":"m-1",` +
		`"parts":[{"kind":"text","text":"hi"},{"kind":"data","data":{"tenant_id":"tenant-evil"},"media_type":"application/json"}]}`
	if !ok || string(input) != wantInput {
		t.Fatalf("turn input\n got %s\nwant %s", input, wantInput)
	}
	if _, err := h.ing.GetTask(as("key-alice"), &a2a.GetTaskRequest{ID: task.ID}); err != nil {
		t.Fatalf("GetTask as alice: %v", err)
	}
	_, err = h.ing.GetTask(as("key-mallory"), &a2a.GetTaskRequest{ID: task.ID})
	wantErr(t, err, a2a.ErrTaskNotFound)
}

func TestServerGeneratesContextID(t *testing.T) {
	h := newHarness(t)
	task := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	if !contextRe.MatchString(task.ContextID) {
		t.Fatalf("context id %q", task.ContextID)
	}
}

func TestUnsignedPublishOnlyWithDevFlag(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.SigningKey = nil; c.AllowUnsignedDev = true })
	h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	if sig := h.pub.packets(t)[0].GetSignature(); len(sig) != 0 {
		t.Fatalf("dev mode signature length %d, want 0", len(sig))
	}
	h = newHarness(t, func(c *Config) { c.AllowUnsignedDev = true })
	h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	if err := capsdk.VerifyPacketSignature(h.pub.packets(t)[0], &h.key.PublicKey); err != nil {
		t.Fatalf("key plus dev flag must still sign: %v", err)
	}
}

func TestIdempotencyKeyIsLengthPrefixed(t *testing.T) {
	hexRe := regexp.MustCompile(`^[0-9a-f]{64}$`)
	k := idempotencyKey("ab", "c", "", "m")
	if !hexRe.MatchString(k) || k != refKey("ab", "c", "", "m") {
		t.Fatalf("key %q, want %q", k, refKey("ab", "c", "", "m"))
	}
	pairs := [][2]string{
		{idempotencyKey("ab", "c", "", "m"), idempotencyKey("a", "bc", "", "m")},
		{idempotencyKey("t", "alice", "", "m"), idempotencyKey("t", "bob", "", "m")},
		{idempotencyKey("t", "p", "x", ""), idempotencyKey("t", "p", "", "x")},
		{idempotencyKey("t", "p", "task", "m"), idempotencyKey("t", "p", "", "m")},
	}
	for i, p := range pairs {
		if p[0] == p[1] {
			t.Fatalf("pair %d collides: %s", i, p[0])
		}
	}
}

// ---- DoD-3: idempotency and ordering ----

func TestDuplicateMessageIDReturnsSameTaskWithoutPublishing(t *testing.T) {
	h := newHarness(t)
	first := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	again := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	if again.ID != first.ID || again.ContextID != first.ContextID {
		t.Fatalf("duplicate returned %s/%s, want %s/%s", again.ID, again.ContextID, first.ID, first.ContextID)
	}
	if h.pub.count() != 1 || h.in.writeCount() != 1 {
		t.Fatalf("publishes %d writes %d, want 1 and 1", h.pub.count(), h.in.writeCount())
	}
	_, err := h.send(as("key-alice"), textMsg("m-1", "different"))
	wantErr(t, err, a2a.ErrInvalidParams)
	if n := h.pub.count(); n != 1 {
		t.Fatalf("publishes after conflicting duplicate = %d, want 1", n)
	}
	bob := h.mustSend(t, as("key-bob"), textMsg("m-1", "hi"))
	if bob.ID == first.ID {
		t.Fatal("bob's message landed in alice's task")
	}
	if n := h.pub.count(); n != 2 {
		t.Fatalf("publishes = %d, want 2", n)
	}
}

func TestReservationPrecedesPublish(t *testing.T) {
	h := newHarness(t)
	var hookErr error
	h.pub.onPublish = func(_ string, data []byte) {
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(data, &pkt); err != nil {
			hookErr = err
			return
		}
		jr := pkt.GetJobRequest()
		if _, err := h.ing.GetTask(as("key-alice"), &a2a.GetTaskRequest{ID: a2a.TaskID(jr.GetWorkflowId())}); err != nil {
			hookErr = fmt.Errorf("task not reserved before publish: %w", err)
			return
		}
		if _, ok := h.in.get("ctx:" + jr.GetJobId()); !ok {
			hookErr = errors.New("turn input not written before publish")
		}
	}
	done := make(chan error, 1)
	go func() { _, err := h.send(as("key-alice"), textMsg("m-1", "hi")); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SendMessage: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SendMessage deadlocked: is the store locked across publish?")
	}
	if h.pub.attempts() != 1 || hookErr != nil {
		t.Fatalf("attempts %d hook error %v", h.pub.attempts(), hookErr)
	}
}

func runConcurrent(n int, fn func(i int) (*a2a.Task, error)) ([]*a2a.Task, []error) {
	tasks, errs := make([]*a2a.Task, n), make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tasks[i], errs[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	return tasks, errs
}

func TestConcurrentDuplicatesPublishOnce(t *testing.T) {
	h := newHarness(t)
	tasks, errs := runConcurrent(32, func(int) (*a2a.Task, error) {
		return h.send(as("key-alice"), textMsg("m-1", "hi"))
	})
	for i := range tasks {
		if errs[i] != nil || tasks[i].ID != tasks[0].ID {
			t.Fatalf("goroutine %d: err %v, task %v", i, errs[i], tasks[i])
		}
	}
	if h.pub.count() != 1 || h.in.writeCount() != 1 {
		t.Fatalf("publishes %d writes %d, want 1 and 1", h.pub.count(), h.in.writeCount())
	}
}

func TestConcurrentDistinctMessagesPublishEach(t *testing.T) {
	h := newHarness(t)
	tasks, errs := runConcurrent(32, func(i int) (*a2a.Task, error) {
		return h.send(as("key-alice"), textMsg(fmt.Sprintf("m-%d", i), "hi"))
	})
	seen := map[a2a.TaskID]bool{}
	for i := range tasks {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		seen[tasks[i].ID] = true
	}
	if len(seen) != 32 || h.pub.count() != 32 || h.ing.store.size() != 32 {
		t.Fatalf("distinct tasks %d publishes %d store %d, want 32", len(seen), h.pub.count(), h.ing.store.size())
	}
}

func TestPublishFailureReleasesReservation(t *testing.T) {
	h := newHarness(t)
	h.pub.failNext = 1
	task, err := h.send(as("key-alice"), textMsg("m-1", "hi"))
	wantErr(t, err, a2a.ErrInternalError)
	if task != nil || strings.Contains(err.Error(), "secret-detail") {
		t.Fatalf("task %v err %q", task, err)
	}
	if n := h.ing.store.size(); n != 0 {
		t.Fatalf("store entries after failed publish = %d, want 0", n)
	}
	retry := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	if h.pub.attempts() != 2 || h.pub.count() != 1 {
		t.Fatalf("attempts %d publishes %d, want 2 and 1", h.pub.attempts(), h.pub.count())
	}
	if _, err := h.ing.GetTask(as("key-alice"), &a2a.GetTaskRequest{ID: retry.ID}); err != nil {
		t.Fatalf("GetTask after retry: %v", err)
	}
}

func TestInputFailureReleasesReservation(t *testing.T) {
	h := newHarness(t)
	h.in.fail = errors.New("redis: secret-detail")
	_, err := h.send(as("key-alice"), textMsg("m-1", "hi"))
	wantErr(t, err, a2a.ErrInternalError)
	if strings.Contains(err.Error(), "secret-detail") || h.pub.attempts() != 0 || h.ing.store.size() != 0 {
		t.Fatalf("err %q attempts %d store %d", err, h.pub.attempts(), h.ing.store.size())
	}
	h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	if n := h.pub.count(); n != 1 {
		t.Fatalf("publishes after retry = %d, want 1", n)
	}
}

// ---- DoD-4: owner scoping ----

func TestGetTaskHidesOtherOwners(t *testing.T) {
	h := newHarness(t)
	task := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	cases := []struct {
		name, key string
		id        a2a.TaskID
	}{
		{"other tenant", "key-mallory", task.ID},
		{"other principal", "key-bob", task.ID},
		{"unknown id", "key-alice", "a2a-task-ffffffffffffffffffffffffffffffff"},
		{"malformed id", "key-alice", "../../etc/passwd"},
		{"empty id", "key-alice", ""},
	}
	var firstMsg string
	for _, c := range cases {
		_, err := h.ing.GetTask(as(c.key), &a2a.GetTaskRequest{ID: c.id})
		wantErr(t, err, a2a.ErrTaskNotFound)
		if firstMsg == "" {
			firstMsg = err.Error()
		} else if err.Error() != firstMsg {
			t.Fatalf("%s: message %q differs from %q", c.name, err.Error(), firstMsg)
		}
	}
	got, err := h.ing.GetTask(as("key-alice"), &a2a.GetTaskRequest{ID: task.ID})
	if err != nil || got.ID != task.ID || len(got.History) != 1 || got.History[0].Parts[0].Text() != "hi" {
		t.Fatalf("owner GetTask: %v %+v", err, got)
	}
	srv := httptest.NewServer(h.ing.Handler())
	defer srv.Close()
	var first *rpcError
	for _, c := range cases {
		idJSON, _ := json.Marshal(string(c.id))
		_, resp := rpc(t, srv.URL, []string{"Bearer " + c.key}, "GetTask", `{"id":`+string(idJSON)+`}`)
		e := resp.Error
		if e == nil || e.Code != -32001 || e.reason() != "TASK_NOT_FOUND" {
			t.Fatalf("%s wire: %+v", c.name, e)
		}
		if first == nil {
			first = e
		} else if e.Message != first.Message {
			t.Fatalf("%s wire message %q differs from %q", c.name, e.Message, first.Message)
		}
	}
}

func TestSendToExistingTaskIsUnsupported(t *testing.T) {
	h := newHarness(t)
	task := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	next := textMsg("m-2", "more")
	next.TaskID = task.ID
	_, err := h.send(as("key-alice"), next)
	wantErr(t, err, a2a.ErrUnsupportedOperation)
	_, err = h.send(as("key-mallory"), next)
	wantErr(t, err, a2a.ErrTaskNotFound)
	next.TaskID = "a2a-task-ffffffffffffffffffffffffffffffff"
	_, err = h.send(as("key-alice"), next)
	wantErr(t, err, a2a.ErrTaskNotFound)
	if n := h.pub.count(); n != 1 {
		t.Fatalf("publishes = %d, want 1", n)
	}
}

// Spec §6: push notifications are excluded, so an inline push config is
// rejected rather than silently dropped, and nothing is published.
func TestSendWithPushConfigIsRejected(t *testing.T) {
	h := newHarness(t)
	_, err := h.ing.SendMessage(as("key-alice"), &a2a.SendMessageRequest{
		Message: textMsg("m-1", "hi"),
		Config:  &a2a.SendMessageConfig{PushConfig: &a2a.PushConfig{URL: "https://hook.example/cb"}},
	})
	wantErr(t, err, a2a.ErrPushNotificationNotSupported)
	h.wantNothingStored(t)
	// A config without a push config is accepted.
	res, err := h.ing.SendMessage(as("key-alice"), &a2a.SendMessageRequest{
		Message: textMsg("m-1", "hi"), Config: &a2a.SendMessageConfig{ReturnImmediately: true},
	})
	if err != nil || res == nil || h.pub.count() != 1 {
		t.Fatalf("config without push: res %v err %v publishes %d", res, err, h.pub.count())
	}
}

// ---- DoD-5: parts validation ----

func nested(depth int) any {
	var v any = "x"
	for i := range depth {
		if i%2 == 0 {
			v = []any{v}
		} else {
			v = map[string]any{"k": v}
		}
	}
	return v
}

func dataOfSize(n int) any { return map[string]any{"k": strings.Repeat("a", n-8)} } // {"k":"…"}

func partsMsg(parts ...*a2a.Part) *a2a.Message {
	return &a2a.Message{ID: "m-1", Role: a2a.MessageRoleUser, Parts: parts}
}

func dataPart(v any) *a2a.Part { return &a2a.Part{Content: a2a.Data{Value: v}} }

func TestPartsValidation(t *testing.T) {
	text := func(s string) *a2a.Part { return a2a.NewTextPart(s) }
	withMedia := func(mt string) *a2a.Part { p := text("hi"); p.MediaType = mt; return p }
	msg := func(f func(m *a2a.Message)) *a2a.Message { m := textMsg("m-1", "hi"); f(m); return m }
	seventeen := make([]*a2a.Part, 17)
	for i := range seventeen {
		seventeen[i] = text("p")
	}
	big := strings.Repeat("a", 60_000)
	rejected := []struct {
		name string
		msg  *a2a.Message
		want error
	}{
		{"nil message", nil, a2a.ErrInvalidParams},
		{"zero parts", partsMsg(), a2a.ErrInvalidParams},
		{"17 parts", partsMsg(seventeen...), a2a.ErrInvalidParams},
		{"url part", partsMsg(a2a.NewFileURLPart("https://example.com/a.pdf", "application/pdf")), a2a.ErrUnsupportedContentType},
		{"raw part", partsMsg(a2a.NewRawPart([]byte("x"))), a2a.ErrUnsupportedContentType},
		{"nil part", partsMsg(nil), a2a.ErrInvalidParams},
		{"nil content", partsMsg(&a2a.Part{}), a2a.ErrInvalidParams},
		{"text 65537 bytes", partsMsg(text(strings.Repeat("a", 65_537))), a2a.ErrInvalidParams},
		{"data 65537 bytes", partsMsg(dataPart(dataOfSize(65_537))), a2a.ErrInvalidParams},
		{"data depth 33", partsMsg(dataPart(nested(33))), a2a.ErrInvalidParams},
		{"data not JSON", partsMsg(dataPart(func() {})), a2a.ErrInvalidParams},
		{"turn input over 262144", partsMsg(text(big), text(big), text(big), text(big), text(big)), a2a.ErrInvalidParams},
		{"media type 128 chars", partsMsg(withMedia("a/" + strings.Repeat("b", 126))), a2a.ErrInvalidParams},
		{"media type no slash", partsMsg(withMedia("textplain")), a2a.ErrInvalidParams},
		{"messageId empty", msg(func(m *a2a.Message) { m.ID = "" }), a2a.ErrInvalidParams},
		{"messageId 129 bytes", msg(func(m *a2a.Message) { m.ID = strings.Repeat("m", 129) }), a2a.ErrInvalidParams},
		{"messageId slash", msg(func(m *a2a.Message) { m.ID = "m/1" }), a2a.ErrInvalidParams},
		{"contextId 129 bytes", msg(func(m *a2a.Message) { m.ContextID = strings.Repeat("c", 129) }), a2a.ErrInvalidParams},
		{"contextId space", msg(func(m *a2a.Message) { m.ContextID = "ctx 1" }), a2a.ErrInvalidParams},
		{"role agent", msg(func(m *a2a.Message) { m.Role = a2a.MessageRoleAgent }), a2a.ErrInvalidParams},
		{"role unspecified", msg(func(m *a2a.Message) { m.Role = a2a.MessageRoleUnspecified }), a2a.ErrInvalidParams},
	}
	for _, c := range rejected {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.send(as("key-alice"), c.msg)
			wantErr(t, err, c.want)
			h.wantNothingStored(t)
		})
	}
	t.Run("nil request", func(t *testing.T) {
		h := newHarness(t)
		_, err := h.ing.SendMessage(as("key-alice"), nil)
		wantErr(t, err, a2a.ErrInvalidParams)
		h.wantNothingStored(t)
	})
	accepted := map[string]*a2a.Message{
		"text 65536 bytes":     partsMsg(text(strings.Repeat("a", 65_536))),
		"data 65536 bytes":     partsMsg(dataPart(dataOfSize(65_536))),
		"data depth 32":        partsMsg(dataPart(nested(32))),
		"16 parts":             partsMsg(seventeen[:16]...),
		"media type 127 chars": partsMsg(withMedia("a/" + strings.Repeat("b", 125))),
		"messageId 128 bytes":  msg(func(m *a2a.Message) { m.ID = "Az09._:-" + strings.Repeat("m", 120) }),
		"contextId 128 bytes":  msg(func(m *a2a.Message) { m.ContextID = strings.Repeat("c", 128) }),
		"empty text":           partsMsg(text("")),
	}
	for name, m := range accepted {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.mustSend(t, as("key-alice"), m)
			if n := h.pub.count(); n != 1 {
				t.Fatalf("publishes = %d, want 1", n)
			}
		})
	}
}

// ---- spec vectors ----

type vecIdentity struct{ Tenant, Principal string }

type vecPart struct {
	Kind      string          `json:"kind"`
	Text      string          `json:"text"`
	URL       string          `json:"url"`
	Data      json.RawMessage `json:"data"`
	MediaType string          `json:"media_type"`
}

type vecTurn struct {
	MessageID string    `json:"message_id"`
	TaskRef   *string   `json:"task_ref"`
	Parts     []vecPart `json:"parts"`
}

type vecCase struct {
	ID      string       `json:"id"`
	Caller  *vecIdentity `json:"caller"`
	Owner   *vecIdentity `json:"owner"`
	Turns   []vecTurn    `json:"turns"`
	A2ACall *struct {
		Method  string `json:"method"`
		TaskRef string `json:"task_ref"`
	} `json:"a2a_call"`
	ExpectedPublishes struct {
		JobRequest int `json:"job_request"`
	} `json:"expected_publishes"`
	ExpectedError string `json:"expected_error"`
}

var vectorErrors = map[string]error{
	"":                           nil,
	"TASK_NOT_FOUND":             a2a.ErrTaskNotFound,
	"TASK_NOT_CANCELABLE":        a2a.ErrTaskNotCancelable,
	"UNSUPPORTED_OPERATION":      a2a.ErrUnsupportedOperation,
	"CONTENT_TYPE_NOT_SUPPORTED": a2a.ErrUnsupportedContentType,
	"INVALID_PARAMS":             a2a.ErrInvalidParams,
}

func keyFor(t *testing.T, id *vecIdentity) string {
	t.Helper()
	for k, v := range testIdents {
		if v.Tenant == id.Tenant && v.Principal == id.Principal {
			return k
		}
	}
	t.Fatalf("no test key for %+v", id)
	return ""
}

func vecMessage(t *testing.T, turn vecTurn, first a2a.TaskID) *a2a.Message {
	t.Helper()
	m := &a2a.Message{ID: turn.MessageID, Role: a2a.MessageRoleUser}
	if turn.TaskRef != nil {
		m.TaskID = first
	}
	for _, p := range turn.Parts {
		switch p.Kind {
		case "text":
			m.Parts = append(m.Parts, a2a.NewTextPart(p.Text))
		case "url":
			m.Parts = append(m.Parts, a2a.NewFileURLPart(a2a.URL(p.URL), p.MediaType))
		case "data":
			var v any
			if err := json.Unmarshal(p.Data, &v); err != nil {
				t.Fatal(err)
			}
			m.Parts = append(m.Parts, dataPart(v))
		default:
			t.Fatalf("unknown vector part kind %q", p.Kind)
		}
	}
	return m
}

func TestVectorsIngressCases(t *testing.T) {
	raw, err := os.ReadFile("../../spec/interop/vectors/a2a-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []vecCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"oversize-part-count-rejected": true, "url-part-rejected": true, "duplicate-message-id": true,
		"send-to-busy-task": true, "cross-tenant-get-task": true, "cross-principal-get-task": true, "unknown-task-get-task": true}
	ran := 0
	for _, c := range file.Cases {
		if !want[c.ID] {
			continue
		}
		ran++
		t.Run(c.ID, func(t *testing.T) { runVector(t, c) })
	}
	if ran != len(want) {
		t.Fatalf("ran %d ingress vectors, want %d", ran, len(want))
	}
}

func runVector(t *testing.T, c vecCase) {
	h := newHarness(t)
	caller := keyFor(t, c.Caller)
	owner := caller
	if c.Owner != nil {
		owner = keyFor(t, c.Owner)
	}
	target, ok := vectorErrors[c.ExpectedError]
	if !ok {
		t.Fatalf("unmapped expected_error %q", c.ExpectedError)
	}
	setup := c.Turns
	if c.A2ACall.Method == "SendMessage" {
		setup = c.Turns[:len(c.Turns)-1]
	}
	var first a2a.TaskID
	for _, turn := range setup {
		task := h.mustSend(t, as(owner), vecMessage(t, turn, first))
		if first == "" {
			first = task.ID
		}
	}
	var err error
	switch c.A2ACall.Method {
	case "SendMessage":
		_, err = h.send(as(caller), vecMessage(t, c.Turns[len(c.Turns)-1], first))
	case "GetTask":
		id := first
		if c.A2ACall.TaskRef == "unknown" {
			id = "a2a-task-ffffffffffffffffffffffffffffffff"
		}
		_, err = h.ing.GetTask(as(caller), &a2a.GetTaskRequest{ID: id})
	default:
		t.Fatalf("unexpected method %q", c.A2ACall.Method)
	}
	if target == nil && err != nil || target != nil && !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
	if n := h.pub.count(); n != c.ExpectedPublishes.JobRequest {
		t.Fatalf("publishes = %d, want %d", n, c.ExpectedPublishes.JobRequest)
	}
}

// ---- unsupported methods ----

func TestUnsupportedMethodsAuthenticateFirst(t *testing.T) {
	h := newHarness(t)
	first := func(seq func(func(a2a.Event, error) bool)) error {
		var got error
		seq(func(_ a2a.Event, e error) bool { got = e; return false })
		return got
	}
	calls := map[string]struct {
		call func(context.Context) error
		want error
	}{
		"ListTasks": {func(ctx context.Context) error { _, e := h.ing.ListTasks(ctx, &a2a.ListTasksRequest{}); return e }, a2a.ErrUnsupportedOperation},
		"CancelTask": {func(ctx context.Context) error {
			_, e := h.ing.CancelTask(ctx, &a2a.CancelTaskRequest{ID: "a2a-task-x"})
			return e
		}, a2a.ErrUnsupportedOperation},
		"SubscribeToTask": {func(ctx context.Context) error {
			return first(h.ing.SubscribeToTask(ctx, &a2a.SubscribeToTaskRequest{ID: "a2a-task-x"}))
		}, a2a.ErrUnsupportedOperation},
		"SendStreamingMessage": {func(ctx context.Context) error {
			return first(h.ing.SendStreamingMessage(ctx, &a2a.SendMessageRequest{Message: textMsg("m-1", "hi")}))
		}, a2a.ErrUnsupportedOperation},
		"GetExtendedAgentCard": {func(ctx context.Context) error {
			_, e := h.ing.GetExtendedAgentCard(ctx, &a2a.GetExtendedAgentCardRequest{})
			return e
		}, a2a.ErrUnsupportedOperation},
		"GetTaskPushConfig": {func(ctx context.Context) error {
			_, e := h.ing.GetTaskPushConfig(ctx, &a2a.GetTaskPushConfigRequest{})
			return e
		}, a2a.ErrPushNotificationNotSupported},
		"ListTaskPushConfigs": {func(ctx context.Context) error {
			_, e := h.ing.ListTaskPushConfigs(ctx, &a2a.ListTaskPushConfigRequest{})
			return e
		}, a2a.ErrPushNotificationNotSupported},
		"CreateTaskPushConfig": {func(ctx context.Context) error {
			_, e := h.ing.CreateTaskPushConfig(ctx, &a2a.PushConfig{})
			return e
		}, a2a.ErrPushNotificationNotSupported},
		"DeleteTaskPushConfig": {func(ctx context.Context) error {
			return h.ing.DeleteTaskPushConfig(ctx, &a2a.DeleteTaskPushConfigRequest{})
		}, a2a.ErrPushNotificationNotSupported},
	}
	for name, c := range calls {
		t.Run(name, func(t *testing.T) {
			wantErr(t, c.call(context.Background()), a2a.ErrUnauthenticated)
			wantErr(t, c.call(as("key-eve")), a2a.ErrUnauthenticated)
			wantErr(t, c.call(as("key-alice")), c.want)
		})
	}
	h.wantNothingStored(t)
}

// ---- configuration ----

func TestNewFailsClosed(t *testing.T) {
	alice := hashKey("key-alice")
	setKey := func(hash string, id Identity) func(*Config) {
		return func(c *Config) { c.APIKeys = map[string]Identity{hash: id} }
	}
	ok := Identity{Tenant: "t", Principal: "p"}
	cases := map[string]func(*Config){
		"nil keys":            func(c *Config) { c.APIKeys = nil },
		"empty keys":          func(c *Config) { c.APIKeys = map[string]Identity{} },
		"uppercase hash":      setKey(strings.ToUpper(alice), ok),
		"short hash":          setKey(alice[:62], ok),
		"non-hex hash":        setKey(strings.Repeat("g", 64), ok),
		"empty tenant":        setKey(alice, Identity{Principal: "p"}),
		"empty principal":     setKey(alice, Identity{Tenant: "t"}),
		"tenant 257 bytes":    setKey(alice, Identity{Tenant: strings.Repeat("t", 257), Principal: "p"}),
		"tenant newline":      setKey(alice, Identity{Tenant: "t\n", Principal: "p"}),
		"principal NUL":       setKey(alice, Identity{Tenant: "t", Principal: "p\x00"}),
		"tenant invalid utf8": setKey(alice, Identity{Tenant: "\xff", Principal: "p"}),
		"empty topic":         func(c *Config) { c.Topic = "" },
		"topic star":          func(c *Config) { c.Topic = "job.*" },
		"topic gt":            func(c *Config) { c.Topic = "job.>" },
		"topic space":         func(c *Config) { c.Topic = "job a" },
		"empty sender":        func(c *Config) { c.SenderID = "" },
		"sender space":        func(c *Config) { c.SenderID = "a2a bridge" },
		"sender api-gateway":  func(c *Config) { c.SenderID = "api-gateway" },
		"sender workflow":     func(c *Config) { c.SenderID = "workflow-engine" },
		"sender scheduler":    func(c *Config) { c.SenderID = "cordum-scheduler" },
		"sender gateway case": func(c *Config) { c.SenderID = "API-Gateway" },
		"nil publisher":       func(c *Config) { c.Publisher = nil },
		"nil inputs":          func(c *Config) { c.Inputs = nil },
		"no key no dev flag":  func(c *Config) { c.SigningKey = nil },
		"negative max tasks":  func(c *Config) { c.MaxTasks = -1 },
		"max tasks over 1e6":  func(c *Config) { c.MaxTasks = 1_000_001 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, _, _, _ := testConfig(t)
			mutate(&cfg)
			if ing, err := New(cfg); err == nil || ing != nil {
				t.Fatalf("New accepted invalid config: %v", err)
			}
		})
	}
	valid := map[string]func(*Config){
		"defaults":           func(*Config) {},
		"unsigned dev":       func(c *Config) { c.SigningKey = nil; c.AllowUnsignedDev = true },
		"max tasks 1":        func(c *Config) { c.MaxTasks = 1 },
		"max tasks 1e6":      func(c *Config) { c.MaxTasks = 1_000_000 },
		"nil rand and clock": func(c *Config) { c.Rand = nil; c.Now = nil },
	}
	for name, mutate := range valid {
		t.Run("valid "+name, func(t *testing.T) {
			cfg, _, _, _ := testConfig(t)
			mutate(&cfg)
			if _, err := New(cfg); err != nil {
				t.Fatalf("New: %v", err)
			}
		})
	}
	cfg, _, _, _ := testConfig(t)
	ing, _ := New(cfg)
	if ing == nil || ing.store.max != defaultMaxTasks {
		t.Fatalf("MaxTasks 0 must default to %d", defaultMaxTasks)
	}
}
