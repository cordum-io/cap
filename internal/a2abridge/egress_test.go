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
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/cordum-io/cap/v2/sdk/go/client"
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

// egPublisher records every (subject, raw, decoded BusPacket). When fail
// returns an error, the publish fails and records nothing.
type egPublisher struct {
	mu      sync.Mutex
	sent    []egSent
	fail    func(subject string) error
	forward client.Publisher
}

func (p *egPublisher) Publish(subject string, data []byte) error {
	pkt := &agentv1.BusPacket{}
	if err := proto.Unmarshal(data, pkt); err != nil {
		return err
	}
	p.mu.Lock()
	fail, forward := p.fail, p.forward
	p.mu.Unlock()
	if fail != nil {
		if err := fail(subject); err != nil {
			return err
		}
	}
	p.mu.Lock()
	p.sent = append(p.sent, egSent{subject: subject, raw: slices.Clone(data), pkt: pkt})
	p.mu.Unlock()
	if forward != nil {
		return forward.Publish(subject, data)
	}
	return nil
}

// request returns the published packet whose JobRequest runs job, or nil.
func (p *egPublisher) request(job string) *agentv1.BusPacket {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.sent {
		if s.subject == capsdk.SubjectSubmit && s.pkt.GetJobRequest().GetJobId() == job {
			return s.pkt
		}
	}
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

// egInputs is a mutex map. When fail is set, a write fails and stores nothing.
type egInputs struct {
	sync.Map
	fail error
}

func (f *egInputs) PutTurnInput(_ context.Context, key string, data []byte) error {
	if f.fail != nil {
		return f.fail
	}
	f.Store(key, slices.Clone(data))
	return nil
}

func (f *egInputs) count() (n int) {
	f.Range(func(any, any) bool { n++; return true })
	return n
}

type egHarness struct {
	ing                   *Ingress
	pub                   *egPublisher
	in                    *egInputs
	results               *egResults
	worker, sched, bridge *ecdsa.PrivateKey
	graceMu               sync.Mutex
	callbacks             []func()
	graced                chan struct{}
	vectorCancel          <-chan egCall
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
	h := &egHarness{pub: &egPublisher{}, in: &egInputs{}, results: &egResults{}, worker: egKey(t), sched: egKey(t), bridge: egKey(t)}
	keys := map[string]Identity{}
	for k, id := range egIdents {
		sum := sha256.Sum256([]byte(k))
		keys[hex.EncodeToString(sum[:])] = id
	}
	return h, Config{
		APIKeys: keys, Topic: "job.a2a.agent", SenderID: "a2a-bridge", SigningKey: h.bridge,
		Publisher: h.pub, Inputs: h.in, Results: h.results,
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
	h.graced = make(chan struct{}, 64)
	ing.afterFunc = func(_ time.Duration, f func()) {
		h.graceMu.Lock()
		h.callbacks = append(h.callbacks, sync.OnceFunc(f))
		h.graceMu.Unlock()
		h.graced <- struct{}{}
	}
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
	missing := newEgHarness(t, func(c *Config) { c.Results = nil })
	missingSub := &egSubs{}
	if _, err := missing.ing.Start(missingSub); err == nil || len(missingSub.subjects) != 0 {
		t.Fatal("Start without Results must fail before subscribing")
	}
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
	// MaxBlockingWait: 0 is the 60 s default, otherwise 1 s to 300 s (spec §6).
	waits := map[time.Duration]bool{-time.Second: false, 999 * time.Millisecond: false, 301 * time.Second: false,
		0: true, time.Second: true, 300 * time.Second: true}
	for wait, ok := range waits {
		_, cfg := egConfig(t)
		cfg.MaxBlockingWait = wait
		if _, err := New(cfg); (err == nil) != ok {
			t.Fatalf("New with MaxBlockingWait %s: err %v, want accepted %v", wait, err, ok)
		}
	}
	for grace, ok := range map[time.Duration]bool{-time.Second: false, 999 * time.Millisecond: false,
		61 * time.Second: false, 0: true, time.Second: true, 60 * time.Second: true} {
		_, cfg := egConfig(t)
		cfg.CancelGrace = grace
		ing, err := New(cfg)
		if (err == nil) != ok {
			t.Fatalf("CancelGrace %s: %v, accepted=%v", grace, err, ok)
		}
		if ok && (ing.grace != grace && !(grace == 0 && ing.grace == 30*time.Second)) {
			t.Fatal("wrong grace")
		}
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
	egCompletedOverNATS(t, h, publish)
	egCancelOverNATS(t, h, nc, pc, publish)
	egOK(t, stop())
	for _, s := range subs.subs {
		if s.IsValid() {
			t.Fatalf("subscription %q survived stop", s.Subject)
		}
	}
}

func egCompletedOverNATS(t *testing.T, h *egHarness, publish func(string, []byte)) {
	t.Helper()
	id3, job3 := h.start(t, "key-alice", "msg-3")
	publish(capsdk.SubjectProgress, h.progress(t, job3, egRunning))
	egEventually(t, "task 3 WORKING", func() bool { return h.state("key-alice", id3) == a2a.TaskStateWorking })
	publish(capsdk.SubjectResult, h.resultEnvelope(t, job3, egComplete("completed")))
	egEventually(t, "task 3 COMPLETED with artifact", func() bool {
		got, err := h.ing.GetTask(egAs("key-alice"), &a2a.GetTaskRequest{ID: id3})
		return err == nil && got.Status.State == a2a.TaskStateCompleted && len(got.Artifacts) == 1
	})
	h.want(t, "key-alice", id3, a2a.TaskStateSubmitted, a2a.TaskStateWorking, a2a.TaskStateCompleted)
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
		Kind      string          `json:"kind"`
		Turn      int             `json:"turn"`
		JobStatus string          `json:"job_status"`
		Envelope  json.RawMessage `json:"envelope"`
		Duplicate bool            `json:"duplicate"`
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
	"TASK_NOT_CANCELABLE":        a2a.ErrTaskNotCancelable,
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
	if len(file.Cases) != 34 {
		t.Fatalf("%d vectors, want 34", len(file.Cases))
	}
	for _, c := range file.Cases {
		t.Run(c.ID, func(t *testing.T) { egRunVector(t, c) })
	}
}

// egReplay replays the CAP events as the owner; it returns the task id and turns sent.
func (h *egHarness) egReplay(t *testing.T, c egVecCase, owner string) (a2a.TaskID, int) {
	t.Helper()
	var task a2a.TaskID
	jobs, sent := map[int]string{}, 0
	previous := map[int][]byte{}
	for _, ev := range c.CapEvents {
		v, known := agentv1.JobStatus_value[ev.JobStatus]
		st := agentv1.JobStatus(v)
		switch {
		case ev.Kind == "submit_published":
			// Turn N sends turns[N-1]; task_ref "same" names the turn-1 task.
			published, _ := h.pub.counts()
			got, err := h.send(owner, egVecMessage(t, c.Turns[ev.Turn-1], task))
			if err != nil {
				t.Fatalf("turn %d: %v", ev.Turn, err)
			}
			if now, _ := h.pub.counts(); now != published+1 {
				t.Fatalf("turn %d published %d JobRequests, want 1", ev.Turn, now-published)
			}
			task, jobs[ev.Turn], sent = got.ID, h.pub.lastJob(), sent+1
		case ev.Kind == "cancel_published":
			h.vectorCancel = h.cancel(t, egAs(egKeyFor(t, c.Caller)), task)
			h.waitGraces(t, 1)
		case ev.Kind == "cancel_grace_expired":
			h.fireGraces()
		case !known:
			t.Fatalf("unknown job_status %q", ev.JobStatus)
		case ev.Kind == "progress":
			h.ing.deliver(capsdk.SubjectProgress, h.progress(t, jobs[ev.Turn], st))
		case ev.Kind == "result" && st == agentv1.JobStatus_JOB_STATUS_DENIED:
			h.ing.deliver(capsdk.SubjectDLQ, h.dlq(t, jobs[ev.Turn], st))
		case ev.Kind == "result":
			raw := h.result(t, jobs[ev.Turn], st)
			if st == agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
				raw = h.resultEnvelope(t, jobs[ev.Turn], ev.Envelope)
			}
			if ev.Duplicate {
				raw = previous[ev.Turn]
				if raw == nil {
					t.Fatal("duplicate without a previous result")
				}
			}
			previous[ev.Turn] = raw
			h.ing.deliver(capsdk.SubjectResult, raw)
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
	cancel := h.vectorCancel
	var err error
	switch {
	case c.A2ACall != nil && c.A2ACall.Method == "CancelTask":
		if cancel == nil {
			cancel = h.cancel(t, egAs(caller), task)
		}
		err = egCancelReply(t, cancel).err
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
	rec, _ := h.ing.store.get(egIdents[caller], string(task))
	egArtifactOrder(t, rec.events)
	if order := egStatusOrder(rec.events); !slices.Equal(order, c.ExpectedEventOrder) {
		t.Fatalf("status events %v, want %v", order, c.ExpectedEventOrder)
	}
	req, cancels := h.pub.counts()
	if req != c.ExpectedPublishes.JobRequest || cancels != c.ExpectedPublishes.JobCancel {
		t.Fatalf("publishes: %d JobRequests, %d JobCancels; want %+v", req, cancels, c.ExpectedPublishes)
	}
}

// egResults bounds reads and can gate one key without holding a store lock.
type egResults struct {
	sync.Map
	err     error
	key     string
	entered chan struct{}
	gate    chan struct{}
}

var _ ResultReader = (*egResults)(nil)

func (r *egResults) ReadResult(ctx context.Context, key string, maxBytes int) ([]byte, error) {
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second || maxBytes != 262144 {
		return nil, errors.New("missing read deadline or wrong bound")
	}
	if key == r.key && r.gate != nil {
		select {
		case r.entered <- struct{}{}:
		default:
		}
		select {
		case <-r.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	v, ok := r.Load(key)
	if !ok {
		return nil, errors.New("missing result")
	}
	raw := v.([]byte)
	if len(raw) > maxBytes {
		return nil, errors.New("oversize result")
	}
	return slices.Clone(raw), nil
}

func egEnvelope(state, fields string) []byte {
	return []byte(`{"media_type":"` + EnvelopeMediaType + `","a2a_state":"` + state + `"` + fields + `}`)
}

const egText = `{"kind":"text","text":"answer"}`
const egArtifact = `{"artifact_id":"answer","name":"answer","parts":[` + egText + `]}`

func egComplete(state string) []byte {
	return egEnvelope(state, `,"message":{"parts":[`+egText+`]},"artifacts":[`+egArtifact+`]`)
}

func (h *egHarness) resultEnvelope(t *testing.T, job string, envelope []byte) []byte {
	if len(envelope) != 0 && string(envelope) != "null" {
		h.results.Store("res:"+job, slices.Clone(envelope))
	}
	p := egResultPkt(egWorker, job, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
	p.GetJobResult().ResultPtr = "redis://res:" + job
	return egSign(t, h.worker, p)
}

func egArtifactOrder(t *testing.T, events []a2a.Event) {
	t.Helper()
	for idx, event := range events {
		if ev, ok := event.(*a2a.TaskArtifactUpdateEvent); ok {
			if !ev.LastChunk || idx+1 >= len(events) {
				t.Fatal("artifact missing lastChunk or following status")
			}
			if _, ok := events[len(events)-1].(*a2a.TaskStatusUpdateEvent); !ok {
				t.Fatal("artifact follows final status")
			}
		}
	}
}

func TestEnvelopeValidation(t *testing.T) {
	message := func(parts string) string { return `,"message":{"parts":[` + parts + `]}` }
	text := func(n int) string { return `{"kind":"text","text":` + strconv.Quote(strings.Repeat("x", n)) + `}` }
	data := func(depth int) string {
		return `{"kind":"data","data":` + strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth) + `}`
	}
	artifacts := func(n int) string {
		return `,"artifacts":[` + strings.TrimSuffix(strings.Repeat(egArtifact+",", n), ",") + `]`
	}
	good := map[string][]byte{
		"empty completed":         egEnvelope("completed", ""),
		"input required":          egEnvelope("input-required", message(egText)),
		"auth required":           egEnvelope("auth-required", message(egText)+`,"artifacts":[]`),
		"depth32":                 egEnvelope("completed", message(data(32))),
		"text65536":               egEnvelope("completed", message(text(65536))),
		"eight artifacts":         egEnvelope("completed", artifacts(8)),
		"sixteen parts":           egEnvelope("completed", message(strings.TrimSuffix(strings.Repeat(egText+",", 16), ","))),
		"media type":              egEnvelope("completed", message(`{"kind":"text","text":"ok","media_type":"text/plain"}`)),
		"large integer":           egEnvelope("completed", message(`{"kind":"data","data":{"n":9007199254740993}}`)),
		"multibyte artifact name": egEnvelope("completed", `,"artifacts":[{"artifact_id":"a","name":`+strconv.Quote(strings.Repeat("é", 256))+`,"parts":[`+egText+`]}]`),
	}
	for name, raw := range good {
		t.Run(name, func(t *testing.T) {
			out, err := parseEnvelope(raw)
			if err != nil || out == nil {
				t.Fatalf("valid envelope: %v", err)
			}
		})
	}
	for name, raw := range egInvalidEnvelopes(message, text, data, artifacts) {
		t.Run(name, func(t *testing.T) {
			if _, err := parseEnvelope(raw); err == nil {
				t.Fatal("invalid envelope accepted")
			}
		})
	}
}

func egInvalidEnvelopes(message func(string) string, text, data, artifacts func(int) string) map[string][]byte {
	return map[string][]byte{
		"fold state":             []byte(strings.Replace(string(egEnvelope("completed", "")), "a2a_state", "a2a_\u017ftate", 1)),
		"fold duplicate state":   egEnvelope("completed", ",\"a2a_\u017ftate\":\"input-required\""+message(egText)),
		"fold message":           egEnvelope("input-required", strings.Replace(message(egText), "message", "me\u017f\u017fage", 1)),
		"fold message parts":     egEnvelope("completed", strings.Replace(message(egText), "parts", "part\u017f", 1)),
		"fold artifact parts":    egEnvelope("completed", strings.Replace(artifacts(1), "parts", "part\u017f", 1)),
		"fold artifacts":         egEnvelope("completed", strings.Replace(artifacts(1), "artifacts", "artifact\u017f", 1)),
		"invalid UTF-8 text":     egEnvelope("completed", message(`{"kind":"text","text":"`+string([]byte{0xff})+`"}`)),
		"case alias":             []byte(strings.Replace(string(egEnvelope("completed", "")), "media_type", "MEDIA_TYPE", 1)),
		"duplicate field":        egEnvelope("completed", `,"a2a_state":"completed"`),
		"wrong type":             []byte(`{"media_type":"bad","a2a_state":"completed"}`),
		"unknown state":          egEnvelope("working", ""),
		"missing message":        egEnvelope("input-required", ""),
		"unknown member":         egEnvelope("completed", `,"token":"secret"`),
		"null message":           egEnvelope("completed", `,"message":null`),
		"null artifacts":         egEnvelope("completed", `,"artifacts":null`),
		"null artifact":          egEnvelope("completed", `,"artifacts":[null]`),
		"auth artifact":          egComplete("auth-required"),
		"auth data":              egEnvelope("auth-required", message(data(1))),
		"zero parts":             egEnvelope("completed", message("")),
		"seventeen parts":        egEnvelope("completed", message(strings.TrimSuffix(strings.Repeat(egText+",", 17), ","))),
		"nine artifacts":         egEnvelope("completed", artifacts(9)),
		"text65537":              egEnvelope("completed", message(text(65537))),
		"depth33":                egEnvelope("completed", message(data(33))),
		"oversize data":          egEnvelope("completed", message(`{"kind":"data","data":`+strconv.Quote(strings.Repeat("x", 65535))+`}`)),
		"url":                    egEnvelope("completed", message(`{"kind":"url","url":"https://example.test"}`)),
		"text and data":          egEnvelope("completed", message(`{"kind":"text","text":"x","data":{}}`)),
		"data and text":          egEnvelope("completed", message(`{"kind":"data","text":"x","data":{}}`)),
		"null text":              egEnvelope("completed", message(`{"kind":"text","text":null}`)),
		"empty media type":       egEnvelope("completed", message(`{"kind":"text","text":"x","media_type":""}`)),
		"null media type":        egEnvelope("completed", message(`{"kind":"text","text":"x","media_type":null}`)),
		"bad media type":         egEnvelope("completed", message(`{"kind":"text","text":"x","media_type":"not a type"}`)),
		"bad artifact id":        egEnvelope("completed", `,"artifacts":[{"artifact_id":"a/b","parts":[`+egText+`]}]`),
		"long artifact name":     egEnvelope("completed", `,"artifacts":[{"artifact_id":"a","name":`+strconv.Quote(strings.Repeat("é", 257))+`,"parts":[`+egText+`]}]`),
		"null artifact name":     egEnvelope("completed", `,"artifacts":[{"artifact_id":"a","name":null,"parts":[`+egText+`]}]`),
		"unknown message member": egEnvelope("completed", `,"message":{"parts":[`+egText+`],"token":"secret"}`),
		"trailing data":          append(egEnvelope("completed", ""), []byte(` {}`)...),
		"whole envelope bound":   egEnvelope("completed", message(strings.TrimSuffix(strings.Repeat(text(65536)+",", 4), ","))),
	}
}

func TestSucceededWithoutValidEnvelopeFails(t *testing.T) {
	for _, mode := range []string{"wrong pointer", "reader error", "oversize", "missing", "fold state"} {
		t.Run(mode, func(t *testing.T) {
			h := newEgHarness(t)
			id, job := h.start(t, "key-alice", "msg-1")
			raw := h.resultEnvelope(t, job, egComplete("completed"))
			switch mode {
			case "wrong pointer":
				h.results.Store("res:other", egComplete("completed"))
				pkt := egResultPkt(egWorker, job, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
				pkt.GetJobResult().ResultPtr = "redis://res:other"
				raw = egSign(t, h.worker, pkt)
			case "reader error":
				h.results.err = errors.New("read failed")
			case "oversize":
				h.results.Store("res:"+job, []byte(strings.Repeat("x", 262145)))
			case "missing":
				h.results.Delete("res:" + job)
			case "fold state":
				h.results.Store("res:"+job, []byte(strings.Replace(string(egEnvelope("completed", "")), "a2a_state", "a2a_\u017ftate", 1)))
			}
			h.ing.deliver(capsdk.SubjectResult, raw)
			h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateFailed)
		})
	}
}

func egWait(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestDuplicateResultCausesOneTransition(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	raw := h.resultEnvelope(t, job, egComplete("completed"))
	done := make(chan struct{}, 16)
	for range 16 {
		go func() { h.ing.deliver(capsdk.SubjectResult, raw); done <- struct{}{} }()
	}
	for range 16 {
		egWait(t, done, "duplicate delivery")
	}
	h.ing.deliver(capsdk.SubjectResult, raw)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateCompleted)
	rec, _ := h.ing.store.get(egIdents["key-alice"], string(id))
	if len(rec.events) != 3 || len(rec.task().Artifacts) != 1 {
		t.Fatalf("events=%d artifacts=%d", len(rec.events), len(rec.task().Artifacts))
	}
	egArtifactOrder(t, rec.events)
	// A stale completion without a live reservation must never replace a winner (E32).
	h2 := newEgHarness(t)
	id2, job2 := h2.start(t, "key-alice", "msg-2")
	h2.ing.deliver(capsdk.SubjectDLQ, h2.dlq(t, job2, agentv1.JobStatus_JOB_STATUS_DENIED))
	out, err := parseEnvelope(egComplete("completed"))
	egOK(t, err)
	h2.ing.store.commitResult(job2, 1, out, egNow)
	h2.want(t, "key-alice", id2, a2a.TaskStateSubmitted, a2a.TaskStateRejected)
}

func TestDuplicateInterruptedResultAppliesOnce(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	raw := h.resultEnvelope(t, job, egComplete("input-required"))
	h.ing.deliver(capsdk.SubjectResult, raw)
	h.ing.deliver(capsdk.SubjectResult, raw)
	h.ing.deliver(capsdk.SubjectResult, h.result(t, job, egFailed))
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired)
	got, err := h.ing.GetTask(egAs("key-alice"), &a2a.GetTaskRequest{ID: id})
	egOK(t, err)
	if got.Status.State.Terminal() || len(got.Artifacts) != 1 || len(got.History) != 2 {
		t.Fatalf("wrong interrupted task: %+v", got)
	}
	msg := got.Status.Message
	if msg == nil || msg.Role != a2a.MessageRoleAgent || msg.TaskID != id || msg.ContextID != got.ContextID || got.History[1].ID != msg.ID {
		t.Fatal("status/history message binding")
	}
	rec, _ := h.ing.store.get(egIdents["key-alice"], string(id))
	if len(rec.events) != 3 {
		t.Fatalf("events=%d", len(rec.events))
	}
	egArtifactOrder(t, rec.events)
}

func TestFirstTerminalIsReservedBeforeTheEnvelopeRead(t *testing.T) {
	for _, mode := range []string{"dlq-during-read", "reader-error-still-wins", "grace-during-read"} {
		t.Run(mode, func(t *testing.T) {
			h := newEgHarness(t)
			id, job := h.start(t, "key-alice", "msg-1")
			h.results.key, h.results.entered, h.results.gate = "res:"+job, make(chan struct{}, 1), make(chan struct{})
			if mode == "reader-error-still-wins" {
				h.results.err = errors.New("read failed")
			}
			var cancel <-chan egCall
			if mode == "grace-during-read" {
				cancel = h.cancel(t, egAs("key-alice"), id)
				h.waitGraces(t, 1)
			}
			raw := h.resultEnvelope(t, job, egComplete("completed"))
			done := make(chan struct{})
			go func() { h.ing.deliver(capsdk.SubjectResult, raw); close(done) }()
			egWait(t, h.results.entered, "envelope read")
			h.fireGraces()
			h.ing.deliver(capsdk.SubjectDLQ, h.dlq(t, job, agentv1.JobStatus_JOB_STATUS_DENIED))
			h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job, egRunning))
			h.want(t, "key-alice", id, a2a.TaskStateSubmitted)
			close(h.results.gate)
			egWait(t, done, "settled result")
			state, artifacts, events := a2a.TaskStateCompleted, 1, 3
			if h.results.err != nil {
				state, artifacts, events = a2a.TaskStateFailed, 0, 2
			}
			h.ing.deliver(capsdk.SubjectDLQ, h.dlq(t, job, agentv1.JobStatus_JOB_STATUS_DENIED))
			h.want(t, "key-alice", id, a2a.TaskStateSubmitted, state)
			rec, _ := h.ing.store.get(egIdents["key-alice"], string(id))
			if len(rec.events) != events || len(rec.task().Artifacts) != artifacts {
				t.Fatal("wrong event/artifact count")
			}
			egArtifactOrder(t, rec.events)
			if cancel != nil {
				egIs(t, egCancelReply(t, cancel).err, a2a.ErrTaskNotCancelable)
			}
		})
	}
}

func TestEnvelopeSnapshotsAreIndependent(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	data := `{"kind":"data","data":{"nested":[9007199254740993]}}`
	env := egEnvelope("completed", `,"message":{"parts":[`+data+`]},"artifacts":[{"artifact_id":"a","parts":[`+data+`]}]`)
	h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job, env))
	before, _ := h.ing.store.get(egIdents["key-alice"], string(id))
	got, err := h.ing.GetTask(egAs("key-alice"), &a2a.GetTaskRequest{ID: id})
	egOK(t, err)
	for _, part := range []*a2a.Part{got.Artifacts[0].Parts[0], got.History[1].Parts[0], got.Status.Message.Parts[0]} {
		value := part.Content.(a2a.Data).Value.(map[string]interface{})["nested"].([]interface{})
		if value[0] != json.Number("9007199254740993") {
			t.Fatal("lost number precision")
		}
		value[0] = "changed"
	}
	snap, _ := h.ing.store.get(egIdents["key-alice"], string(id))
	snap.events[1].(*a2a.TaskArtifactUpdateEvent).Artifact.Parts[0].MediaType = "changed"
	status := snap.events[2].(*a2a.TaskStatusUpdateEvent)
	status.Status.Message.Parts[0].Content = a2a.Text("changed")
	*status.Status.Timestamp = time.Time{}
	after, _ := h.ing.store.get(egIdents["key-alice"], string(id))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("caller mutated stored snapshot")
	}
}

func TestEnvelopeReadTimeoutSettlesFailed(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	h.results.key, h.results.entered, h.results.gate = "res:"+job, make(chan struct{}, 1), make(chan struct{})
	started := time.Now()
	h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job, egComplete("completed")))
	if elapsed := time.Since(started); elapsed < 4*time.Second || elapsed > 6*time.Second {
		t.Fatalf("read timeout took %s", elapsed)
	}
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateFailed)
}

