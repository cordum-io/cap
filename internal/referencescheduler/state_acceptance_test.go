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
	if s == nil {
		f.t.Fatal("New returned nil scheduler")
	}
	return s
}
func (f *fixture) ref(size uint64) *agentv1.ResourceRef {
	return &agentv1.ResourceRef{ResolverId: "nats-kv", Uri: "nats-kv://acceptance/" + f.tenantA + "/" + strings.Repeat("01", 32),
		Sha256: []byte(strings.Repeat("\x01", 32)), MediaType: "application/json", SizeBytes: size}
}
func (f *fixture) request(id string) *agentv1.JobRequest {
	return &agentv1.JobRequest{JobId: id, Topic: f.topicA, TenantId: f.tenantA,
		Meta: &agentv1.JobMetadata{Capability: f.capability}, ContextRef: f.ref(512)}
}
func progress(id string, d *agentv1.DispatchIdentity) *agentv1.JobProgress {
	return &agentv1.JobProgress{JobId: id, Status: agentv1.JobStatus_JOB_STATUS_RUNNING, Dispatch: proto.Clone(d).(*agentv1.DispatchIdentity)}
}
func (f *fixture) result(d *agentv1.DispatchIdentity, status agentv1.JobStatus) *agentv1.JobResult {
	return &agentv1.JobResult{JobId: f.jobID, Status: status, Dispatch: proto.Clone(d).(*agentv1.DispatchIdentity), ResultRef: f.ref(128)}
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
	if j.Status != want {
		t.Fatalf("status: %v; want %v", j.Status, want)
	}
}
func diffJob(a, b rs.Job) string {
	if a.TenantID != b.TenantID || a.JobID != b.JobID || a.Topic != b.Topic || a.Capability != b.Capability {
		return "identity"
	}
	if a.Status != b.Status {
		return "status"
	}
	if a.Acked != b.Acked {
		return "acked"
	}
	if !proto.Equal(a.Dispatch, b.Dispatch) {
		return "dispatch"
	}
	if !proto.Equal(a.Result, b.Result) {
		return "result"
	}
	if len(a.Artifacts) != len(b.Artifacts) {
		return "artifact count"
	}
	for i := range a.Artifacts {
		if !proto.Equal(a.Artifacts[i], b.Artifacts[i]) {
			return "artifact"
		}
	}
	if !a.Deadline.Equal(b.Deadline) {
		return "deadline"
	}
	if !a.DispatchedAt.Equal(b.DispatchedAt) {
		return "dispatched at"
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return "updated at"
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
				if v.IsVerified() {
					t.Fatal("rejected packet is verified")
				}
				return
			}
			if !v.IsVerified() || v.Subject() != rs.SubjectSubmit || v.Tenant() != f.tenantA || v.Sender() != f.clientA {
				t.Fatal("wrong verified authority")
			}
			got := v.Packet()
			got.Signature = nil
			if !proto.Equal(got, p) {
				t.Fatal("verified payload differs")
			}
		})
	}
	t.Run("zero_value_is_unverified", func(t *testing.T) {
		if (capsdk.VerifiedProductionPacket{}).IsVerified() {
			t.Fatal("zero proof is verified")
		}
	})
	t.Run("fake_clock_is_exact", func(t *testing.T) {
		f := newFixture(t)
		at := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
		if !f.clock.Now().Equal(at) {
			t.Fatal("clock origin")
		}
		f.clock.Advance(time.Second + time.Nanosecond)
		if !f.clock.Now().Equal(at.Add(time.Second + time.Nanosecond)) {
			t.Fatal("clock advance")
		}
	})
}

func TestStateCoreAcceptance(t *testing.T) {
	t.Run("B00_config_fails_closed", configCases)
	t.Run("B01_verified_packets_only", admissionCases)
	t.Run("B02_tenant_and_subject_isolation", isolationCases)
	t.Run("B03_correlation_reserved_before_publish", dispatchCases)
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
			if tc.want == nil && s == nil {
				t.Fatal("nil scheduler")
			}
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
		r.JobId = "job-4"
		want = rs.ErrInvalidJob
	case "empty_tenant_mirror_is_accepted":
		r.TenantId = ""
		want = nil
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
				if got[i] != want[i] {
					f.t.Fatalf("subject %d: %q; want %q", i, got[i], want[i])
				}
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
			if !rx.MatchString(d1.Identity.GetDispatchId()) || !rx.MatchString(d2.Identity.GetDispatchId()) || d1.Identity.GetDispatchId() == d2.Identity.GetDispatchId() {
				f.t.Fatal("default dispatch IDs must be distinct 128-bit lowercase hex")
			}
		}, func(c *rs.Config) { c.NewDispatchID = nil }},
		{"failing_dispatch_id_source_reserves_nothing", func(f *fixture, s *rs.Scheduler) {
			before := f.submit(s)
			f.clock.Advance(time.Second)
			_, e := s.Dispatch(f.tenantA, f.jobID, f.workerA)
			if e == nil {
				f.t.Fatal("ID source failure accepted")
			}
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
	if d.TenantID != f.tenantA || d.JobID != f.jobID || d.Subject != "worker.worker-a.jobs" || d.Subject != rs.WorkerSubject(f.workerA) || !proto.Equal(d.Identity, identity) || !proto.Equal(d.Request, req) {
		f.t.Fatalf("dispatch: %+v", d)
	}
	want := rs.Job{TenantID: f.tenantA, JobID: f.jobID, Topic: f.topicA, Capability: f.capability, Status: agentv1.JobStatus_JOB_STATUS_DISPATCHED,
		Dispatch: identity, DispatchedAt: at, Deadline: at.Add(5 * time.Minute), UpdatedAt: at}
	requireJob(f.t, f.get(s), want)
}
