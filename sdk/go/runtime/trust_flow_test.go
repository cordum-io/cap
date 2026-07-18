package runtime

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type cancelAwareBus struct {
	mockNATS
	contextCalls  atomic.Int32
	fallbackCalls atomic.Int32
}

func (b *cancelAwareBus) Request(string, []byte, time.Duration) (*nats.Msg, error) {
	b.fallbackCalls.Add(1)
	return nil, errors.New("fallback request used")
}

func (b *cancelAwareBus) RequestWithContext(ctx context.Context, _ string, _ []byte) (*nats.Msg, error) {
	b.contextCalls.Add(1)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestAgentStartRunsAuthenticatedTrustFlowBeforeSubscriptions(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	if err := agent.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = agent.Close() })
	if agent.SenderID != bus.config.WorkerID {
		t.Fatalf("SenderID=%q, want worker trust identity", agent.SenderID)
	}
	wantOrder := []string{
		"request:" + capsdk.WorkerHandshakeChallengeSubject,
		"request:" + capsdk.WorkerHandshakeAuthenticateSubject,
		"subscribe:job.runtime",
	}
	for i, want := range wantOrder {
		if i >= len(bus.order) || bus.order[i] != want {
			t.Fatalf("order=%v, want prefix %v", bus.order, wantOrder)
		}
	}
	if token, exp := agent.SessionToken(); !strings.HasPrefix(token, "session-") || !exp.After(time.Now()) {
		t.Fatalf("session=(%q,%v), want verified live token", token, exp)
	}
	if bus.capability == nil || !proto.Equal(bus.capability, agent.capabilityHandshake()) {
		t.Fatal("authenticated and broadcast capability handshakes differ")
	}
	if len(bus.purposes) != 1 || bus.purposes[0] != agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE {
		t.Fatalf("purposes=%v, want ISSUE", bus.purposes)
	}
	if len(bus.authTokens) != 1 || bus.authTokens[0] != "" {
		t.Fatalf("issue auth tokens=%q, want empty", bus.authTokens)
	}
	if len(bus.traceIDs) != 2 || bus.traceIDs[0] == "" || bus.traceIDs[0] != bus.traceIDs[1] {
		t.Fatalf("trust trace IDs=%q, want one nonempty stable value", bus.traceIDs)
	}
	assertGenericHandshake(t, bus)
}

func assertGenericHandshake(t *testing.T, bus *trustTestBus) {
	t.Helper()
	for _, published := range bus.published {
		if published.subject != capsdk.SubjectHandshake {
			continue
		}
		packet := &agentv1.BusPacket{}
		if err := proto.Unmarshal(published.data, packet); err != nil {
			t.Fatalf("decode generic handshake: %v", err)
		}
		if packet.GetTraceId() == "" || !strings.HasPrefix(packet.GetAuthToken(), "session-") {
			t.Fatalf("generic handshake trace/token=(%q,%q)", packet.GetTraceId(), packet.GetAuthToken())
		}
		if !proto.Equal(packet.GetHandshake(), bus.capability) {
			t.Fatal("generic handshake did not mirror authenticated capability payload")
		}
		return
	}
	t.Fatal("generic handshake was not published")
}

func TestAgentStartTrustFailureHonorsModeWithoutInstallingToken(t *testing.T) {
	for _, test := range []struct {
		mode      HandshakeMode
		wantError bool
		wantSubs  int
	}{
		{mode: HandshakeModeWarn, wantSubs: 1},
		{mode: HandshakeModeEnforce, wantError: true},
	} {
		t.Run(string(test.mode), func(t *testing.T) {
			agent, bus := newTrustTestAgent(t, test.mode)
			bus.unsignedResult = true
			err := agent.Start()
			if (err != nil) != test.wantError {
				t.Fatalf("Start error=%v, wantError=%v", err, test.wantError)
			}
			if token, _ := agent.SessionToken(); token != "" {
				t.Fatalf("unverified token installed: %q", token)
			}
			if got := len(bus.subs); got != test.wantSubs {
				t.Fatalf("subscriptions=%d, want %d", got, test.wantSubs)
			}
			_ = agent.Close()
		})
	}
}

func TestAgentStartRejectsPartialTrustConfigBeforeTransport(t *testing.T) {
	for _, mode := range []HandshakeMode{HandshakeModeWarn, HandshakeModeEnforce} {
		t.Run(string(mode), func(t *testing.T) {
			agent := &Agent{
				NATSURL: "nats://127.0.0.1:1", Store: NewInMemoryBlobStore(),
				HandshakeMode: mode, WorkerTrust: capsdk.WorkerTrustConfig{WorkerID: "partial"},
				Logger: silentLogger(), IOTTimeout: 20 * time.Millisecond,
			}
			Register(agent, "job.partial", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
			err := agent.Start()
			if err == nil || !strings.Contains(err.Error(), "configuration") {
				t.Fatalf("Start error=%v, want pre-transport configuration failure", err)
			}
		})
	}
}

func TestAgentStartRejectsUnboundedTrustTuning(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Agent)
	}{
		{name: "timeout", mutate: func(agent *Agent) { agent.HandshakeTimeout = 2 * time.Minute }},
		{name: "retries", mutate: func(agent *Agent) { agent.HandshakeRetries = 11 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			agent, _ := newTrustTestAgent(t, HandshakeModeEnforce)
			test.mutate(agent)
			if err := agent.Start(); err == nil || !strings.Contains(err.Error(), "configuration") {
				t.Fatalf("Start error=%v, want bounded configuration failure", err)
			}
		})
	}
}

