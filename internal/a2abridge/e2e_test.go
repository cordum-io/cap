package a2abridge_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/examples/a2a-bridge/turns"
	"github.com/cordum-io/cap/v2/internal/a2abridge"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/cordum-io/cap/v2/sdk/go/runtime"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	e2eDemoTopic   = "job.a2a.demo"
	e2eDeniedTopic = "job.a2a.denied"
	e2eWorkerID    = "a2a-demo-worker"
	e2eScheduler   = "cordum-scheduler"
	e2eAPIKey      = "e2e-api-key-7c1f0d2a9b"
)

// TestA2ABridgeE2E drives the bridge with the official a2a-go client over HTTP
// and real NATS, through an UNGOVERNED test scheduler, to a runtime.Agent that
// runs the shipped example turn logic (examples/a2a-bridge/turns).
func TestA2ABridgeE2E(t *testing.T) {
	e := newE2E(t)
	demo, denied := e.bridge(t, e2eDemoTopic), e.bridge(t, e2eDeniedTopic)

	t.Run("heartbeats", func(t *testing.T) { e.heartbeats(t) })
	t.Run("working-input-required-completed", func(t *testing.T) {
		e.sched.setHold(true)
		task := demo.send(t, "plan a trip to Lisbon", nil, true)
		job1 := e.sched.waitJob(t, task.ID, 1)
		demo.waitState(t, task.ID, a2a.TaskStateWorking)
		e.sched.setHold(false)
		e.sched.release(job1)
		got := demo.waitState(t, task.ID, a2a.TaskStateInputRequired)
		if got.Status.Message == nil || len(got.Status.Message.Parts) != 1 {
			t.Fatalf("INPUT_REQUIRED status message = %+v, want 1 part", got.Status.Message)
		}
		done := demo.send(t, "two travellers, next week", got, false)
		if done.Status.State != a2a.TaskStateCompleted || len(done.Artifacts) != 1 {
			t.Fatalf("turn 2 = %s with %d artifacts, want COMPLETED with 1", done.Status.State, len(done.Artifacts))
		}
		if job2 := e.sched.waitJob(t, task.ID, 2); job2 == job1 {
			t.Fatalf("turn 2 reused job %s", job1)
		}
	})
	t.Run("cancel-in-flight", func(t *testing.T) {
		e.sched.setHold(true)
		defer e.sched.setHold(false)
		task := demo.send(t, "book a hotel", nil, true)
		job := e.sched.waitJob(t, task.ID, 1)
		demo.waitState(t, task.ID, a2a.TaskStateWorking)
		if got := demo.cancel(t, task.ID); got.Status.State != a2a.TaskStateCanceled {
			t.Fatalf("CancelTask = %s, want CANCELED after the grace", got.Status.State)
		}
		e.capture.sync(t, demo.nc)
		if n := e.capture.cancels(job); n != 1 {
			t.Fatalf("JobCancel packets for %s = %d, want 1", job, n)
		}
	})
	var authTask *a2a.Task
	t.Run("needs-auth", func(t *testing.T) {
		authTask = demo.send(t, "needs-auth", nil, false)
		if authTask.Status.State != a2a.TaskStateAuthRequired || authTask.Status.State.Terminal() {
			t.Fatalf("needs-auth = %s, want non-terminal AUTH_REQUIRED", authTask.Status.State)
		}
		e.capture.sync(t, demo.nc)
		e.capture.assertNoCredentials(t)
		job := e.sched.waitJob(t, authTask.ID, 1)
		for _, key := range []string{"ctx:" + job, "res:" + job} {
			blob, err := e.store.Get(context.Background(), key)
			if err != nil || leaksCredential(blob) {
				t.Fatalf("blob %s: err=%v, leaks credential=%t", key, err, leaksCredential(blob))
			}
		}
	})
	t.Run("cancel-interrupted", func(t *testing.T) {
		if authTask == nil {
			t.Fatal("needs-auth produced no task")
		}
		e.capture.sync(t, demo.nc)
		before := e.capture.cancels("")
		start := time.Now()
		if got := demo.cancel(t, authTask.ID); got.Status.State != a2a.TaskStateCanceled {
			t.Fatalf("CancelTask on AUTH_REQUIRED = %s, want CANCELED", got.Status.State)
		}
		if took := time.Since(start); took >= time.Second {
			t.Fatalf("CancelTask on an interrupted task took %s, want no grace wait", took)
		}
		e.capture.sync(t, demo.nc)
		if after := e.capture.cancels(""); after != before {
			t.Fatalf("JobCancel count %d -> %d, want no publish for an interrupted task", before, after)
		}
	})
	t.Run("denied-topic", func(t *testing.T) {
		if got := denied.send(t, "anything", nil, false); got.Status.State != a2a.TaskStateRejected {
			t.Fatalf("denied topic = %s, want REJECTED", got.Status.State)
		}
	})
}