// egSubscriber lets Start succeed without NATS; tests that use it never call stop.
type egSubscriber struct{}

func (egSubscriber) Subscribe(string, nats.MsgHandler) (*nats.Subscription, error) {
	return &nats.Subscription{}, nil
}

// egJobOf is the job id of the turn that messageID opens on taskID ("" for turn 1).
func egJobOf(tenant, principal string, taskID a2a.TaskID, messageID string) string {
	return "a2a-" + idempotencyKey(tenant, principal, string(taskID), messageID)
}

func egIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
}

// resume sends one text message from caller to taskID, with ReturnImmediately.
func (h *egHarness) resume(caller string, taskID a2a.TaskID, messageID, contextID, text string) (*a2a.Task, error) {
	return h.send(caller, &a2a.Message{ID: messageID, Role: a2a.MessageRoleUser, TaskID: taskID,
		ContextID: contextID, Parts: a2a.ContentParts{a2a.NewTextPart(text)}})
}

// interrupt ends job's turn with an input-required or auth-required envelope
// that carries a one-part agent message, and returns the delivered result.
func (h *egHarness) interrupt(t *testing.T, job, state string) []byte {
	t.Helper()
	raw := h.resultEnvelope(t, job, egEnvelope(state, `,"message":{"parts":[`+egText+`]}`))
	h.ing.deliver(capsdk.SubjectResult, raw)
	return raw
}

