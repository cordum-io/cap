// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRunTimeoutCapsWithoutOverflow(t *testing.T) {
	s := &Scheduler{limits: Limits{RunTimeout: 5 * time.Minute, MaxRunTimeout: 10 * time.Minute}}
	for _, tc := range []struct {
		ms   int64
		want time.Duration
	}{{0, 5 * time.Minute}, {-1, 5 * time.Minute}, {1, time.Millisecond}, {599999, 599999 * time.Millisecond},
		{600000, 10 * time.Minute}, {math.MaxInt64, 10 * time.Minute}} {
		if got := s.runTimeout(tc.ms); got != tc.want {
			t.Errorf("runTimeout(%d) = %v; want %v", tc.ms, got, tc.want)
		}
	}
	s.limits.RunTimeout = 20 * time.Minute
	if got := s.runTimeout(0); got != 10*time.Minute {
		t.Errorf("RunTimeout above MaxRunTimeout: %v", got)
	}
	s.limits.MaxRunTimeout = 500 * time.Microsecond
	if got := s.runTimeout(1); got != 500*time.Microsecond {
		t.Errorf("sub-millisecond cap: %v", got)
	}
}

func TestFenceAdmitsOnlyTheHoldingDispatch(t *testing.T) {
	d := &agentv1.DispatchIdentity{DispatchId: "d1", Attempt: 1, AssignedWorkerId: "w1"}
	job := &Job{Status: agentv1.JobStatus_JOB_STATUS_RUNNING, Dispatch: d}
	with := func(change func(*agentv1.DispatchIdentity)) *agentv1.DispatchIdentity {
		e := clone(d)
		change(e)
		return e
	}
	for _, tc := range []struct {
		name   string
		job    *Job
		event  *agentv1.DispatchIdentity
		sender string
		want   error
	}{
		{"holding_dispatch", job, d, "w1", nil},
		{"other_dispatch_id", job, with(func(e *agentv1.DispatchIdentity) { e.DispatchId = "d2" }), "w1", ErrStaleAttempt},
		{"future_attempt", job, with(func(e *agentv1.DispatchIdentity) { e.Attempt = 2 }), "w1", ErrStaleAttempt},
		{"no_identity", job, nil, "w1", ErrStaleAttempt},
		{"other_sender", job, d, "w2", ErrWrongWorker},
		{"other_assigned_worker", job, with(func(e *agentv1.DispatchIdentity) { e.AssignedWorkerId = "w2" }), "w1", ErrWrongWorker},
		{"rescheduled_job_holds_no_dispatch", &Job{Status: agentv1.JobStatus_JOB_STATUS_SCHEDULED, Dispatch: d}, d, "w1", ErrStaleAttempt},
		{"never_dispatched", &Job{Status: agentv1.JobStatus_JOB_STATUS_DENIED}, &agentv1.DispatchIdentity{}, "w1", ErrWrongWorker},
	} {
		if err := fence(tc.job, tc.event, tc.sender); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v; want %v", tc.name, err, tc.want)
		}
	}
}

func scheduledStore() (*store, jobKey) {
	k := jobKey{"t1", "j1"}
	st := &store{jobs: map[jobKey]*record{}}
	st.jobs[k] = &record{job: Job{TenantID: "t1", JobID: "j1", Status: agentv1.JobStatus_JOB_STATUS_SCHEDULED},
		request: &agentv1.JobRequest{JobId: "j1", Topic: "topic"}}
	return st, k
}

func scheduledScheduler(newID func() (string, error)) (*Scheduler, jobKey) {
	st, k := scheduledStore()
	return &Scheduler{limits: Limits{RunTimeout: time.Minute, MaxRunTimeout: time.Minute}, now: time.Now, newID: newID,
		store: store{jobs: st.jobs}}, k
}