type e2e struct {
	url                            string
	bridgeKey, workerKey, schedKey *ecdsa.PrivateKey
	store                          *runtime.InMemoryBlobStore
	capture                        *capture
	sched                          *ungovernedTestScheduler
	hbConn                         *nats.Conn
	resultKeys                     map[string]*ecdsa.PublicKey
}

func newE2E(t *testing.T) *e2e {
	t.Helper()
	// Loopback only: binding every interface trips the Windows firewall prompt.
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	must(t, err)
	go ns.Start()
	t.Cleanup(ns.Shutdown)
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats-server not ready")
	}
	e := &e2e{url: ns.ClientURL(), bridgeKey: newKey(t), workerKey: newKey(t), schedKey: newKey(t),
		store: runtime.NewInMemoryBlobStore()}
	e.resultKeys = map[string]*ecdsa.PublicKey{e2eWorkerID: &e.workerKey.PublicKey, e2eScheduler: &e.schedKey.PublicKey}
	e.capture = newCapture(t, e.conn(t))
	e.sched = newScheduler(t, e.conn(t), &e.bridgeKey.PublicKey, e.schedKey)
	e.hbConn = e.conn(t)
	e.startWorker(t)
	return e
}

func (e *e2e) conn(t *testing.T) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(e.url)
	must(t, err)
	t.Cleanup(nc.Close)
	return nc
}