// published asserts the exact JobRequest count and stored-input count.
func (h *egHarness) published(t *testing.T, requests, inputs int) {
	t.Helper()
	if got, _ := h.pub.counts(); got != requests || h.in.count() != inputs {
		t.Fatalf("%d JobRequests and %d inputs; want %d and %d", got, h.in.count(), requests, inputs)
	}
}

func (h *egHarness) task(t *testing.T, id a2a.TaskID) *a2a.Task {
	t.Helper()
	got, err := h.ing.GetTask(egAs("key-alice"), &a2a.GetTaskRequest{ID: id})
	egOK(t, err)
	return got
}

func TestResumeOpensNextTurnWithParentAndStep(t *testing.T) {
	h := newEgHarness(t)
	id, job1 := h.start(t, "key-alice", "msg-1")
	turn1 := h.interrupt(t, job1, "input-required")
	before := h.task(t, id)
	_, err := h.resume("key-alice", id, "msg-2", "other-context", "From SFO")
	egIs(t, err, a2a.ErrInvalidParams)
	for _, key := range []string{"key-bob", "key-mallory"} {
		_, err = h.resume(key, id, "msg-2", "", "From SFO")
		egIs(t, err, a2a.ErrTaskNotFound)
	}
	h.published(t, 1, 1)
	got, err := h.resume("key-alice", id, "msg-2", "", "From SFO")
	egOK(t, err)
	key := idempotencyKey("tenant-a", "alice", string(id), "msg-2")
	job2 := egJobOf("tenant-a", "alice", id, "msg-2")
	pkt := h.pub.request(job2)
	if err := capsdk.VerifyPacketSignature(pkt, &h.bridge.PublicKey); err != nil {
		t.Fatalf("turn 2 signature: %v", err)
	}
	jr := pkt.GetJobRequest()
	fields := []string{jr.GetJobId(), jr.GetParentJobId(), jr.GetWorkflowId(), jr.GetTopic(), jr.GetContextPtr(),
		jr.GetTenantId(), jr.GetPrincipalId(), jr.GetMeta().GetTenantId(), jr.GetMeta().GetActorId(),
		jr.GetMeta().GetIdempotencyKey(), pkt.GetSenderId(), pkt.GetTraceId()}
	want := []string{job2, job1, string(id), "job.a2a.agent", "redis://ctx:" + job2,
		"tenant-a", "alice", "tenant-a", "alice", key, "a2a-bridge", string(id)}
	if !slices.Equal(fields, want) || jr.GetStepIndex() != 2 {
		t.Fatalf("turn 2 JobRequest step %d\n got %q\nwant %q", jr.GetStepIndex(), fields, want)
	}
	v, _ := h.in.Load("ctx:" + job2)
	input, _ := v.([]byte)
	wantInput := `{"task_id":"` + string(id) + `","context_id":"` + before.ContextID +
		`","turn":2,"message_id":"msg-2","parts":[{"kind":"text","text":"From SFO"}]}`
	if string(input) != wantInput {
		t.Fatalf("turn 2 input\n got %s\nwant %s", input, wantInput)
	}
	task := h.task(t, id)
	msg2 := &a2a.Message{ID: "msg-2", ContextID: before.ContextID, TaskID: id, Role: a2a.MessageRoleUser,
		Parts: a2a.ContentParts{a2a.NewTextPart("From SFO")}}
	if !reflect.DeepEqual(got, task) || task.Status.State != a2a.TaskStateSubmitted || task.Status.Message != nil ||
		len(task.History) != 3 || !reflect.DeepEqual(task.History[:2], before.History) || !reflect.DeepEqual(task.History[2], msg2) {
		t.Fatalf("resumed task %+v", task)
	}
	// Turn 1's job is no longer current: its result and progress change nothing.
	rec, _ := h.ing.store.get(egIdents["key-alice"], string(id))
	h.ing.deliver(capsdk.SubjectResult, turn1)
	h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job1, egRunning))
	if again, _ := h.ing.store.get(egIdents["key-alice"], string(id)); len(again.events) != len(rec.events) {
		t.Fatalf("turn 1 packets changed turn 2: %d events, want %d", len(again.events), len(rec.events))
	}
	if retry, err := h.resume("key-alice", id, "msg-2", "", "From SFO"); err != nil || retry.ID != id {
		t.Fatalf("retried msg-2: %v", err)
	}
	h.published(t, 2, 2)
	egResumeRefused(t, h, id, "msg-3")
	h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job2, egComplete("completed")))
	egResumeRefused(t, h, id, "msg-4")
	h.published(t, 2, 2)
	if len(h.task(t, id).Artifacts) != 1 {
		t.Fatal("turn 2 artifact missing")
	}
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired,
		a2a.TaskStateSubmitted, a2a.TaskStateCompleted)
}

