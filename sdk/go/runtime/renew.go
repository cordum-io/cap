package runtime

// Session-token auto-renew. After a successful Phase-2 handshake the
// agent holds a short-lived token (default 1h). This file runs the
// background loop that requests a fresh token at exp - lifetime/2
// so the cutover is seamless from the scheduler's point of view —
// dispatch never sees an expired token on the inbound path.
//
// A renewal always proves possession of the current active token. Failure
// never falls back to a tokenless ISSUE exchange: WARN retains an existing
// token only while it remains unexpired, while ENFORCE clears it.

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

// stopRenewLoop cancels the renew goroutine and waits for it to exit.
func (a *Agent) stopRenewLoop() {
	if a == nil || a.renew == nil {
		return
	}
	r := a.renew
	r.once.Do(func() {
		r.cancel()
		<-r.done
	})
}

// renewLoop is the renew goroutine's main body. It waits until the
// computed renew-at and performs an authenticated renew. Exits cleanly on
// context cancellation without issuing a tokenless fallback.
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
			continue
		}
		a.Logger.Warn("cap-runtime: authenticated session renewal failed",
			"worker_id", a.SenderID,
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
