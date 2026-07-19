package tck

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// mustStartNATS starts the embedded server or FAILS the test. There is no skip:
// a missing transport is a failure, never a silent pass. This is the property
// the whole step exists to guarantee.
func mustStartNATS(t *testing.T) *EmbeddedNATS {
	t.Helper()
	n, err := StartEmbeddedNATS()
	if err != nil {
		t.Fatalf("embedded NATS must start (no skip permitted): %v", err)
	}
	t.Cleanup(n.Close)
	return n
}

func mustConnect(t *testing.T, n *EmbeddedNATS, opts ...nats.Option) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(n.URL(), append([]nats.Option{nats.Timeout(2 * time.Second)}, opts...)...)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	return nc
}

func waitFor(t *testing.T, d time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s: %s", d, msg)
}

func TestEmbeddedNATSRoundTrips(t *testing.T) {
	nc := mustConnect(t, mustStartNATS(t))
	sub, err := nc.SubscribeSync("sys.job.result")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := nc.Publish("sys.job.result", []byte("ok")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	msg, err := sub.NextMsg(2 * time.Second)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if string(msg.Data) != "ok" {
		t.Fatalf("got %q", msg.Data)
	}
}

// A queue group delivers each message to exactly one member: the total received
// equals the total published, with no duplication.
func TestQueueGroupSingleDelivery(t *testing.T) {
	nc := mustConnect(t, mustStartNATS(t))
	var a, b int64
	if _, err := nc.QueueSubscribe("sys.job.submit", "pool", func(*nats.Msg) { atomic.AddInt64(&a, 1) }); err != nil {
		t.Fatalf("qsub a: %v", err)
	}
	if _, err := nc.QueueSubscribe("sys.job.submit", "pool", func(*nats.Msg) { atomic.AddInt64(&b, 1) }); err != nil {
		t.Fatalf("qsub b: %v", err)
	}
	const n = 20
	for i := 0; i < n; i++ {
		_ = nc.Publish("sys.job.submit", []byte("job"))
	}
	_ = nc.Flush()
	waitFor(t, 2*time.Second, func() bool { return atomic.LoadInt64(&a)+atomic.LoadInt64(&b) == n }, "all queued messages delivered once")
	if got := atomic.LoadInt64(&a) + atomic.LoadInt64(&b); got != n {
		t.Fatalf("queue delivered %d, want exactly %d (no duplication)", got, n)
	}
}

// A broadcast subject reaches every plain subscriber, as heartbeat/handshake
// discovery relies on.
func TestBroadcastReachesAllSubscribers(t *testing.T) {
	nc := mustConnect(t, mustStartNATS(t))
	var a, b int64
	if _, err := nc.Subscribe("sys.heartbeat", func(*nats.Msg) { atomic.AddInt64(&a, 1) }); err != nil {
		t.Fatalf("sub a: %v", err)
	}
	if _, err := nc.Subscribe("sys.heartbeat", func(*nats.Msg) { atomic.AddInt64(&b, 1) }); err != nil {
		t.Fatalf("sub b: %v", err)
	}
	_ = nc.Publish("sys.heartbeat", []byte("beat"))
	_ = nc.Flush()
	waitFor(t, 2*time.Second, func() bool { return atomic.LoadInt64(&a) == 1 && atomic.LoadInt64(&b) == 1 }, "both subscribers see the broadcast")
}

func TestRequestReplyNegotiation(t *testing.T) {
	nc := mustConnect(t, mustStartNATS(t))
	if _, err := nc.Subscribe("sys.negotiate", func(m *nats.Msg) { _ = m.Respond([]byte("v1")) }); err != nil {
		t.Fatalf("responder: %v", err)
	}
	reply, err := nc.Request("sys.negotiate", []byte("hello"), 2*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if string(reply.Data) != "v1" {
		t.Fatalf("negotiated reply %q", reply.Data)
	}
}

func TestRequestTimesOutWithNoResponder(t *testing.T) {
	nc := mustConnect(t, mustStartNATS(t))
	if _, err := nc.Request("sys.no.responder", []byte("x"), 300*time.Millisecond); err == nil {
		t.Fatal("expected a timeout/no-responder error, got nil")
	}
}

// A client with reconnect enabled recovers when the server returns at the same
// address, and its subscriptions are automatically re-established.
func TestReconnectAndResubscribe(t *testing.T) {
	n1, err := StartEmbeddedNATS()
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(n1.Close) // idempotent; guards against a Fatalf before the manual close
	port := n1.Port()
	reconnected := make(chan struct{}, 1)
	nc := mustConnect(t, n1,
		nats.ReconnectWait(100*time.Millisecond),
		nats.MaxReconnects(-1),
		nats.ReconnectHandler(func(*nats.Conn) {
			select {
			case reconnected <- struct{}{}:
			default:
			}
		}),
	)
	sub, err := nc.SubscribeSync("sys.heartbeat")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	n1.Close() // kill the server under the client

	n2, err := StartEmbeddedNATSOnPort(port)
	if err != nil {
		t.Fatalf("restart on port %d: %v", port, err)
	}
	defer n2.Close()

	select {
	case <-reconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("client did not reconnect")
	}
	waitFor(t, 2*time.Second, func() bool { return nc.IsConnected() }, "reconnected and live")
	if err := nc.Publish("sys.heartbeat", []byte("beat")); err != nil {
		t.Fatalf("publish after reconnect: %v", err)
	}
	_ = nc.Flush()
	if msg, err := sub.NextMsg(2 * time.Second); err != nil || string(msg.Data) != "beat" {
		t.Fatalf("re-subscribed delivery failed: msg=%v err=%v", msg, err)
	}
}

// Two servers must bind distinct ports and share no messages.
func TestPortIsolationNoCrosstalk(t *testing.T) {
	a, b := mustStartNATS(t), mustStartNATS(t)
	if a.Port() == b.Port() {
		t.Fatalf("isolated servers share port %d", a.Port())
	}
	nca, ncb := mustConnect(t, a), mustConnect(t, b)
	subB, err := ncb.SubscribeSync("iso.subj")
	if err != nil {
		t.Fatalf("subscribe B: %v", err)
	}
	_ = ncb.Flush()
	_ = nca.Publish("iso.subj", []byte("only-A"))
	_ = nca.Flush()
	if _, err := subB.NextMsg(300 * time.Millisecond); err == nil {
		t.Fatal("message crossed between isolated servers")
	}
}

func TestParallelServersStartWithoutCollision(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("srv-%d", i), func(t *testing.T) {
			t.Parallel()
			mustConnect(t, mustStartNATS(t))
		})
	}
}

func TestStoreRemovedOnClose(t *testing.T) {
	n, err := StartEmbeddedNATS()
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	dir := n.StoreDir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("store should exist while running: %v", err)
	}
	n.Close()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("store dir not removed on Close: %v", err)
	}
}