// egResumeRefused asserts that msgID to a busy or terminal task is refused
// before its input is written.
func egResumeRefused(t *testing.T, h *egHarness, id a2a.TaskID, msgID string) {
	t.Helper()
	_, err := h.resume("key-alice", id, msgID, "", "more")
	egIs(t, err, a2a.ErrUnsupportedOperation)
	if _, ok := h.in.Load("ctx:" + egJobOf("tenant-a", "alice", id, msgID)); ok {
		t.Fatalf("%s wrote an input", msgID)
	}
}

func TestResumeStopsAtSixtyFourTurns(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	h.interrupt(t, job, "input-required")
	for turn := 2; turn <= 64; turn++ {
		msgID := "msg-" + strconv.Itoa(turn)
		_, err := h.resume("key-alice", id, msgID, "", "more")
		egOK(t, err)
		next := egJobOf("tenant-a", "alice", id, msgID)
		if jr := h.pub.request(next).GetJobRequest(); jr.GetStepIndex() != int32(turn) || jr.GetParentJobId() != job {
			t.Fatalf("turn %d: step_index %d parent %q, want parent %q", turn, jr.GetStepIndex(), jr.GetParentJobId(), job)
		}
		job = next
		h.interrupt(t, job, "input-required")
	}
	_, err := h.resume("key-alice", id, "msg-65", "", "more")
	egIs(t, err, a2a.ErrUnsupportedOperation)
	h.published(t, 64, 64)
	if st := h.state("key-alice", id); st != a2a.TaskStateInputRequired {
		t.Fatalf("after the refused 65th turn the task is %s", st)
	}
}

