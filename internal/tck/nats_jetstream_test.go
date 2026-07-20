package tck

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func mustJetStream(t *testing.T, nc *nats.Conn) nats.JetStreamContext {
	t.Helper()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream context: %v", err)
	}
	return js
}

// A NAK'd JetStream message is redelivered (at-least-once), and its delivery
// count reflects the retry — the guarantee a retrying worker depends on.
func TestJetStreamAtLeastOnceRedelivery(t *testing.T) {
	nc := mustConnect(t, mustStartNATS(t))
	js := mustJetStream(t, nc)
	if _, err := js.AddStream(&nats.StreamConfig{
		Name:     "TCKREDELIVER",
		Subjects: []string{"tck.redeliver.>"},
		Storage:  nats.FileStorage,
	}); err != nil {
		t.Fatalf("add stream: %v", err)
	}
	if _, err := js.Publish("tck.redeliver.a", []byte("payload")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	sub, err := js.PullSubscribe("tck.redeliver.a", "worker", nats.AckExplicit(), nats.AckWait(time.Second))
	if err != nil {
		t.Fatalf("pull subscribe: %v", err)
	}

	first, err := sub.Fetch(1, nats.MaxWait(2*time.Second))
	if err != nil || len(first) != 1 {
		t.Fatalf("first fetch: msgs=%d err=%v", len(first), err)
	}
	if err := first[0].Nak(); err != nil {
		t.Fatalf("nak: %v", err)
	}

	second, err := sub.Fetch(1, nats.MaxWait(2*time.Second))
	if err != nil || len(second) != 1 {
		t.Fatalf("redelivery fetch: msgs=%d err=%v", len(second), err)
	}
	meta, err := second[0].Metadata()
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if meta.NumDelivered < 2 {
		t.Fatalf("expected redelivery (NumDelivered>=2), got %d", meta.NumDelivered)
	}
	if err := second[0].Ack(); err != nil {
		t.Fatalf("ack: %v", err)
	}
}

// Publishing the same Nats-Msg-Id twice inside the dedup window stores the
// message once: duplicate delivery causes no duplicate side effect.
func TestJetStreamDuplicateSuppression(t *testing.T) {
	nc := mustConnect(t, mustStartNATS(t))
	js := mustJetStream(t, nc)
	if _, err := js.AddStream(&nats.StreamConfig{
		Name:       "TCKDEDUP",
		Subjects:   []string{"tck.dedup.>"},
		Storage:    nats.FileStorage,
		Duplicates: 2 * time.Minute,
	}); err != nil {
		t.Fatalf("add stream: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := js.Publish("tck.dedup.a", []byte("once"), nats.MsgId("dup-key-1")); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	info, err := js.StreamInfo("TCKDEDUP")
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("duplicate suppression failed: stream holds %d messages, want 1", info.State.Msgs)
	}
}