func TestStoreUpdateSwapsOnlyOnCommit(t *testing.T) {
	st, k := scheduledStore()
	before := st.jobs[k]
	refuse := func(err error) func(*Job, *agentv1.JobRequest) (bool, error) {
		return func(j *Job, _ *agentv1.JobRequest) (bool, error) {
			j.Status = agentv1.JobStatus_JOB_STATUS_RUNNING
			return false, err
		}
	}
	if j, err := st.update(k, refuse(ErrStaleAttempt)); err != ErrStaleAttempt || j.JobID != "" || st.jobs[k] != before {
		t.Fatalf("refusal: %+v %v; record replaced %v", j, err, st.jobs[k] != before)
	}
	if j, err := st.update(k, refuse(ErrTerminal)); err != ErrTerminal || j.Status != agentv1.JobStatus_JOB_STATUS_SCHEDULED || st.jobs[k] != before {
		t.Fatalf("ErrTerminal must return the stored snapshot unchanged: %+v %v", j, err)
	}
	called := false
	if _, err := st.update(jobKey{"t2", "j1"}, func(*Job, *agentv1.JobRequest) (bool, error) { called = true; return true, nil }); err != ErrJobNotFound || called {
		t.Fatalf("other tenant: %v, apply called %v", err, called)
	}
	j, err := st.update(k, func(j *Job, _ *agentv1.JobRequest) (bool, error) {
		j.Status = agentv1.JobStatus_JOB_STATUS_CANCELLED
		return true, nil
	})
	if err != nil || j.Status != agentv1.JobStatus_JOB_STATUS_CANCELLED || st.jobs[k] == before || before.job.Status != agentv1.JobStatus_JOB_STATUS_SCHEDULED {
		t.Fatalf("commit: %+v %v", j, err)
	}
}

func TestStoreInsertIsIdempotentOrConflicts(t *testing.T) {
	st, k := scheduledStore()
	same := &record{job: Job{Status: agentv1.JobStatus_JOB_STATUS_DENIED}, request: &agentv1.JobRequest{JobId: "j1", Topic: "topic"}}
	if j, inserted, err := st.insert(k, same); err != nil || inserted || j.Status != agentv1.JobStatus_JOB_STATUS_SCHEDULED {
		t.Fatalf("duplicate: %+v %v %v", j, inserted, err)
	}
	other := &record{request: &agentv1.JobRequest{JobId: "j1", Topic: "other"}}
	if _, inserted, err := st.insert(k, other); err != ErrJobConflict || inserted || st.jobs[k].request.GetTopic() != "topic" {
		t.Fatalf("conflict: %v %v", inserted, err)
	}
}

func TestStoreSweepSortsChangedJobs(t *testing.T) {
	st := &store{jobs: map[jobKey]*record{}}
	for _, k := range []jobKey{{"b", "1"}, {"a", "2"}, {"a", "1"}, {"c", "0"}} {
		st.jobs[k] = &record{job: Job{TenantID: k.tenant, JobID: k.job, Status: agentv1.JobStatus_JOB_STATUS_RUNNING}}
	}
	got := st.sweep(func(j *Job) bool { return j.TenantID != "c" }, func(j *Job) { j.Status = agentv1.JobStatus_JOB_STATUS_TIMEOUT })
	var keys []string
	for _, j := range got {
		keys = append(keys, j.TenantID+"/"+j.JobID)
	}
	if strings.Join(keys, ",") != "a/1,a/2,b/1" || st.jobs[jobKey{"c", "0"}].job.Status != agentv1.JobStatus_JOB_STATUS_RUNNING {
		t.Fatalf("sweep order %v", keys)
	}
}

func TestCloneJobIsDeep(t *testing.T) {
	orig := Job{Dispatch: &agentv1.DispatchIdentity{DispatchId: "d1"}, Result: &agentv1.JobResult{JobId: "j1"},
		Artifacts: []*agentv1.ResourceRef{{Uri: "u1"}}}
	c := cloneJob(orig)
	c.Dispatch.DispatchId, c.Result.JobId, c.Artifacts[0].Uri = "x", "x", "x"
	c.Artifacts = append(c.Artifacts, &agentv1.ResourceRef{})
	if orig.Dispatch.DispatchId != "d1" || orig.Result.JobId != "j1" || orig.Artifacts[0].Uri != "u1" || len(orig.Artifacts) != 1 {
		t.Fatalf("clone shares memory: %+v", orig)
	}
	if empty := cloneJob(Job{}); empty.Dispatch != nil || empty.Result != nil || empty.Artifacts != nil {
		t.Fatalf("nil fields must stay nil: %+v", empty)
	}
}

