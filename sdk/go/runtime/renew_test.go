package runtime

// Renew-loop tests. Each case drives the real Agent.performRenew /
// performHandshake flow with a scripted scheduler — no mocks of the
// handshake protocol itself, the wire is exercised byte-for-byte.

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

// scriptedBus is handshakeBus with per-subject responder routing so
// tests can script different behaviours for sys.worker.handshake vs
// sys.worker.handshake.renew without a single shared state machine.
type scriptedBus struct {
	handshakeBus
	onInitial func(data []byte) ([]byte, error)
	onRenew   func(data []byte) ([]byte, error)
}

func newScriptedBus(
	onInitial func(data []byte) ([]byte, error),
	onRenew func(data []byte) ([]byte, error),
) *scriptedBus {
	sb := &scriptedBus{
		handshakeBus: *newHandshakeBus(nil),
		onInitial:    onInitial,
		onRenew:      onRenew,
	}
	sb.handshakeBus.respond = func(subj string, data []byte) ([]byte, error) {
		switch subj {
		case capsdk.WorkerHandshakeSubject:
			if sb.onInitial == nil {
				return nil, fmt.Errorf("no initial-handshake responder configured")
			}
			return sb.onInitial(data)
		case capsdk.WorkerHandshakeRenewSubject:
			if sb.onRenew == nil {
				return nil, fmt.Errorf("no renew responder configured")
			}
			return sb.onRenew(data)
		default:
			return nil, fmt.Errorf("unexpected subject %q", subj)
		}
	}
	return sb
}

func TestPerformRenew_RotatesToken(t *testing.T) {
	t.Parallel()
	var renewCalls atomic.Int32
	bus := newScriptedBus(
		func(data []byte) ([]byte, error) {
			return acceptResponse(t, data, "token-initial"), nil
		},
		func(data []byte) ([]byte, error) {
			renewCalls.Add(1)
			return acceptResponse(t, data, "token-renewed"), nil
		},
	)
	a := newAgentFixture(t, &bus.handshakeBus, HandshakeModeWarn)
	// Swap the bus to the scripted one (newAgentFixture attached the
	// unscripted handshakeBus by value; we need the scripted instance
	// so the per-subject routing kicks in).
	a.NATS = bus

	// Seed the initial token. We call performHandshake directly
	// rather than Start() so the renew goroutine doesn't race the
	// test assertions.
	if _, err := a.performHandshake(context.Background()); err != nil {
		t.Fatalf("initial handshake: %v", err)
	}
	token, _ := a.SessionToken()
	if token != "token-initial" {
		t.Fatalf("initial token=%q want token-initial", token)
	}

	// Now drive a renew directly.
	ok := a.attemptRenewOnce(context.Background())
	if !ok {
		t.Fatal("attemptRenewOnce returned false; want true")
	}
	rotated, _ := a.SessionToken()
	if rotated != "token-renewed" {
		t.Fatalf("rotated token=%q want token-renewed", rotated)
	}
	if renewCalls.Load() != 1 {
		t.Fatalf("expected exactly 1 renew call; got %d", renewCalls.Load())
	}
}

func TestPerformRenew_RejectedForcesFreshHandshake(t *testing.T) {
	t.Parallel()
	var initialCalls, renewCalls atomic.Int32
	bus := newScriptedBus(
		func(data []byte) ([]byte, error) {
			initialCalls.Add(1)
			return acceptResponse(t, data, fmt.Sprintf("token-initial-%d", initialCalls.Load())), nil
		},
		func(data []byte) ([]byte, error) {
			renewCalls.Add(1)
			return rejectResponse(t, data, capsdk.HandshakeRejectReplay), nil
		},
	)
	a := newAgentFixture(t, &bus.handshakeBus, HandshakeModeWarn)
	a.NATS = bus

	if _, err := a.performHandshake(context.Background()); err != nil {
		t.Fatalf("initial handshake: %v", err)
	}
	// attemptRenewOnce returns false on rejection.
	if ok := a.attemptRenewOnce(context.Background()); ok {
		t.Fatal("renew rejection must return false")
	}
	// Renew loop would then call attemptFreshHandshakeOnce; simulate
	// that call directly.
	if ok := a.attemptFreshHandshakeOnce(context.Background()); !ok {
		t.Fatal("fresh handshake after renew rejection must succeed")
	}
	after, _ := a.SessionToken()
	if after != "token-initial-2" {
		t.Fatalf("post-fallback token=%q want token-initial-2", after)
	}
}

