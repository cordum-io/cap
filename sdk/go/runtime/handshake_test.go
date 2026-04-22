package runtime

// Agent.Start Phase-2 handshake integration tests. Each case drives a
// real Agent + a real HandshakeRequest encoded by capsdk.Marshal… and
// a test-local NATSRequester that produces canned responses. No
// mocks of the handshake protocol itself — the wire is exercised
// byte-for-byte, so the tests catch protocol-shape regressions as
// well as flow-control bugs.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

// handshakeBus is mockNATS with a Request method layered on so the
// Agent's NATSRequester type assertion succeeds. responder runs for
// every Request call and returns the bytes the scheduler would have
// replied with.
type handshakeBus struct {
	mockNATS
	respond func(subj string, data []byte) ([]byte, error)
	calls   atomic.Int32
}

func newHandshakeBus(respond func(subj string, data []byte) ([]byte, error)) *handshakeBus {
	return &handshakeBus{
		mockNATS: *newMockNATS(),
		respond:  respond,
	}
}

func (h *handshakeBus) Request(subj string, data []byte, _ time.Duration) (*nats.Msg, error) {
	h.calls.Add(1)
	if h.respond == nil {
		return nil, errors.New("handshake-bus: no responder configured")
	}
	payload, err := h.respond(subj, data)
	if err != nil {
		return nil, err
	}
	return &nats.Msg{Subject: subj, Data: payload}, nil
}

// silentLogger discards log output so the tests stay quiet under -v.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newAgentFixture(t *testing.T, bus *handshakeBus, mode HandshakeMode) *Agent {
	t.Helper()
	a := &Agent{
		NATS:     bus,
		Store:    NewInMemoryBlobStore(),
		SenderID: "worker-fixture",
		Tenant:   "tenant-fixture",
		SDKVersion:    "cap-go/v2.9.0-test",
		HandshakeMode: mode,
		HandshakeTimeout: 200 * time.Millisecond,
		HandshakeRetries: 2,
		Logger:   silentLogger(),
	}
	Register(a, "job.test", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	return a
}

func acceptResponse(t *testing.T, data []byte, token string) []byte {
	t.Helper()
	req, err := capsdk.UnmarshalHandshakeRequest(data)
	if err != nil {
		t.Fatalf("test-scheduler: parse request: %v", err)
	}
	resp := &capsdk.HandshakeResponse{
		SessionToken: token,
		TokenExp:     time.Now().Add(1 * time.Hour).UTC(),
		RequestID:    req.RequestID,
	}
	out, err := capsdk.MarshalHandshakeResponse(resp)
	if err != nil {
		t.Fatalf("test-scheduler: marshal response: %v", err)
	}
	return out
}

func rejectResponse(t *testing.T, data []byte, reason string) []byte {
	t.Helper()
	req, err := capsdk.UnmarshalHandshakeRequest(data)
	if err != nil {
		t.Fatalf("test-scheduler: parse request: %v", err)
	}
	resp := &capsdk.HandshakeResponse{
		Rejected:  true,
		Reason:    reason,
		RequestID: req.RequestID,
	}
	out, err := capsdk.MarshalHandshakeResponse(resp)
	if err != nil {
		t.Fatalf("test-scheduler: marshal response: %v", err)
	}
	return out
}

func TestAgentStart_HandshakeOffSkipsEntirely(t *testing.T) {
	t.Parallel()
	bus := newHandshakeBus(func(string, []byte) ([]byte, error) {
		t.Fatal("scheduler must not receive a handshake when mode=off")
		return nil, nil
	})
	a := newAgentFixture(t, bus, HandshakeModeOff)
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if bus.calls.Load() != 0 {
		t.Fatalf("off mode must not call Request; got %d calls", bus.calls.Load())
	}
	token, exp := a.SessionToken()
	if token != "" || !exp.IsZero() {
		t.Fatalf("off mode must not install a session token; got %q exp=%v", token, exp)
	}
}

func TestAgentStart_HandshakeWarnSucceedsOnAccept(t *testing.T) {
	t.Parallel()
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		return acceptResponse(t, data, "token-acc"), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	token, exp := a.SessionToken()
	if token != "token-acc" {
		t.Fatalf("token=%q want token-acc", token)
	}
	if exp.IsZero() {
		t.Fatalf("exp must be populated")
	}
	if bus.calls.Load() != 1 {
		t.Fatalf("expected exactly 1 handshake call; got %d", bus.calls.Load())
	}
}

