package a2abridge

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	mathrandv2 "math/rand/v2"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var ( // Every helper is eg-prefixed: this file needs nothing from ingress_test.go.
	egNow    = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	egIdents = map[string]Identity{
		"key-alice":   {Tenant: "tenant-a", Principal: "alice"},
		"key-bob":     {Tenant: "tenant-a", Principal: "bob"},
		"key-mallory": {Tenant: "tenant-b", Principal: "mallory"},
	}
)

const (
	egWorker  = "worker-1"
	egSched   = "cordum-scheduler"
	egRunning = agentv1.JobStatus_JOB_STATUS_RUNNING
	egFailed  = agentv1.JobStatus_JOB_STATUS_FAILED
)

type egSent struct {
	subject string
	raw     []byte
	pkt     *agentv1.BusPacket
}

// egPublisher records every (subject, raw, decoded BusPacket).
type egPublisher struct {
	mu   sync.Mutex
	sent []egSent
}

func (p *egPublisher) Publish(subject string, data []byte) error {
	pkt := &agentv1.BusPacket{}
	if err := proto.Unmarshal(data, pkt); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, egSent{subject: subject, raw: slices.Clone(data), pkt: pkt})
	return nil
}

// counts returns the JobRequests on sys.job.submit and JobCancels on sys.job.cancel.
func (p *egPublisher) counts() (requests, cancels int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.sent {
		switch {
		case s.subject == capsdk.SubjectSubmit && s.pkt.GetJobRequest() != nil:
			requests++
		case s.subject == capsdk.SubjectCancel && s.pkt.GetJobCancel() != nil:
			cancels++
		}
	}
	return requests, cancels
}

func (p *egPublisher) lastJob() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sent[len(p.sent)-1].pkt.GetJobRequest().GetJobId()
}

// egInputs is a mutex map.
type egInputs struct{ sync.Map }

func (f *egInputs) PutTurnInput(_ context.Context, key string, data []byte) error {
	f.Store(key, slices.Clone(data))
	return nil
}

type egHarness struct {
	ing                   *Ingress
	pub                   *egPublisher
	worker, sched, bridge *ecdsa.PrivateKey
}

func egOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func egKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	egOK(t, err)
	return k
}

// egConfig is a valid Config with ResultKeys {worker-1, cordum-scheduler}.
func egConfig(t *testing.T) (*egHarness, Config) {
	t.Helper()
	h := &egHarness{pub: &egPublisher{}, worker: egKey(t), sched: egKey(t), bridge: egKey(t)}
	keys := map[string]Identity{}
	for k, id := range egIdents {
		sum := sha256.Sum256([]byte(k))
		keys[hex.EncodeToString(sum[:])] = id
	}
	return h, Config{
		APIKeys: keys, Topic: "job.a2a.agent", SenderID: "a2a-bridge", SigningKey: h.bridge,
		Publisher: h.pub, Inputs: &egInputs{},
		Rand: mathrandv2.NewChaCha8([32]byte{9}), Now: func() time.Time { return egNow },
		ResultKeys: map[string]*ecdsa.PublicKey{egWorker: &h.worker.PublicKey, egSched: &h.sched.PublicKey},
	}
}

func newEgHarness(t *testing.T, mutate ...func(*Config)) *egHarness {
	t.Helper()
	h, cfg := egConfig(t)
	for _, m := range mutate {
		m(&cfg)
	}
	ing, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h.ing = ing
	return h
}

func egAs(key string) context.Context {
	params := a2asrv.NewServiceParams(map[string][]string{"Authorization": {"Bearer " + key}})
	ctx, _ := a2asrv.NewCallContext(context.Background(), params)
	return ctx
}

