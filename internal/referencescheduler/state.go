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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
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

// maxIDBytes bounds job_id, topic, worker ID and dispatch ID; R3 allows any bound in [256, 4096].
const maxIDBytes = 256

// WorkerSubject returns "worker." + workerID + ".jobs".
func WorkerSubject(workerID string) string { return "worker." + workerID + ".jobs" }

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
type Scheduler struct {
	limits Limits
	allow  map[AllowRule]bool
	now    func() time.Time
	newID  func() (string, error)
	store  store
}

// New refuses absent clock, empty allowlist and nonpositive limits.
func New(cfg Config) (*Scheduler, error) {
	l := cfg.Limits
	if cfg.Now == nil || len(cfg.Allow) == 0 || l.MaxInputBytes == 0 || l.MaxOutputBytes == 0 || l.MaxArtifacts <= 0 ||
		l.MaxAttempts == 0 || l.DispatchTimeout <= 0 || l.RunTimeout <= 0 || l.MaxRunTimeout <= 0 {
		return nil, ErrInvalidConfig
	}
	s := &Scheduler{limits: l, allow: make(map[AllowRule]bool, len(cfg.Allow)), now: cfg.Now,
		newID: cfg.NewDispatchID, store: store{jobs: make(map[jobKey]*record)}}
	for _, rule := range cfg.Allow {
		s.allow[rule] = true
	}
	if s.newID == nil {
		s.newID = randomDispatchID
	}
	return s, nil
}

// Submit admits verified requests, recording SCHEDULED or DENIED; identical
// requests under the same tenant-qualified job key are idempotent.
func (s *Scheduler) Submit(v capsdk.VerifiedProductionPacket) (Job, error) {
	p, err := admit(v, SubjectSubmit)
	if err != nil {
		return Job{}, err
	}
	req := p.GetJobRequest()
	if req == nil {
		return Job{}, ErrWrongPayload
	}
	if !validRequest(req, p.GetIdentity()) {
		return Job{}, ErrInvalidJob
	}
	j := Job{TenantID: v.Tenant(), JobID: req.GetJobId(), Topic: req.GetTopic(), Capability: req.GetMeta().GetCapability(),
		Status: agentv1.JobStatus_JOB_STATUS_SCHEDULED, UpdatedAt: s.now()}
	var refusal error
	switch {
	case !s.allow[AllowRule{TenantID: j.TenantID, Topic: j.Topic, Capability: j.Capability}]:
		refusal = ErrDenied
		decide(&j, agentv1.JobStatus_JOB_STATUS_DENIED, agentv1.ErrorCode_ERROR_CODE_JOB_PERMISSION_DENIED)
	case req.GetContextRef().GetSizeBytes() > s.limits.MaxInputBytes:
		refusal = ErrInputTooLarge
		decide(&j, agentv1.JobStatus_JOB_STATUS_DENIED, agentv1.ErrorCode_ERROR_CODE_JOB_RESOURCE_EXHAUSTED)
	}
	stored, inserted, err := s.store.insert(jobKey{j.TenantID, j.JobID}, &record{job: j, request: req})
	if err != nil || !inserted {
		return stored, err
	}
	return stored, refusal
}

// Dispatch requires SCHEDULED and reserves a new identity and deadline before return.
func (s *Scheduler) Dispatch(tenantID, jobID, workerID string) (Dispatch, error) {
	if !validID(workerID) {
		return Dispatch{}, ErrInvalidJob
	}
	id, err := s.newID()
	if err != nil || !validID(id) {
		return Dispatch{}, ErrInvalidConfig
	}
	now := s.now()
	var out Dispatch
	_, err = s.store.update(jobKey{tenantID, jobID}, func(j *Job, req *agentv1.JobRequest) (bool, error) {
		if terminal(j.Status) {
			return false, ErrTerminal
		}
		if j.Status != agentv1.JobStatus_JOB_STATUS_SCHEDULED {
			return false, ErrInvalidTransition
		}
		j.Dispatch = &agentv1.DispatchIdentity{DispatchId: id, Attempt: j.Dispatch.GetAttempt() + 1, AssignedWorkerId: workerID}
		j.Status, j.Acked, j.DispatchedAt, j.UpdatedAt = agentv1.JobStatus_JOB_STATUS_DISPATCHED, false, now, now
		j.Deadline = now.Add(s.runTimeout(req.GetBudget().GetDeadlineMs()))
		request := clone(req)
		request.Dispatch = clone(j.Dispatch)
		out = Dispatch{TenantID: tenantID, JobID: jobID, Subject: WorkerSubject(workerID), Identity: clone(j.Dispatch), Request: request}
		return true, nil
	})
	if err != nil {
		return Dispatch{}, err
	}
	return out, nil
}

