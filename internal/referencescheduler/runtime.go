// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
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
	// ponytail: serialize reactions; shard by job only if throughput requires it.
	mu      sync.Mutex
	live    map[jobKey]*agentv1.IdentityBinding
	pending map[jobKey]bool
	cursor  map[string]int
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
	return &Runtime{opts: opts, core: core, issuer: issuer, admit: admit, ready: make(chan struct{}), sem: make(chan struct{}, c.MaxConcurrency),
		live: make(map[jobKey]*agentv1.IdentityBinding), pending: make(map[jobKey]bool), cursor: make(map[string]int)}, nil
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
func (r *Runtime) tickOnce() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls.Add(1)
	for _, j := range r.core.Tick() {
		r.publishTerminal(j)
	}
	for k := range r.pending {
		j, err := r.core.Get(k.tenant, k.job)
		if err == nil && j.Status == agentv1.JobStatus_JOB_STATUS_SCHEDULED {
			r.dispatch(j)
		} else {
			delete(r.pending, k)
		}
	}
}

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
	v, err := r.admit.Admit(subject, raw)
	if err != nil {
		r.drop(subject, err)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, err := r.apply(subject, v)
	if j.JobID == "" || (subject == SubjectResult && err == ErrTerminal) {
		if err != nil {
			r.drop(subject, err)
		}
		return
	}
	if j.Status == agentv1.JobStatus_JOB_STATUS_SCHEDULED {
		r.dispatch(j)
	}
	if terminal(j.Status) {
		r.publishTerminal(j)
	}
	if subject == SubjectClientCancel && err == nil && j.Result.GetDispatch() != nil {
		cancel := &agentv1.JobCancel{JobId: j.JobID, Dispatch: clone(j.Result.Dispatch)}
		if err := r.publishSigned(SubjectCancel, r.identityFor(j), func(p *agentv1.BusPacket) { p.Payload = &agentv1.BusPacket_JobCancel{JobCancel: cancel} }); err != nil {
			r.drop(subject, err)
		}
	}
}

func (r *Runtime) apply(subject string, v capsdk.VerifiedProductionPacket) (Job, error) {
	if subject == SubjectSubmit {
		return r.submit(v)
	}
	r.calls.Add(1)
	switch subject {
	case SubjectProgress:
		_, err := r.core.ApplyProgress(v)
		return Job{}, err
	case SubjectResult:
		before, _ := r.core.Get(v.Tenant(), v.Packet().GetJobResult().GetJobId())
		j, err := r.core.ApplyResult(v)
		if terminal(before.Status) && j.JobID != "" {
			return j, ErrTerminal
		}
		return j, err
	default:
		return r.core.Cancel(v)
	}
}

func (r *Runtime) submit(v capsdk.VerifiedProductionPacket) (Job, error) {
	p := v.Packet()
	k := jobKey{v.Tenant(), p.GetJobRequest().GetJobId()}
	if _, exists := r.live[k]; !exists {
		if len(r.live) >= r.opts.Config.MaxLiveJobs {
			return Job{}, ErrRuntimeCapacity
		}
		r.live[k] = nil
	}
	r.calls.Add(1)
	j, err := r.core.Submit(v)
	if j.JobID != "" && r.live[k] == nil {
		r.live[k] = clone(p.Identity)
	}
	return j, err
}

func (r *Runtime) identityFor(j Job) *agentv1.IdentityBinding {
	if id := r.live[jobKey{j.TenantID, j.JobID}]; id != nil {
		return id
	}
	return &agentv1.IdentityBinding{TenantId: j.TenantID}
}

func (r *Runtime) dispatch(j Job) {
	k := jobKey{j.TenantID, j.JobID}
	r.pending[k] = true
	workers := r.opts.Config.TopicWorkers[j.Topic]
	for offset := range workers {
		i := (r.cursor[j.Topic] + offset) % len(workers)
		w := workers[i]
		if s, ok := r.issuer.Active(w); !ok || s.TenantID != j.TenantID {
			continue
		}
		r.cursor[j.Topic] = (i + 1) % len(workers)
		d, err := r.core.Dispatch(j.TenantID, j.JobID, w)
		r.calls.Add(1)
		if err == nil {
			err = r.publishSigned(d.Subject, r.identityFor(j), func(p *agentv1.BusPacket) { p.Payload = &agentv1.BusPacket_JobRequest{JobRequest: d.Request} })
			if err == nil {
				r.calls.Add(1)
				err = r.core.AckDispatch(d)
				delete(r.pending, k)
			}
		}
		if err != nil {
			r.drop(SubjectSubmit, err)
		}
		return
	}
}

func (r *Runtime) publishTerminal(j Job) {
	delete(r.pending, jobKey{j.TenantID, j.JobID})
	if err := r.publishSigned(SubjectTerminal, r.identityFor(j), func(p *agentv1.BusPacket) { p.Payload = &agentv1.BusPacket_JobResult{JobResult: j.Result} }); err != nil {
		r.drop(SubjectTerminal, err)
	}
}