func (h *egHarness) send(key string, m *a2a.Message) (*a2a.Task, error) {
	res, err := h.ing.SendMessage(egAs(key), &a2a.SendMessageRequest{
		Message: m, Config: &a2a.SendMessageConfig{ReturnImmediately: true}})
	if err != nil {
		return nil, err
	}
	task, ok := res.(*a2a.Task)
	if !ok || task == nil {
		return nil, errors.New("SendMessage did not return a task")
	}
	return task, nil
}

// start opens a task for key and returns its id and its turn-1 job id.
func (h *egHarness) start(t *testing.T, key, msgID string) (a2a.TaskID, string) {
	t.Helper()
	task, err := h.send(key, &a2a.Message{ID: msgID, Role: a2a.MessageRoleUser,
		Parts: a2a.ContentParts{a2a.NewTextPart("Book me a flight")}})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	return task.ID, h.pub.lastJob()
}

func egStatusOrder(events []a2a.Event) []a2a.TaskState {
	var out []a2a.TaskState
	for _, e := range events {
		if ev, ok := e.(*a2a.TaskStatusUpdateEvent); ok {
			out = append(out, ev.Status.State)
		}
	}
	return out
}

// state is what GetTask shows key; UNSPECIFIED when key cannot see the task.
func (h *egHarness) state(key string, id a2a.TaskID) a2a.TaskState {
	task, err := h.ing.GetTask(egAs(key), &a2a.GetTaskRequest{ID: id})
	if err != nil {
		return a2a.TaskStateUnspecified
	}
	return task.Status.State
}

// want asserts the state GetTask reports and the exact status-event order.
func (h *egHarness) want(t *testing.T, key string, id a2a.TaskID, order ...a2a.TaskState) {
	t.Helper()
	rec, _ := h.ing.store.get(egIdents[key], string(id))
	if st, got := h.state(key, id), egStatusOrder(rec.events); st != order[len(order)-1] || !slices.Equal(got, order) {
		t.Fatalf("state %s, events %v; want events %v", st, got, order)
	}
}

func egResultPkt(sender, job string, st agentv1.JobStatus) *agentv1.BusPacket {
	return &agentv1.BusPacket{TraceId: "trace-" + job, SenderId: sender, CreatedAt: timestamppb.New(egNow),
		ProtocolVersion: capsdk.DefaultProtocolVersion, Payload: &agentv1.BusPacket_JobResult{
			JobResult: &agentv1.JobResult{JobId: job, Status: st, WorkerId: sender}}}
}

func egProgressPkt(job string, st agentv1.JobStatus) *agentv1.BusPacket {
	return &agentv1.BusPacket{TraceId: "trace-" + job, SenderId: egWorker, CreatedAt: timestamppb.New(egNow),
		ProtocolVersion: capsdk.DefaultProtocolVersion, Payload: &agentv1.BusPacket_JobProgress{
			JobProgress: &agentv1.JobProgress{JobId: job, Status: st, Percent: 50}}}
}

// egSign signs pkt with key (nil leaves it unsigned) and marshals it.
func egSign(t *testing.T, key *ecdsa.PrivateKey, pkt *agentv1.BusPacket) []byte {
	t.Helper()
	if key != nil {
		egOK(t, capsdk.SignPacket(pkt, key))
	}
	raw, err := capsdk.MarshalDeterministic(pkt)
	egOK(t, err)
	return raw
}

func (h *egHarness) result(t *testing.T, job string, st agentv1.JobStatus) []byte {
	return egSign(t, h.worker, egResultPkt(egWorker, job, st))
}

func (h *egHarness) dlq(t *testing.T, job string, st agentv1.JobStatus) []byte {
	return egSign(t, h.sched, egResultPkt(egSched, job, st))
}

func (h *egHarness) progress(t *testing.T, job string, st agentv1.JobStatus) []byte {
	return egSign(t, h.worker, egProgressPkt(job, st))
}

type egDrop struct {
	name, subject string
	raw           []byte
}