// AckDispatch acknowledges the current identity without overwriting progress or results.
func (s *Scheduler) AckDispatch(d Dispatch) error {
	_, err := s.store.update(jobKey{d.TenantID, d.JobID}, func(j *Job, _ *agentv1.JobRequest) (bool, error) {
		if d.Identity == nil || !proto.Equal(d.Identity, j.Dispatch) {
			return false, ErrStaleAttempt
		}
		if j.Status != agentv1.JobStatus_JOB_STATUS_DISPATCHED || j.Acked {
			return false, nil // R8: a job already past DISPATCHED keeps its snapshot
		}
		j.Acked = true
		return true, nil
	})
	return err
}

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
func (s *Scheduler) Tick() []Job {
	return nil
}

// Get returns a deep snapshot by tenant-qualified job key or ErrJobNotFound.
func (s *Scheduler) Get(tenantID, jobID string) (Job, error) {
	return s.store.get(jobKey{tenantID, jobID})
}

// admit runs the checks every packet method makes before it reads state.
func admit(v capsdk.VerifiedProductionPacket, subject string) (*agentv1.BusPacket, error) {
	if !v.IsVerified() {
		return nil, ErrUnverified
	}
	if v.Subject() != subject {
		return nil, ErrWrongSubject
	}
	return v.Packet(), nil // a deep copy this call owns
}

// runTimeout is budget.deadline_ms (RunTimeout when not positive) capped at
// MaxRunTimeout; comparing in milliseconds first avoids Duration overflow.
func (s *Scheduler) runTimeout(deadlineMs int64) time.Duration {
	d := s.limits.RunTimeout
	if deadlineMs > 0 {
		d = s.limits.MaxRunTimeout
		if deadlineMs < s.limits.MaxRunTimeout.Milliseconds() {
			d = time.Duration(deadlineMs) * time.Millisecond
		}
	}
	return min(d, s.limits.MaxRunTimeout)
}

// decide records a terminal state the scheduler itself decides, with a
// scheduler-authored result naming the job and the dispatch that held it.
func decide(j *Job, status agentv1.JobStatus, code agentv1.ErrorCode) {
	j.Status = status
	j.Result = &agentv1.JobResult{JobId: j.JobID, Status: status, ErrorCodeEnum: code,
		ErrorCode: legacyErrorCodes[code], Dispatch: clone(j.Dispatch)}
}

// legacyErrorCodes are the spec/13 error_code strings kept for older consumers.
var legacyErrorCodes = map[agentv1.ErrorCode]string{
	agentv1.ErrorCode_ERROR_CODE_JOB_TIMEOUT:           "TIMEOUT",
	agentv1.ErrorCode_ERROR_CODE_JOB_PERMISSION_DENIED: "PERMISSION_DENIED",
}

func terminal(st agentv1.JobStatus) bool {
	switch st {
	case agentv1.JobStatus_JOB_STATUS_SUCCEEDED, agentv1.JobStatus_JOB_STATUS_FAILED, agentv1.JobStatus_JOB_STATUS_FAILED_FATAL,
		agentv1.JobStatus_JOB_STATUS_CANCELLED, agentv1.JobStatus_JOB_STATUS_DENIED, agentv1.JobStatus_JOB_STATUS_TIMEOUT:
		return true
	}
	return false
}

func validID(id string) bool { return id != "" && len(id) <= maxIDBytes }

// validRequest bounds the job key and topic, requires context_ref, and requires
// every nonempty identity mirror to equal the verified binding (spec/19).
func validRequest(req *agentv1.JobRequest, identity *agentv1.IdentityBinding) bool {
	return validID(req.GetJobId()) && validID(req.GetTopic()) && req.GetContextRef() != nil &&
		capsdk.ValidateIdentityBinding(req, identity) == nil
}

// randomDispatchID returns 128 bits from crypto/rand as 32 lowercase hex characters (R9).
func randomDispatchID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b) // never fails since Go 1.24
	return hex.EncodeToString(b), err
}

var (
	// ErrNotImplemented is stub only; the implementation deletes it.
	ErrNotImplemented = errors.New("referencescheduler: not implemented")
)
