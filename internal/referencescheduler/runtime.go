// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
)

// RuntimeOptions requires resolved configuration and a concurrency-safe clock.
type RuntimeOptions struct {
	Config    *ResolvedConfig
	Now       func() time.Time
	TickEvery time.Duration
	Logger    *slog.Logger
}

// RuntimeStats counts drops, core transitions and successful production publishes.
type RuntimeStats struct{ Dropped, CoreCalls, Published uint64 }

// Runtime is a single-replica, process-lifetime scheduler.
type Runtime struct {
	opts                      RuntimeOptions
	core                      *Scheduler
	issuer                    *Issuer
	admit                     *Admitter
	ready                     chan struct{}
	nc                        *nats.Conn
	subs                      []*nats.Subscription
	goroutines                atomic.Int64
	dropped, calls, published atomic.Uint64
	started                   atomic.Bool
	gate                      sync.Mutex
	closing                   bool
	wg                        sync.WaitGroup
	sem                       chan struct{}
}

func NewRuntime(opts RuntimeOptions) (*Runtime, error) {
	if opts.Config == nil || opts.Now == nil || opts.TickEvery <= 0 {
		return nil, ErrInvalidConfig
	}
	c := opts.Config
	if c.MaxConcurrency <= 0 || c.MaxLiveJobs <= 0 {
		return nil, ErrInvalidConfig
	}
	core, err := New(Config{Limits: c.Limits, Allow: c.Allow, Now: opts.Now})
	if err != nil {
		return nil, err
	}
	workers := make([]TrustedWorker, 0, len(c.Workers))
	for _, w := range c.Workers {
		workers = append(workers, TrustedWorker{w.WorkerID, w.TenantID, w.AgentID, w.ProofKeyID, w.ProofKey})
	}
	issuer, err := NewIssuer(IssuerConfig{SchedulerID: c.SchedulerID, ServerKeyID: c.TrustKeyID, ServerKey: c.TrustKey,
		Audience: capsdk.WorkerHandshakeAudience, Workers: workers, Now: opts.Now, ChallengeTTL: c.ChallengeTTL, SessionTTL: c.SessionTTL, MaxChallenges: c.MaxChallenges})
	if err != nil {
		return nil, err
	}
	admit, err := NewAdmitter(c, issuer, capsdk.NewInMemoryReplayStore(), opts.Now)
	if err != nil {
		return nil, err
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Runtime{opts: opts, core: core, issuer: issuer, admit: admit, ready: make(chan struct{}), sem: make(chan struct{}, c.MaxConcurrency)}, nil
}

// Run serves once; cancellation closes transport and joins every callback and tick.
func (r *Runtime) Run(ctx context.Context) error {
	if ctx == nil || !r.started.CompareAndSwap(false, true) {
		return ErrInvalidConfig
	}
	var err error
	if r.nc, err = dial(r.opts.Config); err != nil {
		return ErrRuntimeTransport
	}
	defer r.drain()
	for _, subject := range []string{SubjectSubmit, SubjectProgress, SubjectResult, SubjectClientCancel, capsdk.WorkerHandshakeChallengeSubject, capsdk.WorkerHandshakeAuthenticateSubject} {
		sub, err := r.nc.Subscribe(subject, r.guarded(ctx, subject))
		if err != nil {
			return ErrRuntimeTransport
		}
		r.subs = append(r.subs, sub)
	}
	if err = r.nc.FlushTimeout(time.Second); err != nil {
		return ErrRuntimeTransport
	}
	r.wg.Add(1)
	r.goroutines.Add(1)
	go r.tickLoop(ctx)
	close(r.ready)
	<-ctx.Done()
	return nil
}
func (r *Runtime) Ready() <-chan struct{} { return r.ready }
func (r *Runtime) Stats() RuntimeStats {
	return RuntimeStats{r.dropped.Load(), r.calls.Load(), r.published.Load()}
}
func (r *Runtime) tickOnce() {}

func (r *Runtime) tickLoop(ctx context.Context) {
	defer r.wg.Done()
	defer r.goroutines.Add(-1)
	ticker := time.NewTicker(r.opts.TickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tickOnce()
		}
	}
}

func (r *Runtime) packet(subject string, raw []byte) {
	if _, err := r.admit.Admit(subject, raw); err != nil {
		r.drop(subject, err)
	}
}