// startWorker runs the example turn logic in a real runtime.Agent. It has its
// own connection because Agent.Close drains the connection it was given.
func (e *e2e) startWorker(t *testing.T) {
	t.Helper()
	agent := &runtime.Agent{NATS: e.conn(t), Store: e.store, PrivateKey: e.workerKey, SenderID: e2eWorkerID,
		PublicKeys:    map[string]*ecdsa.PublicKey{e2eScheduler: &e.schedKey.PublicKey},
		HandshakeMode: runtime.HandshakeModeOff, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	must(t, turns.Register(agent, e2eDemoTopic, e2eWorkerID))
	must(t, agent.Start())
	t.Cleanup(func() { _ = agent.Close() })
}

func (e *e2e) heartbeats(t *testing.T) {
	const interval = 50 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	build := func() *agentv1.BusPacket {
		return turns.HeartbeatPacket(e2eWorkerID, "a2a-demo", 4, []string{e2eDemoTopic})
	}
	sign := func(p *agentv1.BusPacket) error { return capsdk.SignPacket(p, e.workerKey) }
	go func() { defer close(done); turns.RunHeartbeats(ctx, e.hbConn, interval, build, sign) }()
	waitFor(t, "a verified heartbeat", func() bool { return e.capture.heartbeats(t, &e.workerKey.PublicKey) > 0 })
	cancel()
	<-done
	e.capture.sync(t, e.hbConn)
	seen := e.capture.heartbeats(t, &e.workerKey.PublicKey)
	time.Sleep(3 * interval)
	e.capture.sync(t, e.hbConn)
	if after := e.capture.heartbeats(t, &e.workerKey.PublicKey); after != seen {
		t.Fatalf("heartbeats after cancel: %d -> %d", seen, after)
	}
}

// bridgeClient is one bridge instance (one topic) and an a2a-go client for it.
type bridgeClient struct {
	nc  *nats.Conn
	cl  *a2aclient.Client
	ctx context.Context
}

func (e *e2e) bridge(t *testing.T, topic string) *bridgeClient {
	t.Helper()
	nc := e.conn(t)
	sum := sha256.Sum256([]byte(e2eAPIKey))
	ing, err := a2abridge.New(a2abridge.Config{
		APIKeys: map[string]a2abridge.Identity{hex.EncodeToString(sum[:]): {Tenant: "tenant-a", Principal: "alice"}},
		Topic:   topic, SenderID: "cap-a2a-bridge", SigningKey: e.bridgeKey, Publisher: nc,
		Inputs: blobIO{e.store}, Results: blobIO{e.store}, ResultKeys: e.resultKeys,
		CancelGrace: time.Second, MaxBlockingWait: 10 * time.Second,
	})
	must(t, err)
	stop, err := ing.Start(nc)
	must(t, err)
	t.Cleanup(func() { _ = stop() })
	must(t, nc.Flush())
	srv := httptest.NewServer(ing.Handler())
	t.Cleanup(srv.Close)
	card := &a2a.AgentCard{Name: "cap-a2a-bridge-e2e",
		SupportedInterfaces: []*a2a.AgentInterface{a2a.NewAgentInterface(srv.URL, a2a.TransportProtocolJSONRPC)}}
	cl, err := a2aclient.NewFromCard(context.Background(), card)
	must(t, err)
	t.Cleanup(func() { _ = cl.Destroy() })
	ctx := a2aclient.AttachServiceParams(context.Background(),
		a2aclient.ServiceParams{"Authorization": {"Bearer " + e2eAPIKey}})
	return &bridgeClient{nc: nc, cl: cl, ctx: ctx}
}

// send posts one text message, as a follow-up to task when it is non-nil.
func (b *bridgeClient) send(t *testing.T, text string, task *a2a.Task, immediate bool) *a2a.Task {
	t.Helper()
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))
	if task != nil {
		msg = a2a.NewMessageForTask(a2a.MessageRoleUser, task, a2a.NewTextPart(text))
	}
	res, err := b.cl.SendMessage(b.ctx, &a2a.SendMessageRequest{Message: msg,
		Config: &a2a.SendMessageConfig{ReturnImmediately: immediate}})
	must(t, err)
	got, ok := res.(*a2a.Task)
	if !ok {
		t.Fatalf("SendMessage returned %T, want *a2a.Task", res)
	}
	return got
}

func (b *bridgeClient) cancel(t *testing.T, id a2a.TaskID) *a2a.Task {
	t.Helper()
	got, err := b.cl.CancelTask(b.ctx, &a2a.CancelTaskRequest{ID: id})
	must(t, err)
	return got
}

// waitState polls GetTask until want, failing at once on any other terminal state.
func (b *bridgeClient) waitState(t *testing.T, id a2a.TaskID, want a2a.TaskState) *a2a.Task {
	t.Helper()
	var last *a2a.Task
	waitFor(t, fmt.Sprintf("task %s in %s", id, want), func() bool {
		got, err := b.cl.GetTask(b.ctx, &a2a.GetTaskRequest{ID: id})
		must(t, err)
		if got.Status.State != want && got.Status.State.Terminal() {
			t.Fatalf("task %s ended %s, want %s", id, got.Status.State, want)
		}
		last = got
		return got.Status.State == want
	})
	return last
}

// blobIO lets the bridge share the worker's in-memory store.
type blobIO struct{ s *runtime.InMemoryBlobStore }

func (b blobIO) PutTurnInput(ctx context.Context, key string, data []byte) error {
	return b.s.Set(ctx, key, data)
}

func (b blobIO) ReadResult(ctx context.Context, key string, maxBytes int) ([]byte, error) {
	v, err := b.s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(v) > maxBytes {
		return nil, errors.New("result larger than maxBytes")
	}
	return v, nil
}

