package a2abridge

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

var storeAlice = Identity{Tenant: "tenant-a", Principal: "alice"}

func testRecord(id string) taskRecord {
	text := "hi"
	return taskRecord{
		id: id, contextID: "ctx-" + id, jobID: "a2a-job-" + id, messageID: "m-1",
		turn: 1, state: a2a.TaskStateSubmitted, created: fixedTime,
		parts: []turnPart{
			{Kind: "text", Text: &text},
			{Kind: "data", Data: json.RawMessage(`{"a":1}`), MediaType: "application/json"},
		},
	}
}

func digestOf(s string) [32]byte { return sha256.Sum256([]byte(s)) }

func mustReserve(t *testing.T, s *store, id Identity, key string, rec taskRecord) taskRecord {
	t.Helper()
	snap, created, err := s.reserve(id, key, digestOf("parts"), rec)
	if err != nil || !created {
		t.Fatalf("reserve %s: created %v err %v", key, created, err)
	}
	return snap
}

func TestStoreCapacityRejectsWithoutEviction(t *testing.T) {
	s := newStore(2, 2, time.Hour)
	mustReserve(t, s, storeAlice, "k0", testRecord("t0"))
	mustReserve(t, s, storeAlice, "k1", testRecord("t1"))
	_, created, err := s.reserve(storeAlice, "k2", digestOf("parts"), testRecord("t2"))
	if created || !errors.Is(err, a2a.ErrInternalError) {
		t.Fatalf("third reserve: created %v err %v, want internal error", created, err)
	}
	if s.size() != 2 || !s.exists(storeAlice, "t0") || !s.exists(storeAlice, "t1") || s.exists(storeAlice, "t2") {
		t.Fatalf("store after full reserve: size %d", s.size())
	}
	// A duplicate of an existing turn is still answered when the store is full.
	snap, created, err := s.reserve(storeAlice, "k0", digestOf("parts"), testRecord("t9"))
	if err != nil || created || snap.id != "t0" {
		t.Fatalf("duplicate on full store: %v %v %q", err, created, snap.id)
	}
}

func TestCapacityRejectsNewTaskWithoutPublishing(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.MaxTasks = 4 }) // two tenants: quota 2
	first := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	h.mustSend(t, as("key-alice"), textMsg("m-2", "hi"))
	_, err := h.send(as("key-alice"), textMsg("m-3", "hi"))
	wantErr(t, err, a2a.ErrInternalError)
	if h.pub.count() != 2 || h.pub.attempts() != 2 || h.in.writeCount() != 2 || h.ing.store.size() != 2 {
		t.Fatalf("publishes %d attempts %d writes %d store %d, want 2", h.pub.count(), h.pub.attempts(), h.in.writeCount(), h.ing.store.size())
	}
	if _, err := h.ing.GetTask(as("key-alice"), &a2a.GetTaskRequest{ID: first.ID}); err != nil {
		t.Fatalf("first task evicted: %v", err)
	}
}

func TestTenantQuotaRefusesOnlyThatTenant(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.MaxTasks = 4 }) // quota 2 per tenant
	first := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi"))
	h.mustSend(t, as("key-bob"), textMsg("m-1", "hi")) // bob shares tenant-a's quota
	_, err := h.send(as("key-alice"), textMsg("m-2", "hi"))
	wantErr(t, err, a2a.ErrInternalError)
	if h.pub.attempts() != 2 || h.in.writeCount() != 2 || h.ing.store.size() != 2 {
		t.Fatalf("at quota: attempts %d writes %d store %d, want 2", h.pub.attempts(), h.in.writeCount(), h.ing.store.size())
	}
	// A duplicate is still answered at quota, and tenant-b still creates a task.
	if dup := h.mustSend(t, as("key-alice"), textMsg("m-1", "hi")); dup.ID != first.ID {
		t.Fatalf("duplicate at quota returned %q, want %q", dup.ID, first.ID)
	}
	h.mustSend(t, as("key-mallory"), textMsg("m-1", "hi"))
	if h.pub.count() != 3 || h.in.writeCount() != 3 || h.ing.store.size() != 3 {
		t.Fatalf("tenant-b: publishes %d writes %d store %d, want 3", h.pub.count(), h.in.writeCount(), h.ing.store.size())
	}
}

// reserveAt reserves a fresh task id under key at now.
func reserveAt(s *store, key, id string, now time.Time) (taskRecord, bool, error) {
	rec := testRecord(id)
	rec.created = now
	return s.reserve(storeAlice, key, digestOf("parts"), rec)
}

