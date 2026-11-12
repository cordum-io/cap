// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"context"
	"crypto/ecdsa"
	"io"
	"log/slog"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/internal/tck"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type rtFixture struct {
	r      *Runtime
	cfg    *ResolvedConfig
	nc     *nats.Conn
	subs   map[string]*nats.Subscription
	keys   map[string]*ecdsa.PrivateKey
	now    atomic.Int64
	cancel context.CancelFunc
	done   chan error
}

func rtConfig(t *testing.T) *rtFixture {
	t.Helper()
	f := &rtFixture{keys: map[string]*ecdsa.PrivateKey{}, subs: map[string]*nats.Subscription{}}
	f.now.Store(time.Now().UnixNano())
	f.cfg = &ResolvedConfig{SchedulerID: "scheduler", SigningKeyID: "k1", SigningKey: trKey(t), TrustKeyID: "trust", TrustKey: trKey(t),
		Workers: map[string]ResolvedWorker{}, Clients: map[string]ResolvedClient{}, Allow: []AllowRule{{"t1", "topicA", "capA"}},
		TopicWorkers: map[string][]string{"topicA": {"w2", "w3", "w1"}}, MaxPacketBytes: 65536, MaxConcurrency: 4, MaxLiveJobs: 2,
		MaxChallenges: 8, ChallengeTTL: 20 * time.Second, SessionTTL: 5 * time.Minute,
		Limits: Limits{MaxInputBytes: 1024, MaxOutputBytes: 1024, MaxArtifacts: 4, MaxAttempts: 2, DispatchTimeout: 10 * time.Second, RunTimeout: 30 * time.Second, MaxRunTimeout: time.Minute}}
	for _, id := range []string{"w1", "w2", "w3", "c1"} {
		f.keys[id] = trKey(t)
		tenant := "t1"
		if id == "w2" {
			tenant = "t2"
		}
		keys := map[string]*ecdsa.PublicKey{"k1": &f.keys[id].PublicKey}
		f.cfg.Workers[id] = ResolvedWorker{WorkerID: id, TenantID: tenant, AgentID: id, ProofKeyID: "k1", ProofKey: &f.keys[id].PublicKey, SigningKeys: keys}
	}
	delete(f.cfg.Workers, "c1")
	f.cfg.Clients["c1"] = ResolvedClient{SenderID: "c1", TenantID: "t1", SigningKeys: map[string]*ecdsa.PublicKey{"k1": &f.keys["c1"].PublicKey}}
	return f
}

func (f *rtFixture) clock() time.Time { return time.Unix(0, f.now.Load()) }