func TestUnverifiedOrUnknownPacketsAreDropped(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	good := h.result(t, job, egFailed)
	tampered := slices.Clone(good)
	tampered[len(tampered)-1] ^= 0x01
	mismatch := egResultPkt(egWorker, job, egFailed)
	mismatch.GetJobResult().WorkerId = "worker-2"
	oversize := egResultPkt(egWorker, job, egFailed)
	oversize.GetJobResult().ErrorMessage = strings.Repeat("x", capsdk.DefaultProductionMaxRawBytes)
	drops := []egDrop{
		{"unsigned", capsdk.SubjectResult, egSign(t, nil, egResultPkt(egWorker, job, egFailed))},
		{"wrong key", capsdk.SubjectResult, egSign(t, egKey(t), egResultPkt(egWorker, job, egFailed))},
		{"unknown sender", capsdk.SubjectResult, egSign(t, h.worker, egResultPkt("worker-9", job, egFailed))},
		{"tampered last byte", capsdk.SubjectResult, tampered},
		{"malformed", capsdk.SubjectResult, []byte{0xff, 0xff, 0xff, 0x01}},
		{"empty", capsdk.SubjectResult, nil},
		{"oversize", capsdk.SubjectResult, egSign(t, h.worker, oversize)},
		{"worker_id != sender_id", capsdk.SubjectResult, egSign(t, h.worker, mismatch)},
		{"unknown job", capsdk.SubjectResult, h.result(t, "a2a-unknown-job", egFailed)},
		{"result on the progress subject", capsdk.SubjectProgress, good},
		{"progress on the result subject", capsdk.SubjectResult, h.progress(t, job, egRunning)},
		{"progress on the dlq subject", capsdk.SubjectDLQ, h.progress(t, job, egRunning)},
	}
	for _, d := range drops {
		t.Run(d.name, func(t *testing.T) {
			h.ing.deliver(d.subject, d.raw)
			h.want(t, "key-alice", id, a2a.TaskStateSubmitted)
		})
	}
	h.ing.deliver(capsdk.SubjectResult, good)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateFailed)
	// Terminal is final: a later result or progress for the job changes nothing.
	h.ing.deliver(capsdk.SubjectDLQ, h.dlq(t, job, agentv1.JobStatus_JOB_STATUS_DENIED))
	h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job, egRunning))
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateFailed)
	if req, cancels := h.pub.counts(); req != 1 || cancels != 0 {
		t.Fatalf("publishes: %d JobRequests, %d JobCancels; want 1 and 0", req, cancels)
	}
}

// egProd signs pkt under CAP-PRODUCTION for audience aud, claiming tenant.
func egProd(t *testing.T, key *ecdsa.PrivateKey, aud, tenant string, pkt *agentv1.BusPacket) []byte {
	t.Helper()
	pkt.Identity = &agentv1.IdentityBinding{TenantId: tenant}
	msgID := make([]byte, 16)
	_, err := crand.Read(msgID)
	egOK(t, err)
	pkt.SignatureMetadata = &agentv1.SignatureMetadata{
		ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm,
		MessageId: msgID, Audience: aud, ExpiresAt: timestamppb.New(time.Now().Add(2 * time.Minute)), KeyId: "k1",
	}
	raw, err := capsdk.SignProductionPacket(pkt, key)
	egOK(t, err)
	return raw
}

