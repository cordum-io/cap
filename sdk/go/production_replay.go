package capsdk

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

type ReplayOutcome uint8

const (
	ReplayOutcomeFirst ReplayOutcome = iota + 1
	ReplayOutcomeDuplicate
)

// ReplayStore admits a message-id exactly once per (tenant, audience, sender)
// scope until expires. Implementations MUST fail closed: any error (including
// backend unavailability) must be surfaced as an error, never treated as a
// silent first-seen admission. Redis- or other durable-backed implementations
// (e.g. Cordum's production boundary) satisfy this interface alongside
// InMemoryReplayStore.
type ReplayStore interface {
	Admit(tenant, audience, sender string, messageID, digest []byte, expires time.Time) (ReplayOutcome, error)
}

var _ ReplayStore = (*InMemoryReplayStore)(nil)

var (
	ErrReplayConflict         = errors.New("capsdk: replay digest conflict")
	ErrReplayStoreUnavailable = errors.New("capsdk: replay store unavailable")
)

type replayEntry struct {
	digest  []byte
	expires time.Time
}

type InMemoryReplayStore struct {
	mu      sync.Mutex
	entries map[string]replayEntry
}

func NewInMemoryReplayStore() *InMemoryReplayStore {
	return &InMemoryReplayStore{entries: make(map[string]replayEntry)}
}

func (s *InMemoryReplayStore) Admit(tenant, audience, sender string, messageID, digest []byte, expires time.Time) (ReplayOutcome, error) {
	now := time.Now()
	if s == nil || tenant == "" || audience == "" || sender == "" || len(messageID) != 16 || len(digest) == 0 || !expires.After(now) {
		return 0, ErrReplayStoreUnavailable
	}
	key := replayTupleKey([]byte(tenant), []byte(audience), []byte(sender), messageID)
	s.mu.Lock()
	defer s.mu.Unlock()
	for candidate, entry := range s.entries {
		if !entry.expires.After(now) {
			delete(s.entries, candidate)
		}
	}
	if existing, ok := s.entries[key]; ok {
		if bytes.Equal(existing.digest, digest) {
			return ReplayOutcomeDuplicate, nil
		}
		return 0, ErrReplayConflict
	}
	s.entries[key] = replayEntry{digest: append([]byte(nil), digest...), expires: expires}
	return ReplayOutcomeFirst, nil
}

func replayTupleKey(parts ...[]byte) string {
	size := 0
	for _, part := range parts {
		size += 4 + len(part)
	}
	key := make([]byte, 0, size)
	var length [4]byte
	for _, part := range parts {
		binary.BigEndian.PutUint32(length[:], uint32(len(part)))
		key = append(key, length[:]...)
		key = append(key, part...)
	}
	return string(key)
}
