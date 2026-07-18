package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPerformRenewRequiresCurrentSession(t *testing.T) {
	var renewCalls atomic.Int32
	bus := newScriptedBus(nil, func(data []byte) ([]byte, error) {
		renewCalls.Add(1)
		return acceptResponse(t, data, "forged-renewal"), nil
	})
	agent := newAgentFixture(t, &bus.handshakeBus, HandshakeModeEnforce)
	agent.NATS = bus

	obtained, err := agent.performRenew(context.Background())
	if err == nil || obtained {
		t.Errorf("renew without an active session = (%v, %v), want (false, error)", obtained, err)
	}
	if renewCalls.Load() != 0 {
		t.Errorf("renew without an active session made %d requests, want 0", renewCalls.Load())
	}
	if token, _ := agent.SessionToken(); token != "" {
		t.Errorf("renew without an active session installed %q", token)
	}
}

func TestPerformRenewDoesNotInstallUnsignedResult(t *testing.T) {
	bus := newScriptedBus(nil, func(data []byte) ([]byte, error) {
		return acceptResponse(t, data, "attacker-renewal"), nil
	})
	agent := newAgentFixture(t, &bus.handshakeBus, HandshakeModeEnforce)
	agent.NATS = bus
	agent.setSession("current-token", time.Now().Add(time.Hour))

	obtained, err := agent.performRenew(context.Background())
	if err == nil || obtained {
		t.Errorf("unsigned renew result = (%v, %v), want (false, error)", obtained, err)
	}
	if token, _ := agent.SessionToken(); token != "current-token" {
		t.Errorf("unsigned renew result replaced active token with %q", token)
	}
}
