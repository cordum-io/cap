// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

// Package referencescheduler is the in-memory state core of CAP's Apache-2.0
// reference scheduler, a single-process, single-replica control plane (see
// docs/adoption/framework-adapters-design.md section 2). State changes accept
// only capsdk.VerifiedProductionPacket values. The core holds no ResourceRegistry,
// network client or goroutine. Errors are bare sentinels carrying no tenant,
// subject, payload, pointer or signature bytes.
package referencescheduler

import (
	"errors"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

const (
	// SubjectSubmit admits client JobRequest packets.
	SubjectSubmit = "sys.job.submit"
	// SubjectResult admits worker JobResult packets.
	SubjectResult = "sys.job.result"
	// SubjectProgress admits worker JobProgress packets.
	SubjectProgress = "sys.job.progress"
	// SubjectCancel carries scheduler-to-worker fenced cancellation.
	SubjectCancel = "sys.job.cancel"
	// SubjectClientCancel admits authenticated client JobCancel packets.
	SubjectClientCancel = "refsched.job.cancel"
	// SubjectTerminal carries the scheduler-signed terminal winner.
	SubjectTerminal = "refsched.job.terminal"
)

// WorkerSubject returns "worker." + workerID + ".jobs".
func WorkerSubject(workerID string) string { return "" }

// Limits bounds declared resources, attempts and injected-clock deadlines.
type Limits struct {
	MaxInputBytes   uint64        // bound on context_ref.size_bytes
	MaxOutputBytes  uint64        // bound on result_ref.size_bytes
	MaxArtifacts    int           // bound on accepted artifact_refs per job
	MaxAttempts     uint64        // >= 1; attempts are numbered from 1
	DispatchTimeout time.Duration // DISPATCHED with no admitted progress/result -> TIMEOUT
	RunTimeout      time.Duration // run deadline when budget.deadline_ms == 0
	MaxRunTimeout   time.Duration // upper bound applied to budget.deadline_ms
}

// AllowRule matches tenant, topic and capability exactly.
type AllowRule struct{ TenantID, Topic, Capability string }

// Config supplies positive limits, a nonempty allowlist and the required clock.
type Config struct {
	Limits        Limits
	Allow         []AllowRule
	Now           func() time.Time       // required; the injected clock
	NewDispatchID func() (string, error) // nil: 128-bit crypto/rand hex
}

// Job is a deep-copied snapshot; mutating it never changes scheduler state.
type Job struct {
	TenantID, JobID, Topic, Capability string
	Status                             agentv1.JobStatus
	Dispatch                           *agentv1.DispatchIdentity // nil until the first Dispatch
	Acked                              bool                      // AckDispatch seen for Dispatch
	Result                             *agentv1.JobResult        // the terminal winner; nil before terminal
	Artifacts                          []*agentv1.ResourceRef
	Deadline                           time.Time // zero until dispatched
	DispatchedAt                       time.Time // zero until dispatched; DispatchTimeout counts from here
	UpdatedAt                          time.Time
}

// Dispatch carries correlation persisted before the runtime publishes Request.
type Dispatch struct {
	TenantID, JobID string
	Subject         string                    // WorkerSubject(Identity.AssignedWorkerId)
	Identity        *agentv1.DispatchIdentity // persisted before Dispatch returns
	Request         *agentv1.JobRequest       // admitted request with .Dispatch = Identity
}

// Scheduler is the in-memory state core; its fields are unexported and it is
// safe for concurrent use.
type Scheduler struct{}

// New refuses absent clock, empty allowlist and nonpositive limits.
func New(cfg Config) (*Scheduler, error) { return nil, ErrNotImplemented }

// Submit admits verified requests, recording SCHEDULED or DENIED; identical
// requests under the same tenant-qualified job key are idempotent.
func (s *Scheduler) Submit(v capsdk.VerifiedProductionPacket) (Job, error) {
	return Job{}, ErrNotImplemented
}

// Dispatch requires SCHEDULED and reserves a new identity and deadline before return.
func (s *Scheduler) Dispatch(tenantID, jobID, workerID string) (Dispatch, error) {
	return Dispatch{}, ErrNotImplemented
}

// AckDispatch acknowledges the current identity without overwriting progress or results.
func (s *Scheduler) AckDispatch(d Dispatch) error { return ErrNotImplemented }

// ApplyProgress fences the verified worker and attempt and appends bounded artifacts
// only before terminal, advancing DISPATCHED to RUNNING.
func (s *Scheduler) ApplyProgress(v capsdk.VerifiedProductionPacket) (Job, error) {
	return Job{}, ErrNotImplemented
}

// ApplyResult fences the verified worker and attempt; the first terminal wins,
// while retryable failures reschedule within MaxAttempts.
func (s *Scheduler) ApplyResult(v capsdk.VerifiedProductionPacket) (Job, error) {
	return Job{}, ErrNotImplemented
}

// Cancel admits only the owning tenant's client cancel; a terminal winner is unchanged.
func (s *Scheduler) Cancel(v capsdk.VerifiedProductionPacket) (Job, error) {
	return Job{}, ErrNotImplemented
}

// Tick uses Config.Now to expire dispatch and run deadlines; changed jobs are
// returned sorted by (TenantID, JobID).
func (s *Scheduler) Tick() []Job { return nil }

// Get returns a deep snapshot by tenant-qualified job key or ErrJobNotFound.
func (s *Scheduler) Get(tenantID, jobID string) (Job, error) { return Job{}, ErrNotImplemented }

var (
	// ErrInvalidConfig rejects absent clock, empty allowlist or nonpositive limits.
	ErrInvalidConfig = errors.New("referencescheduler: invalid config")
	// ErrUnverified rejects packets without SDK verification proof.
	ErrUnverified = errors.New("referencescheduler: packet not verified")
	// ErrWrongSubject rejects packets verified for another subject.
	ErrWrongSubject = errors.New("referencescheduler: wrong subject")
	// ErrWrongPayload rejects a payload incompatible with the operation.
	ErrWrongPayload = errors.New("referencescheduler: wrong payload")
	// ErrInvalidJob rejects invalid identifiers, missing context or tenant disagreement.
	ErrInvalidJob = errors.New("referencescheduler: invalid job")
	// ErrDenied accompanies DENIED on an allowlist miss.
	ErrDenied = errors.New("referencescheduler: denied")
	// ErrJobNotFound gives no existence oracle across tenants.
	ErrJobNotFound = errors.New("referencescheduler: job not found")
	// ErrJobConflict rejects a different request under an existing job key.
	ErrJobConflict = errors.New("referencescheduler: job conflict")
	// ErrInvalidTransition rejects a disallowed nonterminal transition.
	ErrInvalidTransition = errors.New("referencescheduler: invalid transition")
	// ErrTerminal preserves the first terminal winner.
	ErrTerminal = errors.New("referencescheduler: job is terminal")
	// ErrStaleAttempt rejects a different dispatch ID or attempt.
	ErrStaleAttempt = errors.New("referencescheduler: stale or future attempt")
	// ErrWrongWorker rejects a sender or assigned worker other than the active worker.
	ErrWrongWorker = errors.New("referencescheduler: wrong worker")
	// ErrInputTooLarge accompanies DENIED for an oversized declared input.
	ErrInputTooLarge = errors.New("referencescheduler: input too large")
	// ErrOutputTooLarge accompanies FAILED with RESOURCE_EXHAUSTED for oversized output.
	ErrOutputTooLarge = errors.New("referencescheduler: output too large")
	// ErrTooManyArtifacts rejects an entire append that would exceed MaxArtifacts.
	ErrTooManyArtifacts = errors.New("referencescheduler: too many artifacts")
	// ErrNotImplemented is stub only; the implementation deletes it.
	ErrNotImplemented = errors.New("referencescheduler: not implemented")
)
