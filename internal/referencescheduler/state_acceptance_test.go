//go:build refsched_acceptance

// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

// Held-out state-core contract: docs/adoption/framework-adapters-design.md section 2.5.
// B00_config_fails_closed: required clock, allowlist and positive limits.
// B01_verified_packets_only: verification, subject, payload and attempt admission.
// B02_tenant_and_subject_isolation: tenant-qualified keys and exact authority.
// B03_correlation_reserved_before_publish: atomic dispatch reservation.
// B04_duplicate_submit_idempotent: immutable requests and deep snapshots.
// B05_result_before_ack_not_lost: progress/results survive late acknowledgements.
// B06_retry_fences_attempts: retry limits and identity fencing.
// B07_cancel_result_race_one_winner: first terminal wins under concurrency.
// B08_artifacts_only_before_terminal: bounded, ordered preterminal append.
// B09_size_bounds: declared resource sizes and identifier bounds.
// B10_injected_clock_deadlines: exact timeout boundaries without sleeps.
// B11_error_text_canary: bare typed errors disclose no request data.
//
// R1 Refusals change nothing. A call that returns an error leaves the stored job unchanged, UpdatedAt included. Section 2.5 makes three exceptions, each recording a state and returning it: Submit records DENIED with ErrDenied, and also with ErrInputTooLarge; ApplyResult records FAILED with ErrOutputTooLarge. ErrTerminal changes nothing, and where the method returns a Job, that Job is the winner's snapshot. The tests advance the clock before every refused call.
//
// R2 Job.Capability, and the Capability an AllowRule matches, is JobRequest.meta.capability. It is the only capability field on the wire (proto/cordum/agent/v1/job.proto, JobMetadata.capability).
//
// R3 A 256-byte job_id or topic is accepted. A 4097-byte one is ErrInvalidJob and records nothing. The implementation chooses the exact bound within [256, 4096].
//
// R4 A worker-reported CANCELLED result on a job that is not CANCELLED is ErrInvalidTransition and changes nothing. On a CANCELLED job it returns nil and the job stays CANCELLED.
//
// R5 For a terminal state decided by a worker result, Job.Result is proto.Equal to the winning JobResult. For FAILED from output-too-large or from retry exhaustion, only Job.Status and Result's error_code and error_code_enum are pinned. For DENIED, client CANCELLED and TIMEOUT, Result's content is not pinned beyond all snapshots of the job agreeing.
//
// R6 UpdatedAt is Config.Now() at the job's latest state change, and DispatchedAt is Now at Dispatch. The tests pin UpdatedAt only after Submit and Dispatch, and through R1.
//
// R7 A worker result is accepted from DISPATCHED as well as from RUNNING. spec/07-state-machine.md, Responsibilities: 'Worker: moves DISPATCHED/RUNNING -> SUCCEEDED/FAILED'.
//
// R8 AckDispatch with the current identity, on a job already past DISPATCHED, returns nil and leaves the snapshot identical.
//
// R9 With Config.NewDispatchID nil, a dispatch id is 32 lowercase hex characters.
//
// Run: go test -tags refsched_acceptance ./internal/referencescheduler/
// The implementation removes only line 1 and the blank line 2; it does not edit anything else in this file.
package referencescheduler_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	rs "github.com/cordum-io/cap/v2/internal/referencescheduler"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *fakeClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.at }
func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); defer c.mu.Unlock(); c.at = c.at.Add(d) }