func TestProductionModeVerifiesEveryPacket(t *testing.T) {
	var worker *ecdsa.PublicKey
	h := newEgHarness(t, func(c *Config) {
		worker = c.ResultKeys[egWorker]
		c.ResultKeys = nil
		c.ProductionTrust = func(audience, tenant, sender string) (capsdk.ProductionTrustStore, bool) {
			if sender != egWorker {
				return capsdk.ProductionTrustStore{}, false
			}
			return capsdk.ProductionTrustStore{Audience: audience, Tenant: tenant, Sender: sender,
				PublicKeys: map[string]*ecdsa.PublicKey{"k1": worker}}, true
		}
	})
	id, job := h.start(t, "key-alice", "msg-1")
	failed := func() *agentv1.BusPacket { return egResultPkt(egWorker, job, egFailed) }
	drops := []egDrop{
		{"legacy-signed result", capsdk.SubjectResult, h.result(t, job, egFailed)},
		{"tenant-b claim on a tenant-a job", capsdk.SubjectResult, egProd(t, h.worker, capsdk.SubjectResult, "tenant-b", failed())},
		{"progress audience on the result subject", capsdk.SubjectResult, egProd(t, h.worker, capsdk.SubjectProgress, "tenant-a", failed())},
		{"untrusted key", capsdk.SubjectResult, egProd(t, egKey(t), capsdk.SubjectResult, "tenant-a", failed())},
		{"untrusted sender", capsdk.SubjectResult, egProd(t, h.worker, capsdk.SubjectResult, "tenant-a", egResultPkt("worker-9", job, egFailed))},
	}
	for _, d := range drops {
		t.Run(d.name, func(t *testing.T) {
			h.ing.deliver(d.subject, d.raw)
			h.want(t, "key-alice", id, a2a.TaskStateSubmitted)
		})
	}
	h.ing.deliver(capsdk.SubjectProgress, egProd(t, h.worker, capsdk.SubjectProgress, "tenant-a", egProgressPkt(job, egRunning)))
	h.ing.deliver(capsdk.SubjectResult, egProd(t, h.worker, capsdk.SubjectResult, "tenant-a", failed()))
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateWorking, a2a.TaskStateFailed)
}

func TestProgressNeverMovesBackOrEndsATurn(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job, egRunning))
	for _, st := range []agentv1.JobStatus{agentv1.JobStatus_JOB_STATUS_PENDING, agentv1.JobStatus_JOB_STATUS_SUCCEEDED, egFailed, 0} {
		h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job, st))
		h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateWorking)
	}
	// A FAILED_RETRYABLE result is progress-class: no event, and the turn goes on.
	h.ing.deliver(capsdk.SubjectResult, h.result(t, job, agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE))
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateWorking)
	// A turn whose job is already settled (turnDone, as slice 2's reserveResult
	// leaves it while the envelope is read) ignores every later packet.
	id2, job2 := h.start(t, "key-alice", "msg-2")
	h.ing.store.mu.Lock()
	h.ing.store.tasks[taskKey{tenant: "tenant-a", principal: "alice", id: string(id2)}].turnDone = true
	h.ing.store.mu.Unlock()
	h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job2, egRunning))
	h.ing.deliver(capsdk.SubjectResult, h.result(t, job2, egFailed))
	h.want(t, "key-alice", id2, a2a.TaskStateSubmitted)
}

// egSubs records each subscription; the failAt-th Subscribe call fails.
type egSubs struct {
	nc       *nats.Conn
	failAt   int
	subjects []string
	subs     []*nats.Subscription
}

func (s *egSubs) Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error) {
	s.subjects = append(s.subjects, subject)
	if s.nc == nil || len(s.subjects) == s.failAt {
		return nil, errors.New("subscribe refused")
	}
	sub, err := s.nc.Subscribe(subject, cb)
	if err == nil {
		s.subs = append(s.subs, sub)
	}
	return sub, err
}

