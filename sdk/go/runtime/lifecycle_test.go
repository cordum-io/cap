package runtime

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type lifecycleBus struct {
	requestErr error
	failSubAt  int
	subCalls   atomic.Int32
	closed     atomic.Int32
}

func (b *lifecycleBus) Publish(string, []byte) error { return nil }

func (b *lifecycleBus) QueueSubscribe(string, string, nats.MsgHandler) (*nats.Subscription, error) {
	call := int(b.subCalls.Add(1))
	if b.failSubAt == call {
		return nil, errors.New("subscription failed")
	}
	return &nats.Subscription{}, nil
}

func (b *lifecycleBus) Request(string, []byte, time.Duration) (*nats.Msg, error) {
	return nil, b.requestErr
}

func (b *lifecycleBus) Close() { b.closed.Add(1) }

type lifecycleStore struct{ closed atomic.Int32 }

func (*lifecycleStore) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (*lifecycleStore) Set(context.Context, string, []byte) error   { return nil }
func (s *lifecycleStore) Close() error {
	s.closed.Add(1)
	return nil
}

func TestStartFailureClosesOnlyRuntimeOwnedResources(t *testing.T) {
	agent, fixtureBus := newTrustTestAgent(t, HandshakeModeEnforce)
	ownedBus := &lifecycleBus{requestErr: errors.New("scheduler unavailable")}
	ownedStore := &lifecycleStore{}
	agent.NATS, agent.Store = nil, nil
	agent.WorkerTrust = fixtureBus.config
	agent.connectNATS = func(string, string, time.Duration) (NATSConn, error) { return ownedBus, nil }
	agent.openStore = func(string) (BlobStore, error) { return ownedStore, nil }
	if err := agent.Start(); err == nil {
		t.Fatal("Start succeeded despite trust transport failure")
	}
	if ownedBus.closed.Load() != 1 || ownedStore.closed.Load() != 1 {
		t.Fatalf("owned cleanup bus=%d store=%d, want 1 each", ownedBus.closed.Load(), ownedStore.closed.Load())
	}
	if agent.NATS != nil || agent.Store != nil || len(agent.subs) != 0 {
		t.Fatal("owned resources remained attached after Start failure")
	}

	injectedBus := &lifecycleBus{requestErr: errors.New("scheduler unavailable")}
	injectedStore := &lifecycleStore{}
	agent, fixtureBus = newTrustTestAgent(t, HandshakeModeEnforce)
	agent.NATS, agent.Store = injectedBus, injectedStore
	agent.WorkerTrust = fixtureBus.config
	if err := agent.Start(); err == nil {
		t.Fatal("Start succeeded despite injected trust transport failure")
	}
	if injectedBus.closed.Load() != 0 || injectedStore.closed.Load() != 0 {
		t.Fatal("Start failure closed injected resources")
	}
}

func TestPartialSubscriptionFailureCleansOwnedResources(t *testing.T) {
	bus := &lifecycleBus{failSubAt: 2}
	store := &lifecycleStore{}
	agent := &Agent{HandshakeMode: HandshakeModeOff, AllowUnsigned: true, Logger: silentLogger()}
	agent.connectNATS = func(string, string, time.Duration) (NATSConn, error) { return bus, nil }
	agent.openStore = func(string) (BlobStore, error) { return store, nil }
	Register(agent, "job.a", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	Register(agent, "job.b", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	if err := agent.Start(); err == nil {
		t.Fatal("Start succeeded despite partial subscription failure")
	}
	if bus.closed.Load() != 1 || store.closed.Load() != 1 || len(agent.subs) != 0 {
		t.Fatalf("cleanup bus=%d store=%d subs=%d", bus.closed.Load(), store.closed.Load(), len(agent.subs))
	}
}

func TestAgentLifecycleRejectsDuplicateStartAndRestartAfterClose(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	requests := len(bus.order)
	if err := agent.Start(); err == nil {
		t.Fatal("duplicate Start succeeded")
	}
	if len(bus.order) != requests {
		t.Fatal("duplicate Start performed transport side effects")
	}
	if err := agent.Close(); err != nil {
		t.Fatal(err)
	}
	if err := agent.Start(); err == nil {
		t.Fatal("Start after Close succeeded")
	}
}

func TestOffModeAllowsHandshakeTuningWithoutTrustConfiguration(t *testing.T) {
	agent := &Agent{
		NATS: &lifecycleBus{}, Store: &lifecycleStore{}, Logger: silentLogger(),
		AllowUnsigned:    true,
		HandshakeTimeout: 250 * time.Millisecond, HandshakeRetries: 2,
	}
	Register(agent, "job.tuning", func(_ Context, _ struct{}) (struct{}, error) {
		return struct{}{}, nil
	})
	if err := agent.Start(); err != nil {
		t.Fatalf("off-mode Start rejected transport tuning: %v", err)
	}
	if err := agent.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOffModeRejectsImplicitUnsignedTransport(t *testing.T) {
	for _, keys := range []map[string]*ecdsa.PublicKey{nil, {}} {
		agent := &Agent{NATS: &lifecycleBus{}, Store: &lifecycleStore{}, Logger: silentLogger(), PublicKeys: keys}
		Register(agent, "job.unsigned", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
		err := agent.Start()
		if err == nil || !strings.Contains(err.Error(), "unsigned") {
			t.Fatalf("Start error = %v, want explicit unsigned opt-in requirement", err)
		}
	}
}
