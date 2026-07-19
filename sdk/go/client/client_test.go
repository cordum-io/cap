package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type mockNATSPublisher struct {
	published chan []byte
}

func (m *mockNATSPublisher) Publish(subject string, data []byte) error {
	m.published <- data
	return nil
}

func (m *mockNATSPublisher) Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error) {
	return nil, nil
}

func (m *mockNATSPublisher) QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	return nil, nil
}

func TestSubmitSigned(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	nc := &mockNATSPublisher{published: make(chan []byte, 1)}

	req := &agentv1.JobRequest{
		JobId: "test-job-1",
		Topic: "test.topic",
	}

	go func() {
		if err := Submit(context.Background(), nc, req, "test-trace", "test-sender", privateKey); err != nil {
			t.Errorf("Submit failed: %v", err)
		}
	}()

	data := <-nc.published
	var packet agentv1.BusPacket
	if err := proto.Unmarshal(data, &packet); err != nil {
		t.Fatalf("Failed to unmarshal packet: %v", err)
	}

	if packet.Signature == nil {
		t.Fatal("Signature is nil")
	}
	if err := capsdk.VerifyPacketSignature(&packet, &privateKey.PublicKey); err != nil {
		t.Fatalf("Signature verification failed: %v", err)
	}
}

func TestSubmit_CanceledContextDoesNotPublish(t *testing.T) {
	nc := &mockNATSPublisher{published: make(chan []byte, 1)}
	req := &agentv1.JobRequest{
		JobId: "test-job-canceled",
		Topic: "test.topic",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Submit(ctx, nc, req, "test-trace", "test-sender", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Submit error = %v, want errors.Is(err, context.Canceled)", err)
	}
	if got := len(nc.published); got != 0 {
		t.Errorf("Submit published %d packets after ctx cancellation, want 0", got)
	}
}

func TestSubmit_DeadlineExceededDoesNotPublish(t *testing.T) {
	nc := &mockNATSPublisher{published: make(chan []byte, 1)}
	req := &agentv1.JobRequest{
		JobId: "test-job-deadline",
		Topic: "test.topic",
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	err := Submit(ctx, nc, req, "test-trace", "test-sender", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Submit error = %v, want errors.Is(err, context.DeadlineExceeded)", err)
	}
	if got := len(nc.published); got != 0 {
		t.Errorf("Submit published %d packets after deadline expiry, want 0", got)
	}
}

func TestClientSubmit_CanceledContextDoesNotPublish(t *testing.T) {
	// Client.NATS is nil: if the ctx guard runs before any NATS use, Submit
	// returns the ctx error cleanly; otherwise it attempts a publish.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Client.Submit panicked instead of returning the ctx error: %v", r)
		}
	}()

	c := &Client{NATS: nil}
	req := &agentv1.JobRequest{
		JobId: "test-job-client-canceled",
		Topic: "test.topic",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Submit(ctx, req, "test-trace", "test-sender", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Client.Submit error = %v, want errors.Is(err, context.Canceled)", err)
	}
}

func TestSubmit_NilContextStillPublishes(t *testing.T) {
	nc := &mockNATSPublisher{published: make(chan []byte, 1)}
	req := &agentv1.JobRequest{
		JobId: "test-job-nil-ctx",
		Topic: "test.topic",
	}

	var nilCtx context.Context
	if err := SubmitUnsigned(nilCtx, nc, req, "test-trace", "test-sender"); err != nil {
		t.Fatalf("Submit with nil ctx failed: %v", err)
	}
	if got := len(nc.published); got != 1 {
		t.Fatalf("Submit with nil ctx published %d packets, want exactly 1", got)
	}
}

func TestSubmitRejectsImplicitUnsignedPacket(t *testing.T) {
	nc := &mockNATSPublisher{published: make(chan []byte, 1)}
	err := Submit(context.Background(), nc, &agentv1.JobRequest{JobId: "job-unsigned", Topic: "test.topic"},
		"trace-unsigned", "test-sender", nil)
	if err == nil || !strings.Contains(err.Error(), "SubmitUnsigned") {
		t.Fatalf("Submit error = %v, want explicit unsigned opt-in requirement", err)
	}
	if len(nc.published) != 0 {
		t.Fatal("implicit unsigned Submit published a packet")
	}
}