func TestStartAndNewFailClosed(t *testing.T) {
	if _, err := newEgHarness(t).ing.Start(nil); err == nil {
		t.Fatal("Start(nil) succeeded")
	}
	none := newEgHarness(t, func(c *Config) { c.ResultKeys = nil })
	sub := &egSubs{}
	if _, err := none.ing.Start(sub); err == nil || len(sub.subjects) != 0 {
		t.Fatalf("Start without a verification mode: err %v, %d subscriptions", err, len(sub.subjects))
	}
	// deliver fails closed on its own too: with no mode a signed result is dropped.
	id, job := none.start(t, "key-alice", "msg-1")
	none.ing.deliver(capsdk.SubjectResult, none.result(t, job, egFailed))
	none.want(t, "key-alice", id, a2a.TaskStateSubmitted)
	// AllowUnsignedDev is the only mode that accepts an unsigned packet.
	dev := newEgHarness(t, func(c *Config) { c.ResultKeys, c.SigningKey, c.AllowUnsignedDev = nil, nil, true })
	id, job = dev.start(t, "key-alice", "msg-1")
	dev.ing.deliver(capsdk.SubjectResult, egSign(t, nil, egResultPkt(egWorker, job, egFailed)))
	dev.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateFailed)
	offCurve := &ecdsa.PublicKey{Curve: elliptic.P256(), X: big.NewInt(1), Y: big.NewInt(1)}
	trust := func(string, string, string) (capsdk.ProductionTrustStore, bool) {
		return capsdk.ProductionTrustStore{}, false
	}
	bad := map[string]func(*Config){
		"both modes":   func(c *Config) { c.ProductionTrust = trust },
		"empty sender": func(c *Config) { c.ResultKeys = map[string]*ecdsa.PublicKey{"": &egKey(t).PublicKey} },
		"nil key":      func(c *Config) { c.ResultKeys = map[string]*ecdsa.PublicKey{egWorker: nil} },
		"off-curve":    func(c *Config) { c.ResultKeys = map[string]*ecdsa.PublicKey{egWorker: offCurve} },
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			_, cfg := egConfig(t)
			mutate(&cfg)
			if _, err := New(cfg); err == nil {
				t.Fatal("New accepted the config")
			}
		})
	}
}

// egEventually polls cond for at most 5 s.
func egEventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func TestEgressOverRealNATS(t *testing.T) {
	// Loopback only: binding every interface trips the Windows firewall prompt.
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	egOK(t, err)
	go ns.Start()
	defer ns.Shutdown()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats-server not ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	egOK(t, err)
	defer nc.Close()
	h := newEgHarness(t)
	id1, job1 := h.start(t, "key-alice", "msg-1")
	id2, job2 := h.start(t, "key-alice", "msg-2")

	partial := &egSubs{nc: nc, failAt: 2}
	if _, err := h.ing.Start(partial); err == nil || len(partial.subs) != 1 || partial.subs[0].IsValid() {
		t.Fatalf("partial Start: err %v, subscriptions %d, first still valid", err, len(partial.subs))
	}
	subs := &egSubs{nc: nc}
	stop, err := h.ing.Start(subs)
	egOK(t, err)
	got := slices.Sorted(slices.Values(subs.subjects))
	if want := []string{capsdk.SubjectDLQ, capsdk.SubjectProgress, capsdk.SubjectResult}; !slices.Equal(got, want) {
		t.Fatalf("subscribed %v, want %v", got, want)
	}
	egOK(t, nc.Flush())
	pc, err := nats.Connect(ns.ClientURL())
	egOK(t, err)
	defer pc.Close()
	publish := func(subject string, raw []byte) {
		egOK(t, pc.Publish(subject, raw))
		egOK(t, pc.Flush())
	}
	publish(capsdk.SubjectProgress, h.progress(t, job1, egRunning))
	egEventually(t, "task 1 WORKING", func() bool { return h.state("key-alice", id1) == a2a.TaskStateWorking })
	publish(capsdk.SubjectResult, h.result(t, job1, egFailed))
	publish(capsdk.SubjectDLQ, h.dlq(t, job2, agentv1.JobStatus_JOB_STATUS_DENIED))
	egEventually(t, "task 1 FAILED and task 2 REJECTED", func() bool {
		return h.state("key-alice", id1) == a2a.TaskStateFailed && h.state("key-alice", id2) == a2a.TaskStateRejected
	})
	h.want(t, "key-alice", id1, a2a.TaskStateSubmitted, a2a.TaskStateWorking, a2a.TaskStateFailed)
	h.want(t, "key-alice", id2, a2a.TaskStateSubmitted, a2a.TaskStateRejected)
	egOK(t, stop())
	for _, s := range subs.subs {
		if s.IsValid() {
			t.Fatalf("subscription %q survived stop", s.Subject)
		}
	}
}