func TestFailedResumePublishRestoresInterruptedTurn(t *testing.T) {
	var tick atomic.Int64 // every timestamp differs, so a restore must keep the old one
	h := newEgHarness(t, func(c *Config) {
		c.Now = func() time.Time { return egNow.Add(time.Duration(tick.Add(1)) * time.Second) }
	})
	id, job1 := h.start(t, "key-alice", "msg-1")
	h.interrupt(t, job1, "input-required")
	before := h.task(t, id)
	job2, boom := egJobOf("tenant-a", "alice", id, "msg-2"), errors.New("boom")
	for _, mode := range []string{"input", "publish"} {
		h.in.fail, h.pub.fail = nil, nil
		if mode == "input" {
			h.in.fail = boom
		} else {
			h.pub.fail = func(subject string) error {
				if subject == capsdk.SubjectSubmit {
					return boom
				}
				return nil
			}
		}
		_, err := h.resume("key-alice", id, "msg-2", "", "From SFO")
		egIs(t, err, a2a.ErrInternalError)
		h.ing.store.mu.Lock()
		rec := h.ing.store.tasks[taskKey{tenant: "tenant-a", principal: "alice", id: string(id)}]
		turn, job := rec.turn, rec.jobID
		h.ing.store.mu.Unlock()
		if _, indexed := h.ing.store.owner(job2); turn != 1 || job != job1 || indexed {
			t.Fatalf("%s failure left turn %d job %q, job 2 indexed %v", mode, turn, job, indexed)
		}
		h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job2, egComplete("completed")))
		if after := h.task(t, id); !reflect.DeepEqual(after, before) {
			t.Fatalf("%s failure did not restore the task\n got %+v\nwant %+v", mode, after, before)
		}
		if requests, _ := h.pub.counts(); requests != 1 {
			t.Fatalf("%s failure: %d JobRequests, want 1", mode, requests)
		}
	}
	h.in.fail, h.pub.fail = nil, nil
	_, err := h.resume("key-alice", id, "msg-2", "", "From SFO")
	egOK(t, err)
	if jr := h.pub.request(job2).GetJobRequest(); jr.GetStepIndex() != 2 || jr.GetParentJobId() != job1 {
		t.Fatalf("retry: step_index %d parent %q", jr.GetStepIndex(), jr.GetParentJobId())
	}
	h.published(t, 2, 2)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateSubmitted)
	// A result that settles the new turn before the publish reports failure
	// wins: release must not restore over it.
	id2, job := h.start(t, "key-alice", "msg-a")
	h.interrupt(t, job, "input-required")
	won := h.resultEnvelope(t, egJobOf("tenant-a", "alice", id2, "msg-b"), egComplete("completed"))
	h.pub.fail = func(string) error { h.ing.deliver(capsdk.SubjectResult, won); return boom }
	_, err = h.resume("key-alice", id2, "msg-b", "", "go")
	egIs(t, err, a2a.ErrInternalError)
	h.want(t, "key-alice", id2, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired,
		a2a.TaskStateSubmitted, a2a.TaskStateCompleted)
}

func TestConcurrentResumesOpenOneTurn(t *testing.T) {
	h := newEgHarness(t)
	id, job1 := h.start(t, "key-alice", "msg-1")
	h.interrupt(t, job1, "input-required")
	gate, errs := make(chan struct{}), make(chan error, 16)
	for n := 2; n <= 17; n++ {
		go func() {
			<-gate
			_, err := h.resume("key-alice", id, "msg-"+strconv.Itoa(n), "", "go")
			errs <- err
		}()
	}
	close(gate)
	won := 0
	for range 16 {
		select {
		case err := <-errs:
			if err == nil {
				won++
			} else if !errors.Is(err, a2a.ErrUnsupportedOperation) {
				t.Fatalf("concurrent resume: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the concurrent resumes")
		}
	}
	inputs := 0
	for n := 2; n <= 17; n++ {
		if _, ok := h.in.Load("ctx:" + egJobOf("tenant-a", "alice", id, "msg-"+strconv.Itoa(n))); ok {
			inputs++
		}
	}
	if won != 1 || inputs != 1 {
		t.Fatalf("%d resumes won and %d turn-2 inputs written; want 1 and 1", won, inputs)
	}
	h.published(t, 2, 2)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateSubmitted)
}

type egCall struct {
	task *a2a.Task
	took time.Duration
	err  error
}