func TestDecideAuthorsTerminalResult(t *testing.T) {
	j := &Job{JobID: "j1", Dispatch: &agentv1.DispatchIdentity{DispatchId: "d1", Attempt: 2}}
	for code, legacy := range map[agentv1.ErrorCode]string{agentv1.ErrorCode_ERROR_CODE_JOB_TIMEOUT: "TIMEOUT",
		agentv1.ErrorCode_ERROR_CODE_JOB_PERMISSION_DENIED: "PERMISSION_DENIED", agentv1.ErrorCode_ERROR_CODE_JOB_RESOURCE_EXHAUSTED: ""} {
		decide(j, agentv1.JobStatus_JOB_STATUS_TIMEOUT, code)
		want := &agentv1.JobResult{JobId: "j1", Status: agentv1.JobStatus_JOB_STATUS_TIMEOUT, ErrorCodeEnum: code, ErrorCode: legacy, Dispatch: j.Dispatch}
		if j.Status != agentv1.JobStatus_JOB_STATUS_TIMEOUT || !proto.Equal(j.Result, want) || j.Result.Dispatch == j.Dispatch {
			t.Fatalf("decide(%v): %v", code, j.Result)
		}
	}
}

func TestDispatchIDSource(t *testing.T) {
	rx := regexp.MustCompile("^[0-9a-f]{32}$")
	a, errA := randomDispatchID()
	b, errB := randomDispatchID()
	if errA != nil || errB != nil || !rx.MatchString(a) || !rx.MatchString(b) || a == b {
		t.Fatalf("random dispatch IDs %q %q", a, b)
	}
	for name, source := range map[string]func() (string, error){
		"error": func() (string, error) { return "", errors.New("source down") },
		"empty": func() (string, error) { return "", nil }, "oversized": func() (string, error) { return strings.Repeat("d", 257), nil },
	} {
		s, k := scheduledScheduler(source)
		before := s.store.jobs[k]
		if _, err := s.Dispatch(k.tenant, k.job, "w1"); err != ErrInvalidConfig || s.store.jobs[k] != before {
			t.Fatalf("%s source: %v; record replaced %v", name, err, s.store.jobs[k] != before)
		}
	}
}

func TestIdentifierAndRequestBounds(t *testing.T) {
	s, k := scheduledScheduler(randomDispatchID)
	if _, err := s.Dispatch(k.tenant, k.job, strings.Repeat("w", 257)); err != ErrInvalidJob {
		t.Fatalf("oversized worker ID: %v", err)
	}
	id := &agentv1.IdentityBinding{TenantId: "t1", PrincipalId: "p1"}
	for _, tc := range []struct {
		name   string
		change func(*agentv1.JobRequest)
		want   bool
	}{
		{"valid", func(*agentv1.JobRequest) {}, true}, {"job_id_at_bound", func(r *agentv1.JobRequest) { r.JobId = strings.Repeat("j", 256) }, true},
		{"job_id_over_bound", func(r *agentv1.JobRequest) { r.JobId = strings.Repeat("j", 257) }, false},
		{"topic_over_bound", func(r *agentv1.JobRequest) { r.Topic = strings.Repeat("t", 257) }, false},
		{"no_context_ref", func(r *agentv1.JobRequest) { r.ContextRef = nil }, false},
		{"principal_mirror_matches", func(r *agentv1.JobRequest) { r.PrincipalId = "p1" }, true},
		{"principal_mirror_differs", func(r *agentv1.JobRequest) { r.PrincipalId = "p2" }, false},
		{"meta_tenant_differs", func(r *agentv1.JobRequest) { r.Meta.TenantId = "t2" }, false},
		{"env_tenant_differs", func(r *agentv1.JobRequest) { r.Env = map[string]string{"tenant_id": "t2"} }, false},
		{"nested_identity_partial", func(r *agentv1.JobRequest) { r.Identity = &agentv1.IdentityBinding{TenantId: "t1"} }, false},
	} {
		r := &agentv1.JobRequest{JobId: "j1", Topic: "topic", Meta: &agentv1.JobMetadata{Capability: "c"}, ContextRef: &agentv1.ResourceRef{SizeBytes: 1}}
		tc.change(r)
		if got := validRequest(r, id); got != tc.want {
			t.Errorf("%s: %v; want %v", tc.name, got, tc.want)
		}
	}
}