// egVectorsInScope is slice 1's partition of a2a-v1.json (task-9c57b30d,
// comment-4cf9f90b). Slices 2-4 extend it; the last slice runs all 34.
var egVectorsInScope = []string{
	"status-pending", "status-scheduled", "status-dispatched", "status-running", "status-failed-retryable",
	"status-failed", "status-failed-fatal", "status-timeout", "status-denied-rejected",
	"oversize-part-count-rejected", "url-part-rejected", "duplicate-message-id", "send-to-busy-task",
	"unknown-task-get-task",
}

type egVecTurn struct {
	MessageID string                             `json:"message_id"`
	TaskRef   *string                            `json:"task_ref"`
	Parts     []struct{ Kind, Text, URL string } `json:"parts"`
}

type egVecCase struct {
	ID        string      `json:"id"`
	Caller    Identity    `json:"caller"`
	Owner     *Identity   `json:"owner"`
	Turns     []egVecTurn `json:"turns"`
	CapEvents []struct {
		Kind      string `json:"kind"`
		Turn      int    `json:"turn"`
		JobStatus string `json:"job_status"`
	} `json:"cap_events"`
	A2ACall *struct {
		Method  string `json:"method"`
		TaskRef string `json:"task_ref"`
	} `json:"a2a_call"`
	ExpectedState      a2a.TaskState   `json:"expected_a2a_state"`
	ExpectedTerminal   bool            `json:"expected_terminal"`
	ExpectedEventOrder []a2a.TaskState `json:"expected_event_order"`
	ExpectedPublishes  struct {
		JobRequest int `json:"job_request"`
		JobCancel  int `json:"job_cancel"`
	} `json:"expected_publishes"`
	ExpectedError string `json:"expected_error"`
}

var egVecErrors = map[string]error{
	"":                           nil,
	"TASK_NOT_FOUND":             a2a.ErrTaskNotFound,
	"UNSUPPORTED_OPERATION":      a2a.ErrUnsupportedOperation,
	"CONTENT_TYPE_NOT_SUPPORTED": a2a.ErrUnsupportedContentType,
	"INVALID_PARAMS":             a2a.ErrInvalidParams,
}

func egKeyFor(t *testing.T, id Identity) string {
	t.Helper()
	if k := "key-" + id.Principal; egIdents[k] == id {
		return k
	}
	t.Fatalf("no API key for %+v", id)
	return ""
}

func egVecMessage(t *testing.T, turn egVecTurn, task a2a.TaskID) *a2a.Message {
	t.Helper()
	m := &a2a.Message{ID: turn.MessageID, Role: a2a.MessageRoleUser}
	if turn.TaskRef != nil {
		m.TaskID = task
	}
	for _, p := range turn.Parts {
		switch p.Kind {
		case "text":
			m.Parts = append(m.Parts, a2a.NewTextPart(p.Text))
		case "url":
			m.Parts = append(m.Parts, a2a.NewFileURLPart(a2a.URL(p.URL), ""))
		default:
			t.Fatalf("unsupported vector part kind %q", p.Kind)
		}
	}
	return m
}

func TestVectorsAllCases(t *testing.T) {
	raw, err := os.ReadFile("../../spec/interop/vectors/a2a-v1.json")
	egOK(t, err)
	var file struct {
		Cases []egVecCase `json:"cases"`
	}
	egOK(t, json.Unmarshal(raw, &file))
	byID := map[string]egVecCase{}
	for _, c := range file.Cases {
		byID[c.ID] = c
	}
	if len(egVectorsInScope) != 14 {
		t.Fatalf("%d vectors in scope, want 14", len(egVectorsInScope))
	}
	for _, id := range egVectorsInScope {
		c, ok := byID[id]
		if !ok {
			t.Fatalf("vector %q is not in a2a-v1.json", id)
		}
		t.Run(id, func(t *testing.T) { egRunVector(t, c) })
	}
}