func rtStart(t *testing.T) *rtFixture {
	t.Helper()
	f := rtConfig(t)
	srv, err := tck.StartEmbeddedNATS()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	f.cfg.NATSURL = srv.URL()
	f.nc, err = nats.Connect(srv.URL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.nc.Close)
	for _, subject := range []string{WorkerSubject("w1"), WorkerSubject("w2"), WorkerSubject("w3"), SubjectTerminal, SubjectCancel} {
		f.subs[subject], err = f.nc.SubscribeSync(subject)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = f.nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	f.r, err = NewRuntime(RuntimeOptions{Config: f.cfg, Now: f.clock, TickEvery: time.Hour, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel, f.done = cancel, make(chan error, 1)
	go func() { f.done <- f.r.Run(ctx) }()
	t.Cleanup(func() { rtStop(t, f) })
	select {
	case <-f.r.Ready():
	case err = <-f.done:
		f.done = nil
		t.Fatalf("Run before Ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Ready timeout")
	}
	return f
}

func rtStop(t *testing.T, f *rtFixture) {
	t.Helper()
	f.cancel()
	if f.done == nil {
		return
	}
	select {
	case err := <-f.done:
		f.done = nil
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("drain timeout")
	}
}

func rtEventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for !cond() {
		select {
		case <-poll.C:
		case <-deadline.C:
			t.Fatal("condition timeout")
		}
	}
}

func rtExchange(t *testing.T, f *rtFixture, subject string, p *agentv1.BusPacket) *agentv1.BusPacket {
	t.Helper()
	raw, err := capsdk.MarshalWorkerTrustPacket(p)
	if err != nil {
		t.Fatal(err)
	}
	m, err := f.nc.Request(subject, raw, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	p, err = capsdk.UnmarshalWorkerTrustPacket(m.Data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func rtHandshake(t *testing.T, f *rtFixture, worker string) string {
	t.Helper()
	c := capsdk.WorkerTrustConfig{WorkerID: worker, ExpectedAgentID: worker, TenantID: f.cfg.Workers[worker].TenantID,
		Audience: capsdk.WorkerHandshakeAudience, ProofKeyID: "k1", ProofPrivateKey: f.keys[worker], SDKVersion: "test",
		ExpectedSchedulerID: f.cfg.SchedulerID, SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"trust": &f.cfg.TrustKey.PublicKey}}
	request, err := capsdk.BuildWorkerHandshakeChallengeRequest(&c, capsdk.WorkerHandshakeRequestOptions{RequestID: "request", TraceID: "trace", Purpose: trIssue, ClientNonce: make([]byte, 32), CreatedAt: f.clock()})
	if err != nil {
		t.Fatal(err)
	}
	challenge := rtExchange(t, f, capsdk.WorkerHandshakeChallengeSubject, request)
	v, err := capsdk.VerifyWorkerHandshakeChallenge(&c, request, challenge, f.clock())
	if err != nil {
		t.Fatal(err)
	}
	auth, err := capsdk.BuildWorkerHandshakeAuthenticate(&c, v, &agentv1.Handshake{ComponentId: worker, Role: agentv1.ComponentRole_COMPONENT_ROLE_WORKER, SdkVersion: "test", SupportedVersions: []int32{1}}, "", f.clock())
	if err != nil {
		t.Fatal(err)
	}
	result := rtExchange(t, f, capsdk.WorkerHandshakeAuthenticateSubject, auth)
	s, err := capsdk.VerifyWorkerHandshakeResult(&c, v, auth, result, f.clock())
	if err != nil {
		t.Fatal(err)
	}
	return s.Token
}

func rtPacket(t *testing.T, sender, token, subject, job string) *agentv1.BusPacket {
	t.Helper()
	p := adPacket(t, sender, "t1", token, subject, job)
	p.Identity.PrincipalId, p.Identity.ActorId = "p1", "a1"
	if req := p.GetJobRequest(); req != nil {
		req.Topic, req.TenantId, req.ContextRef, req.Meta = "topicA", "t1", &agentv1.ResourceRef{SizeBytes: 1}, &agentv1.JobMetadata{Capability: "capA"}
	}
	return p
}

func rtSend(t *testing.T, f *rtFixture, subject string, p *agentv1.BusPacket) {
	t.Helper()
	rtRaw(t, f, subject, adSign(t, f.keys[p.SenderId], p))
}

func rtRaw(t *testing.T, f *rtFixture, subject string, raw []byte) {
	t.Helper()
	if err := f.nc.Publish(subject, raw); err != nil {
		t.Fatal(err)
	}
	if err := f.nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
}

func rtReceive(t *testing.T, f *rtFixture, subject string) *agentv1.BusPacket {
	t.Helper()
	m, err := f.subs[subject].NextMsg(time.Second)
	if err != nil {
		t.Fatalf("receive %s: %v", subject, err)
	}
	v, err := capsdk.VerifyTrustedProductionPacket(m.Data, capsdk.ProductionTrustStore{Audience: subject, Tenant: "t1", Sender: f.cfg.SchedulerID,
		PublicKeys: map[string]*ecdsa.PublicKey{"k1": &f.cfg.SigningKey.PublicKey}, Now: f.clock})
	if err != nil {
		t.Fatal(err)
	}
	return v.Packet()
}

func rtDispatch(t *testing.T, f *rtFixture, job string) *agentv1.BusPacket {
	t.Helper()
	rtSend(t, f, SubjectSubmit, rtPacket(t, "c1", "", SubjectSubmit, job))
	p := rtReceive(t, f, WorkerSubject("w1"))
	rtEventually(t, func() bool { j, _ := f.r.core.Get("t1", job); return j.Acked })
	return p
}

func rtResult(t *testing.T, dispatch *agentv1.BusPacket, token string) *agentv1.BusPacket {
	t.Helper()
	p := rtPacket(t, "w1", token, SubjectResult, dispatch.GetJobRequest().JobId)
	p.Identity = clone(dispatch.Identity)
	p.GetJobResult().Dispatch = clone(dispatch.GetJobRequest().Dispatch)
	return p
}

func TestRuntime_RejectedPacketsTouchNothing(t *testing.T) {
	f := rtStart(t)
	token := rtHandshake(t, f, "w1")
	d := rtDispatch(t, f, "job")
	for _, name := range []string{"unsigned", "expired", "wrong-key", "wrong-audience", "missing-token", "stale-token", "wrong-attempt"} {
		t.Run(name, func(t *testing.T) {
			p := rtResult(t, d, token)
			key, coreCalls := f.keys["w1"], uint64(0)
			switch name {
			case "expired":
				p.SignatureMetadata.ExpiresAt = timestamppb.New(time.Now().Add(time.Second))
			case "wrong-key":
				key = trKey(t)
			case "wrong-audience":
				p.SignatureMetadata.Audience = SubjectProgress
			case "missing-token":
				p.AuthToken = ""
			case "stale-token":
				p.AuthToken = "stale-token-canary"
			case "wrong-attempt":
				p.GetJobResult().Dispatch.Attempt++
				coreCalls = 1
			}
			raw := adSign(t, key, p)
			if name == "unsigned" {
				var err error
				raw, err = capsdk.MarshalDeterministic(p)
				if err != nil {
					t.Fatal(err)
				}
			}
			if name == "expired" {
				f.now.Add(int64(2 * time.Second))
			}
			before, _ := f.r.core.Get("t1", "job")
			stats := f.r.Stats()
			rtRaw(t, f, SubjectResult, raw)
			rtEventually(t, func() bool { return f.r.Stats().Dropped > stats.Dropped })
			after, _ := f.r.core.Get("t1", "job")
			got := f.r.Stats()
			want := RuntimeStats{stats.Dropped + 1, stats.CoreCalls + coreCalls, stats.Published}
			if got != want || !reflect.DeepEqual(before, after) {
				t.Fatalf("stats %v want %v; snapshot equal=%v", got, want, reflect.DeepEqual(before, after))
			}
			t.Logf("Dropped +1 CoreCalls +%d Published +0; snapshot unchanged", coreCalls)
		})
	}
}

func TestRuntime_FlowWithSDKWorker(t *testing.T) {
	f := rtStart(t)
	token := rtHandshake(t, f, "w1")
	rtHandshake(t, f, "w2")
	before := f.r.Stats().Published
	d := rtDispatch(t, f, "job")
	identity := rtPacket(t, "c1", "", SubjectSubmit, "job").Identity
	if !proto.Equal(d.Identity, identity) || d.GetJobRequest().Dispatch.AssignedWorkerId != "w1" || f.r.Stats().Published != before+1 {
		t.Fatal("dispatch scope, binding or count")
	}
	for _, w := range []string{"w2", "w3"} {
		if n, _, err := f.subs[WorkerSubject(w)].Pending(); err != nil || n != 0 {
			t.Fatal("ineligible worker received dispatch", w, n, err)
		}
	}
	p := rtResult(t, d, token)
	rtSend(t, f, SubjectResult, p)
	winner := rtReceive(t, f, SubjectTerminal)
	if winner.GetJobResult().Status != agentv1.JobStatus_JOB_STATUS_SUCCEEDED || !proto.Equal(winner.Identity, identity) {
		t.Fatal("terminal status or binding")
	}
	// A new signed message carrying the identical winning result is a core no-op, too.
	stats := f.r.Stats()
	rtSend(t, f, SubjectResult, rtResult(t, d, token))
	rtEventually(t, func() bool { return f.r.Stats().Dropped > stats.Dropped })
	if f.r.Stats().Published != stats.Published {
		t.Fatal("repeated result amplified")
	}
	rtDenied(t, f, identity)
}

func rtDenied(t *testing.T, f *rtFixture, identity *agentv1.IdentityBinding) {
	t.Helper()
	p := rtPacket(t, "c1", "", SubjectSubmit, "denied")
	p.GetJobRequest().Meta.Capability = "disallowed"
	before := f.r.Stats().Published
	rtSend(t, f, SubjectSubmit, p)
	first := rtReceive(t, f, SubjectTerminal)
	// Request principal/actor mirrors are absent; both envelope bindings are valid.
	p.Identity.PrincipalId = "p2"
	p.SignatureMetadata.MessageId[0] ^= 1
	rtSend(t, f, SubjectSubmit, p)
	second := rtReceive(t, f, SubjectTerminal)
	if first.GetJobResult().Status != agentv1.JobStatus_JOB_STATUS_DENIED || !proto.Equal(first.GetJobResult(), second.GetJobResult()) || !proto.Equal(second.Identity, identity) || f.r.Stats().Published != before+2 {
		t.Fatal("DENIED duplicate or binding")
	}
}

func TestRuntime_ReadyAndDrain(t *testing.T) {
	f := rtStart(t)
	if len(f.r.subs) != 6 {
		t.Fatal("subscription count")
	}
	for _, sub := range f.r.subs {
		if !sub.IsValid() {
			t.Fatal("ready before live")
		}
	}
	rtRaw(t, f, SubjectSubmit, []byte("invalid"))
	rtEventually(t, func() bool { return f.r.Stats().Dropped == 1 })
	rtStop(t, f)
	for _, sub := range f.r.subs {
		if sub.IsValid() {
			t.Fatal("subscription remains live")
		}
	}
	if !f.r.nc.IsClosed() || f.r.goroutines.Load() != 0 {
		t.Fatal("connection or goroutine leak")
	}
}

func TestRuntime_TickPublishesTimeout(t *testing.T) {
	f := rtStart(t)
	rtHandshake(t, f, "w1")
	rtDispatch(t, f, "job")
	f.now.Add(int64(11 * time.Second))
	f.r.tickOnce()
	if p := rtReceive(t, f, SubjectTerminal); p.GetJobResult().Status != agentv1.JobStatus_JOB_STATUS_TIMEOUT {
		t.Fatal("not TIMEOUT")
	}
}

func TestRuntime_ClientCancel(t *testing.T) {
	f := rtStart(t)
	rtHandshake(t, f, "w1")
	d := rtDispatch(t, f, "job")
	rtSend(t, f, SubjectClientCancel, rtPacket(t, "c1", "", SubjectClientCancel, "job"))
	winner := rtReceive(t, f, SubjectTerminal)
	cancel := rtReceive(t, f, SubjectCancel)
	if winner.GetJobResult().Status != agentv1.JobStatus_JOB_STATUS_CANCELLED || cancel.GetJobCancel().JobId != "job" || !proto.Equal(cancel.GetJobCancel().Dispatch, d.GetJobRequest().Dispatch) || !proto.Equal(cancel.Identity, d.Identity) {
		t.Fatal("cancel fencing or binding")
	}
	before := f.r.Stats().Published
	rtSend(t, f, SubjectClientCancel, rtPacket(t, "c1", "", SubjectClientCancel, "job"))
	if p := rtReceive(t, f, SubjectTerminal); !proto.Equal(p.GetJobResult(), winner.GetJobResult()) || f.r.Stats().Published != before+1 {
		t.Fatal("cancel winner republish")
	}
}

func TestRuntime_LiveJobBound(t *testing.T) {
	f := rtStart(t)
	for _, id := range []string{"one", "two"} {
		rtSend(t, f, SubjectSubmit, rtPacket(t, "c1", "", SubjectSubmit, id))
		rtEventually(t, func() bool { j, _ := f.r.core.Get("t1", id); return j.JobID == id })
	}
	before := f.r.Stats()
	rtSend(t, f, SubjectSubmit, rtPacket(t, "c1", "", SubjectSubmit, "three"))
	rtEventually(t, func() bool { return f.r.Stats().Dropped == before.Dropped+1 })
	if got := f.r.Stats(); got.CoreCalls != before.CoreCalls || got.Published != before.Published {
		t.Fatal("bound touched core or published")
	}
	if _, err := f.r.core.Get("t1", "three"); err != ErrJobNotFound {
		t.Fatal(err)
	}
}

func TestNewRuntime_InvalidOptions(t *testing.T) {
	f := rtConfig(t)
	for _, opts := range []RuntimeOptions{{}, {Config: f.cfg}, {Config: f.cfg, Now: f.clock}, {Config: f.cfg, Now: f.clock, TickEvery: -1}} {
		if r, err := NewRuntime(opts); r != nil || err != ErrInvalidConfig {
			t.Fatalf("%v %v", r, err)
		}
	}
}