func TestStoreEvictsTerminalTaskAtTTL(t *testing.T) {
	s := newStore(4, 4, time.Hour)
	mustReserve(t, s, storeAlice, "k0", testRecord("t0"))
	statusAt := fixedTime.Add(time.Minute)
	if !s.applyResult("a2a-job-t0", agentv1.JobStatus_JOB_STATUS_FAILED, statusAt) {
		t.Fatal("applyResult did not end the task")
	}
	snap, created, err := reserveAt(s, "k0", "t1", statusAt.Add(time.Hour-time.Nanosecond))
	if err != nil || created || snap.id != "t0" || !s.exists(storeAlice, "t0") {
		t.Fatalf("duplicate inside the window: err %v created %v id %q", err, created, snap.id)
	}
	snap, created, err = reserveAt(s, "k0", "t1", statusAt.Add(time.Hour))
	if err != nil || !created || snap.id != "t1" || s.exists(storeAlice, "t0") {
		t.Fatalf("reserve at the TTL: err %v created %v id %q", err, created, snap.id)
	}
	for k, ref := range s.turns {
		if ref.key.id == "t0" {
			t.Fatalf("turns[%s] still names the evicted task", k)
		}
	}
	for j, ref := range s.jobs {
		if ref.key.id == "t0" {
			t.Fatalf("jobs[%s] still names the evicted task", j)
		}
	}
	if s.size() != 1 || s.tenants[storeAlice.Tenant] != 1 {
		t.Fatalf("size %d tenant count %d, want 1 and 1", s.size(), s.tenants[storeAlice.Tenant])
	}
}

func TestStoreNeverEvictsNonTerminalTasks(t *testing.T) {
	s := newStore(8, 8, time.Hour)
	want := map[string]a2a.TaskState{"t0": a2a.TaskStateSubmitted, "t1": a2a.TaskStateWorking,
		"t2": a2a.TaskStateInputRequired, "t3": a2a.TaskStateAuthRequired}
	for id, state := range want {
		mustReserve(t, s, storeAlice, "k-"+id, testRecord(id))
		job := "a2a-job-" + id
		switch state {
		case a2a.TaskStateWorking:
			s.applyProgress(job, agentv1.JobStatus_JOB_STATUS_RUNNING, fixedTime)
		case a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired:
			turn, ok := s.reserveResult(job)
			if !ok {
				t.Fatalf("reserveResult %s", job)
			}
			s.commitResult(job, turn, &turnOutcome{state: state}, fixedTime)
		}
	}
	if _, _, err := reserveAt(s, "k-late", "late", fixedTime.Add(10*time.Hour)); err != nil {
		t.Fatalf("late reserve: %v", err)
	}
	for id, state := range want {
		if rec, ok := s.get(storeAlice, id); !ok || rec.state != state {
			t.Fatalf("%s: found %v state %v, want %v", id, ok, rec.state, state)
		}
	}
}

func TestSendMessageDedupIsBoundedByTerminalTTL(t *testing.T) {
	now := fixedTime
	h := newHarness(t, func(c *Config) {
		c.Now = func() time.Time { return now }
		c.TerminalTaskTTL = time.Minute
	})
	alice := as("key-alice")
	old := h.mustSend(t, alice, textMsg("m-1", "hi"))
	oldJob := h.pub.packets(t)[0].GetJobRequest().GetJobId()
	oldInput, _ := h.in.get("ctx:" + oldJob)
	now = now.Add(time.Second)
	if !h.ing.store.applyResult(oldJob, agentv1.JobStatus_JOB_STATUS_FAILED, now) {
		t.Fatal("applyResult did not end the task")
	}
	// Inside the window the duplicate is the same task, with no input write or publish.
	now = now.Add(time.Minute - time.Nanosecond)
	if dup := h.mustSend(t, alice, textMsg("m-1", "hi")); dup.ID != old.ID || h.pub.attempts() != 1 || h.in.writeCount() != 1 {
		t.Fatalf("in-window duplicate: task %q attempts %d writes %d", dup.ID, h.pub.attempts(), h.in.writeCount())
	}
	// After the TTL the same messageId is accepted as a new task with a new job id.
	now = now.Add(time.Nanosecond)
	fresh := h.mustSend(t, alice, textMsg("m-1", "hi"))
	pkts := h.pub.packets(t)
	if fresh.ID == old.ID || len(pkts) != 2 {
		t.Fatalf("post-expiry retry: task %q (old %q) publishes %d", fresh.ID, old.ID, len(pkts))
	}
	newJob := pkts[1].GetJobRequest().GetJobId()
	if newJob == oldJob || newJob != refJob(string(fresh.ID), refKey("tenant-a", "alice", "", "m-1")) {
		t.Fatalf("post-expiry job id %q, old %q", newJob, oldJob)
	}
	if got, _ := h.in.get("ctx:" + oldJob); string(got) != string(oldInput) {
		t.Fatalf("ctx:%s was rewritten\n got %s\nwant %s", oldJob, got, oldInput)
	}
	_, err := h.ing.GetTask(alice, &a2a.GetTaskRequest{ID: old.ID})
	wantErr(t, err, a2a.ErrTaskNotFound)
	if h.ing.store.applyResult(oldJob, agentv1.JobStatus_JOB_STATUS_CANCELLED, now) {
		t.Fatal("a late result for the evicted job changed a task")
	}
	if got, err := h.ing.GetTask(alice, &a2a.GetTaskRequest{ID: fresh.ID}); err != nil || got.Status.State != a2a.TaskStateSubmitted {
		t.Fatalf("new task after the late result: %v %v", got, err)
	}
}

