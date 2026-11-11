package a2abridge

import (
	"bytes"
	"sync"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// taskKey is the composite owner-scoped key of a task. As a comparable
// struct it has no field-boundary ambiguity, so it needs no encoding.
type taskKey struct{ tenant, principal, id string }

// taskRecord is the bridge's record of one task.
type taskRecord struct {
	id, contextID, jobID, messageID string
	turn                            int
	state                           a2a.TaskState
	created                         time.Time
	parts                           []turnPart
}

// clone returns a deep copy, so no caller shares memory with the store.
func (r taskRecord) clone() taskRecord {
	parts := make([]turnPart, len(r.parts))
	for i, p := range r.parts {
		if p.Text != nil {
			text := *p.Text
			p.Text = &text
		}
		p.Data = bytes.Clone(p.Data)
		parts[i] = p
	}
	r.parts = parts
	return r
}

// turnRef binds an idempotency key to the turn it created.
type turnRef struct {
	key    taskKey
	turn   int
	digest [32]byte
}

// store is the bounded in-memory correlation store. Every method holds the
// lock for its whole critical section and does no I/O under it. It never
// evicts and is not durable: a durable store and terminal-task expiry are out
// of scope (spec §6 and the egress slice).
type store struct {
	mu    sync.Mutex
	max   int
	tasks map[taskKey]*taskRecord
	turns map[string]turnRef
}

func newStore(max int) *store {
	return &store{max: max, tasks: map[taskKey]*taskRecord{}, turns: map[string]turnRef{}}
}

// reserve records fresh under idemKey before anything is published. A known
// key with the same parts digest returns the existing task (created=false);
// a different digest is INVALID_PARAMS. A full store is an internal error.
func (s *store) reserve(id Identity, idemKey string, digest [32]byte, fresh taskRecord) (taskRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ref, ok := s.turns[idemKey]; ok {
		if ref.digest != digest {
			return taskRecord{}, false, a2a.ErrInvalidParams
		}
		rec, live := s.tasks[ref.key]
		if !live { // unreachable: release removes the key with its task
			return taskRecord{}, false, a2a.ErrInternalError
		}
		return rec.clone(), false, nil
	}
	key := taskKey{tenant: id.Tenant, principal: id.Principal, id: fresh.id}
	if _, taken := s.tasks[key]; taken || len(s.tasks) >= s.max {
		return taskRecord{}, false, a2a.ErrInternalError
	}
	rec := fresh.clone()
	s.tasks[key] = &rec
	s.turns[idemKey] = turnRef{key: key, turn: rec.turn, digest: digest}
	return rec.clone(), true, nil
}

// release undoes a reservation whose publish failed. It deletes the task only
// while the released turn is still the task's current turn.
func (s *store) release(idemKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.turns[idemKey]
	if !ok {
		return
	}
	delete(s.turns, idemKey)
	if rec, ok := s.tasks[ref.key]; ok && rec.turn == ref.turn {
		delete(s.tasks, ref.key)
	}
}

// get returns a deep copy of the caller's task. A foreign or unknown id takes
// the same single lookup and reports false.
func (s *store) get(id Identity, taskID string) (taskRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tasks[taskKey{tenant: id.Tenant, principal: id.Principal, id: taskID}]
	if !ok {
		return taskRecord{}, false
	}
	return rec.clone(), true
}

// exists reports whether the caller owns taskID.
func (s *store) exists(id Identity, taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tasks[taskKey{tenant: id.Tenant, principal: id.Principal, id: taskID}]
	return ok
}

func (s *store) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tasks)
}