// call runs SendMessage in the background; a nil cfg is a default blocking call.
func (h *egHarness) call(ctx context.Context, m *a2a.Message, cfg *a2a.SendMessageConfig) <-chan egCall {
	out := make(chan egCall, 1)
	go func() {
		start := time.Now()
		res, err := h.ing.SendMessage(ctx, &a2a.SendMessageRequest{Message: m, Config: cfg})
		task, _ := res.(*a2a.Task)
		if err == nil && task == nil {
			err = errors.New("SendMessage did not return a task")
		}
		out <- egCall{task: task, took: time.Since(start), err: err}
	}()
	return out
}

// egAwait waits at most 5 s for a call that returns state within [least, most].
func egAwait(t *testing.T, ch <-chan egCall, state a2a.TaskState, least, most time.Duration) *a2a.Task {
	t.Helper()
	select {
	case c := <-ch:
		if c.err != nil || c.task.Status.State != state || c.took < least || c.took > most {
			t.Fatalf("call: err %v task %+v took %s; want %s in [%s, %s]", c.err, c.task, c.took, state, least, most)
		}
		return c.task
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for a %s call", state)
	}
	return nil
}

func (h *egHarness) graces() []func() {
	h.graceMu.Lock()
	defer h.graceMu.Unlock()
	return slices.Clone(h.callbacks)
}

func (h *egHarness) fireGrace(n int) { h.graces()[n]() }
func (h *egHarness) fireGraces() {
	for _, f := range h.graces() {
		f()
	}
}

func (h *egHarness) waitGraces(t *testing.T, n int) {
	t.Helper()
	for len(h.graces()) < n {
		egWait(t, h.graced, "cancel grace scheduled")
	}
	if len(h.graces()) != n {
		t.Fatalf("graces=%d, want %d", len(h.graces()), n)
	}
}

func (h *egHarness) cancels() []*agentv1.BusPacket {
	h.pub.mu.Lock()
	defer h.pub.mu.Unlock()
	var out []*agentv1.BusPacket
	for _, s := range h.pub.sent {
		if s.subject == capsdk.SubjectCancel && s.pkt.GetJobCancel() != nil {
			out = append(out, s.pkt)
		}
	}
	return out
}

func (h *egHarness) wantCancels(t *testing.T, id a2a.TaskID, jobs ...string) {
	t.Helper()
	pkts := h.cancels()
	if len(pkts) != len(jobs) {
		t.Fatalf("JobCancels=%d, want %d", len(pkts), len(jobs))
	}
	for n, p := range pkts {
		egOK(t, capsdk.ValidateBusPacket(p))
		egOK(t, capsdk.VerifyPacketSignature(p, &h.bridge.PublicKey))
		if p.TraceId != string(id) || p.SenderId != "a2a-bridge" || p.GetJobCancel().JobId != jobs[n] ||
			p.GetJobCancel().RequestedBy != "alice" || p.GetJobCancel().Reason != "a2a CancelTask" {
			t.Fatalf("bad cancel: %v", p)
		}
	}
}

func egAsync(fn func() (*a2a.Task, error)) <-chan egCall {
	out := make(chan egCall, 1)
	go func() { task, err := fn(); out <- egCall{task: task, err: err} }()
	return out
}

func (h *egHarness) cancel(t *testing.T, ctx context.Context, id a2a.TaskID) <-chan egCall {
	t.Helper()
	ctx, stop := context.WithCancel(ctx)
	t.Cleanup(stop) // no deadline: a stranded waiter must fail the bounded assertion
	return egAsync(func() (*a2a.Task, error) { return h.ing.CancelTask(ctx, &a2a.CancelTaskRequest{ID: id}) })
}

func egCancelReply(t *testing.T, ch <-chan egCall) egCall {
	t.Helper()
	select {
	case c := <-ch:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for CancelTask/resume reply")
	}
	return egCall{}
}

func egCanceled(t *testing.T, ch <-chan egCall, state a2a.TaskState) {
	t.Helper()
	c := egCancelReply(t, ch)
	egOK(t, c.err)
	if c.task == nil || c.task.Status.State != state {
		t.Fatalf("task=%v, want %s", c.task, state)
	}
}

func (h *egHarness) joinCancel(t *testing.T, id a2a.TaskID) <-chan egCall {
	t.Helper()
	step, _, _, attempt, abort, err := h.ing.store.requestCancel(egIdents["key-alice"], string(id), egNow)
	egOK(t, err)
	if step != cancelWait || attempt == 0 || abort == nil {
		t.Fatal("not joining the pending attempt")
	}
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	return egAsync(func() (*a2a.Task, error) {
		return h.ing.awaitCancel(ctx, egIdents["key-alice"], string(id), attempt, abort)
	})
}

// egGate blocks the first call only, with bounded waits and cleanup on failed assertions.
func egGate(t *testing.T, failure error) (func() error, <-chan struct{}, func()) {
	t.Helper()
	entered, gate := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	t.Cleanup(release)
	var calls atomic.Int32
	return func() error {
		if calls.Add(1) != 1 {
			return nil
		}
		close(entered)
		select {
		case <-gate:
		case <-time.After(5 * time.Second):
			return errors.New("publish gate release timed out")
		}
		return failure
	}, entered, release
}

func TestCancelPublishesOneSignedJobCancel(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job, egRunning))
	gate, entered, release := egGate(t, nil)
	h.pub.fail = func(s string) error {
		if s == capsdk.SubjectCancel {
			return gate()
		}
		return nil
	}
	a := h.cancel(t, egAs("key-alice"), id)
	egWait(t, entered, "first cancel publish")
	b := h.joinCancel(t, id)
	release()
	h.waitGraces(t, 1)
	h.wantCancels(t, id, job)
	c := h.cancel(t, egAs("key-alice"), id)
	h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job, egComplete("completed")))
	for _, ch := range []<-chan egCall{a, b, c} {
		egIs(t, egCancelReply(t, ch).err, a2a.ErrTaskNotCancelable)
	}
	h.fireGraces()
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateWorking, a2a.TaskStateCompleted)
	h.wantCancels(t, id, job)
	h.waitGraces(t, 1)
}

func TestCancelGraceAndInterruptedAndTerminal(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	c := h.cancel(t, egAs("key-alice"), id)
	h.waitGraces(t, 1)
	h.fireGraces()
	egCanceled(t, c, a2a.TaskStateCanceled)
	h.ing.deliver(capsdk.SubjectResult, h.result(t, job, agentv1.JobStatus_JOB_STATUS_CANCELLED))
	h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job, egComplete("completed")))
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateCanceled)
	id2, job2 := h.start(t, "key-alice", "msg-2")
	h.interrupt(t, job2, "input-required")
	egCanceled(t, h.cancel(t, egAs("key-alice"), id2), a2a.TaskStateCanceled)
	h.want(t, "key-alice", id2, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateCanceled)
	egIs(t, egCancelReply(t, h.cancel(t, egAs("key-alice"), id2)).err, a2a.ErrTaskNotCancelable)
	for _, key := range []string{"key-mallory", "key-bob"} {
		egIs(t, egCancelReply(t, h.cancel(t, egAs(key), id)).err, a2a.ErrTaskNotFound)
	}
	egIs(t, egCancelReply(t, h.cancel(t, egAs("key-alice"), "unknown")).err, a2a.ErrTaskNotFound)
	_, err := h.ing.CancelTask(egAs("key-alice"), nil)
	egIs(t, err, a2a.ErrTaskNotFound)
	h.wantCancels(t, id, job)
	h.waitGraces(t, 1)
}

