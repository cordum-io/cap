package a2abridge

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
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
	s := newStore(2)
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
	h := newHarness(t, func(c *Config) { c.MaxTasks = 2 })
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

func TestStoreGetReturnsCopies(t *testing.T) {
	s := newStore(4)
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
	s := newStore(4)
	mustReserve(t, s, storeAlice, "k0", testRecord("t0"))
	s.release("k0")
	if s.size() != 0 || s.exists(storeAlice, "t0") {
		t.Fatal("release left the task behind")
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
	s := newStore(4)
	mustReserve(t, s, storeAlice, "k0", testRecord("t0"))
	s.tasks[taskKey{storeAlice.Tenant, storeAlice.Principal, "t0"}].turn = 2
	s.release("k0")
	if !s.exists(storeAlice, "t0") {
		t.Fatal("release of a stale turn deleted the task")
	}
}

func TestStoreDuplicateDigest(t *testing.T) {
	s := newStore(4)
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
	s := newStore(4)
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
