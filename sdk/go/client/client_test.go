package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	agentv1 "github.com/coretexos/cap/go/coretex/agent/v1"
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

	signature := packet.Signature
	packet.Signature = nil
	unsignedData, err := proto.Marshal(&packet)
	if err != nil {
		t.Fatalf("Failed to marshal unsigned packet: %v", err)
	}

	hash := sha256.Sum256(unsignedData)
	if !ecdsa.VerifyASN1(&privateKey.PublicKey, hash[:], signature) {
		t.Fatal("Signature verification failed")
	}
}