// egReplay replays the CAP events as the owner; it returns the task id and turns sent.
func (h *egHarness) egReplay(t *testing.T, c egVecCase, owner string) (a2a.TaskID, int) {
	t.Helper()
	var task a2a.TaskID
	jobs, sent := map[int]string{}, 0
	for _, ev := range c.CapEvents {
		v, known := agentv1.JobStatus_value[ev.JobStatus]
		st := agentv1.JobStatus(v)
		switch {
		case ev.Kind == "submit_published":
			got, err := h.send(owner, egVecMessage(t, c.Turns[sent], task))
			if err != nil {
				t.Fatalf("turn %d: %v", ev.Turn, err)
			}
			task, jobs[ev.Turn], sent = got.ID, h.pub.lastJob(), sent+1
		case !known:
			t.Fatalf("unknown job_status %q", ev.JobStatus)
		case ev.Kind == "progress":
			h.ing.deliver(capsdk.SubjectProgress, h.progress(t, jobs[ev.Turn], st))
		case ev.Kind == "result" && st == agentv1.JobStatus_JOB_STATUS_DENIED:
			h.ing.deliver(capsdk.SubjectDLQ, h.dlq(t, jobs[ev.Turn], st))
		case ev.Kind == "result":
			h.ing.deliver(capsdk.SubjectResult, h.result(t, jobs[ev.Turn], st))
		default:
			t.Fatalf("unknown cap event %q", ev.Kind)
		}
	}
	return task, sent
}

func egRunVector(t *testing.T, c egVecCase) {
	h := newEgHarness(t)
	caller, owner := egKeyFor(t, c.Caller), egKeyFor(t, c.Caller)
	if c.Owner != nil {
		owner = egKeyFor(t, *c.Owner)
	}
	target, ok := egVecErrors[c.ExpectedError]
	if !ok {
		t.Fatalf("unmapped expected_error %q", c.ExpectedError)
	}
	task, sent := h.egReplay(t, c, owner)
	var err error
	switch {
	case c.A2ACall != nil && c.A2ACall.Method == "SendMessage":
		var got *a2a.Task
		if got, err = h.send(caller, egVecMessage(t, c.Turns[sent], task)); err == nil && task == "" {
			task = got.ID
		}
	case c.A2ACall != nil && c.A2ACall.Method == "GetTask":
		ref := task
		if c.A2ACall.TaskRef == "unknown" {
			ref = a2a.TaskID("a2a-task-" + strings.Repeat("0", 32))
		}
		_, err = h.ing.GetTask(egAs(caller), &a2a.GetTaskRequest{ID: ref})
	case c.A2ACall != nil:
		t.Fatalf("unexpected a2a_call method %q", c.A2ACall.Method)
	}
	if (target == nil) != (err == nil) || target != nil && !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
	if st := h.state(caller, task); st != c.ExpectedState || st.Terminal() != c.ExpectedTerminal {
		t.Fatalf("caller sees %s (terminal %v), want %s (terminal %v)", st, st.Terminal(), c.ExpectedState, c.ExpectedTerminal)
	}
	rec, _ := h.ing.store.get(egIdents[owner], string(task))
	if order := egStatusOrder(rec.events); !slices.Equal(order, c.ExpectedEventOrder) {
		t.Fatalf("status events %v, want %v", order, c.ExpectedEventOrder)
	}
	req, cancels := h.pub.counts()
	if req != c.ExpectedPublishes.JobRequest || cancels != c.ExpectedPublishes.JobCancel || cancels != 0 {
		t.Fatalf("publishes: %d JobRequests, %d JobCancels; want %+v", req, cancels, c.ExpectedPublishes)
	}
}