func TestCancelPublishFailureCanBeRetried(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	var calls atomic.Int32
	h.pub.fail = func(s string) error {
		if s == capsdk.SubjectCancel && calls.Add(1) == 1 {
			return errors.New("publish failed")
		}
		return nil
	}
	egIs(t, egCancelReply(t, h.cancel(t, egAs("key-alice"), id)).err, a2a.ErrInternalError)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted)
	h.wantCancels(t, id)
	h.waitGraces(t, 0)
	retry := h.cancel(t, egAs("key-alice"), id)
	h.waitGraces(t, 1)
	h.ing.deliver(capsdk.SubjectResult, h.result(t, job, agentv1.JobStatus_JOB_STATUS_CANCELLED))
	egCanceled(t, retry, a2a.TaskStateCanceled)
	h.wantCancels(t, id, job)
}

func TestCancelOutlivesItsCaller(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job, egRunning))
	ctx, stop := context.WithCancel(egAs("key-alice"))
	defer stop()
	c := h.cancel(t, ctx, id)
	h.waitGraces(t, 1)
	stop()
	egCanceled(t, c, a2a.TaskStateWorking)
	h.interrupt(t, job, "input-required")
	h.fireGraces()
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateWorking, a2a.TaskStateInputRequired, a2a.TaskStateCanceled)
	h.wantCancels(t, id, job)
}

func TestPendingCancelEndsAnInterruptedTurn(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	c := h.cancel(t, egAs("key-alice"), id)
	h.waitGraces(t, 1)
	h.interrupt(t, job, "input-required")
	egCanceled(t, c, a2a.TaskStateCanceled)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateCanceled)
	h.wantCancels(t, id, job)
}

func TestCancelJoinerOfFailedPublishGetsInternal(t *testing.T) {
	h := newEgHarness(t)
	id, job := h.start(t, "key-alice", "msg-1")
	gate, entered, release := egGate(t, errors.New("publish failed"))
	h.pub.fail = func(s string) error {
		if s == capsdk.SubjectCancel {
			return gate()
		}
		return nil
	}
	a := h.cancel(t, egAs("key-alice"), id)
	egWait(t, entered, "first cancel publish")
	b := h.joinCancel(t, id)
	release()
	for _, c := range []<-chan egCall{a, b} {
		egIs(t, egCancelReply(t, c).err, a2a.ErrInternalError)
	}
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted)
	h.wantCancels(t, id)
	h.waitGraces(t, 0)
	c := h.cancel(t, egAs("key-alice"), id)
	h.waitGraces(t, 1)
	h.fireGraces()
	egCanceled(t, c, a2a.TaskStateCanceled)
	h.wantCancels(t, id, job)
}

func TestCancelDuringFailedResumeCancelsTheRestoredTurn(t *testing.T) {
	for _, mode := range []string{"waiter-cancels-restored-turn", "stale-attempt-is-cleared",
		"stale-failure-cannot-abort-a-new-attempt", "same-message-retry-ignores-stale-grace"} {
		t.Run(mode, func(t *testing.T) { egFailedResumeCancel(t, mode) })
	}
	t.Run("publisher-missed-restore-cancels-the-new-turn", func(t *testing.T) { egMissedRestore(t, false) })
	t.Run("joiner-of-stale-failure-cancels-the-new-turn", func(t *testing.T) { egMissedRestore(t, true) })
}

// egMissedRestore lets a resume replace the restored turn before the old
// attempt's waiter (its publisher, or a joiner of its failed publish) watches.
func egMissedRestore(t *testing.T, joiner bool) {
	t.Helper()
	h := newEgHarness(t)
	id, job1 := h.start(t, "key-alice", "msg-1")
	h.interrupt(t, job1, "input-required")
	submit, entered, release := egGate(t, errors.New("resume failed"))
	var failure error
	if joiner {
		failure = errors.New("stale cancel failed")
	}
	cancel, cancelEntered, cancelRelease := egGate(t, failure)
	h.pub.fail = func(s string) error {
		switch s {
		case capsdk.SubjectSubmit:
			return submit()
		case capsdk.SubjectCancel:
			return cancel()
		}
		return nil
	}
	r := egAsync(func() (*a2a.Task, error) { return h.resume("key-alice", id, "msg-2", "", "resume") })
	egWait(t, entered, "resume publish")
	a := h.cancel(t, egAs("key-alice"), id)
	egWait(t, cancelEntered, "old cancel publish")
	var attempt uint64
	var abort <-chan struct{}
	if joiner {
		var step cancelStep
		var err error
		step, _, _, attempt, abort, err = h.ing.store.requestCancel(egIdents["key-alice"], string(id), egNow)
		egOK(t, err)
		if rec, _ := h.ing.store.get(egIdents["key-alice"], string(id)); step != cancelWait || attempt == 0 || attempt != rec.cancelAttempt {
			t.Fatalf("B did not join A's attempt: step %v attempt %d, current %d", step, attempt, rec.cancelAttempt)
		}
	}
	release()
	egIs(t, egCancelReply(t, r).err, a2a.ErrInternalError)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired)
	_, err := h.resume("key-alice", id, "msg-3", "", "resume")
	egOK(t, err)
	job3 := egJobOf("tenant-a", "alice", id, "msg-3")
	if rec, _ := h.ing.store.get(egIdents["key-alice"], string(id)); rec.turn != 2 || rec.jobID != job3 {
		t.Fatalf("wrong new turn/job: %d/%s", rec.turn, rec.jobID)
	}
	cancelRelease()
	if joiner {
		egMissedRestoreJoiner(t, h, id, job3, a, attempt, abort)
	} else {
		egMissedRestorePublisher(t, h, id, job3, a)
	}
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateSubmitted, a2a.TaskStateCanceled)
}

func egMissedRestorePublisher(t *testing.T, h *egHarness, id a2a.TaskID, job3 string, a <-chan egCall) {
	t.Helper()
	h.waitGraces(t, 2)
	h.wantCancels(t, id, egJobOf("tenant-a", "alice", id, "msg-2"), job3)
	h.fireGrace(0)
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateSubmitted)
	select {
	case c := <-a:
		t.Fatalf("A returned before the new attempt's grace: %+v", c)
	default:
	}
	h.fireGrace(1)
	egCanceled(t, a, a2a.TaskStateCanceled)
	h.wantCancels(t, id, egJobOf("tenant-a", "alice", id, "msg-2"), job3)
}

func egMissedRestoreJoiner(t *testing.T, h *egHarness, id a2a.TaskID, job3 string, a <-chan egCall, attempt uint64, abort <-chan struct{}) {
	t.Helper()
	egIs(t, egCancelReply(t, a).err, a2a.ErrInternalError)
	select {
	case <-abort:
		t.Fatal("a stale publish failure closed the superseded attempt's abort")
	default:
	}
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	b := egAsync(func() (*a2a.Task, error) {
		return h.ing.awaitCancel(ctx, egIdents["key-alice"], string(id), attempt, abort)
	})
	h.waitGraces(t, 1)
	h.wantCancels(t, id, job3)
	h.fireGraces()
	egCanceled(t, b, a2a.TaskStateCanceled)
	h.wantCancels(t, id, job3)
}