type fixture struct {
	t                                                                                       *testing.T
	clock                                                                                   fakeClock
	mu                                                                                      sync.Mutex
	next                                                                                    int
	key                                                                                     *ecdsa.PrivateKey
	tenantA, tenantB, workerA, workerB, clientA, clientB, topicA, topicB, capability, jobID string
	secrets                                                                                 []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	requireErr(t, err, nil)
	return &fixture{t: t, key: key, clock: fakeClock{at: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)},
		tenantA: "tenant-a", tenantB: "tenant-b", workerA: "worker-a", workerB: "worker-b",
		clientA: "client-a", clientB: "client-b", topicA: "job.refsched.acceptance", topicB: "job.refsched.other",
		capability: "acceptance.echo", jobID: "job-1"}
}
func (f *fixture) config() rs.Config {
	return rs.Config{Limits: rs.Limits{MaxInputBytes: 1024, MaxOutputBytes: 2048, MaxArtifacts: 3,
		MaxAttempts: 3, DispatchTimeout: 30 * time.Second, RunTimeout: 5 * time.Minute, MaxRunTimeout: 10 * time.Minute},
		Allow: []rs.AllowRule{{TenantID: f.tenantA, Topic: f.topicA, Capability: f.capability},
			{TenantID: f.tenantB, Topic: f.topicB, Capability: f.capability}},
		Now: f.clock.Now, NewDispatchID: func() (string, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.next++
			return fmt.Sprintf("dispatch-%04d", f.next), nil
		}}
}
func (f *fixture) scheduler(change func(*rs.Config)) *rs.Scheduler {
	f.t.Helper()
	cfg := f.config()
	if change != nil {
		change(&cfg)
	}
	s, err := rs.New(cfg)
	if err != nil {
		f.t.Fatalf("New: %v", err)
	}
	require(f.t, s != nil, "New returned nil scheduler")
	return s
}
func (f *fixture) ref(size uint64) *agentv1.ResourceRef {
	bucket := "acceptance"
	if strings.Contains(f.tenantA, "CANARY") {
		bucket = "bucket-CANARY"
	}
	return &agentv1.ResourceRef{ResolverId: "nats-kv", Uri: "nats-kv://" + bucket + "/" + f.tenantA + "/" + strings.Repeat("01", 32),
		Sha256: []byte(strings.Repeat("\x01", 32)), MediaType: "application/json", SizeBytes: size}
}
func (f *fixture) request(id string) *agentv1.JobRequest {
	return &agentv1.JobRequest{JobId: id, Topic: f.topicA, TenantId: f.tenantA,
		Meta: &agentv1.JobMetadata{Capability: f.capability}, ContextRef: f.ref(512),
		Labels: map[string]string{"canary": "label-CANARY-5a"}}
}
func progress(id string, d *agentv1.DispatchIdentity) *agentv1.JobProgress {
	return &agentv1.JobProgress{JobId: id, Status: agentv1.JobStatus_JOB_STATUS_RUNNING, Dispatch: proto.Clone(d).(*agentv1.DispatchIdentity)}
}
func (f *fixture) result(d *agentv1.DispatchIdentity, status agentv1.JobStatus) *agentv1.JobResult {
	return &agentv1.JobResult{JobId: f.jobID, Status: status, Dispatch: proto.Clone(d).(*agentv1.DispatchIdentity), ResultRef: f.ref(128),
		ResultPtr: "ptr-CANARY-3a", ErrorMessage: "message-CANARY-4a"}
}
func cancelOf(id string) *agentv1.JobCancel { return &agentv1.JobCancel{JobId: id} }
func (f *fixture) envelope(subject, tenant, sender string, payload proto.Message) *agentv1.BusPacket {
	id := make([]byte, 16)
	_, err := rand.Read(id)
	requireErr(f.t, err, nil)
	p := &agentv1.BusPacket{TraceId: hex.EncodeToString(id), SenderId: sender, ProtocolVersion: capsdk.DefaultProtocolVersion,
		Identity: &agentv1.IdentityBinding{TenantId: tenant},
		SignatureMetadata: &agentv1.SignatureMetadata{ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm,
			MessageId: id, Audience: subject, ExpiresAt: timestamppb.New(time.Now().Add(2 * time.Minute)), KeyId: "acceptance-k1"}}
	switch v := payload.(type) {
	case *agentv1.JobRequest:
		p.Payload = &agentv1.BusPacket_JobRequest{JobRequest: v}
	case *agentv1.JobProgress:
		p.Payload = &agentv1.BusPacket_JobProgress{JobProgress: v}
	case *agentv1.JobResult:
		p.Payload = &agentv1.BusPacket_JobResult{JobResult: v}
	case *agentv1.JobCancel:
		p.Payload = &agentv1.BusPacket_JobCancel{JobCancel: v}
	case nil:
	default:
		f.t.Fatalf("unsupported fixture payload %T", payload)
	}
	return p
}
func (f *fixture) tryPacket(p *agentv1.BusPacket, mode string) (capsdk.VerifiedProductionPacket, error) {
	f.t.Helper()
	var raw []byte
	var err error
	trust := capsdk.ProductionTrustStore{Audience: p.SignatureMetadata.Audience, Tenant: p.Identity.TenantId, Sender: p.SenderId,
		PublicKeys: map[string]*ecdsa.PublicKey{"acceptance-k1": &f.key.PublicKey}}
	if mode == "unsigned" {
		raw, err = proto.Marshal(p)
	} else {
		raw, err = capsdk.SignProductionPacket(p, f.key)
	}
	requireErr(f.t, err, nil)
	if mode == "expired" {
		trust.Now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	}
	if mode == "wrong_key" {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		requireErr(f.t, e, nil)
		trust.PublicKeys["acceptance-k1"] = &key.PublicKey
	}
	signed := &agentv1.BusPacket{}
	requireErr(f.t, proto.Unmarshal(raw, signed), nil)
	for _, b := range [][]byte{signed.Signature, p.SignatureMetadata.MessageId} {
		if len(b) > 0 {
			f.secrets = append(f.secrets, hex.EncodeToString(b), base64.StdEncoding.EncodeToString(b))
		}
	}
	return capsdk.VerifyTrustedProductionPacket(raw, trust)
}
func (f *fixture) packet(subject, tenant, sender string, payload proto.Message) capsdk.VerifiedProductionPacket {
	f.t.Helper()
	v, err := f.tryPacket(f.envelope(subject, tenant, sender, payload), "")
	requireErr(f.t, err, nil)
	return v
}
func (f *fixture) submit(s *rs.Scheduler) rs.Job {
	f.t.Helper()
	j, e := s.Submit(f.packet(rs.SubjectSubmit, f.tenantA, f.clientA, f.request(f.jobID)))
	requireErr(f.t, e, nil)
	want := rs.Job{TenantID: f.tenantA, JobID: f.jobID, Topic: f.topicA, Capability: f.capability,
		Status: agentv1.JobStatus_JOB_STATUS_SCHEDULED, UpdatedAt: f.clock.Now()}
	requireJob(f.t, j, want)
	requireJob(f.t, f.get(s), j)
	return j
}
func (f *fixture) dispatch(s *rs.Scheduler) rs.Dispatch {
	f.t.Helper()
	f.submit(s)
	d, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
	requireErr(f.t, e, nil)
	requireStatus(f.t, f.get(s), agentv1.JobStatus_JOB_STATUS_DISPATCHED)
	return d
}
func (f *fixture) run(s *rs.Scheduler) rs.Dispatch {
	f.t.Helper()
	d := f.dispatch(s)
	j, e := s.ApplyProgress(f.packet(rs.SubjectProgress, f.tenantA, f.workerA, progress(f.jobID, d.Identity)))
	requireErr(f.t, e, nil)
	requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_RUNNING)
	requireJob(f.t, j, f.get(s))
	return d
}
func (f *fixture) get(s *rs.Scheduler) rs.Job {
	f.t.Helper()
	j, e := s.Get(f.tenantA, f.jobID)
	requireErr(f.t, e, nil)
	return j
}
func requireErr(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error: %v; want %v", got, want)
	}
}
func requireStatus(t *testing.T, j rs.Job, want agentv1.JobStatus) {
	t.Helper()
	require(t, j.Status == want, fmt.Sprintf("status: %v; want %v", j.Status, want))
}
func diffJob(a, b rs.Job) string {
	for _, field := range []struct {
		name  string
		equal bool
	}{
		{"identity", a.TenantID == b.TenantID && a.JobID == b.JobID && a.Topic == b.Topic && a.Capability == b.Capability},
		{"status", a.Status == b.Status}, {"acked", a.Acked == b.Acked},
		{"dispatch", proto.Equal(a.Dispatch, b.Dispatch)}, {"result", proto.Equal(a.Result, b.Result)},
		{"artifact count", len(a.Artifacts) == len(b.Artifacts)},
		{"deadline", a.Deadline.Equal(b.Deadline)}, {"dispatched at", a.DispatchedAt.Equal(b.DispatchedAt)}, {"updated at", a.UpdatedAt.Equal(b.UpdatedAt)},
	} {
		if !field.equal {
			return field.name
		}
	}
	for i := range a.Artifacts {
		if !proto.Equal(a.Artifacts[i], b.Artifacts[i]) {
			return "artifact"
		}
	}
	return ""
}
func requireJob(t *testing.T, a, b rs.Job) {
	t.Helper()
	if d := diffJob(a, b); d != "" {
		t.Fatalf("job differs in %s: got %+v; want %+v", d, a, b)
	}
}
func (f *fixture) requireUnchanged(s *rs.Scheduler, before rs.Job) {
	f.t.Helper()
	requireJob(f.t, f.get(s), before)
}
func (f *fixture) refused(s *rs.Scheduler, want error, call func() error) {
	f.t.Helper()
	before := f.get(s)
	f.clock.Advance(time.Second)
	requireErr(f.t, call(), want)
	f.requireUnchanged(s, before)
}
func (f *fixture) missing(s *rs.Scheduler, tenant, id string) {
	f.t.Helper()
	_, e := s.Get(tenant, id)
	requireErr(f.t, e, rs.ErrJobNotFound)
}

type stateCase struct {
	name      string
	check     func(*fixture, *rs.Scheduler)
	configure func(*rs.Config)
}

func cases(t *testing.T, rows []stateCase) {
	t.Helper()
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) { f := newFixture(t); s := f.scheduler(row.configure); row.check(f, s) })
	}
}
func TestAcceptanceFixtures(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		want       error
	}{
		{"verified_packet_round_trip", "", nil}, {"expired_signature_is_refused", "expired", capsdk.ErrSignatureExpired},
		{"wrong_key_is_refused", "wrong_key", capsdk.ErrInvalidSignature}, {"unsigned_packet_is_refused", "unsigned", capsdk.ErrMissingSignature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			p := f.envelope(rs.SubjectSubmit, f.tenantA, f.clientA, f.request(f.jobID))
			v, e := f.tryPacket(p, tc.mode)
			requireErr(t, e, tc.want)
			if tc.want != nil {
				require(t, !v.IsVerified(), "rejected packet is verified")
				return
			}
			require(t, !(!v.IsVerified() || v.Subject() != rs.SubjectSubmit || v.Tenant() != f.tenantA || v.Sender() != f.clientA), "wrong verified authority")
			got := v.Packet()
			got.Signature = nil
			require(t, proto.Equal(got, p), "verified payload differs")
		})
	}
	t.Run("zero_value_is_unverified", func(t *testing.T) {
		require(t, !((capsdk.VerifiedProductionPacket{}).IsVerified()), "zero proof is verified")
	})
	t.Run("fake_clock_is_exact", func(t *testing.T) {
		f := newFixture(t)
		at := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
		require(t, f.clock.Now().Equal(at), "clock origin")
		f.clock.Advance(time.Second + time.Nanosecond)
		require(t, f.clock.Now().Equal(at.Add(time.Second+time.Nanosecond)), "clock advance")
	})
}
func TestStateCoreAcceptance(t *testing.T) {
	t.Run("B00_config_fails_closed", configCases)
	t.Run("B01_verified_packets_only", admissionCases)
	t.Run("B02_tenant_and_subject_isolation", isolationCases)
	t.Run("B03_correlation_reserved_before_publish", dispatchCases)
	t.Run("B04_duplicate_submit_idempotent", duplicateCases)
	t.Run("B05_result_before_ack_not_lost", ackCases)
	t.Run("B06_retry_fences_attempts", retryCases)
	t.Run("B07_cancel_result_race_one_winner", terminalCases)
	t.Run("B08_artifacts_only_before_terminal", artifactCases)
	t.Run("B09_size_bounds", sizeCases)
	t.Run("B10_injected_clock_deadlines", deadlineCases)
	t.Run("B11_error_text_canary", canaryCases)
}

