package worker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockNATSConn struct {
	published     chan *nats.Msg
	subscriptions map[string]nats.MsgHandler
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
		NATS:     nc,
		Subject:  "test.worker",
		SenderID: "test-worker",
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
	unsignedData, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		t.Fatalf("Failed to marshal unsigned packet: %v", err)
	}
	hash := sha256.Sum256(unsignedData)
	signature, err := ecdsa.SignASN1(rand.Reader, clientKey, hash[:])
	if err != nil {
		t.Fatalf("Failed to sign packet: %v", err)
	}
	packet.Signature = signature
	signedData, err := capsdk.MarshalDeterministic(packet)
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

	if err := capsdk.VerifyPacketSignature(&resultPacket, &workerKey.PublicKey); err != nil {
		t.Fatalf("Result signature verification failed: %v", err)
	}

	if resultPacket.GetJobResult().GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Errorf("Unexpected job status: %v", resultPacket.GetJobResult().GetStatus())
	}
}

func TestHeartbeatPayloadsPassBusPacketValidation(t *testing.T) {
	builders := []struct {
		name  string
		build func() ([]byte, error)
	}{
		{
			name: "HeartbeatPayload",
			build: func() ([]byte, error) {
				return HeartbeatPayload("w-1", "pool-a", 1, 4, 12.5)
			},
		},
		{
			name: "HeartbeatPayloadWithMemory",
			build: func() ([]byte, error) {
				return HeartbeatPayloadWithMemory("w-1", "pool-a", 1, 4, 12.5, 33.3)
			},
		},
		{
			name: "HeartbeatPayloadWithProgress",
			build: func() ([]byte, error) {
				return HeartbeatPayloadWithProgress("w-1", "pool-a", 1, 4, 12.5, 33.3, 50, "halfway")
			},
		},
	}

	for _, tc := range builders {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.build()
			if err != nil {
				t.Fatalf("%s failed: %v", tc.name, err)
			}
			pkt := decodeBusPacket(t, data)

			if err := capsdk.ValidateBusPacket(pkt); err != nil {
				t.Fatalf("ValidateBusPacket rejected heartbeat payload: %v", err)
			}
			if pkt.GetTraceId() != "w-1" {
				t.Fatalf("TraceId = %q, want %q", pkt.GetTraceId(), "w-1")
			}
			if pkt.GetCreatedAt() == nil {
				t.Fatal("CreatedAt is nil")
			}
			if pkt.GetHeartbeat().GetWorkerId() != "w-1" {
				t.Fatalf("Heartbeat.WorkerId = %q, want w-1", pkt.GetHeartbeat().GetWorkerId())
			}
		})
	}
}

func TestProgressAndCancelPayloadsPassBusPacketValidation(t *testing.T) {
	cases := []struct {
		name        string
		build       func() ([]byte, error)
		wantTraceID string
	}{
		{
			name: "ProgressPayload",
			build: func() ([]byte, error) {
				return ProgressPayload("worker-1", "job-1", "step-1", 50, "halfway")
			},
			wantTraceID: "job-1",
		},
		{
			name: "CancelPayload",
			build: func() ([]byte, error) {
				return CancelPayload("worker-1", "job-1", "timeout", "user-7")
			},
			wantTraceID: "job-1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.build()
			if err != nil {
				t.Fatalf("%s failed: %v", tc.name, err)
			}
			pkt := decodeBusPacket(t, data)

			if err := capsdk.ValidateBusPacket(pkt); err != nil {
				t.Fatalf("ValidateBusPacket rejected payload: %v", err)
			}
			if pkt.GetTraceId() != tc.wantTraceID {
				t.Fatalf("TraceId = %q, want %q", pkt.GetTraceId(), tc.wantTraceID)
			}
			if pkt.GetCreatedAt() == nil {
				t.Fatal("CreatedAt is nil")
			}
		})
	}
}

func decodeBusPacket(t *testing.T, data []byte) *agentv1.BusPacket {
	t.Helper()
	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	return &pkt
}

