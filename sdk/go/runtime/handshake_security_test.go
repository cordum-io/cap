package runtime

import (
	"strings"
	"testing"
	"time"
)

func TestAgentStartRejectsUnknownHandshakeModeBeforeConnect(t *testing.T) {
	for _, test := range []struct {
		name     string
		explicit HandshakeMode
		env      string
	}{
		{name: "explicit", explicit: HandshakeMode("enforec")},
		{name: "environment", env: "enforec"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(EnvHandshakeMode, test.env)
			agent := &Agent{
				NATSURL:       "nats://127.0.0.1:1",
				Store:         NewInMemoryBlobStore(),
				SenderID:      "worker-mode",
				Tenant:        "tenant-mode",
				HandshakeMode: test.explicit,
				IOTTimeout:    20 * time.Millisecond,
			}
			Register(agent, "job.mode", func(_ Context, _ struct{}) (struct{}, error) {
				return struct{}{}, nil
			})

			err := agent.Start()
			if err == nil {
				t.Fatal("unknown handshake mode must fail startup")
			}
			if !strings.Contains(err.Error(), "handshake mode") || !strings.Contains(err.Error(), "enforec") {
				t.Fatalf("startup must reject the mode before connecting; got %v", err)
			}
		})
	}
}

func TestAgentStartDoesNotInstallUnsignedHandshakeResult(t *testing.T) {
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		return acceptResponse(t, data, "attacker-token"), nil
	})
	agent := newAgentFixture(t, bus, HandshakeModeEnforce)

	err := agent.Start()
	if err == nil {
		t.Error("unsigned handshake result must fail enforce-mode startup")
	}
	if token, _ := agent.SessionToken(); token != "" {
		t.Errorf("unsigned result installed session token %q", token)
	}
}
