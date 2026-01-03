package worker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/coretexos/cap/v2/sdk/go"
	agentv1 "github.com/coretexos/cap/v2/coretex/agent/v1"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockNATSConn struct {
	published      chan *nats.Msg
	subscriptions  map[string]nats.MsgHandler
}

func (m *mockNATSConn) Publish(subject string, data []byte) error {
	msg := &nats.Msg{Subject: subject, Data: data}
	m.published <- msg
	return nil
}

func (m *mockNATSConn) QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	m.subscriptions[subj] = cb
	return &nats.Subscription{}, nil
}

func TestWorkerE2E(t *testing.T) {
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	workerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate worker key: %v", err)
	}

	nc := &mockNATSConn{
		published:     make(chan *nats.Msg, 1),
		subscriptions: make(map[string]nats.MsgHandler),
	}

	worker := &Worker{
		NATS:       nc,
		Subject:    "test.worker",
		SenderID:   "test-worker",
		Handler: func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		},
		PublicKeys: map[string]*ecdsa.PublicKey{"test-client": &clientKey.PublicKey},
		PrivateKey: workerKey,
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("worker.Start() failed: %v", err)
	}

	// Create and sign a job request from a "client"
	req := &agentv1.JobRequest{
		JobId: "test-job-1",
		Topic: "test.worker",
	}
	packet := &agentv1.BusPacket{
		TraceId:         "test-trace",
		SenderId:        "test-client",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	unsignedData, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("Failed to marshal unsigned packet: %v", err)
	}
	hash := sha256.Sum256(unsignedData)
	signature, err := ecdsa.SignASN1(rand.Reader, clientKey, hash[:])
	if err != nil {
		t.Fatalf("Failed to sign packet: %v", err)
	}
	packet.Signature = signature
	signedData, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("Failed to marshal signed packet: %v", err)
	}

	// "Publish" the message to the worker
	handler, ok := nc.subscriptions["test.worker"]
	if !ok {
		t.Fatal("Worker did not subscribe to the subject")
	}
	handler(&nats.Msg{Subject: "test.worker", Data: signedData})

	// Wait for the worker to publish the result
	var resultMsg *nats.Msg
	select {
	case resultMsg = <-nc.published:
	case <-time.After(1 * time.Second):
		t.Fatal("Worker did not publish a result")
	}

	var resultPacket agentv1.BusPacket
	if err := proto.Unmarshal(resultMsg.Data, &resultPacket); err != nil {
		t.Fatalf("Failed to unmarshal result packet: %v", err)
	}

	// Verify the signature of the result packet
	resultSignature := resultPacket.Signature
	resultPacket.Signature = nil
	unsignedResultData, err := proto.Marshal(&resultPacket)
	if err != nil {
		t.Fatalf("Failed to marshal unsigned result packet: %v", err)
	}
	resultHash := sha256.Sum256(unsignedResultData)
	if !ecdsa.VerifyASN1(&workerKey.PublicKey, resultHash[:], resultSignature) {
		t.Fatal("Result signature verification failed")
	}

	if resultPacket.GetJobResult().GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Errorf("Unexpected job status: %v", resultPacket.GetJobResult().GetStatus())
	}
}