// B00: section 2.5 New, including nil dispatch-ID source (R9).
func configCases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*rs.Config)
		want   error
	}{
		{"nil_now", func(c *rs.Config) { c.Now = nil }, rs.ErrInvalidConfig},
		{"empty_allow", func(c *rs.Config) { c.Allow = nil }, rs.ErrInvalidConfig},
		{"zero_max_attempts", func(c *rs.Config) { c.Limits.MaxAttempts = 0 }, rs.ErrInvalidConfig},
		{"zero_max_input_bytes", func(c *rs.Config) { c.Limits.MaxInputBytes = 0 }, rs.ErrInvalidConfig},
		{"zero_max_output_bytes", func(c *rs.Config) { c.Limits.MaxOutputBytes = 0 }, rs.ErrInvalidConfig},
		{"zero_max_artifacts", func(c *rs.Config) { c.Limits.MaxArtifacts = 0 }, rs.ErrInvalidConfig},
		{"negative_max_artifacts", func(c *rs.Config) { c.Limits.MaxArtifacts = -1 }, rs.ErrInvalidConfig},
		{"zero_dispatch_timeout", func(c *rs.Config) { c.Limits.DispatchTimeout = 0 }, rs.ErrInvalidConfig},
		{"negative_dispatch_timeout", func(c *rs.Config) { c.Limits.DispatchTimeout = -1 }, rs.ErrInvalidConfig},
		{"zero_run_timeout", func(c *rs.Config) { c.Limits.RunTimeout = 0 }, rs.ErrInvalidConfig},
		{"negative_run_timeout", func(c *rs.Config) { c.Limits.RunTimeout = -1 }, rs.ErrInvalidConfig},
		{"zero_max_run_timeout", func(c *rs.Config) { c.Limits.MaxRunTimeout = 0 }, rs.ErrInvalidConfig},
		{"negative_max_run_timeout", func(c *rs.Config) { c.Limits.MaxRunTimeout = -1 }, rs.ErrInvalidConfig},
		{"valid_config", func(c *rs.Config) {}, nil}, {"nil_dispatch_id_source_is_valid", func(c *rs.Config) { c.NewDispatchID = nil }, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.config()
			tc.change(&c)
			s, e := rs.New(c)
			requireErr(t, e, tc.want)
			require(t, !(tc.want == nil && s == nil), "nil scheduler")
		})
	}
}

// B01: section 2.5 packet-method checks run before mutation (R1).
func admissionCases(t *testing.T) {
	for _, tc := range []struct {
		name, op, fault string
		want            error
	}{
		{"submit_zero_value", "submit", "zero", rs.ErrUnverified}, {"submit_unsigned", "submit", "unsigned", rs.ErrUnverified},
		{"submit_expired", "submit", "expired", rs.ErrUnverified}, {"submit_wrong_key", "submit", "wrong_key", rs.ErrUnverified},
		{"progress_zero_value", "progress", "zero", rs.ErrUnverified}, {"result_zero_value", "result", "zero", rs.ErrUnverified}, {"cancel_zero_value", "cancel", "zero", rs.ErrUnverified},
		{"submit_wrong_subject", "submit", "subject", rs.ErrWrongSubject}, {"progress_wrong_subject", "progress", "subject", rs.ErrWrongSubject},
		{"result_wrong_subject", "result", "subject", rs.ErrWrongSubject}, {"cancel_on_worker_cancel_subject", "cancel", "subject", rs.ErrWrongSubject},
		{"submit_wrong_payload", "submit", "payload", rs.ErrWrongPayload}, {"submit_no_payload", "submit", "none", rs.ErrWrongPayload},
		{"progress_wrong_payload", "progress", "payload", rs.ErrWrongPayload}, {"result_wrong_payload", "result", "payload", rs.ErrWrongPayload}, {"cancel_wrong_payload", "cancel", "payload", rs.ErrWrongPayload},
		{"progress_wrong_attempt", "progress", "attempt", rs.ErrStaleAttempt}, {"result_wrong_attempt", "result", "attempt", rs.ErrStaleAttempt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			s := f.scheduler(nil)
			var d rs.Dispatch
			if tc.op != "submit" {
				d = f.dispatch(s)
			}
			v := badAdmission(f, tc.op, tc.fault, d.Identity)
			call := func() error { _, e := packetCall(s, tc.op, v); return e }
			if tc.op == "submit" {
				f.clock.Advance(time.Second)
				requireErr(t, call(), tc.want)
				f.missing(s, f.tenantA, f.jobID)
			} else {
				f.refused(s, tc.want, call)
			}
		})
	}
}
func packetCall(s *rs.Scheduler, op string, v capsdk.VerifiedProductionPacket) (rs.Job, error) {
	switch op {
	case "submit":
		return s.Submit(v)
	case "progress":
		return s.ApplyProgress(v)
	case "result":
		return s.ApplyResult(v)
	default:
		return s.Cancel(v)
	}
}
func badAdmission(f *fixture, op, fault string, d *agentv1.DispatchIdentity) capsdk.VerifiedProductionPacket {
	if fault == "zero" {
		return capsdk.VerifiedProductionPacket{}
	}
	subject, sender := rs.SubjectSubmit, f.clientA
	var body proto.Message = f.request(f.jobID)
	if d != nil {
		d = proto.Clone(d).(*agentv1.DispatchIdentity)
		if fault == "attempt" {
			d.Attempt++
		}
	}
	switch op {
	case "progress":
		subject, sender, body = rs.SubjectProgress, f.workerA, progress(f.jobID, d)
	case "result":
		subject, sender, body = rs.SubjectResult, f.workerA, f.result(d, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
	case "cancel":
		subject, body = rs.SubjectClientCancel, cancelOf(f.jobID)
	}
	if fault == "subject" {
		switch op {
		case "progress":
			subject = rs.SubjectResult
		case "cancel":
			subject = rs.SubjectCancel
		default:
			subject = rs.SubjectProgress
		}
	}
	if fault == "payload" {
		body = &agentv1.JobResult{JobId: f.jobID}
		if op == "result" {
			body = &agentv1.JobProgress{JobId: f.jobID}
		}
		if op == "cancel" {
			body = f.request(f.jobID)
		}
	}
	if fault == "none" {
		body = nil
	}
	p := f.envelope(subject, f.tenantA, sender, body)
	want := map[string]error{"unsigned": capsdk.ErrMissingSignature, "expired": capsdk.ErrSignatureExpired, "wrong_key": capsdk.ErrInvalidSignature}[fault]
	v, e := f.tryPacket(p, fault)
	requireErr(f.t, e, want)
	return v
}

// B02: section 2.5 job key, allowlist, tenant mirror and worker fence (R1,R2).
func isolationCases(t *testing.T) {
	for _, name := range []string{"get_other_tenant", "dispatch_other_tenant", "ack_other_tenant", "progress_other_tenant", "result_other_tenant", "cancel_other_tenant",
		"same_job_id_other_tenant_is_separate", "allowlist_is_per_tenant", "allowlist_matches_topic_exactly", "allowlist_matches_capability_exactly",
		"tenant_mirror_mismatch_records_nothing", "empty_tenant_mirror_is_accepted", "progress_from_other_worker", "result_from_other_worker", "assigned_worker_mismatch_in_payload", "unknown_job"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			s := f.scheduler(nil)
			if strings.Contains(name, "allowlist") || strings.Contains(name, "mirror") {
				isolationSubmit(f, s, name)
				return
			}
			var d rs.Dispatch
			if name == "dispatch_other_tenant" {
				f.submit(s)
			} else {
				d = f.dispatch(s)
			}
			if name == "same_job_id_other_tenant_is_separate" {
				before := f.get(s)
				f.clock.Advance(time.Second)
				r := f.request(f.jobID)
				r.TenantId = f.tenantB
				r.Topic = f.topicB
				j, e := s.Submit(f.packet(rs.SubjectSubmit, f.tenantB, f.clientB, r))
				requireErr(t, e, nil)
				want := rs.Job{TenantID: f.tenantB, JobID: f.jobID, Topic: f.topicB, Capability: f.capability, Status: agentv1.JobStatus_JOB_STATUS_SCHEDULED, UpdatedAt: f.clock.Now()}
				requireJob(t, j, want)
				stored, e := s.Get(f.tenantB, f.jobID)
				requireErr(t, e, nil)
				requireJob(t, j, stored)
				f.requireUnchanged(s, before)
				return
			}
			want := rs.ErrJobNotFound
			if strings.Contains(name, "other_worker") || name == "assigned_worker_mismatch_in_payload" {
				want = rs.ErrWrongWorker
			}
			f.refused(s, want, func() error { return isolationCall(f, s, d, name) })
		})
	}
}
func isolationSubmit(f *fixture, s *rs.Scheduler, name string) {
	r := f.request(f.jobID)
	tenant := f.tenantA
	want := rs.ErrDenied
	switch name {
	case "allowlist_is_per_tenant":
		tenant = f.tenantB
		r.TenantId = f.tenantB
		r.JobId = "job-2"
	case "allowlist_matches_topic_exactly":
		r.Topic += ".x"
	case "allowlist_matches_capability_exactly":
		r.Meta.Capability = "acceptance.other"
	case "tenant_mirror_mismatch_records_nothing":
		tenant = f.tenantB
		r.JobId, want = "job-4", rs.ErrInvalidJob
	case "empty_tenant_mirror_is_accepted":
		r.TenantId, want = "", nil
	}
	f.clock.Advance(time.Second)
	j, e := s.Submit(f.packet(rs.SubjectSubmit, tenant, f.clientA, r))
	requireErr(f.t, e, want)
	if want == rs.ErrInvalidJob {
		f.missing(s, tenant, r.JobId)
		return
	}
	status := agentv1.JobStatus_JOB_STATUS_DENIED
	if want == nil {
		status = agentv1.JobStatus_JOB_STATUS_SCHEDULED
	}
	requireStatus(f.t, j, status)
	stored, e := s.Get(tenant, r.JobId)
	requireErr(f.t, e, nil)
	requireJob(f.t, j, stored)
}
func isolationCall(f *fixture, s *rs.Scheduler, d rs.Dispatch, name string) error {
	var e error
	switch name {
	case "get_other_tenant":
		_, e = s.Get(f.tenantB, f.jobID)
	case "dispatch_other_tenant":
		_, e = s.Dispatch(f.tenantB, f.jobID, f.workerB)
	case "ack_other_tenant":
		d.TenantID = f.tenantB
		e = s.AckDispatch(d)
	case "cancel_other_tenant":
		_, e = s.Cancel(f.packet(rs.SubjectClientCancel, f.tenantB, f.clientB, cancelOf(f.jobID)))
	default:
		tenant, sender, id := f.tenantA, f.workerA, f.jobID
		if strings.HasSuffix(name, "other_tenant") {
			tenant = f.tenantB
		}
		if strings.HasSuffix(name, "other_worker") {
			sender = f.workerB
		}
		if name == "unknown_job" {
			id = "job-404"
		}
		if name == "assigned_worker_mismatch_in_payload" {
			d.Identity = proto.Clone(d.Identity).(*agentv1.DispatchIdentity)
			d.Identity.AssignedWorkerId = f.workerB
		}
		if strings.HasPrefix(name, "result") {
			_, e = s.ApplyResult(f.packet(rs.SubjectResult, tenant, sender, f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)))
		} else {
			_, e = s.ApplyProgress(f.packet(rs.SubjectProgress, tenant, sender, progress(id, d.Identity)))
		}
	}
	return e
}

