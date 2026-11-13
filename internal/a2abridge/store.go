package a2abridge

import (
	"bytes"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
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
	statusAt                        time.Time
	parts                           []turnPart
	artifacts                       []*a2a.Artifact
	history                         []*a2a.Message // after the initial user message in parts
	statusMsg                       *a2a.Message
	settling                        string // SUCCEEDED winner reserved before the result read
	// events are immutable after append; snapshots deep-copy their payloads.
	events []a2a.Event
	// turnDone is set when the current turn's job reached a terminal
	// JobStatus: the first terminal result wins and later events are ignored.
	turnDone bool
	// changed is closed and replaced on every visible change; see notify.
	changed chan struct{}
	// prev is what release restores when a resume's input or publish fails.
	prev *turnUndo
	// A publish failure alone closes cancelAbort; release never closes it.
	cancelTurn    int
	cancelAttempt uint64
	cancelAbort   chan struct{}
}

// maxTurns bounds the turns of one task (spec §3).
const maxTurns = 64

// turnUndo is the interrupted turn a resume replaced; history and events are lengths.
type turnUndo struct {
	jobID           string
	state           a2a.TaskState
	statusMsg       *a2a.Message
	statusAt        time.Time
	history, events int
}

// notify wakes every waiter by closing rec.changed and making a new one.
func notify(rec *taskRecord) {
	if rec.changed != nil {
		close(rec.changed)
	}
	rec.changed = make(chan struct{})
}

// clone returns a deep copy, so no caller shares memory with the store. The
// copy carries neither the undo record nor the change channel.
func (r taskRecord) clone() taskRecord {
	r.prev, r.changed = nil, nil
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
	r.events = slices.Clone(r.events)
	for idx, event := range r.events {
		switch ev := event.(type) {
		case *a2a.TaskStatusUpdateEvent:
			copy := *ev
			copy.Status.Message = cloneMessage(ev.Status.Message)
			if ev.Status.Timestamp != nil {
				ts := *ev.Status.Timestamp
				copy.Status.Timestamp = &ts
			}
			r.events[idx] = &copy
		case *a2a.TaskArtifactUpdateEvent:
			copy := *ev
			copy.Artifact = cloneArtifact(ev.Artifact)
			r.events[idx] = &copy
		}
	}
	r.artifacts = slices.Clone(r.artifacts)
	for idx, a := range r.artifacts {
		r.artifacts[idx] = cloneArtifact(a)
	}
	r.history = slices.Clone(r.history)
	for idx, m := range r.history {
		r.history[idx] = cloneMessage(m)
	}
	r.statusMsg = cloneMessage(r.statusMsg)
	return r
}

// Only the closed envelope's text/data parts enter stored agent messages.
func cloneParts(parts a2a.ContentParts) a2a.ContentParts {
	out := slices.Clone(parts)
	for idx, part := range out {
		copy := *part
		if data, ok := part.Content.(a2a.Data); ok {
			copy.Content = a2a.Data{Value: cloneData(data.Value)}
		}
		out[idx] = &copy
	}
	return out
}