// ungovernedTestScheduler is UNGOVERNED: no policy, no safety kernel, no
// approvals. It stands in for Cordum's scheduler in this test only. It accepts
// a submit signed by the bridge, denies one hard-coded topic with a signed
// DENIED on sys.job.dlq (Cordum's emitDLQ shape), and otherwise reports
// DISPATCHED and relays the job, re-signed as cordum-scheduler, to the worker's
// direct subject, as Cordum dispatches to a heartbeating worker. While hold is
// on it parks jobs until release.
type ungovernedTestScheduler struct {
	t         *testing.T
	nc        *nats.Conn
	bridgePub *ecdsa.PublicKey
	key       *ecdsa.PrivateKey
	mu        sync.Mutex
	hold      bool
	parked    map[string]*agentv1.BusPacket
	jobs      map[string][]string
}

func newScheduler(t *testing.T, nc *nats.Conn, bridgePub *ecdsa.PublicKey, key *ecdsa.PrivateKey) *ungovernedTestScheduler {
	t.Helper()
	s := &ungovernedTestScheduler{t: t, nc: nc, bridgePub: bridgePub, key: key,
		parked: map[string]*agentv1.BusPacket{}, jobs: map[string][]string{}}
	_, err := nc.Subscribe(capsdk.SubjectSubmit, s.onSubmit)
	must(t, err)
	must(t, nc.Flush())
	return s
}

func (s *ungovernedTestScheduler) onSubmit(m *nats.Msg) {
	pkt := &agentv1.BusPacket{}
	if proto.Unmarshal(m.Data, pkt) != nil || capsdk.ValidateBusPacket(pkt) != nil ||
		capsdk.VerifyPacketSignature(pkt, s.bridgePub) != nil || pkt.GetJobRequest() == nil {
		return
	}
	req := pkt.GetJobRequest()
	s.mu.Lock()
	s.jobs[req.GetWorkflowId()] = append(s.jobs[req.GetWorkflowId()], req.GetJobId())
	hold := s.hold
	if hold && req.GetTopic() != e2eDeniedTopic {
		s.parked[req.GetJobId()] = pkt
	}
	s.mu.Unlock()
	if req.GetTopic() == e2eDeniedTopic {
		s.publish(capsdk.SubjectDLQ, &agentv1.BusPacket{TraceId: pkt.GetTraceId(), Payload: &agentv1.BusPacket_JobResult{
			JobResult: &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_DENIED,
				WorkerId: e2eScheduler, ErrorMessage: "denied by the test scheduler"}}})
		return
	}
	s.publish(capsdk.SubjectProgress, &agentv1.BusPacket{TraceId: pkt.GetTraceId(), Payload: &agentv1.BusPacket_JobProgress{
		JobProgress: &agentv1.JobProgress{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_DISPATCHED}}})
	if !hold {
		s.relay(pkt)
	}
}

func (s *ungovernedTestScheduler) relay(pkt *agentv1.BusPacket) {
	s.publish("worker."+e2eWorkerID+".jobs", &agentv1.BusPacket{TraceId: pkt.GetTraceId(),
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: pkt.GetJobRequest()}})
}

// publish signs pkt as cordum-scheduler and publishes it on subject.
func (s *ungovernedTestScheduler) publish(subject string, pkt *agentv1.BusPacket) {
	pkt.SenderId, pkt.CreatedAt, pkt.ProtocolVersion = e2eScheduler, timestamppb.Now(), capsdk.DefaultProtocolVersion
	raw, err := signed(pkt, s.key)
	if err == nil {
		err = s.nc.Publish(subject, raw)
	}
	if err != nil {
		s.t.Errorf("test scheduler publish %s: %v", subject, err)
	}
}

func (s *ungovernedTestScheduler) setHold(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hold = on
}

func (s *ungovernedTestScheduler) release(job string) {
	s.mu.Lock()
	pkt := s.parked[job]
	delete(s.parked, job)
	s.mu.Unlock()
	if pkt == nil {
		s.t.Fatalf("job %s is not parked", job)
	}
	s.relay(pkt)
}