func TestCallbacksRunOutsideTheLock(t *testing.T) {
	var s *Scheduler
	reenter := func() { _, _ = s.Get("t1", "j1") } // deadlocks if the store mutex is held
	s, k := scheduledScheduler(func() (string, error) { reenter(); return "d1", nil })
	s.now = func() time.Time { reenter(); return time.Unix(0, 0) }
	done := make(chan error, 1)
	go func() {
		_, err := s.Dispatch(k.tenant, k.job, "w1")
		s.Tick()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a callback ran while the store mutex was held")
	}
}

func TestConcurrentDispatchReservesOnce(t *testing.T) {
	var n atomic.Int64
	s, k := scheduledScheduler(func() (string, error) { return fmt.Sprintf("d%d", n.Add(1)), nil })
	errs := make(chan error, 16)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Dispatch(k.tenant, k.job, "w1"); errs <- err }()
	}
	wg.Wait()
	close(errs)
	won := 0
	for err := range errs {
		if err == nil {
			won++
		} else if err != ErrInvalidTransition {
			t.Fatalf("Dispatch: %v", err)
		}
	}
	j, err := s.Get(k.tenant, k.job)
	if err != nil || won != 1 || j.Dispatch.GetAttempt() != 1 || j.Status != agentv1.JobStatus_JOB_STATUS_DISPATCHED {
		t.Fatalf("won %d, job %+v, err %v", won, j, err)
	}
}