// B03: section 2.5 Dispatch reserves before return; nil ID source uses R9.
func dispatchCases(t *testing.T) {
	cases(t, []stateCase{
		{"dispatch_persists_identity_before_return", dispatchReservation, nil},
		{"subject_constants", func(f *fixture, s *rs.Scheduler) {
			got := []string{rs.SubjectSubmit, rs.SubjectResult, rs.SubjectProgress, rs.SubjectCancel, rs.SubjectClientCancel, rs.SubjectTerminal, rs.WorkerSubject("w-1")}
			want := []string{"sys.job.submit", "sys.job.result", "sys.job.progress", "sys.job.cancel", "refsched.job.cancel", "refsched.job.terminal", "worker.w-1.jobs"}
			for i := range want {
				require(f.t, got[i] == want[i], fmt.Sprintf("subject %d: %q; want %q", i, got[i], want[i]))
			}
		}, nil},
		{"redispatch_is_refused", func(f *fixture, s *rs.Scheduler) {
			f.dispatch(s)
			f.refused(s, rs.ErrInvalidTransition, func() error { _, e := s.Dispatch(f.tenantA, f.jobID, f.workerB); return e })
		}, nil},
		{"dispatch_requires_worker", func(f *fixture, s *rs.Scheduler) {
			f.submit(s)
			f.refused(s, rs.ErrInvalidJob, func() error { _, e := s.Dispatch(f.tenantA, f.jobID, ""); return e })
		}, nil},
		{"dispatch_unknown_job", func(f *fixture, s *rs.Scheduler) {
			f.clock.Advance(time.Second)
			_, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
			requireErr(f.t, e, rs.ErrJobNotFound)
			f.missing(s, f.tenantA, f.jobID)
		}, nil},
		{"dispatch_terminal_job", func(f *fixture, s *rs.Scheduler) {
			r := f.request(f.jobID)
			r.Topic += ".denied"
			j, e := s.Submit(f.packet(rs.SubjectSubmit, f.tenantA, f.clientA, r))
			requireErr(f.t, e, rs.ErrDenied)
			requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_DENIED)
			f.refused(s, rs.ErrTerminal, func() error { _, e := s.Dispatch(f.tenantA, f.jobID, f.workerA); return e })
		}, nil},
		{"default_dispatch_id_is_random_hex", func(f *fixture, s *rs.Scheduler) {
			d1 := f.dispatch(s)
			f.jobID = "job-2"
			d2 := f.dispatch(s)
			rx := regexp.MustCompile("^[0-9a-f]{32}$")
			require(f.t, rx.MatchString(d1.Identity.GetDispatchId()) && rx.MatchString(d2.Identity.GetDispatchId()) && d1.Identity.GetDispatchId() != d2.Identity.GetDispatchId(), "default dispatch IDs must be distinct 128-bit lowercase hex")
		}, func(c *rs.Config) { c.NewDispatchID = nil }},
		{"failing_dispatch_id_source_reserves_nothing", func(f *fixture, s *rs.Scheduler) {
			before := f.submit(s)
			f.clock.Advance(time.Second)
			_, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
			require(f.t, e != nil, "ID source failure accepted")
			f.requireUnchanged(s, before)
		}, func(c *rs.Config) {
			c.NewDispatchID = func() (string, error) { return "", errors.New("test ID source failure") }
		}},
	})
}
func dispatchReservation(f *fixture, s *rs.Scheduler) {
	f.submit(s)
	f.clock.Advance(time.Second)
	at := f.clock.Now()
	d, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
	requireErr(f.t, e, nil)
	identity := &agentv1.DispatchIdentity{DispatchId: "dispatch-0001", Attempt: 1, AssignedWorkerId: f.workerA}
	req := f.request(f.jobID)
	req.Dispatch = identity
	require(f.t, !(d.TenantID != f.tenantA || d.JobID != f.jobID || d.Subject != "worker.worker-a.jobs" || d.Subject != rs.WorkerSubject(f.workerA) || !proto.Equal(d.Identity, identity) || !proto.Equal(d.Request, req)), fmt.Sprintf("dispatch: %+v", d))
	want := rs.Job{TenantID: f.tenantA, JobID: f.jobID, Topic: f.topicA, Capability: f.capability, Status: agentv1.JobStatus_JOB_STATUS_DISPATCHED,
		Dispatch: identity, DispatchedAt: at, Deadline: at.Add(5 * time.Minute), UpdatedAt: at}
	requireJob(f.t, f.get(s), want)
}
func namedCases(t *testing.T, names string, check func(*fixture, *rs.Scheduler, string), configure func(string, *rs.Config)) {
	t.Helper()
	for _, name := range strings.Fields(names) {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			s := f.scheduler(func(c *rs.Config) {
				if configure != nil {
					configure(name, c)
				}
			})
			check(f, s, name)
		})
	}
}
func (f *fixture) applyResult(s *rs.Scheduler, r *agentv1.JobResult, want error, status agentv1.JobStatus) rs.Job {
	f.t.Helper()
	j, e := s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerA, r))
	requireErr(f.t, e, want)
	requireStatus(f.t, j, status)
	requireJob(f.t, j, f.get(s))
	return j
}
func (f *fixture) applyProgress(s *rs.Scheduler, p *agentv1.JobProgress) rs.Job {
	f.t.Helper()
	j, e := s.ApplyProgress(f.packet(rs.SubjectProgress, f.tenantA, f.workerA, p))
	requireErr(f.t, e, nil)
	requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_RUNNING)
	requireJob(f.t, j, f.get(s))
	return j
}
func (f *fixture) cancel(s *rs.Scheduler) rs.Job {
	f.t.Helper()
	j, e := s.Cancel(f.packet(rs.SubjectClientCancel, f.tenantA, f.clientA, cancelOf(f.jobID)))
	requireErr(f.t, e, nil)
	requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_CANCELLED)
	requireJob(f.t, j, f.get(s))
	return j
}
func requireResult(t *testing.T, j rs.Job, r *agentv1.JobResult) {
	t.Helper()
	require(t, proto.Equal(j.Result, r), fmt.Sprintf("result: %v; want %v", j.Result, r))
}