// waitJob returns the n-th job submitted for task.
func (s *ungovernedTestScheduler) waitJob(t *testing.T, task a2a.TaskID, n int) string {
	t.Helper()
	var job string
	waitFor(t, fmt.Sprintf("job %d of %s", n, task), func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if jobs := s.jobs[string(task)]; len(jobs) >= n {
			job = jobs[n-1]
		}
		return job != ""
	})
	return job
}

// capture records every message on the server, in delivery order.
type capture struct {
	nc   *nats.Conn
	mu   sync.Mutex
	msgs []*nats.Msg
	seq  int
}

func newCapture(t *testing.T, nc *nats.Conn) *capture {
	t.Helper()
	c := &capture{nc: nc}
	_, err := nc.Subscribe(">", func(m *nats.Msg) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.msgs = append(c.msgs, m)
	})
	must(t, err)
	must(t, nc.Flush())
	return c
}

// sync flushes each publisher, then waits until the capture has handled a
// sentinel of its own, so every message published before the call is recorded.
func (c *capture) sync(t *testing.T, publishers ...*nats.Conn) {
	t.Helper()
	for _, p := range publishers {
		must(t, p.Flush())
	}
	c.mu.Lock()
	c.seq++
	subject := fmt.Sprintf("_e2e.sync.%d", c.seq)
	c.mu.Unlock()
	must(t, c.nc.Publish(subject, nil))
	waitFor(t, "capture sync", func() bool { return c.count(func(m *nats.Msg) bool { return m.Subject == subject }) > 0 })
}

func (c *capture) count(match func(*nats.Msg) bool) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, m := range c.msgs {
		if match(m) {
			n++
		}
	}
	return n
}

// cancels counts JobCancel packets for job, or for any job when job is empty.
func (c *capture) cancels(job string) int {
	return c.count(func(m *nats.Msg) bool {
		pkt := &agentv1.BusPacket{}
		if m.Subject != capsdk.SubjectCancel || proto.Unmarshal(m.Data, pkt) != nil || pkt.GetJobCancel() == nil {
			return false
		}
		return job == "" || pkt.GetJobCancel().GetJobId() == job
	})
}

// heartbeats counts valid, signed heartbeats with the advertised worker shape.
func (c *capture) heartbeats(t *testing.T, pub *ecdsa.PublicKey) int {
	return c.count(func(m *nats.Msg) bool {
		pkt := &agentv1.BusPacket{}
		if m.Subject != capsdk.SubjectHeartbeat || proto.Unmarshal(m.Data, pkt) != nil {
			return false
		}
		hb := pkt.GetHeartbeat()
		if capsdk.ValidateBusPacket(pkt) != nil || capsdk.VerifyPacketSignature(pkt, pub) != nil ||
			hb.GetWorkerId() != e2eWorkerID || hb.GetPool() != "a2a-demo" || hb.GetMaxParallelJobs() != 4 {
			t.Errorf("unexpected heartbeat %v", pkt)
			return false
		}
		return true
	})
}

// assertNoCredentials scans every captured message, raw and decoded.
func (c *capture) assertNoCredentials(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if leaksCredential(m.Data) {
			t.Fatalf("message on %s carries a credential", m.Subject)
		}
		pkt := &agentv1.BusPacket{}
		if proto.Unmarshal(m.Data, pkt) == nil && (pkt.GetAuthToken() != "" || pkt.GetHeartbeat().GetAuthToken() != "") {
			t.Fatalf("packet on %s carries an auth_token", m.Subject)
		}
	}
}

func leaksCredential(b []byte) bool {
	lower := bytes.ToLower(b)
	return bytes.Contains(lower, []byte(e2eAPIKey)) || bytes.Contains(lower, []byte("bearer"))
}

func signed(pkt *agentv1.BusPacket, key *ecdsa.PrivateKey) ([]byte, error) {
	if err := capsdk.SignPacket(pkt, key); err != nil {
		return nil, err
	}
	return capsdk.MarshalDeterministic(pkt)
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(t, err)
	return k
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