func TestAgentStartRejectsTrustConfigWhenModeOff(t *testing.T) {
	agent, _ := newTrustTestAgent(t, HandshakeModeOff)
	err := agent.Start()
	if err == nil || !strings.Contains(err.Error(), "mode off conflicts") {
		t.Fatalf("Start error=%v, want contradictory off/config failure", err)
	}
}

func TestAgentStartRejectsNilTrustResponseWithoutPanic(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	bus.nilResponse = true
	if err := agent.Start(); err == nil || !strings.Contains(err.Error(), "nil response") {
		t.Fatalf("Start error=%v, want nil-response failure", err)
	}
	if token, _ := agent.SessionToken(); token != "" {
		t.Fatalf("nil response installed token %q", token)
	}
}

func TestPerformRenewAuthenticatesCurrentSession(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	agent.setSession("current-session", time.Now().Add(time.Hour))
	obtained, err := agent.performRenew(nil)
	if err != nil || !obtained {
		t.Fatalf("performRenew=(%v,%v), want (true,nil)", obtained, err)
	}
	if len(bus.purposes) != 1 || bus.purposes[0] != agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW {
		t.Fatalf("purposes=%v, want RENEW", bus.purposes)
	}
	if len(bus.authTokens) != 1 || bus.authTokens[0] != "current-session" {
		t.Fatalf("renew auth tokens=%q, want current session", bus.authTokens)
	}
	if token, _ := agent.SessionToken(); !strings.HasPrefix(token, "session-") {
		t.Fatalf("renew did not atomically install verified token: %q", token)
	}
}

func TestWarnRenewFailureRetainsOnlyUnexpiredSession(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeWarn)
	bus.unsignedResult = true
	agent.setSession("still-live", time.Now().Add(time.Hour))
	if obtained, err := agent.performRenew(nil); err != nil || obtained {
		t.Fatalf("live-token renew=(%v,%v), want (false,nil)", obtained, err)
	}
	if token, _ := agent.SessionToken(); token != "still-live" {
		t.Fatalf("warn mode discarded live token: %q", token)
	}
	requests := len(bus.order)
	agent.setSession("expired", time.Now().Add(-time.Second))
	if obtained, err := agent.performRenew(nil); err == nil || obtained {
		t.Fatalf("expired-token renew=(%v,%v), want (false,error)", obtained, err)
	}
	if token, _ := agent.SessionToken(); token != "" {
		t.Fatalf("expired token retained: %q", token)
	}
	if len(bus.order) != requests {
		t.Fatal("expired session triggered a renewal request")
	}
}

func TestEnforceRenewFailureClearsCurrentSession(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	bus.unsignedResult = true
	agent.setSession("current-session", time.Now().Add(time.Hour))
	if obtained, err := agent.performRenew(nil); err == nil || obtained {
		t.Fatalf("performRenew=(%v,%v), want (false,error)", obtained, err)
	}
	if token, _ := agent.SessionToken(); token != "" {
		t.Fatalf("enforce mode retained token after failed renew: %q", token)
	}
}

func TestEnforceRenewFailureRemovesJobAdmissions(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agent.Close() })
	if len(agent.subs) != 1 {
		t.Fatalf("subscriptions before renew=%d, want 1", len(agent.subs))
	}
	bus.unsignedResult = true
	if obtained, err := agent.performRenew(nil); err == nil || obtained {
		t.Fatalf("performRenew=(%v,%v), want (false,error)", obtained, err)
	}
	if len(agent.subs) != 0 {
		t.Fatalf("subscriptions after enforce renew failure=%d, want 0", len(agent.subs))
	}
}

func TestTrustRequestUsesContextCancellation(t *testing.T) {
	agent, fixtureBus := newTrustTestAgent(t, HandshakeModeEnforce)
	bus := &cancelAwareBus{mockNATS: *newMockNATS()}
	agent.NATS = bus
	agent.WorkerTrust = fixtureBus.config
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if obtained, err := agent.performHandshake(ctx); err == nil || obtained {
		t.Fatalf("performHandshake=(%v,%v), want cancellation error", obtained, err)
	}
	if bus.contextCalls.Load() != 1 || bus.fallbackCalls.Load() != 0 {
		t.Fatalf("context/fallback calls=%d/%d, want 1/0", bus.contextCalls.Load(), bus.fallbackCalls.Load())
	}
}

func TestCloseWaitsForRenewLoopAndIsIdempotent(t *testing.T) {
	agent, _ := newTrustTestAgent(t, HandshakeModeEnforce)
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	renew := agent.renew
	if renew == nil {
		t.Fatal("successful trust flow did not start renewal loop")
	}
	if err := agent.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-renew.done:
	default:
		t.Fatal("Close returned before renewal loop exited")
	}
	if err := agent.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestAgentStartClonesCallerOwnedTrustMaterial(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agent.Close() })
	wantProofD := agent.workerTrustConfig().ProofPrivateKey.D.String()
	wantSchedulerX := agent.workerTrustConfig().SchedulerPublicKeys["scheduler-runtime-key"].X.String()
	bus.config.ExpectedSchedulerID = "scheduler-attacker"
	bus.config.ProofPrivateKey.D.SetInt64(1)
	delete(bus.config.SchedulerPublicKeys, "scheduler-runtime-key")
	agent.WorkerTrust.ExpectedSchedulerID = "scheduler-direct-mutation"
	if agent.workerTrustConfig().ExpectedSchedulerID != "scheduler-runtime" || agent.workerTrustConfig().ProofPrivateKey.D.String() != wantProofD {
		t.Fatal("caller mutation changed runtime proof identity")
	}
	key := agent.workerTrustConfig().SchedulerPublicKeys["scheduler-runtime-key"]
	if key == nil || key.X.String() != wantSchedulerX {
		t.Fatal("caller mutation changed runtime scheduler pins")
	}
}
