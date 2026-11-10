package a2abridge

import (
	"sync"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// taskKey is the composite owner-scoped key of a task.
type taskKey struct{ tenant, principal, id string }

// taskRecord is the bridge's record of one task.
type taskRecord struct {
	id, contextID, jobID, messageID string
	turn                            int
	state                           a2a.TaskState
	created                         time.Time
	parts                           []turnPart
}

// store is the bounded in-memory correlation store. Not implemented yet: it
// fails closed.
type store struct {
	mu    sync.Mutex
	max   int
	tasks map[taskKey]*taskRecord
	turns map[string]turnRef
}

// turnRef binds an idempotency key to the turn it created.
type turnRef struct {
	key    taskKey
	turn   int
	digest [32]byte
}

func newStore(max int) *store {
	return &store{max: max, tasks: map[taskKey]*taskRecord{}, turns: map[string]turnRef{}}
}

func (s *store) reserve(Identity, string, [32]byte, taskRecord) (taskRecord, bool, error) {
	return taskRecord{}, false, a2a.ErrInternalError
}

func (s *store) release(string) {}

func (s *store) get(Identity, string) (taskRecord, bool) { return taskRecord{}, false }

func (s *store) exists(Identity, string) bool { return false }

func (s *store) size() int { return 0 }