func TestStoreGetReturnsCopies(t *testing.T) {
	s := newStore(4, 4, time.Hour)
	rec := testRecord("t0")
	mustReserve(t, s, storeAlice, "k0", rec)
	// Mutating the caller's record after reserve must not reach the store.
	*rec.parts[0].Text = "mutated-input"
	rec.parts[1].Data[1] = 'X'
	rec.parts = nil

	snap, ok := s.get(storeAlice, "t0")
	if !ok {
		t.Fatal("get: not found")
	}
	*snap.parts[0].Text = "mutated-snapshot"
	snap.parts[1].Data[1] = 'Y'
	snap.parts[0] = turnPart{Kind: "evil"}
	snap.state = a2a.TaskStateCompleted

	again, _ := s.get(storeAlice, "t0")
	if len(again.parts) != 2 || again.parts[0].Kind != "text" || *again.parts[0].Text != "hi" ||
		string(again.parts[1].Data) != `{"a":1}` || again.state != a2a.TaskStateSubmitted {
		t.Fatalf("store was mutated through a copy: %+v", again)
	}
}

func TestStoreReserveReleaseReserve(t *testing.T) {
	s := newStore(4, 4, time.Hour)
	mustReserve(t, s, storeAlice, "k0", testRecord("t0"))
	s.release("k0")
	if s.size() != 0 || s.exists(storeAlice, "t0") || len(s.tenants) != 0 {
		t.Fatalf("release left the task or its quota slot behind: tenants %v", s.tenants)
	}
	snap := mustReserve(t, s, storeAlice, "k0", testRecord("t1"))
	if snap.id != "t1" || !s.exists(storeAlice, "t1") {
		t.Fatalf("re-reserve returned %q", snap.id)
	}
	s.release("unknown") // a no-op
	if s.size() != 1 {
		t.Fatalf("size %d after releasing an unknown key", s.size())
	}
}

func TestStoreReleaseKeepsAdvancedTurn(t *testing.T) {
	s := newStore(4, 4, time.Hour)
	mustReserve(t, s, storeAlice, "k0", testRecord("t0"))
	s.tasks[taskKey{storeAlice.Tenant, storeAlice.Principal, "t0"}].turn = 2
	s.release("k0")
	if !s.exists(storeAlice, "t0") {
		t.Fatal("release of a stale turn deleted the task")
	}
}

func TestStoreDuplicateDigest(t *testing.T) {
	s := newStore(4, 4, time.Hour)
	mustReserve(t, s, storeAlice, "k0", testRecord("t0"))
	snap, created, err := s.reserve(storeAlice, "k0", digestOf("parts"), testRecord("t1"))
	if err != nil || created || snap.id != "t0" {
		t.Fatalf("same digest: %v %v %q", err, created, snap.id)
	}
	_, created, err = s.reserve(storeAlice, "k0", digestOf("other"), testRecord("t2"))
	if created || !errors.Is(err, a2a.ErrInvalidParams) {
		t.Fatalf("different digest: created %v err %v", created, err)
	}
	if s.size() != 1 {
		t.Fatalf("size = %d, want 1", s.size())
	}
}

func TestStoreKeyIsUnambiguous(t *testing.T) {
	s := newStore(4, 4, time.Hour)
	owner := Identity{Tenant: "ab", Principal: "c"}
	mustReserve(t, s, owner, "k0", testRecord("x"))
	for _, other := range []Identity{{Tenant: "a", Principal: "bc"}, {Tenant: "abc", Principal: ""}, {Tenant: "ab", Principal: "cx"}} {
		if _, ok := s.get(other, "x"); ok || s.exists(other, "x") {
			t.Fatalf("%+v can see %+v's task", other, owner)
		}
	}
	if _, ok := s.get(owner, "x"); !ok {
		t.Fatal("owner cannot see its task")
	}
}
