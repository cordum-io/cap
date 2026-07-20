package worker

import (
	"context"
	"reflect"
	"time"
)

const (
	defaultManagedReplayLease      = 15 * time.Second
	defaultManagedReplayRetryDelay = 100 * time.Millisecond
	defaultManagedReplayTimeout    = 5 * time.Second
)

// ManagedReplayState describes the durable disposition of an admitted
// production request.
type ManagedReplayState uint8

const (
	ManagedReplayProcess ManagedReplayState = iota + 1
	ManagedReplayPending
	ManagedReplayCompleted
)

// ManagedReplayEntry is the authenticated replay identity and lease request.
// Stores must copy byte slices they retain.
type ManagedReplayEntry struct {
	Tenant     string
	Audience   string
	Sender     string
	MessageID  []byte
	Digest     []byte
	ExpiresAt  time.Time
	LeaseUntil time.Time
}

// ManagedReplayOutcome is a canonical JobResult outbox record. Complete must
// persist it atomically with the completed replay state. It intentionally
// excludes session tokens and signatures so retries can use current trust.
type ManagedReplayOutcome struct {
	TraceID string
	Result  []byte
}

// ManagedReplayClaim tells the worker whether to process, retry later, or
// replay a previously completed durable outcome.
type ManagedReplayClaim struct {
	State   ManagedReplayState
	LeaseID string
	Outcome ManagedReplayOutcome
}

// ManagedReplayStore provides crash-recoverable admission and a durable result
// outbox. Production configuration rejects stores whose Durable reports false.
type ManagedReplayStore interface {
	Durable() bool
	Begin(context.Context, ManagedReplayEntry) (ManagedReplayClaim, error)
	Renew(context.Context, ManagedReplayEntry, string, time.Time) error
	Complete(context.Context, ManagedReplayEntry, string, ManagedReplayOutcome) error
	Abort(context.Context, ManagedReplayEntry, string) error
}

func cloneManagedReplayEntry(entry ManagedReplayEntry) ManagedReplayEntry {
	entry.MessageID = append([]byte(nil), entry.MessageID...)
	entry.Digest = append([]byte(nil), entry.Digest...)
	return entry
}

func cloneManagedReplayOutcome(outcome ManagedReplayOutcome) ManagedReplayOutcome {
	outcome.Result = append([]byte(nil), outcome.Result...)
	return outcome
}

func validManagedReplayStore(store ManagedReplayStore) bool {
	if store == nil {
		return false
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return false
		}
	}
	return store.Durable()
}