// signed returns a verified packet carrying payload, signed with a fresh key.
func signed(t *testing.T, subject, tenant, sender string, set func(*agentv1.BusPacket)) capsdk.VerifiedProductionPacket {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	p := &agentv1.BusPacket{TraceId: hex.EncodeToString(id), SenderId: sender, ProtocolVersion: capsdk.DefaultProtocolVersion,
		Identity: &agentv1.IdentityBinding{TenantId: tenant}, SignatureMetadata: &agentv1.SignatureMetadata{
			ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm, MessageId: id,
			Audience: subject, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)), KeyId: "k1"}}
	set(p)
	raw, err := capsdk.SignProductionPacket(p, key)
	if err != nil {
		t.Fatal(err)
	}
	v, err := capsdk.VerifyTrustedProductionPacket(raw, capsdk.ProductionTrustStore{Audience: subject, Tenant: tenant,
		Sender: sender, PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey}})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestResultArtifactsAreBoundedWhole(t *testing.T) {
	at := time.Unix(1_900_000_000, 0)
	s, err := New(Config{Limits: Limits{MaxInputBytes: 10, MaxOutputBytes: 10, MaxArtifacts: 2, MaxAttempts: 1,
		DispatchTimeout: time.Minute, RunTimeout: time.Minute, MaxRunTimeout: time.Minute},
		Allow: []AllowRule{{TenantID: "t1", Topic: "topic", Capability: "c"}}, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	req := &agentv1.JobRequest{JobId: "j1", Topic: "topic", Meta: &agentv1.JobMetadata{Capability: "c"}, ContextRef: &agentv1.ResourceRef{SizeBytes: 1}}
	if _, err := s.Submit(signed(t, SubjectSubmit, "t1", "client", func(p *agentv1.BusPacket) {
		p.Payload = &agentv1.BusPacket_JobRequest{JobRequest: req}
	})); err != nil {
		t.Fatal(err)
	}
	d, err := s.Dispatch("t1", "j1", "w1")
	if err != nil {
		t.Fatal(err)
	}
	result := func(n int) capsdk.VerifiedProductionPacket {
		r := &agentv1.JobResult{JobId: "j1", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED, Dispatch: d.Identity}
		for i := range n {
			r.ArtifactRefs = append(r.ArtifactRefs, &agentv1.ResourceRef{Uri: fmt.Sprint("a", i)})
		}
		return signed(t, SubjectResult, "t1", "w1", func(p *agentv1.BusPacket) { p.Payload = &agentv1.BusPacket_JobResult{JobResult: r} })
	}
	if _, err := s.ApplyResult(result(3)); err != ErrTooManyArtifacts {
		t.Fatalf("3 artifacts past MaxArtifacts 2: %v", err)
	}
	if j, _ := s.Get("t1", "j1"); j.Status != agentv1.JobStatus_JOB_STATUS_DISPATCHED || len(j.Artifacts) != 0 {
		t.Fatalf("refused result changed the job: %+v", j)
	}
	if j, err := s.ApplyResult(result(2)); err != nil || j.Status != agentv1.JobStatus_JOB_STATUS_SUCCEEDED || len(j.Artifacts) != 2 {
		t.Fatalf("2 artifacts: %+v %v", j, err)
	}
}

// dispatchedJ1 returns a scheduler holding job j1 of tenant t1, dispatched to w1.
func dispatchedJ1(t *testing.T, attempts uint64) (*Scheduler, Dispatch) {
	t.Helper()
	at := time.Unix(1_900_000_000, 0)
	s, err := New(Config{Limits: Limits{MaxInputBytes: 10, MaxOutputBytes: 10, MaxArtifacts: 4, MaxAttempts: attempts,
		DispatchTimeout: time.Minute, RunTimeout: time.Minute, MaxRunTimeout: time.Minute},
		Allow: []AllowRule{{TenantID: "t1", Topic: "topic", Capability: "c"}}, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	req := &agentv1.JobRequest{JobId: "j1", Topic: "topic", Meta: &agentv1.JobMetadata{Capability: "c"}, ContextRef: &agentv1.ResourceRef{SizeBytes: 1}}
	if _, err := s.Submit(signed(t, SubjectSubmit, "t1", "client", func(p *agentv1.BusPacket) {
		p.Payload = &agentv1.BusPacket_JobRequest{JobRequest: req}
	})); err != nil {
		t.Fatal(err)
	}
	d, err := s.Dispatch("t1", "j1", "w1")
	if err != nil {
		t.Fatal(err)
	}
	return s, d
}

// resultFrom returns a verified copy of r sent by sender.
func resultFrom(t *testing.T, sender string, r *agentv1.JobResult) capsdk.VerifiedProductionPacket {
	t.Helper()
	return signed(t, SubjectResult, "t1", sender, func(p *agentv1.BusPacket) {
		p.Payload = &agentv1.BusPacket_JobResult{JobResult: proto.Clone(r).(*agentv1.JobResult)}
	})
}

// Rule: the output bound is checked before the retry rule.
func TestApplyResultOutputBoundBeatsRetry(t *testing.T) {
	s, d := dispatchedJ1(t, 3)
	r := &agentv1.JobResult{JobId: "j1", Status: agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE, Dispatch: d.Identity,
		ResultRef: &agentv1.ResourceRef{SizeBytes: 11}}
	j, err := s.ApplyResult(resultFrom(t, "w1", r))
	if err != ErrOutputTooLarge || j.Status != agentv1.JobStatus_JOB_STATUS_FAILED ||
		j.Result.GetErrorCodeEnum() != agentv1.ErrorCode_ERROR_CODE_JOB_RESOURCE_EXHAUSTED {
		t.Fatalf("oversized retryable result: %+v %v", j, err)
	}
	if got, _ := s.Get("t1", "j1"); got.Status != agentv1.JobStatus_JOB_STATUS_FAILED ||
		got.Result.GetErrorCodeEnum() != agentv1.ErrorCode_ERROR_CODE_JOB_RESOURCE_EXHAUSTED {
		t.Fatalf("stored job: %+v", got)
	}
}

// Rule: a repeat on a terminal job is a no-op only when the fence also passes.
func TestApplyResultTerminalRepeatNeedsFence(t *testing.T) {
	s, d := dispatchedJ1(t, 1)
	r := &agentv1.JobResult{JobId: "j1", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED, Dispatch: d.Identity}
	if j, err := s.ApplyResult(resultFrom(t, "w1", r)); err != nil || j.Status != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("winner: %+v %v", j, err)
	}
	if _, err := s.ApplyResult(resultFrom(t, "w1", r)); err != nil {
		t.Fatalf("winner's repeat from w1: %v", err)
	}
	if _, err := s.ApplyResult(resultFrom(t, "w2", r)); err != ErrTerminal {
		t.Fatalf("identical result from w2: %v", err)
	}
	if j, _ := s.Get("t1", "j1"); j.Status != agentv1.JobStatus_JOB_STATUS_SUCCEEDED || !proto.Equal(j.Result, r) {
		t.Fatalf("winner changed: %+v", j)
	}
}
