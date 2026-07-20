package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

type managedReplayTestRecord struct {
	digest     []byte
	leaseID    string
	leaseUntil time.Time
	expiresAt  time.Time
	completed  bool
	outcome    ManagedReplayOutcome
}

type managedReplayTestStore struct {
	mu      sync.Mutex
	records map[[32]byte]managedReplayTestRecord
	nextID  int
	durable bool
}

func newManagedReplayTestStore() *managedReplayTestStore {
	return &managedReplayTestStore{records: make(map[[32]byte]managedReplayTestRecord), durable: true}
}

func (s *managedReplayTestStore) Durable() bool { return s != nil && s.durable }

func (s *managedReplayTestStore) Begin(
	ctx context.Context, entry ManagedReplayEntry,
) (ManagedReplayClaim, error) {
	if err := ctx.Err(); err != nil {
		return ManagedReplayClaim{}, err
	}
	now := time.Now()
	if !entry.ExpiresAt.After(now) || !entry.LeaseUntil.After(now) {
		return ManagedReplayClaim{}, capsdk.ErrReplayStoreUnavailable
	}
	key := managedReplayTestKey(entry)
	s.mu.Lock()
	defer s.mu.Unlock()
	record, exists := s.records[key]
	if exists && !bytes.Equal(record.digest, entry.Digest) {
		return ManagedReplayClaim{}, capsdk.ErrReplayConflict
	}
	if exists && record.completed {
		return ManagedReplayClaim{
			State:   ManagedReplayCompleted,
			Outcome: cloneManagedReplayTestOutcome(record.outcome),
		}, nil
	}
	if exists && record.leaseUntil.After(now) {
		return ManagedReplayClaim{State: ManagedReplayPending}, nil
	}
	s.nextID++
	record = managedReplayTestRecord{
		digest: append([]byte(nil), entry.Digest...), leaseID: fmt.Sprintf("lease-%d", s.nextID),
		leaseUntil: entry.LeaseUntil, expiresAt: entry.ExpiresAt,
	}
	s.records[key] = record
	return ManagedReplayClaim{State: ManagedReplayProcess, LeaseID: record.leaseID}, nil
}

func (s *managedReplayTestStore) Renew(
	ctx context.Context, entry ManagedReplayEntry, leaseID string, leaseUntil time.Time,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := managedReplayTestKey(entry)
	record, exists := s.records[key]
	if !exists || record.completed || record.leaseID != leaseID || !leaseUntil.After(time.Now()) {
		return capsdk.ErrReplayConflict
	}
	record.leaseUntil = leaseUntil
	s.records[key] = record
	return nil
}

func (s *managedReplayTestStore) Complete(
	ctx context.Context, entry ManagedReplayEntry, leaseID string, outcome ManagedReplayOutcome,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := managedReplayTestKey(entry)
	record, exists := s.records[key]
	if !exists || record.completed || record.leaseID != leaseID {
		return capsdk.ErrReplayConflict
	}
	record.completed, record.outcome = true, cloneManagedReplayTestOutcome(outcome)
	s.records[key] = record
	return nil
}

func (s *managedReplayTestStore) Abort(
	ctx context.Context, entry ManagedReplayEntry, leaseID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := managedReplayTestKey(entry)
	record, exists := s.records[key]
	if !exists || record.completed || record.leaseID != leaseID {
		return capsdk.ErrReplayConflict
	}
	delete(s.records, key)
	return nil
}

func managedReplayTestKey(entry ManagedReplayEntry) [32]byte {
	parts := [][]byte{[]byte(entry.Tenant), []byte(entry.Audience), []byte(entry.Sender), entry.MessageID}
	buffer := make([]byte, 0)
	var length [4]byte
	for _, part := range parts {
		binary.BigEndian.PutUint32(length[:], uint32(len(part)))
		buffer = append(buffer, length[:]...)
		buffer = append(buffer, part...)
	}
	return sha256.Sum256(buffer)
}

func cloneManagedReplayTestOutcome(outcome ManagedReplayOutcome) ManagedReplayOutcome {
	return ManagedReplayOutcome{TraceID: outcome.TraceID, Result: append([]byte(nil), outcome.Result...)}
}

var _ ManagedReplayStore = (*managedReplayTestStore)(nil)