func egFailedResumeCancel(t *testing.T, mode string) {
	t.Helper()
	h := newEgHarness(t)
	id, job1 := h.start(t, "key-alice", "msg-1")
	h.interrupt(t, job1, "input-required")
	submit, entered, release := egGate(t, errors.New("resume failed"))
	failed, cancelEntered, cancelRelease := egGate(t, errors.New("stale cancel failed"))
	h.pub.fail = func(s string) error {
		if s == capsdk.SubjectSubmit {
			return submit()
		}
		if s == capsdk.SubjectCancel && mode == "stale-failure-cannot-abort-a-new-attempt" {
			return failed()
		}
		return nil
	}
	r := egAsync(func() (*a2a.Task, error) { return h.resume("key-alice", id, "msg-2", "", "resume") })
	egWait(t, entered, "resume publish")
	ctx, stop := context.WithCancel(egAs("key-alice"))
	defer stop()
	a := h.cancel(t, ctx, id)
	if mode == "stale-failure-cannot-abort-a-new-attempt" {
		egWait(t, cancelEntered, "old cancel publish")
	} else {
		h.waitGraces(t, 1)
		h.wantCancels(t, id, egJobOf("tenant-a", "alice", id, "msg-2"))
		if mode != "waiter-cancels-restored-turn" {
			stop()
			egCanceled(t, a, a2a.TaskStateSubmitted)
		}
	}
	release()
	egIs(t, egCancelReply(t, r).err, a2a.ErrInternalError)
	if mode == "waiter-cancels-restored-turn" {
		egCanceled(t, a, a2a.TaskStateCanceled)
		h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateCanceled)
		return
	}
	egRetryAndCancel(t, h, id, mode, a, cancelRelease)
}

func egRetryAndCancel(t *testing.T, h *egHarness, id a2a.TaskID, mode string, a <-chan egCall, cancelRelease func()) {
	t.Helper()
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired)
	message := "msg-3"
	if mode == "same-message-retry-ignores-stale-grace" {
		message = "msg-2"
	}
	_, err := h.resume("key-alice", id, message, "", "resume")
	egOK(t, err)
	job := egJobOf("tenant-a", "alice", id, message)
	rec, _ := h.ing.store.get(egIdents["key-alice"], string(id))
	if rec.turn != 2 || rec.jobID != job {
		t.Fatalf("wrong retry turn/job: %d/%s", rec.turn, rec.jobID)
	}
	egNewAttempt(t, h, id, job, mode, a, cancelRelease)
}

func egNewAttempt(t *testing.T, h *egHarness, id a2a.TaskID, job, mode string, a <-chan egCall, release func()) {
	t.Helper()
	ctx, stop := context.WithCancel(egAs("key-alice"))
	defer stop()
	b := h.cancel(t, ctx, id)
	if mode == "stale-failure-cannot-abort-a-new-attempt" {
		h.waitGraces(t, 1)
		release()
		egIs(t, egCancelReply(t, a).err, a2a.ErrInternalError)
		select {
		case c := <-b:
			t.Fatalf("new successful attempt aborted: %+v", c)
		default:
		}
		h.fireGraces()
		egCanceled(t, b, a2a.TaskStateCanceled)
		h.wantCancels(t, id, job)
	} else {
		h.waitGraces(t, 2)
		stop()
		egCanceled(t, b, a2a.TaskStateSubmitted)
		h.wantCancels(t, id, egJobOf("tenant-a", "alice", id, "msg-2"), job)
		h.fireGrace(0)
		h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateSubmitted)
		h.fireGrace(1)
	}
	h.want(t, "key-alice", id, a2a.TaskStateSubmitted, a2a.TaskStateInputRequired, a2a.TaskStateSubmitted, a2a.TaskStateCanceled)
}

func egCancelOverNATS(t *testing.T, h *egHarness, nc, pc *nats.Conn, publish func(string, []byte)) {
	t.Helper()
	h.pub.forward = nc
	sub, err := pc.SubscribeSync(capsdk.SubjectCancel)
	egOK(t, err)
	defer sub.Unsubscribe()
	egOK(t, pc.Flush())
	id, job := h.start(t, "key-alice", "msg-4")
	c := h.cancel(t, egAs("key-alice"), id)
	msg, err := sub.NextMsg(5 * time.Second)
	egOK(t, err)
	pkt := &agentv1.BusPacket{}
	egOK(t, proto.Unmarshal(msg.Data, pkt))
	egOK(t, capsdk.VerifyPacketSignature(pkt, &h.bridge.PublicKey))
	if pkt.GetJobCancel().GetJobId() != job {
		t.Fatalf("wrong NATS cancel: %v", pkt)
	}
	publish(capsdk.SubjectResult, h.result(t, job, agentv1.JobStatus_JOB_STATUS_CANCELLED))
	egCanceled(t, c, a2a.TaskStateCanceled)
	egOK(t, nc.Flush())
	egOK(t, pc.Flush())
	if n, err := sub.Delivered(); err != nil || n != 1 {
		t.Fatalf("delivered=%d: %v", n, err)
	}
	h.wantCancels(t, id, job)
}

func TestBlockingSendMessageWaitsForTheTurn(t *testing.T) {
	h := newEgHarness(t, func(c *Config) { c.MaxBlockingWait = time.Second })
	// settle < MaxBlockingWait: a waiter that missed the change returns only at the cap.
	alice, quick, settle := egAs("key-alice"), 500*time.Millisecond, 900*time.Millisecond
	msg := func(msgID string, taskID a2a.TaskID) *a2a.Message {
		return &a2a.Message{ID: msgID, Role: a2a.MessageRoleUser, TaskID: taskID,
			Parts: a2a.ContentParts{a2a.NewTextPart("Book me a flight")}}
	}
	published := func(job string) {
		egEventually(t, "JobRequest "+job, func() bool { return h.pub.request(job) != nil })
	}
	// Before Start nothing can settle a turn, so a default call returns at once.
	egAwait(t, h.call(alice, msg("msg-1", ""), nil), a2a.TaskStateSubmitted, 0, quick)
	_, err := h.ing.Start(egSubscriber{})
	egOK(t, err)
	pending, job := h.call(alice, msg("msg-2", ""), nil), egJobOf("tenant-a", "alice", "", "msg-2")
	published(job)
	h.ing.deliver(capsdk.SubjectProgress, h.progress(t, job, egRunning))
	h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job, egComplete("completed")))
	if got := egAwait(t, pending, a2a.TaskStateCompleted, 0, settle); len(got.Artifacts) != 1 {
		t.Fatal("blocking call lost the artifact")
	}
	immediate := &a2a.SendMessageConfig{ReturnImmediately: true}
	egAwait(t, h.call(alice, msg("msg-3", ""), immediate), a2a.TaskStateSubmitted, 0, quick)
	egAwait(t, h.call(alice, msg("msg-4", ""), nil), a2a.TaskStateSubmitted, time.Second, 5*time.Second)
	// An interrupted turn settles the call; a default resume waits for turn 2.
	pending, job = h.call(alice, msg("msg-5", ""), nil), egJobOf("tenant-a", "alice", "", "msg-5")
	published(job)
	h.interrupt(t, job, "input-required")
	task := egAwait(t, pending, a2a.TaskStateInputRequired, 0, settle)
	pending, job = h.call(alice, msg("msg-6", task.ID), nil), egJobOf("tenant-a", "alice", task.ID, "msg-6")
	published(job)
	h.ing.deliver(capsdk.SubjectResult, h.resultEnvelope(t, job, egComplete("completed")))
	egAwait(t, pending, a2a.TaskStateCompleted, 0, settle)
	// ctx ends a wait long before MaxBlockingWait, with the task and no error.
	slow := newEgHarness(t, func(c *Config) { c.MaxBlockingWait = 300 * time.Second })
	_, err = slow.ing.Start(egSubscriber{})
	egOK(t, err)
	ctx, cancel := context.WithTimeout(alice, 100*time.Millisecond)
	defer cancel()
	egAwait(t, slow.call(ctx, msg("msg-1", ""), nil), a2a.TaskStateSubmitted, 50*time.Millisecond, 2*time.Second)
}