func TestProgressPayload(t *testing.T) {
	data, err := ProgressPayload("worker-1", "job-1", "step-1", 50, "halfway done")
	if err != nil {
		t.Fatalf("ProgressPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if pkt.GetSenderId() != "worker-1" {
		t.Errorf("SenderId = %q, want %q", pkt.GetSenderId(), "worker-1")
	}
	if pkt.GetProtocolVersion() != capsdk.DefaultProtocolVersion {
		t.Errorf("ProtocolVersion = %d, want %d", pkt.GetProtocolVersion(), capsdk.DefaultProtocolVersion)
	}
	if pkt.GetCreatedAt() == nil {
		t.Error("CreatedAt is nil")
	}

	jp := pkt.GetJobProgress()
	if jp == nil {
		t.Fatal("JobProgress is nil")
	}
	if jp.GetJobId() != "job-1" {
		t.Errorf("JobId = %q, want %q", jp.GetJobId(), "job-1")
	}
	if jp.GetStepId() != "step-1" {
		t.Errorf("StepId = %q, want %q", jp.GetStepId(), "step-1")
	}
	if jp.GetPercent() != 50 {
		t.Errorf("Percent = %d, want %d", jp.GetPercent(), 50)
	}
	if jp.GetMessage() != "halfway done" {
		t.Errorf("Message = %q, want %q", jp.GetMessage(), "halfway done")
	}
}

func TestHeartbeatPayloadWithAuthToken(t *testing.T) {
	data, err := HeartbeatPayload("worker-auth", "pool-auth", 1, 4, 42.0, WithAuthToken(" token-123 "))
	if err != nil {
		t.Fatalf("HeartbeatPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	hb := pkt.GetHeartbeat()
	if hb == nil {
		t.Fatal("Heartbeat is nil")
	}
	if hb.GetAuthToken() != "token-123" {
		t.Fatalf("AuthToken = %q, want %q", hb.GetAuthToken(), "token-123")
	}
}

func TestHeartbeatPayloadWithAgentName(t *testing.T) {
	data, err := HeartbeatPayload("worker-an", "pool-an", 1, 4, 42.0, WithAgentName("  Claude Code — Billing  "))
	if err != nil {
		t.Fatalf("HeartbeatPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	hb := pkt.GetHeartbeat()
	if hb == nil {
		t.Fatal("Heartbeat is nil")
	}
	// Serializes/deserializes and is sanitized (trimmed) on the way out.
	if hb.GetAgentName() != "Claude Code — Billing" {
		t.Fatalf("AgentName = %q, want %q", hb.GetAgentName(), "Claude Code — Billing")
	}
	// agent_name is a DISPLAY label, NOT an authentication authority: it must
	// never populate the worker attestation credential.
	if hb.GetAuthToken() != "" {
		t.Fatalf("WithAgentName must not set AuthToken; got %q", hb.GetAuthToken())
	}
}

func TestHeartbeatPayloadAgentNameOptionalForOldClients(t *testing.T) {
	// No WithAgentName option => empty agent_name, preserving wire-compatibility
	// with clients built before the field existed.
	data, err := HeartbeatPayload("worker-x", "pool-x", 0, 1, 0)
	if err != nil {
		t.Fatalf("HeartbeatPayload failed: %v", err)
	}
	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	hb := pkt.GetHeartbeat()
	if hb == nil {
		t.Fatal("Heartbeat is nil")
	}
	if hb.GetAgentName() != "" {
		t.Fatalf("AgentName should be empty without WithAgentName; got %q", hb.GetAgentName())
	}
}

func TestWithAgentNameEmptyIsNoop(t *testing.T) {
	// Blank/whitespace labels must not overwrite with empty noise nor panic.
	data, err := HeartbeatPayload("worker-blank", "pool-blank", 0, 1, 0, WithAgentName("   "))
	if err != nil {
		t.Fatalf("HeartbeatPayload failed: %v", err)
	}
	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if got := pkt.GetHeartbeat().GetAgentName(); got != "" {
		t.Fatalf("blank WithAgentName should yield empty AgentName; got %q", got)
	}
}

func TestProgressPayloadZeroPercent(t *testing.T) {
	data, err := ProgressPayload("worker-1", "job-1", "step-1", 0, "")
	if err != nil {
		t.Fatalf("ProgressPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	jp := pkt.GetJobProgress()
	if jp == nil {
		t.Fatal("JobProgress is nil")
	}
	if jp.GetPercent() != 0 {
		t.Errorf("Percent = %d, want 0", jp.GetPercent())
	}
	if jp.GetMessage() != "" {
		t.Errorf("Message = %q, want empty", jp.GetMessage())
	}
}

func TestCancelPayload(t *testing.T) {
	data, err := CancelPayload("worker-1", "job-1", "timeout", "user-7")
	if err != nil {
		t.Fatalf("CancelPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if pkt.GetSenderId() != "worker-1" {
		t.Errorf("SenderId = %q, want %q", pkt.GetSenderId(), "worker-1")
	}
	if pkt.GetProtocolVersion() != capsdk.DefaultProtocolVersion {
		t.Errorf("ProtocolVersion = %d, want %d", pkt.GetProtocolVersion(), capsdk.DefaultProtocolVersion)
	}

	jc := pkt.GetJobCancel()
	if jc == nil {
		t.Fatal("JobCancel is nil")
	}
	if jc.GetJobId() != "job-1" {
		t.Errorf("JobId = %q, want %q", jc.GetJobId(), "job-1")
	}
	if jc.GetReason() != "timeout" {
		t.Errorf("Reason = %q, want %q", jc.GetReason(), "timeout")
	}
	if jc.GetRequestedBy() != "user-7" {
		t.Errorf("RequestedBy = %q, want %q", jc.GetRequestedBy(), "user-7")
	}
}

func TestCancelPayloadEmptyReason(t *testing.T) {
	data, err := CancelPayload("worker-1", "job-1", "", "")
	if err != nil {
		t.Fatalf("CancelPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	jc := pkt.GetJobCancel()
	if jc == nil {
		t.Fatal("JobCancel is nil")
	}
	if jc.GetReason() != "" {
		t.Errorf("Reason = %q, want empty", jc.GetReason())
	}
}

func TestProgressSigningRoundtrip(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	data, err := ProgressPayload("worker-1", "job-1", "step-1", 50, "halfway")
	if err != nil {
		t.Fatalf("ProgressPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if err := capsdk.SignPacket(&pkt, key); err != nil {
		t.Fatalf("SignPacket failed: %v", err)
	}

	if err := capsdk.VerifyPacketSignature(&pkt, &key.PublicKey); err != nil {
		t.Fatalf("VerifyPacketSignature failed: %v", err)
	}
}

func TestProgressCancelIntegrationNATS(t *testing.T) {
	natsURL := os.Getenv("CAP_TEST_NATS_URL")
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}

	nc, err := nats.Connect(natsURL, nats.Timeout(2*time.Second))
	if err != nil {
		t.Skipf("NATS unavailable at %s: %v", natsURL, err)
	}
	defer nc.Drain()

	progressCh := make(chan *nats.Msg, 1)
	cancelCh := make(chan *nats.Msg, 1)

	if _, err := nc.Subscribe(capsdk.SubjectProgress, func(msg *nats.Msg) {
		progressCh <- msg
	}); err != nil {
		t.Fatalf("Subscribe progress failed: %v", err)
	}
	if _, err := nc.Subscribe(capsdk.SubjectCancel, func(msg *nats.Msg) {
		cancelCh <- msg
	}); err != nil {
		t.Fatalf("Subscribe cancel failed: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	progData, err := ProgressPayload("worker-int", "job-int", "step-int", 42, "processing")
	if err != nil {
		t.Fatalf("ProgressPayload failed: %v", err)
	}
	if err := EmitProgress(nc, progData); err != nil {
		t.Fatalf("EmitProgress failed: %v", err)
	}

	select {
	case msg := <-progressCh:
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &pkt); err != nil {
			t.Fatalf("Unmarshal progress failed: %v", err)
		}
		if pkt.GetJobProgress().GetJobId() != "job-int" {
			t.Errorf("JobId = %q, want %q", pkt.GetJobProgress().GetJobId(), "job-int")
		}
		if pkt.GetJobProgress().GetPercent() != 42 {
			t.Errorf("Percent = %d, want 42", pkt.GetJobProgress().GetPercent())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for progress message")
	}

	cancData, err := CancelPayload("worker-int", "job-int", "user-abort", "admin-int")
	if err != nil {
		t.Fatalf("CancelPayload failed: %v", err)
	}
	if err := EmitCancel(nc, cancData); err != nil {
		t.Fatalf("EmitCancel failed: %v", err)
	}

	select {
	case msg := <-cancelCh:
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &pkt); err != nil {
			t.Fatalf("Unmarshal cancel failed: %v", err)
		}
		if pkt.GetJobCancel().GetJobId() != "job-int" {
			t.Errorf("JobId = %q, want %q", pkt.GetJobCancel().GetJobId(), "job-int")
		}
		if pkt.GetJobCancel().GetReason() != "user-abort" {
			t.Errorf("Reason = %q, want %q", pkt.GetJobCancel().GetReason(), "user-abort")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for cancel message")
	}
}

func TestCancelSigningRoundtrip(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	data, err := CancelPayload("worker-1", "job-1", "timeout", "user-7")
	if err != nil {
		t.Fatalf("CancelPayload failed: %v", err)
	}

	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if err := capsdk.SignPacket(&pkt, key); err != nil {
		t.Fatalf("SignPacket failed: %v", err)
	}

	if err := capsdk.VerifyPacketSignature(&pkt, &key.PublicKey); err != nil {
		t.Fatalf("VerifyPacketSignature failed: %v", err)
	}
}