// B04: section 2.5 Submit proto.Equal idempotency and Job deep-copy contract (R1).
func duplicateCases(t *testing.T) {
	namedCases(t, "identical_request_new_packet duplicate_after_dispatch different_context_ref_conflicts different_labels_conflicts snapshots_are_deep_copies", func(f *fixture, s *rs.Scheduler, name string) {
		if name == "snapshots_are_deep_copies" {
			snapshotCopies(f, s)
			return
		}
		f.submit(s)
		if name == "duplicate_after_dispatch" {
			_, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
			requireErr(f.t, e, nil)
		}
		before := f.get(s)
		r := f.request(f.jobID)
		var want error
		switch name {
		case "different_context_ref_conflicts":
			r.ContextRef.Sha256[0] = 2
			want = rs.ErrJobConflict
		case "different_labels_conflicts":
			r.Labels, want = map[string]string{"changed": "yes"}, rs.ErrJobConflict
		}
		f.clock.Advance(time.Second)
		j, e := s.Submit(f.packet(rs.SubjectSubmit, f.tenantA, f.clientA, r))
		requireErr(f.t, e, want)
		f.requireUnchanged(s, before)
		if want == nil {
			requireJob(f.t, j, before)
			requireJob(f.t, j, f.get(s))
		}
	}, nil)
}
func snapshotCopies(f *fixture, s *rs.Scheduler) {
	d := f.dispatch(s)
	p := progress(f.jobID, d.Identity)
	p.ArtifactRefs = []*agentv1.ResourceRef{f.ref(1)}
	f.applyProgress(s, p)
	r := f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
	f.applyResult(s, r, nil, r.Status)
	j := f.get(s)
	j.Dispatch.DispatchId = "changed"
	j.Artifacts[0].Uri = "changed"
	j.Result.ResultRef.Uri = "changed"
	j.Artifacts = append(j.Artifacts, f.ref(9))
	fresh := f.get(s)
	require(f.t, !(fresh.Dispatch.GetDispatchId() != "dispatch-0001" || len(fresh.Artifacts) != 1 || fresh.Artifacts[0].GetUri() != f.ref(1).Uri || fresh.Result.GetResultRef().GetUri() != f.ref(128).Uri), "snapshot mutation leaked into stored job")
}

// B05: section 2.5 AckDispatch and R7/R8 preserve updates that precede the ack.
func ackCases(t *testing.T) {
	namedCases(t, "result_before_ack_is_kept progress_before_ack_is_kept ack_marks_current_dispatch ack_with_other_dispatch_id ack_with_other_attempt ack_unknown_job", func(f *fixture, s *rs.Scheduler, name string) {
		d := f.dispatch(s)
		if strings.Contains(name, "before_ack") {
			var j rs.Job
			if name == "result_before_ack_is_kept" {
				r := f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
				j = f.applyResult(s, r, nil, r.Status)
				requireResult(f.t, j, r)
			} else {
				j = f.applyProgress(s, progress(f.jobID, d.Identity))
			}
			require(f.t, !j.Acked, "ack recorded before acknowledgement")
			f.clock.Advance(time.Second)
			requireErr(f.t, s.AckDispatch(d), nil)
			f.requireUnchanged(s, j)
			f.clock.Advance(31 * time.Second)
			requireTickEmpty(f, s)
			f.requireUnchanged(s, j)
			return
		}
		if name == "ack_marks_current_dispatch" {
			before := f.get(s)
			before.Acked = true
			requireErr(f.t, s.AckDispatch(d), nil)
			requireJob(f.t, f.get(s), before)
			requireErr(f.t, s.AckDispatch(d), nil)
			requireJob(f.t, f.get(s), before)
			return
		}
		want := rs.ErrStaleAttempt
		d.Identity = proto.Clone(d.Identity).(*agentv1.DispatchIdentity)
		switch name {
		case "ack_with_other_dispatch_id":
			d.Identity.DispatchId = "other"
		case "ack_with_other_attempt":
			d.Identity.Attempt++
		case "ack_unknown_job":
			d.JobID = "job-404"
			want = rs.ErrJobNotFound
		}
		f.refused(s, want, func() error { return s.AckDispatch(d) })
	}, nil)
}

// B06: section 2.1 retry exhaustion and section 2.5 active-identity fence (R1,R5).
func retryCases(t *testing.T) {
	namedCases(t, "retryable_failure_reschedules stale_progress_after_retry stale_result_after_retry future_attempt other_dispatch_id stale_ack_after_retry current_attempt_completes attempts_exhausted_is_failed single_attempt_fails_at_once fatal_failure_is_terminal", retryCheck, func(name string, c *rs.Config) {
		if name == "attempts_exhausted_is_failed" {
			c.Limits.MaxAttempts = 2
		}
		if name == "single_attempt_fails_at_once" {
			c.Limits.MaxAttempts = 1
		}
	})
}
func retryCheck(f *fixture, s *rs.Scheduler, name string) {
	d1 := f.run(s)
	r := f.result(d1.Identity, agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE)
	r.ErrorCode = "TRANSIENT"
	r.ErrorCodeEnum = agentv1.ErrorCode_ERROR_CODE_JOB_WORKER_UNAVAILABLE
	if name == "fatal_failure_is_terminal" {
		r.Status = agentv1.JobStatus_JOB_STATUS_FAILED_FATAL
		requireResult(f.t, f.applyResult(s, r, nil, r.Status), r)
		refuseRedispatch(f, s)
		return
	}
	if name == "single_attempt_fails_at_once" {
		j := f.applyResult(s, r, nil, agentv1.JobStatus_JOB_STATUS_FAILED)
		requireRetryCodes(f, j)
		return
	}
	j := f.applyResult(s, r, nil, agentv1.JobStatus_JOB_STATUS_SCHEDULED)
	require(f.t, j.Result == nil, "retry retained terminal result")
	d2, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
	requireErr(f.t, e, nil)
	require(f.t, proto.Equal(d2.Identity, &agentv1.DispatchIdentity{DispatchId: "dispatch-0002", Attempt: 2, AssignedWorkerId: f.workerA}), fmt.Sprintf("retry identity: %v", d2.Identity))
	j = f.get(s)
	requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_DISPATCHED)
	require(f.t, !j.Acked, "retry is already acked")
	switch name {
	case "retryable_failure_reschedules":
		return
	case "attempts_exhausted_is_failed":
		r.Dispatch = proto.Clone(d2.Identity).(*agentv1.DispatchIdentity)
		j = f.applyResult(s, r, nil, agentv1.JobStatus_JOB_STATUS_FAILED)
		requireRetryCodes(f, j)
		refuseRedispatch(f, s)
	case "current_attempt_completes":
		f.applyProgress(s, progress(f.jobID, d2.Identity))
		r = f.result(d2.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
		requireResult(f.t, f.applyResult(s, r, nil, r.Status), r)
	default:
		retryRefusal(f, s, d1, d2, name)
	}
}
func requireRetryCodes(f *fixture, j rs.Job) {
	require(f.t, !(j.Result.GetErrorCode() != "TRANSIENT" || j.Result.GetErrorCodeEnum() != agentv1.ErrorCode_ERROR_CODE_JOB_WORKER_UNAVAILABLE), fmt.Sprintf("retry exhaustion codes: %v", j.Result))
}
func refuseRedispatch(f *fixture, s *rs.Scheduler) {
	f.refused(s, rs.ErrTerminal, func() error { _, e := s.Dispatch(f.tenantA, f.jobID, f.workerA); return e })
}
func retryRefusal(f *fixture, s *rs.Scheduler, d1, d2 rs.Dispatch, name string) {
	d := proto.Clone(d1.Identity).(*agentv1.DispatchIdentity)
	if name == "future_attempt" || name == "other_dispatch_id" {
		d = proto.Clone(d2.Identity).(*agentv1.DispatchIdentity)
		if name == "future_attempt" {
			d.Attempt = 3
		} else {
			d.DispatchId = "dispatch-9999"
		}
	}
	f.refused(s, rs.ErrStaleAttempt, func() error {
		if name == "stale_ack_after_retry" {
			return s.AckDispatch(d1)
		}
		if name == "stale_progress_after_retry" {
			_, e := s.ApplyProgress(f.packet(rs.SubjectProgress, f.tenantA, f.workerA, progress(f.jobID, d)))
			return e
		}
		_, e := s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerA, f.result(d, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)))
		return e
	})
}

