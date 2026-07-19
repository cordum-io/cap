package tck

import (
	"fmt"
	"net"
	"os"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

const natsReadyTimeout = 10 * time.Second

// EmbeddedNATS is an isolated, loopback-only NATS server with JetStream enabled,
// for mandatory real-transport tests. It binds an ephemeral port and a unique
// temporary store, so it is never reachable as, and never connects to, a shared
// or default server. Start failure is returned as an error the caller must fail
// on — there is deliberately no skip path.
type EmbeddedNATS struct {
	srv      *natsserver.Server
	storeDir string
	closed   bool
}

// StartEmbeddedNATS starts a server on an ephemeral port.
func StartEmbeddedNATS() (*EmbeddedNATS, error) { return startEmbeddedNATS(-1) }

// StartEmbeddedNATSOnPort starts a server on an explicit port, for reconnect
// tests that must bring the server back at the same address.
func StartEmbeddedNATSOnPort(port int) (*EmbeddedNATS, error) { return startEmbeddedNATS(port) }

func startEmbeddedNATS(port int) (*EmbeddedNATS, error) {
	dir, err := os.MkdirTemp("", "cap-tck-nats-*")
	if err != nil {
		return nil, fmt.Errorf("tck: nats store dir: %w", err)
	}
	opts := &natsserver.Options{
		Host:       "127.0.0.1", // loopback only
		Port:       port,
		NoLog:      true,
		NoSigs:     true,
		JetStream:  true,
		StoreDir:   dir,
		MaxPayload: 8 * 1024 * 1024,
	}
	srv, err := natsserver.NewServer(opts)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("tck: new nats server: %w", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(natsReadyTimeout) {
		srv.Shutdown()
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("tck: embedded nats not ready within %s", natsReadyTimeout)
	}
	return &EmbeddedNATS{srv: srv, storeDir: dir}, nil
}

// URL returns the loopback client URL.
func (e *EmbeddedNATS) URL() string { return e.srv.ClientURL() }

// Port returns the bound TCP port.
func (e *EmbeddedNATS) Port() int {
	if addr, ok := e.srv.Addr().(*net.TCPAddr); ok {
		return addr.Port
	}
	return 0
}

// StoreDir exposes the temp store for residual-data assertions.
func (e *EmbeddedNATS) StoreDir() string { return e.storeDir }

// Close shuts the server down and removes its store deterministically. It is
// idempotent, so a manual mid-test Close plus a deferred cleanup Close is safe.
func (e *EmbeddedNATS) Close() {
	if e.closed {
		return
	}
	e.closed = true
	e.srv.Shutdown()
	e.srv.WaitForShutdown()
	_ = os.RemoveAll(e.storeDir)
}
