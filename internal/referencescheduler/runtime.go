// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"context"
	"errors"
	"log/slog"
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
	opts       RuntimeOptions
	core       *Scheduler
	issuer     *Issuer
	admit      *Admitter
	ready      chan struct{}
	nc         *nats.Conn
	subs       []*nats.Subscription
	goroutines atomic.Int64
}

func NewRuntime(opts RuntimeOptions) (*Runtime, error) {
	if opts.Config == nil || opts.Now == nil || opts.TickEvery <= 0 {
		return nil, ErrInvalidConfig
	}
	c := opts.Config
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
	return &Runtime{opts: opts, core: core, issuer: issuer, admit: admit, ready: make(chan struct{})}, nil
}

func (r *Runtime) Run(context.Context) error {
	return errors.New("referencescheduler: runtime not implemented")
}
func (r *Runtime) Ready() <-chan struct{} { return r.ready }
func (r *Runtime) Stats() RuntimeStats    { return RuntimeStats{} }
func (r *Runtime) tickOnce()              {}