// B07: section 2.5 Cancel/terminal rules and R4; only the first terminal wins.
func terminalCases(t *testing.T) {
	namedCases(t, "cancel_scheduled cancel_dispatched cancel_running cancel_terminal_returns_winner result_after_cancel_is_refused repeat_of_winning_result_is_accepted conflicting_result_after_terminal worker_cancelled_on_running_job_is_refused worker_cancelled_after_client_cancel_is_accepted cancel_vs_result_race", terminalCheck, nil)
}
func terminalCheck(f *fixture, s *rs.Scheduler, name string) {
	if name == "cancel_vs_result_race" {
		for trial := 0; trial < 32; trial++ {
			if trial > 0 {
				f = newFixture(f.t)
				s = f.scheduler(nil)
			}
			terminalRace(f, s)
		}
		return
	}
	if name == "cancel_scheduled" {
		f.submit(s)
		f.cancel(s)
		return
	}
	if name == "cancel_dispatched" {
		f.dispatch(s)
		f.cancel(s)
		return
	}
	terminalRunning(f, s, name)
}
func terminalRunning(f *fixture, s *rs.Scheduler, name string) {
	d := f.run(s)
	if name == "cancel_running" {
		f.cancel(s)
		return
	}
	r := f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
	if name == "worker_cancelled_on_running_job_is_refused" {
		r.Status = agentv1.JobStatus_JOB_STATUS_CANCELLED
		f.refused(s, rs.ErrInvalidTransition, func() error { _, e := s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerA, r)); return e })
		return
	}
	if name == "result_after_cancel_is_refused" || name == "worker_cancelled_after_client_cancel_is_accepted" {
		f.cancel(s)
	} else {
		requireResult(f.t, f.applyResult(s, r, nil, r.Status), r)
	}
	before := f.get(s)
	want := rs.ErrTerminal
	if name == "repeat_of_winning_result_is_accepted" {
		want = nil
	}
	if name == "conflicting_result_after_terminal" {
		r.Status = agentv1.JobStatus_JOB_STATUS_FAILED
	}
	if name == "worker_cancelled_after_client_cancel_is_accepted" {
		r.Status, want = agentv1.JobStatus_JOB_STATUS_CANCELLED, nil
	}
	f.clock.Advance(time.Second)
	var j rs.Job
	var e error
	if name == "cancel_terminal_returns_winner" {
		j, e = s.Cancel(f.packet(rs.SubjectClientCancel, f.tenantA, f.clientA, cancelOf(f.jobID)))
	} else {
		j, e = s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerA, r))
	}
	requireErr(f.t, e, want)
	if name == "worker_cancelled_after_client_cancel_is_accepted" {
		requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_CANCELLED)
		requireJob(f.t, j, f.get(s))
		return
	}
	requireJob(f.t, j, before)
	f.requireUnchanged(s, before)
}
func terminalRace(f *fixture, s *rs.Scheduler) {
	d := f.run(s)
	packets := make([]capsdk.VerifiedProductionPacket, 16)
	results := make([]*agentv1.JobResult, 16)
	for i := range packets {
		if i < 8 {
			c := cancelOf(f.jobID)
			c.Reason = fmt.Sprintf("race-%d", i)
			packets[i] = f.packet(rs.SubjectClientCancel, f.tenantA, f.clientA, c)
		} else {
			results[i] = f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
			results[i].ExecutionMs = int64(i + 1)
			packets[i] = f.packet(rs.SubjectResult, f.tenantA, f.workerA, results[i])
		}
	}
	racePackets(f, s, packets, results)
}
func racePackets(f *fixture, s *rs.Scheduler, packets []capsdk.VerifiedProductionPacket, results []*agentv1.JobResult) {
	out := make([]struct {
		job rs.Job
		err error
	}, len(packets))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range packets {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if i < 8 {
				out[i].job, out[i].err = s.Cancel(packets[i])
			} else {
				out[i].job, out[i].err = s.ApplyResult(packets[i])
			}
		}(i)
	}
	close(start)
	wg.Wait()
	winner := -1
	for i, item := range out {
		if item.err == nil {
			require(f.t, winner == -1, "multiple terminal winners")
			winner = i
		} else {
			requireErr(f.t, item.err, rs.ErrTerminal)
		}
	}
	require(f.t, winner >= 0, "no terminal winner")
	final := f.get(s)
	if winner < 8 {
		requireStatus(f.t, final, agentv1.JobStatus_JOB_STATUS_CANCELLED)
	} else {
		requireStatus(f.t, final, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
		requireResult(f.t, final, results[winner])
	}
	for _, item := range out {
		requireJob(f.t, item.job, final)
	}
}

