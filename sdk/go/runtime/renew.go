package runtime

// Session-token auto-renew. After a successful Phase-2 handshake the
// agent holds a short-lived token (default 1h). This file runs the
// background loop that requests a fresh token at exp - lifetime/2
// so the cutover is seamless from the scheduler's point of view —
// dispatch never sees an expired token on the inbound path.
//
// Failure ladder:
//   1. attempt renew via WorkerHandshakeRenewSubject.
//   2. on transport / malformed-response failure: performHandshakeOn
//      retries with exponential backoff up to HandshakeRetries.
//   3. if renew still fails: escalate to a fresh handshake over
//      WorkerHandshakeSubject. This covers the case where the
//      scheduler lost the active-token record (e.g. Redis rebuild,
//      miniredis wipe) and needs a brand-new issuance.
//   4. if the fresh handshake also fails: sleep briefly and loop.
//      Callers in warn mode keep running with a stale token; in
//      enforce mode the next outbound packet is rejected by the
//      scheduler middleware, which is the correct fail-closed behaviour.

import (
	"context"
	"sync"
	"time"
)

// DefaultRenewLeeway is the safety margin subtracted from the
// token's expiry when computing when to rotate. 60 s covers the
// capsdk.WorkerHandshakeMaxSkew clock-skew tolerance plus a few
// seconds of network jitter — good enough that renew always
// completes before the scheduler would start rejecting packets.
const DefaultRenewLeeway = 60 * time.Second

// DefaultRenewMinInterval bounds how aggressively the loop can retry
// after a failed renew. Without a floor, a revoked token could spin
// the goroutine in a tight loop refreshing Redis.
const DefaultRenewMinInterval = 30 * time.Second

// renewer wraps the lifecycle state for the renew goroutine. Attached
// to Agent (renew field) so Close() can cancel it cleanly.
type renewer struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// startRenewLoop spawns the renew goroutine. It is a no-op when mode
// is off or when no session token has been installed yet (nothing to
// renew). Called from Start() after a successful handshake.
func (a *Agent) startRenewLoop(parent context.Context) {
	if a == nil {
		return
	}
	if a.activeHandshakeMode() == HandshakeModeOff {
		return
	}
	token, _ := a.SessionToken()
	if token == "" {
		// warn mode may accept Start() without a token. Nothing to renew.
		return
	}
	ctx, cancel := context.WithCancel(parent)
	r := &renewer{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	a.renew = r
	go func() {
		defer close(r.done)
		a.renewLoop(ctx)
	}()
}

// stopRenewLoop cancels the renew goroutine and waits up to a short
// deadline for it to exit. Called from Close().
func (a *Agent) stopRenewLoop() {
	if a == nil || a.renew == nil {
		return
	}
	r := a.renew
	r.once.Do(func() {
		r.cancel()
		select {
		case <-r.done:
		case <-time.After(2 * time.Second):
			// Goroutine stuck in a long sleep — won't free until its
			// own timer fires, but the cancel closes the context so
			// the next tick exits anyway. Log level left to caller.
		}
	})
}

// renewLoop is the renew goroutine's main body. It waits until the
// computed renew-at, performs a renew, and on failure falls back to a
// fresh handshake. Exits cleanly on ctx cancel.
func (a *Agent) renewLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		_, exp := a.SessionToken()
		if exp.IsZero() {
			// Nothing to renew. Wait a bit and retry — a concurrent
			// handshake may install a token.
			if !sleepCtx(ctx, DefaultRenewMinInterval) {
				return
			}
			continue
		}
		wait := a.computeRenewWait(exp)
		if !sleepCtx(ctx, wait) {
			return
		}
		if a.attemptRenewOnce(ctx) {
			continue // success — new exp installed, recompute next wait.
		}
		a.Logger.Warn("cap-runtime: renew exhausted retries; falling back to fresh handshake",
			"agent_id", a.SenderID,
		)
		if a.attemptFreshHandshakeOnce(ctx) {
			continue
		}
		// Fresh handshake also failed. Sleep the min interval before
		// the next iteration so we don't spin.
		a.Logger.Error("cap-runtime: fresh handshake failed during renew loop",
			"agent_id", a.SenderID,
		)
		if !sleepCtx(ctx, DefaultRenewMinInterval) {
			return
		}
	}
}

// attemptRenewOnce performs a single renew attempt. Returns true on
// success (token rotated), false on any failure.
func (a *Agent) attemptRenewOnce(ctx context.Context) bool {
	obtained, err := a.performRenew(ctx)
	if err != nil {
		a.Logger.Warn("cap-runtime: renew failed",
			"agent_id", a.SenderID,
			"error", err,
		)
		return false
	}
	return obtained
}

// attemptFreshHandshakeOnce performs a single fresh-handshake attempt
// as the renew-failure fallback. Returns true on success.
func (a *Agent) attemptFreshHandshakeOnce(ctx context.Context) bool {
	obtained, err := a.performHandshake(ctx)
	if err != nil {
		a.Logger.Warn("cap-runtime: fresh handshake failed",
			"agent_id", a.SenderID,
			"error", err,
		)
		return false
	}
	return obtained
}

// computeRenewWait returns the duration until the next renew tick.
// Target: exp - lifetime/2 when lifetime is known, else exp minus a
// fixed leeway. Guards against negative durations (already-expired
// token) by returning a minimum interval so the loop still cycles
// rather than spinning.
func (a *Agent) computeRenewWait(exp time.Time) time.Duration {
	now := time.Now()
	lifetime := exp.Sub(now)
	if lifetime <= 0 {
		// Already past exp — renew immediately, but bounded by the
		// min interval to prevent a tight loop against a revoked
		// token that keeps failing.
		return DefaultRenewMinInterval
	}
	target := lifetime / 2
	if target > lifetime-DefaultRenewLeeway {
		target = lifetime - DefaultRenewLeeway
	}
	if target < DefaultRenewMinInterval {
		target = DefaultRenewMinInterval
	}
	if target > lifetime {
		target = lifetime
	}
	return target
}
