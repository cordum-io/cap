package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
)

type managedReplayLeaseState struct {
	cancelRenew  context.CancelFunc
	cancelHandle context.CancelFunc
	done         chan struct{}
	once         sync.Once
	mu           sync.Mutex
	err          error
}

func (w *ManagedWorker) startManagedReplayLease(
	parent context.Context, work managedReplayWork, message *nats.Msg,
) (context.Context, *managedReplayLeaseState) {
	handlerCtx, cancelHandler := context.WithCancel(parent)
	renewCtx, cancelRenew := context.WithCancel(parent)
	state := &managedReplayLeaseState{
		cancelRenew: cancelRenew, cancelHandle: cancelHandler, done: make(chan struct{}),
	}
	go func() {
		defer close(state.done)
		w.renewManagedReplayLease(renewCtx, work, message, state)
	}()
	return handlerCtx, state
}

func (w *ManagedWorker) renewManagedReplayLease(
	ctx context.Context, work managedReplayWork, message *nats.Msg, state *managedReplayLeaseState,
) {
	interval := managedReplayLease(w.cfg.Production) / 3
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.renewManagedReplay(ctx, work, message); err != nil {
				state.fail(err)
				return
			}
		}
	}
}

func (w *ManagedWorker) renewManagedReplay(
	ctx context.Context, work managedReplayWork, message *nats.Msg,
) error {
	now := time.Now()
	leaseUntil := now.Add(managedReplayLease(w.cfg.Production))
	if leaseUntil.After(work.entry.ExpiresAt) {
		leaseUntil = work.entry.ExpiresAt
	}
	if !leaseUntil.After(now) {
		return capsdk.ErrReplayStoreUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, defaultManagedReplayTimeout)
	defer cancel()
	err := w.cfg.Production.Replay.Renew(
		callCtx, cloneManagedReplayEntry(work.entry), work.claim.LeaseID, leaseUntil,
	)
	if err != nil {
		return managedReplayError(err)
	}
	if isManagedJetStreamMessage(message) {
		if err := message.InProgress(); err != nil {
			return capsdk.ErrReplayStoreUnavailable
		}
	}
	return nil
}

func (s *managedReplayLeaseState) fail(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
	s.cancelHandle()
}

func (s *managedReplayLeaseState) stop() error {
	s.once.Do(func() {
		s.cancelRenew()
		<-s.done
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *managedReplayLeaseState) finish() {
	s.cancelHandle()
}

func (w *ManagedWorker) completeManagedReplay(
	work managedReplayWork, outcome ManagedReplayOutcome,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultManagedReplayTimeout)
	defer cancel()
	err := w.cfg.Production.Replay.Complete(
		ctx, cloneManagedReplayEntry(work.entry), work.claim.LeaseID, cloneManagedReplayOutcome(outcome),
	)
	if err != nil {
		return managedReplayError(err)
	}
	return nil
}

func (w *ManagedWorker) abortManagedReplay(work managedReplayWork) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultManagedReplayTimeout)
	defer cancel()
	if err := w.cfg.Production.Replay.Abort(
		ctx, cloneManagedReplayEntry(work.entry), work.claim.LeaseID,
	); err != nil && !errors.Is(err, capsdk.ErrReplayConflict) {
		w.logManagedProductionError("abort lease", err)
	}
}
