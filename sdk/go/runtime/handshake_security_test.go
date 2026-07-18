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
		want     string
	}{
		{name: "missing", want: "explicit"},
		{name: "explicit", explicit: HandshakeMode("enforec"), want: "enforec"},
		{name: "environment", env: "enforec", want: "enforec"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(EnvHandshakeMode, test.env)
			agent := &Agent{
				NATSURL:       "nats://127.0.0.1:1",
				Store:         NewInMemoryBlobStore(),
				SenderID:      "worker-mode",
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
			if !strings.Contains(err.Error(), "handshake mode") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("startup must reject the mode before connecting; got %v", err)
			}
		})
	}
}