func cloneData(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, item := range v {
			out[key] = cloneData(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for idx, item := range v {
			out[idx] = cloneData(item)
		}
		return out
	case json.RawMessage:
		return bytes.Clone(v)
	default:
		return value // immutable JSON scalar, including json.Number
	}
}

func cloneMessage(m *a2a.Message) *a2a.Message {
	if m == nil {
		return nil
	}
	copy := *m
	copy.Parts = cloneParts(m.Parts)
	return &copy
}

func cloneArtifact(a *a2a.Artifact) *a2a.Artifact {
	copy := *a
	copy.Parts = cloneParts(a.Parts)
	return &copy
}

type turnOutcome struct {
	state     a2a.TaskState
	message   *a2a.Message
	artifacts []*a2a.Artifact
}

// turnRef binds an idempotency key to the turn it created.
type turnRef struct {
	key    taskKey
	turn   int
	digest [32]byte
}

// jobRef binds a job id to the task and turn it runs.
type jobRef struct {
	key  taskKey
	turn int
}

// store is the bounded in-memory correlation store. Every method holds the
// lock for its whole critical section and does no I/O under it. It never
// evicts and is not durable: a durable store and terminal-task expiry are out
// of scope (spec §6 and the egress slice).
type store struct {
	cancelSeq uint64 // monotonically allocated under mu; survives turn release
	mu        sync.Mutex
	max       int
	tasks     map[taskKey]*taskRecord
	turns     map[string]turnRef
	jobs      map[string]jobRef
}

func newStore(max int) *store {
	return &store{max: max, tasks: map[taskKey]*taskRecord{}, turns: map[string]turnRef{}, jobs: map[string]jobRef{}}
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
	rec.changed = make(chan struct{})
	appendStatus(&rec, rec.created)
	s.tasks[key] = &rec
	s.turns[idemKey] = turnRef{key: key, turn: rec.turn, digest: digest}
	s.jobs[rec.jobID] = jobRef{key: key, turn: rec.turn}
	return rec.clone(), true, nil
}

// reserveTurn opens turn N+1 of an interrupted task before any I/O, in one
// compare-and-set: INPUT_REQUIRED or AUTH_REQUIRED becomes SUBMITTED with the
// new job indexed and msg in history. A known key returns the task
// (created=false). A task in flight, settling, terminal or at maxTurns is
// UNSUPPORTED_OPERATION. parentJobID is turn N's job.
func (s *store) reserveTurn(id Identity, taskID, idemKey string, digest [32]byte, contextID, jobID string,
	msg *a2a.Message, now time.Time) (snap taskRecord, parentJobID string, created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ref, ok := s.turns[idemKey]; ok {
		rec, live := s.tasks[ref.key]
		switch {
		case ref.digest != digest:
			return taskRecord{}, "", false, invalid("messageId was already used with different parts")
		case !live: // unreachable: release removes the key with its task
			return taskRecord{}, "", false, a2a.ErrInternalError
		}
		return rec.clone(), "", false, nil
	}
	key := taskKey{tenant: id.Tenant, principal: id.Principal, id: taskID}
	rec, ok := s.tasks[key]
	switch {
	case !ok:
		return taskRecord{}, "", false, a2a.ErrTaskNotFound
	case contextID != "" && contextID != rec.contextID:
		return taskRecord{}, "", false, invalid("contextId does not match the task")
	case rec.state != a2a.TaskStateInputRequired && rec.state != a2a.TaskStateAuthRequired || rec.turn >= maxTurns:
		return taskRecord{}, "", false, a2a.ErrUnsupportedOperation
	}
	rec.prev = &turnUndo{jobID: rec.jobID, state: rec.state, statusMsg: rec.statusMsg, statusAt: rec.statusAt,
		history: len(rec.history), events: len(rec.events)}
	parentJobID = rec.jobID
	rec.turn, rec.jobID, rec.turnDone = rec.turn+1, jobID, false
	msg.TaskID, msg.ContextID = a2a.TaskID(rec.id), rec.contextID
	rec.history = append(rec.history, msg)
	s.turns[idemKey] = turnRef{key: key, turn: rec.turn, digest: digest}
	s.jobs[jobID] = jobRef{key: key, turn: rec.turn}
	setState(rec, a2a.TaskStateSubmitted, nil, now)
	return rec.clone(), parentJobID, true, nil
}

// release undoes a reservation whose input write or publish failed, only while
// the released turn is still current. Turn 1 deletes the task. A later turn
// restores the interrupted turn exactly, unless a result already settled it
// (turnDone): that commit must never be stranded.
func (s *store) release(idemKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.turns[idemKey]
	if !ok {
		return
	}
	delete(s.turns, idemKey)
	rec, ok := s.tasks[ref.key]
	switch {
	case !ok || rec.turn != ref.turn:
	case ref.turn == 1:
		notify(rec)
		delete(s.tasks, ref.key)
		delete(s.jobs, rec.jobID)
	case !rec.turnDone && rec.prev != nil:
		p := rec.prev
		delete(s.jobs, rec.jobID)
		rec.turn, rec.jobID, rec.state, rec.statusMsg, rec.statusAt = rec.turn-1, p.jobID, p.state, p.statusMsg, p.statusAt
		rec.history, rec.events = rec.history[:p.history], rec.events[:p.events]
		rec.turnDone, rec.prev = true, nil
		rec.cancelTurn, rec.cancelAttempt, rec.cancelAbort = 0, 0, nil
		notify(rec)
	}
}

// get returns a deep copy of the caller's task. A foreign or unknown id takes
// the same single lookup and reports false.
func (s *store) get(id Identity, taskID string) (taskRecord, bool) {
	rec, _, ok := s.watch(id, taskID)
	return rec, ok
}

// watch is get plus the channel the task's next change closes, both from one
// locked lookup, so a waiter cannot miss a change between reading and waiting.
func (s *store) watch(id Identity, taskID string) (taskRecord, <-chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tasks[taskKey{tenant: id.Tenant, principal: id.Principal, id: taskID}]
	if !ok {
		return taskRecord{}, nil, false
	}
	return rec.clone(), rec.changed, true
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

// appendStatus records rec's current state as one status event.
func appendStatus(rec *taskRecord, now time.Time) {
	rec.statusAt = now
	rec.events = append(rec.events, &a2a.TaskStatusUpdateEvent{TaskID: a2a.TaskID(rec.id),
		ContextID: rec.contextID, Status: a2a.TaskStatus{State: rec.state, Message: rec.statusMsg, Timestamp: &now}})
}

// setState moves rec to state with one status event, only on a change, and
// notifies the waiters.
func setState(rec *taskRecord, state a2a.TaskState, msg *a2a.Message, now time.Time) bool {
	if rec.state == state && msg == nil {
		return false
	}
	rec.state = state
	rec.statusMsg = msg
	appendStatus(rec, now)
	notify(rec)
	return true
}

// current returns the task whose current turn runs jobID while that turn can
// still change. The caller holds s.mu.
func (s *store) current(jobID string) (*taskRecord, bool) {
	ref, ok := s.jobs[jobID]
	if !ok {
		return nil, false
	}
	rec, ok := s.tasks[ref.key]
	if !ok || rec.turn != ref.turn || rec.turnDone || rec.state.Terminal() {
		return nil, false
	}
	return rec, true
}

// owner returns the tenant of the task that runs jobID, for production
// verification: the trust store is chosen by the task, never by the packet.
func (s *store) owner(jobID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.jobs[jobID]
	return ref.key.tenant, ok
}

// progressState maps a non-terminal JobStatus (spec §2).
func progressState(st agentv1.JobStatus) (a2a.TaskState, bool) {
	switch st {
	case agentv1.JobStatus_JOB_STATUS_PENDING, agentv1.JobStatus_JOB_STATUS_SCHEDULED:
		return a2a.TaskStateSubmitted, true
	case agentv1.JobStatus_JOB_STATUS_DISPATCHED, agentv1.JobStatus_JOB_STATUS_RUNNING,
		agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE:
		return a2a.TaskStateWorking, true
	}
	return a2a.TaskStateUnspecified, false
}

// terminalState maps a terminal JobStatus other than SUCCEEDED (spec §2).
func terminalState(st agentv1.JobStatus) (a2a.TaskState, bool) {
	switch st {
	case agentv1.JobStatus_JOB_STATUS_DENIED:
		return a2a.TaskStateRejected, true
	case agentv1.JobStatus_JOB_STATUS_CANCELLED:
		return a2a.TaskStateCanceled, true
	case agentv1.JobStatus_JOB_STATUS_FAILED, agentv1.JobStatus_JOB_STATUS_FAILED_FATAL,
		agentv1.JobStatus_JOB_STATUS_TIMEOUT:
		return a2a.TaskStateFailed, true
	}
	return a2a.TaskStateUnspecified, false
}

// applyProgress applies a progress hint for jobID. A hint never ends a turn
// and never moves WORKING back to SUBMITTED. It reports whether rec changed.
func (s *store) applyProgress(jobID string, st agentv1.JobStatus, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.current(jobID)
	state, progress := progressState(st)
	if !ok || !progress || state == a2a.TaskStateSubmitted && rec.state == a2a.TaskStateWorking {
		return false
	}
	return setState(rec, state, nil, now)
}

// applyResult applies a JobResult status for jobID. The first terminal status
// of the current turn wins; a progress-class status (FAILED_RETRYABLE
// included) is applied as progress. It reports whether the task changed.
func (s *store) applyResult(jobID string, st agentv1.JobStatus, now time.Time) bool {
	if _, progress := progressState(st); progress {
		return s.applyProgress(jobID, st, now)
	}
	// SUCCEEDED is handled by reserveResult/readEnvelope/commitResult, not here.
	state, terminal := terminalState(st)
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.current(jobID)
	if !terminal || !ok {
		return false
	}
	rec.turnDone = true
	return setState(rec, state, nil, now)
}

// reserveResult chooses the first terminal winner before any result I/O.
func (s *store) reserveResult(job string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.current(job)
	if !ok {
		return 0, false
	}
	rec.turnDone, rec.settling = true, job
	return rec.turn, true
}

// commitResult applies only the reserved turn; artifacts precede its status.
func (s *store) commitResult(job string, turn int, out *turnOutcome, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.jobs[job]
	if !ok {
		return
	}
	rec, ok := s.tasks[ref.key]
	if !ok || rec.turn != turn || rec.settling != job {
		return
	}
	rec.settling = ""
	if out == nil {
		setState(rec, a2a.TaskStateFailed, nil, now)
		return
	}
	for _, artifact := range out.artifacts {
		a := cloneArtifact(artifact)
		rec.artifacts = append(rec.artifacts, a)
		rec.events = append(rec.events, &a2a.TaskArtifactUpdateEvent{
			TaskID: a2a.TaskID(rec.id), ContextID: rec.contextID, Artifact: a, LastChunk: true})
	}
	msg := cloneMessage(out.message)
	if msg != nil {
		msg.TaskID, msg.ContextID = a2a.TaskID(rec.id), rec.contextID
		rec.history = append(rec.history, msg)
	}
	setState(rec, out.state, msg, now)
	if rec.cancelTurn == rec.turn && (out.state == a2a.TaskStateInputRequired || out.state == a2a.TaskStateAuthRequired) {
		setState(rec, a2a.TaskStateCanceled, nil, now)
	}
}

type cancelStep int

const (
	cancelPublish cancelStep = iota
	cancelWait
	cancelDone
)

// requestCancel reserves or joins an attempt. A settling result is still in
// flight: its reserved winner, not the cancel's grace, will finish the task.
func (s *store) requestCancel(id Identity, taskID string, now time.Time) (step cancelStep, snap taskRecord, jobID string, attempt uint64, abort <-chan struct{}, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tasks[taskKey{tenant: id.Tenant, principal: id.Principal, id: taskID}]
	switch {
	case !ok:
		return 0, taskRecord{}, "", 0, nil, a2a.ErrTaskNotFound
	case rec.state.Terminal():
		return 0, taskRecord{}, "", 0, nil, a2a.ErrTaskNotCancelable
	case rec.state == a2a.TaskStateInputRequired || rec.state == a2a.TaskStateAuthRequired:
		setState(rec, a2a.TaskStateCanceled, nil, now)
		return cancelDone, rec.clone(), "", 0, nil, nil
	case rec.cancelTurn == rec.turn:
		return cancelWait, rec.clone(), rec.jobID, rec.cancelAttempt, rec.cancelAbort, nil
	}
	s.cancelSeq++
	rec.cancelAttempt, rec.cancelTurn, rec.cancelAbort = s.cancelSeq, rec.turn, make(chan struct{})
	return cancelPublish, rec.clone(), rec.jobID, rec.cancelAttempt, rec.cancelAbort, nil
}

// cancelFailed wakes only this attempt's joiners; a stale failure cannot
// abort a successful later attempt, even on a retry of the same message.
func (s *store) cancelFailed(id Identity, taskID string, attempt uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tasks[taskKey{tenant: id.Tenant, principal: id.Principal, id: taskID}]
	if !ok || attempt == 0 || rec.cancelAttempt != attempt {
		return
	}
	close(rec.cancelAbort)
	rec.cancelTurn, rec.cancelAttempt, rec.cancelAbort = 0, 0, nil
	notify(rec)
}

// expireCancel cannot defeat a reserved result or an unrelated attempt.
func (s *store) expireCancel(id Identity, taskID string, attempt uint64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tasks[taskKey{tenant: id.Tenant, principal: id.Principal, id: taskID}]
	if !ok || attempt == 0 || rec.cancelAttempt != attempt || rec.turnDone || rec.state.Terminal() {
		return
	}
	rec.turnDone = true
	setState(rec, a2a.TaskStateCanceled, nil, now)
}
