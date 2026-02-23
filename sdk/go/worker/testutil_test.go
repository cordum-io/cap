package worker

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startTestNATS(t *testing.T) (*server.Server, string) {
	t.Helper()
	opts := &server.Options{
		Port:       -1,
		Host:       "127.0.0.1",
		NoLog:      true,
		NoSigs:     true,
		MaxPending: 64 * 1024 * 1024,
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start nats server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(func() { ns.Shutdown() })
	return ns, ns.ClientURL()
}

func testNATSConn(t *testing.T, url string) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect to nats: %v", err)
	}
	t.Cleanup(func() { nc.Close() })
	return nc
}