// B08: section 2.5 ApplyProgress appends bounded artifacts only before terminal.
func artifactCases(t *testing.T) {
	namedCases(t, "first_progress_without_status_starts_running progress_artifacts_append_in_order artifacts_past_max_are_refused_whole progress_after_worker_terminal progress_after_client_cancel progress_after_timeout progress_with_terminal_status_is_refused", artifactCheck, nil)
}
func artifactCheck(f *fixture, s *rs.Scheduler, name string) {
	d := f.dispatch(s)
	p := progress(f.jobID, d.Identity)
	a1, a2, a3, a4 := f.ref(1), f.ref(2), f.ref(3), f.ref(4)
	p.ArtifactRefs = []*agentv1.ResourceRef{a1}
	if name == "first_progress_without_status_starts_running" {
		p.Status = agentv1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
	if name == "progress_after_timeout" {
		f.clock.Advance(30 * time.Second)
		requireTimeout(f, s)
	} else {
		f.applyProgress(s, p)
	}
	if name == "first_progress_without_status_starts_running" {
		requireArtifacts(f, f.get(s), []*agentv1.ResourceRef{a1})
		return
	}
	if name == "progress_artifacts_append_in_order" {
		p.ArtifactRefs = []*agentv1.ResourceRef{a2, a3}
		j := f.applyProgress(s, p)
		requireArtifacts(f, j, []*agentv1.ResourceRef{a1, a2, a3})
		return
	}
	want := rs.ErrTerminal
	p.ArtifactRefs = []*agentv1.ResourceRef{a2}
	switch name {
	case "artifacts_past_max_are_refused_whole":
		f.applyProgress(s, p)
		p.ArtifactRefs = []*agentv1.ResourceRef{a3, a4}
		want = rs.ErrTooManyArtifacts
	case "progress_after_worker_terminal":
		r := f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
		f.applyResult(s, r, nil, r.Status)
	case "progress_after_client_cancel":
		f.cancel(s)
	case "progress_with_terminal_status_is_refused":
		p.Status = agentv1.JobStatus_JOB_STATUS_SUCCEEDED
		want = rs.ErrInvalidTransition
	}
	f.refused(s, want, func() error {
		j, e := s.ApplyProgress(f.packet(rs.SubjectProgress, f.tenantA, f.workerA, p))
		if want == rs.ErrTerminal {
			requireJob(f.t, j, f.get(s))
		}
		return e
	})
}
func requireArtifacts(f *fixture, j rs.Job, want []*agentv1.ResourceRef) {
	require(f.t, len(j.Artifacts) == len(want), fmt.Sprintf("artifacts: got %d; want %d", len(j.Artifacts), len(want)))
	for i := range want {
		require(f.t, proto.Equal(j.Artifacts[i], want[i]), fmt.Sprintf("artifact %d differs", i))
	}
}

// B09: section 2.4 declared-size bounds and section 2.5 validity; identifier R3.
func sizeCases(t *testing.T) {
	namedCases(t, "input_at_bound_is_scheduled input_over_bound_is_denied output_at_bound_succeeds output_over_bound_fails_resource_exhausted missing_context_ref_records_nothing empty_job_id_is_invalid empty_topic_records_nothing long_ids_within_bound_are_accepted oversized_job_id_records_nothing oversized_topic_records_nothing", sizeCheck, func(name string, c *rs.Config) {
		n := 0
		if name == "long_ids_within_bound_are_accepted" {
			n = 256
		}
		if name == "oversized_topic_records_nothing" {
			n = 4097
		}
		if n > 0 {
			rule := c.Allow[0]
			rule.Topic = strings.Repeat("t", n)
			c.Allow = append(c.Allow, rule)
		}
	})
}
func sizeCheck(f *fixture, s *rs.Scheduler, name string) {
	if strings.HasPrefix(name, "output_") {
		d := f.run(s)
		r := f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
		r.ResultRef.SizeBytes = 2048
		if name == "output_at_bound_succeeds" {
			requireResult(f.t, f.applyResult(s, r, nil, r.Status), r)
		} else {
			r.ResultRef.SizeBytes++
			f.clock.Advance(time.Second)
			j := f.applyResult(s, r, rs.ErrOutputTooLarge, agentv1.JobStatus_JOB_STATUS_FAILED)
			require(f.t, !(j.Result.GetErrorCodeEnum() != agentv1.ErrorCode_ERROR_CODE_JOB_RESOURCE_EXHAUSTED), fmt.Sprintf("output error code: %v", j.Result))
		}
		return
	}
	r := f.request(f.jobID)
	var want error
	status := agentv1.JobStatus_JOB_STATUS_SCHEDULED
	switch name {
	case "input_at_bound_is_scheduled":
		r.ContextRef.SizeBytes = 1024
	case "input_over_bound_is_denied":
		r.ContextRef.SizeBytes, want = 1025, rs.ErrInputTooLarge
		status = agentv1.JobStatus_JOB_STATUS_DENIED
	case "missing_context_ref_records_nothing":
		r.ContextRef, want = nil, rs.ErrInvalidJob
	case "empty_job_id_is_invalid":
		r.JobId, want = "", rs.ErrInvalidJob
	case "empty_topic_records_nothing":
		r.Topic, want = "", rs.ErrInvalidJob
	case "long_ids_within_bound_are_accepted":
		r.JobId, r.Topic = strings.Repeat("j", 256), strings.Repeat("t", 256)
	case "oversized_job_id_records_nothing":
		r.JobId, want = strings.Repeat("j", 4097), rs.ErrInvalidJob
	case "oversized_topic_records_nothing":
		r.Topic, want = strings.Repeat("t", 4097), rs.ErrInvalidJob
	}
	f.clock.Advance(time.Second)
	j, e := s.Submit(f.packet(rs.SubjectSubmit, f.tenantA, f.clientA, r))
	requireErr(f.t, e, want)
	if want == rs.ErrInvalidJob {
		f.missing(s, f.tenantA, r.JobId)
		return
	}
	requireStatus(f.t, j, status)
	stored, e := s.Get(f.tenantA, r.JobId)
	requireErr(f.t, e, nil)
	requireJob(f.t, j, stored)
	require(f.t, !(j.TenantID != f.tenantA || j.JobID != r.JobId || j.Dispatch != nil), fmt.Sprintf("submit snapshot: %+v", j))
}

// B10: section 2.5 Dispatch/Tick, using only Config.Now (R6).
func deadlineCases(t *testing.T) {
	namedCases(t, "dispatch_timeout_boundary progress_replaces_dispatch_timeout_with_deadline budget_deadline_is_used dispatched_job_honours_run_deadline budget_deadline_is_capped run_timeout_is_capped scheduled_job_never_times_out tick_sorts_by_tenant_then_job result_after_timeout_is_refused", deadlineCheck, func(name string, c *rs.Config) {
		if name == "run_timeout_is_capped" {
			c.Limits.RunTimeout = 20 * time.Minute
		}
	})
}
func requireTickEmpty(f *fixture, s *rs.Scheduler) {
	f.t.Helper()
	before := f.get(s)
	if jobs := s.Tick(); len(jobs) != 0 {
		f.t.Fatalf("unexpected Tick changes: %+v", jobs)
	}
	f.requireUnchanged(s, before)
}
func requireTimeout(f *fixture, s *rs.Scheduler) {
	f.t.Helper()
	jobs := s.Tick()
	require(f.t, len(jobs) == 1, fmt.Sprintf("Tick: got %d changes; want 1", len(jobs)))
	requireStatus(f.t, jobs[0], agentv1.JobStatus_JOB_STATUS_TIMEOUT)
	requireJob(f.t, jobs[0], f.get(s))
}
func deadlineCheck(f *fixture, s *rs.Scheduler, name string) {
	if name == "tick_sorts_by_tenant_then_job" {
		sortedTimeouts(f, s)
		return
	}
	if name == "scheduled_job_never_times_out" {
		j := f.submit(s)
		f.clock.Advance(24 * time.Hour)
		requireTickEmpty(f, s)
		f.requireUnchanged(s, j)
		return
	}
	r := f.request(f.jobID)
	deadline := 5 * time.Minute
	switch name {
	case "budget_deadline_is_used":
		r.Budget = &agentv1.Budget{DeadlineMs: 60000}
		deadline = time.Minute
	case "dispatched_job_honours_run_deadline":
		r.Budget = &agentv1.Budget{DeadlineMs: 10000}
		deadline = 10 * time.Second
	case "budget_deadline_is_capped":
		r.Budget = &agentv1.Budget{DeadlineMs: 3600000}
		deadline = 10 * time.Minute
	case "run_timeout_is_capped":
		deadline = 10 * time.Minute
	}
	_, e := s.Submit(f.packet(rs.SubjectSubmit, f.tenantA, f.clientA, r))
	requireErr(f.t, e, nil)
	at := f.clock.Now()
	d, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
	requireErr(f.t, e, nil)
	j := f.get(s)
	require(f.t, j.Deadline.Equal(at.Add(deadline)) && j.DispatchedAt.Equal(at), fmt.Sprintf("dispatch deadline: %+v", j))
	if strings.HasSuffix(name, "is_capped") {
		return
	}
	deadlineTicks(f, s, name, d)
}
func deadlineTicks(f *fixture, s *rs.Scheduler, name string, d rs.Dispatch) {
	if name == "progress_replaces_dispatch_timeout_with_deadline" {
		f.clock.Advance(10 * time.Second)
		f.applyProgress(s, progress(f.jobID, d.Identity))
		f.clock.Advance(21 * time.Second)
		requireTickEmpty(f, s)
		f.clock.Advance(5*time.Minute - 31*time.Second - time.Nanosecond)
		requireTickEmpty(f, s)
		f.clock.Advance(time.Nanosecond)
		requireTimeout(f, s)
		return
	}
	if name == "budget_deadline_is_used" {
		f.clock.Advance(time.Second)
		f.applyProgress(s, progress(f.jobID, d.Identity))
		f.clock.Advance(59*time.Second - time.Nanosecond)
		requireTickEmpty(f, s)
		f.clock.Advance(time.Nanosecond)
		requireTimeout(f, s)
		return
	}
	wait := 30 * time.Second
	if name == "dispatched_job_honours_run_deadline" {
		wait = 10 * time.Second
	}
	f.clock.Advance(wait - time.Nanosecond)
	requireTickEmpty(f, s)
	f.clock.Advance(time.Nanosecond)
	requireTimeout(f, s)
	requireTickEmpty(f, s)
	if name == "result_after_timeout_is_refused" {
		before := f.get(s)
		f.refused(s, rs.ErrTerminal, func() error {
			j, e := s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerA, f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)))
			requireJob(f.t, j, before)
			return e
		})
	}
}
func sortedTimeouts(f *fixture, s *rs.Scheduler) {
	keys := []struct{ tenant, id, topic, worker string }{{f.tenantB, "job-a", f.topicB, f.workerB}, {f.tenantA, "job-z", f.topicA, f.workerA}, {f.tenantA, "job-b", f.topicA, f.workerA}}
	for _, k := range keys {
		r := f.request(k.id)
		r.TenantId = k.tenant
		r.Topic = k.topic
		_, e := s.Submit(f.packet(rs.SubjectSubmit, k.tenant, f.clientA, r))
		requireErr(f.t, e, nil)
		_, e = s.Dispatch(k.tenant, k.id, k.worker)
		requireErr(f.t, e, nil)
	}
	f.clock.Advance(30 * time.Second)
	jobs := s.Tick()
	require(f.t, len(jobs) == 3, fmt.Sprintf("Tick changed %d jobs; want 3", len(jobs)))
	for i, k := range []int{2, 1, 0} {
		require(f.t, !(jobs[i].TenantID != keys[k].tenant || jobs[i].JobID != keys[k].id), fmt.Sprintf("Tick sort at %d: %+v", i, jobs[i]))
		requireStatus(f.t, jobs[i], agentv1.JobStatus_JOB_STATUS_TIMEOUT)
		j, e := s.Get(keys[k].tenant, keys[k].id)
		requireErr(f.t, e, nil)
		requireJob(f.t, jobs[i], j)
	}
}