func TestAgentStart_HandshakeWarnSwallowsTransportError(t *testing.T) {
	t.Parallel()
	bus := newHandshakeBus(func(string, []byte) ([]byte, error) {
		return nil, errors.New("network wedged")
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	// warn mode should NOT fail Start() even if every retry fails.
	if err := a.Start(); err != nil {
		t.Fatalf("warn mode must not surface transport errors; got %v", err)
	}
	token, _ := a.SessionToken()
	if token != "" {
		t.Fatalf("no token should be installed after transport failure; got %q", token)
	}
	if int(bus.calls.Load()) != 2 {
		t.Fatalf("warn mode should consume all retries; got %d calls (want %d)", bus.calls.Load(), 2)
	}
}

func TestAgentStart_HandshakeEnforceFailsOnTransportError(t *testing.T) {
	t.Parallel()
	bus := newHandshakeBus(func(string, []byte) ([]byte, error) {
		return nil, errors.New("network wedged")
	})
	a := newAgentFixture(t, bus, HandshakeModeEnforce)
	err := a.Start()
	if err == nil {
		t.Fatal("enforce mode must return an error on persistent transport failure")
	}
}

func TestAgentStart_HandshakeRejectionIsTerminal(t *testing.T) {
	t.Parallel()
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		return rejectResponse(t, data, capsdk.HandshakeRejectUnknownAgent), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeEnforce)
	err := a.Start()
	if err == nil {
		t.Fatal("enforce mode must surface rejection as an error")
	}
	var rej *HandshakeRejectedError
	if !errors.As(err, &rej) {
		t.Fatalf("expected HandshakeRejectedError; got %T: %v", err, err)
	}
	if rej.Reason != capsdk.HandshakeRejectUnknownAgent {
		t.Errorf("reason=%q want %q", rej.Reason, capsdk.HandshakeRejectUnknownAgent)
	}
	// Rejection is terminal → exactly one handshake call, no retries.
	if bus.calls.Load() != 1 {
		t.Fatalf("rejection must not retry; got %d calls", bus.calls.Load())
	}
}

func TestAgentStart_HandshakeRetriesThenSucceeds(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		n := attempts.Add(1)
		if n < 2 {
			return nil, errors.New("transient")
		}
		return acceptResponse(t, data, "token-retry"), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	token, _ := a.SessionToken()
	if token != "token-retry" {
		t.Fatalf("token=%q want token-retry", token)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts before success; got %d", attempts.Load())
	}
}

func TestAgentStart_HandshakeRequestIDMismatchRetries(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		n := attempts.Add(1)
		if n == 1 {
			// First attempt: reply with a mismatched request_id. The
			// Agent must treat this as a transport-equivalent error
			// and retry rather than install the token.
			req, _ := capsdk.UnmarshalHandshakeRequest(data)
			resp := &capsdk.HandshakeResponse{
				SessionToken: "token-bad",
				TokenExp:     time.Now().Add(1 * time.Hour).UTC(),
				RequestID:    req.RequestID + "-tampered",
			}
			raw, _ := json.Marshal(resp)
			return raw, nil
		}
		return acceptResponse(t, data, "token-good"), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	token, _ := a.SessionToken()
	if token != "token-good" {
		t.Fatalf("request_id mismatch must be discarded; got token=%q", token)
	}
}

func TestAgentStart_HandshakeRequiresTenantInEnforce(t *testing.T) {
	t.Parallel()
	bus := newHandshakeBus(nil)
	a := newAgentFixture(t, bus, HandshakeModeEnforce)
	a.Tenant = "" // empty
	err := a.Start()
	if err == nil {
		t.Fatal("enforce mode must refuse empty Tenant")
	}
}

func TestAgentStart_HandshakeSkipsWhenBusDoesNotSupportRequest(t *testing.T) {
	t.Parallel()
	a := &Agent{
		NATS:          newMockNATS(), // mockNATS does NOT implement NATSRequester
		Store:         NewInMemoryBlobStore(),
		SenderID:      "worker-no-req",
		Tenant:        "tenant",
		SDKVersion:    "cap-go/v2",
		HandshakeMode: HandshakeModeWarn,
		Logger:        silentLogger(),
	}
	Register(a, "job.test", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	if err := a.Start(); err != nil {
		t.Fatalf("warn mode must not fail when bus lacks Request; got %v", err)
	}
}

func TestAgentStart_HandshakeEnforceRejectsBusWithoutRequest(t *testing.T) {
	t.Parallel()
	a := &Agent{
		NATS:          newMockNATS(),
		Store:         NewInMemoryBlobStore(),
		SenderID:      "worker-no-req",
		Tenant:        "tenant",
		SDKVersion:    "cap-go/v2",
		HandshakeMode: HandshakeModeEnforce,
		Logger:        silentLogger(),
	}
	Register(a, "job.test", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	err := a.Start()
	if err == nil {
		t.Fatal("enforce mode must refuse a bus that doesn't implement NATSRequester")
	}
}

func TestHandshakeBackoff_Monotonic(t *testing.T) {
	t.Parallel()
	prev := handshakeBackoff(0)
	for attempt := 1; attempt < 5; attempt++ {
		cur := handshakeBackoff(attempt)
		if cur <= prev {
			t.Fatalf("backoff not monotonically increasing: attempt=%d got %v prev %v", attempt, cur, prev)
		}
		prev = cur
	}
}

func TestHandshakeBuildRequest_NonceUniqueness(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	var mu sync.Mutex
	const n = 256
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := buildHandshakeRequest("a", "t", "v", nil)
			if err != nil {
				t.Errorf("build: %v", err)
				return
			}
			mu.Lock()
			if seen[req.Nonce] {
				t.Errorf("duplicate nonce under concurrent build: %q", req.Nonce)
			}
			seen[req.Nonce] = true
			mu.Unlock()
			if req.RequestID == "" {
				t.Error("request_id must not be empty")
			}
		}()
	}
	wg.Wait()
}

func TestSleepCtx_CancelExitsEarly(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepCtx(ctx, 5*time.Second) {
		t.Fatal("cancelled context must cause sleepCtx to return false")
	}
}

func TestAttachExtractSessionToken_Roundtrip(t *testing.T) {
	t.Parallel()
	packet := &agentv1.BusPacket{SenderId: "w1"}
	attachSessionToken(packet, "canonical-token")
	if got := ExtractSessionToken(packet); got != "canonical-token" {
		t.Fatalf("roundtrip token=%q", got)
	}
}

func TestAttachSessionToken_NilAndEmpty(t *testing.T) {
	t.Parallel()
	attachSessionToken(nil, "ignored")
	p := &agentv1.BusPacket{}
	attachSessionToken(p, "")
	if got := ExtractSessionToken(p); got != "" {
		t.Errorf("empty token must not attach; got %q", got)
	}
}

func TestPublishResult_AttachesToken(t *testing.T) {
	t.Parallel()
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		return acceptResponse(t, data, "token-publish"), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Simulate a publishResult by looking at what gets published.
	// Since publishResult is internal, we trigger it via a fake job
	// request flowing through a.handleMessage.
	ctx := Context{
		Packet: &agentv1.BusPacket{TraceId: "t"},
		Logger: silentLogger(),
	}
	a.publishResult(ctx, &agentv1.JobResult{JobId: "job-1", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED})
	last, ok := bus.lastPublished()
	if !ok {
		t.Fatal("no packet published")
	}
	var decoded agentv1.BusPacket
	if err := proto.Unmarshal(last.data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := ExtractSessionToken(&decoded); got != "token-publish" {
		t.Fatalf("result packet token=%q want token-publish", got)
	}
}

func TestAdvertisedCapabilities_StableOrder(t *testing.T) {
	t.Parallel()
	a := &Agent{}
	Register(a, "job.c", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	Register(a, "job.a", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	Register(a, "job.b", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	caps := a.advertisedCapabilities()
	want := []string{"job.a", "job.b", "job.c"}
	if len(caps) != len(want) {
		t.Fatalf("len=%d want %d", len(caps), len(want))
	}
	for i := range want {
		if caps[i] != want[i] {
			t.Errorf("caps[%d]=%q want %q", i, caps[i], want[i])
		}
	}
}
