package capsdk

import (
	"bytes"
	"errors"
	"sync"
	"time"
)

type ReplayOutcome uint8

const (
	ReplayOutcomeFirst ReplayOutcome = iota + 1
	ReplayOutcomeDuplicate
)

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
	if s == nil || tenant == "" || audience == "" || sender == "" || len(messageID) != 16 || len(digest) == 0 || !expires.After(time.Now()) {
		return 0, ErrReplayStoreUnavailable
	}
	key := tenant + "\x00" + audience + "\x00" + sender + "\x00" + string(messageID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.entries[key]; ok && existing.expires.After(time.Now()) {
		if bytes.Equal(existing.digest, digest) {
			return ReplayOutcomeDuplicate, nil
		}
		return 0, ErrReplayConflict
	}
	s.entries[key] = replayEntry{digest: append([]byte(nil), digest...), expires: expires}
	return ReplayOutcomeFirst, nil
}