// B11: section 2.5 bare-sentinel error text, with canaries in all request surfaces.
func canaryCases(t *testing.T) {
	for _, tc := range []struct {
		name string
		want error
	}{
		{"invalid_config", rs.ErrInvalidConfig}, {"unverified", rs.ErrUnverified}, {"wrong_subject", rs.ErrWrongSubject}, {"wrong_payload", rs.ErrWrongPayload},
		{"invalid_job", rs.ErrInvalidJob}, {"denied", rs.ErrDenied}, {"input_too_large", rs.ErrInputTooLarge}, {"job_not_found", rs.ErrJobNotFound},
		{"job_conflict", rs.ErrJobConflict}, {"invalid_transition", rs.ErrInvalidTransition}, {"terminal", rs.ErrTerminal}, {"stale_attempt", rs.ErrStaleAttempt},
		{"wrong_worker", rs.ErrWrongWorker}, {"output_too_large", rs.ErrOutputTooLarge}, {"too_many_artifacts", rs.ErrTooManyArtifacts},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.tenantA, f.tenantB = "tenant-CANARY-7a", "tenant-CANARY-7b"
			f.workerA, f.workerB = "worker-CANARY-8a", "worker-CANARY-8b"
			f.clientA = "client-CANARY-9a"
			f.jobID, f.topicA, f.capability = "job-CANARY-0a", "topic.CANARY.1a", "cap.CANARY.2a"
			var err error
			if tc.name == "invalid_config" {
				c := f.config()
				c.Now = nil
				_, err = rs.New(c)
			} else {
				s := f.scheduler(nil)
				err = canaryCall(f, s, tc.name)
			}
			requireErr(t, err, tc.want)
			require(t, err.Error() == tc.want.Error(), fmt.Sprintf("wrapped error: %q", err))
			needles := append([]string{"CANARY", rs.SubjectSubmit, rs.SubjectResult, rs.SubjectProgress, rs.SubjectCancel, rs.SubjectClientCancel, rs.SubjectTerminal, "worker.", "0x", "%!"}, f.secrets...)
			for _, needle := range needles {
				require(t, !strings.Contains(err.Error(), needle), fmt.Sprintf("error leaks canary %q", needle))
			}
		})
	}
}
func canaryCall(f *fixture, s *rs.Scheduler, name string) error {
	r := f.request(f.jobID)
	switch name {
	case "unverified", "wrong_subject", "wrong_payload", "invalid_job", "denied", "input_too_large":
		subject := rs.SubjectSubmit
		var body proto.Message = r
		switch name {
		case "wrong_subject":
			subject = rs.SubjectProgress
		case "wrong_payload":
			body = &agentv1.JobResult{JobId: f.jobID, ResultPtr: "ptr-CANARY-3a", ErrorMessage: "message-CANARY-4a"}
		case "invalid_job":
			r.ContextRef = nil
		case "denied":
			r.Topic += ".denied"
		case "input_too_large":
			r.ContextRef.SizeBytes = 1025
		}
		p := f.envelope(subject, f.tenantA, f.clientA, body)
		var v capsdk.VerifiedProductionPacket
		if name == "unverified" {
			var e error
			v, e = f.tryPacket(p, "unsigned")
			requireErr(f.t, e, capsdk.ErrMissingSignature)
		} else {
			var e error
			v, e = f.tryPacket(p, "")
			requireErr(f.t, e, nil)
		}
		f.clock.Advance(time.Second)
		j, e := s.Submit(v)
		if name == "denied" || name == "input_too_large" {
			requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_DENIED)
			requireJob(f.t, j, f.get(s))
		} else {
			f.missing(s, f.tenantA, f.jobID)
		}
		return e
	}
	return canaryExisting(f, s, name)
}
func canaryExisting(f *fixture, s *rs.Scheduler, name string) error {
	d := f.run(s)
	r := f.result(d.Identity, agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
	if name == "terminal" {
		f.applyResult(s, r, nil, r.Status)
	}
	before := f.get(s)
	f.clock.Advance(time.Second)
	var e error
	switch name {
	case "job_not_found":
		_, e = s.Get(f.tenantB, f.jobID)
	case "job_conflict":
		req := f.request(f.jobID)
		req.Labels["canary"] = "different-CANARY"
		_, e = s.Submit(f.packet(rs.SubjectSubmit, f.tenantA, f.clientA, req))
	case "invalid_transition":
		_, e = s.Dispatch(f.tenantA, f.jobID, f.workerA)
	case "terminal":
		j, err := s.Cancel(f.packet(rs.SubjectClientCancel, f.tenantA, f.clientA, cancelOf(f.jobID)))
		e = err
		requireJob(f.t, j, before)
	case "stale_attempt":
		r.Dispatch.Attempt++
		_, e = s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerA, r))
	case "wrong_worker":
		_, e = s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerB, r))
	case "output_too_large":
		r.ResultRef.SizeBytes = 2049
		j, err := s.ApplyResult(f.packet(rs.SubjectResult, f.tenantA, f.workerA, r))
		requireStatus(f.t, j, agentv1.JobStatus_JOB_STATUS_FAILED)
		requireJob(f.t, j, f.get(s))
		return err
	case "too_many_artifacts":
		p := progress(f.jobID, d.Identity)
		p.ArtifactRefs = []*agentv1.ResourceRef{f.ref(1), f.ref(2), f.ref(3), f.ref(4)}
		_, e = s.ApplyProgress(f.packet(rs.SubjectProgress, f.tenantA, f.workerA, p))
	}
	f.requireUnchanged(s, before)
	return e
}

func require(t *testing.T, ok bool, message string) {
	t.Helper()
	if !ok {
		t.Fatal(message)
	}
}