func TestPerformRenew_TransportErrorRetries(t *testing.T) {
	t.Parallel()
	var renewAttempts atomic.Int32
	bus := newScriptedBus(
		func(data []byte) ([]byte, error) {
			return acceptResponse(t, data, "token-initial"), nil
		},
		func(data []byte) ([]byte, error) {
			n := renewAttempts.Add(1)
			if n < 2 {
				return nil, errors.New("network partition")
			}
			return acceptResponse(t, data, "token-recovered"), nil
		},
	)
	a := newAgentFixture(t, &bus.handshakeBus, HandshakeModeWarn)
	a.NATS = bus
	if _, err := a.performHandshake(context.Background()); err != nil {
		t.Fatalf("initial: %v", err)
	}
	if ok := a.attemptRenewOnce(context.Background()); !ok {
		t.Fatal("renew with transient + recovery must succeed")
	}
	got, _ := a.SessionToken()
	if got != "token-recovered" {
		t.Fatalf("token=%q want token-recovered", got)
	}
}

func TestComputeRenewWait_HalfLifetime(t *testing.T) {
	t.Parallel()
	a := &Agent{}
	exp := time.Now().Add(1 * time.Hour)
	wait := a.computeRenewWait(exp)
	// 1h/2 = 30m, bounded by DefaultRenewLeeway (60s) below full expiry.
	if wait < 25*time.Minute || wait > 35*time.Minute {
		t.Fatalf("wait=%v want ~30m for 1h lifetime", wait)
	}
}

func TestComputeRenewWait_ClampsToLifetime(t *testing.T) {
	t.Parallel()
	a := &Agent{}
	// Lifetime 10s < DefaultRenewMinInterval 30s: the min floor
	// pushes target above lifetime, so the second clamp brings it
	// back. We should wait ≤ lifetime to avoid never renewing.
	exp := time.Now().Add(10 * time.Second)
	wait := a.computeRenewWait(exp)
	if wait > 10*time.Second+time.Second { // small slack for scheduler
		t.Fatalf("wait=%v must not exceed lifetime=10s", wait)
	}
	if wait <= 0 {
		t.Fatalf("wait=%v must be positive for a future exp", wait)
	}
}

func TestComputeRenewWait_RespectsMinIntervalWhenRoomy(t *testing.T) {
	t.Parallel()
	a := &Agent{}
	// Lifetime 2m — half is 60s, leeway-adjusted is 60s - 60s = 0.
	// Floor clamps to DefaultRenewMinInterval (30s).
	exp := time.Now().Add(2 * time.Minute)
	wait := a.computeRenewWait(exp)
	if wait < DefaultRenewMinInterval-time.Second {
		t.Fatalf("wait=%v must not fall below DefaultRenewMinInterval=%v with roomy lifetime", wait, DefaultRenewMinInterval)
	}
}

func TestComputeRenewWait_AlreadyExpired(t *testing.T) {
	t.Parallel()
	a := &Agent{}
	exp := time.Now().Add(-5 * time.Minute)
	wait := a.computeRenewWait(exp)
	if wait != DefaultRenewMinInterval {
		t.Fatalf("wait=%v want %v for expired token", wait, DefaultRenewMinInterval)
	}
}

func TestStartRenewLoop_NoOpWhenOff(t *testing.T) {
	t.Parallel()
	a := &Agent{HandshakeMode: HandshakeModeOff, Logger: silentLogger()}
	a.startRenewLoop(context.Background())
	if a.renew != nil {
		t.Fatal("off mode must not spawn renew goroutine")
	}
}

func TestStartRenewLoop_NoOpWhenNoToken(t *testing.T) {
	t.Parallel()
	a := &Agent{HandshakeMode: HandshakeModeWarn, Logger: silentLogger()}
	a.startRenewLoop(context.Background())
	if a.renew != nil {
		t.Fatal("warn mode without a token must not spawn renew goroutine")
	}
}

func TestStopRenewLoop_Idempotent(t *testing.T) {
	t.Parallel()
	a := &Agent{}
	a.stopRenewLoop() // nil-safe
	a.stopRenewLoop()
}
