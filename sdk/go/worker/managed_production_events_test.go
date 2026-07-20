package worker

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func TestManagedProductionEventsEchoAuthorityAndUseFreshSubjectBoundSignatures(t *testing.T) {
	_, natsURL := startTestNATS(t)
	observer := testNATSConn(t, natsURL)
	messages := make(chan *nats.Msg, 2)
	sub, err := observer.ChanSubscribe("sys.job.*", messages)
	if err != nil {
		t.Fatalf("subscribe production events: %v", err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	if err := observer.Flush(); err != nil {
		t.Fatalf("flush subscription: %v", err)
	}

	schedulerKey := managedP256Key(t)
	workerKey := managedP256Key(t)
	worker := managedOutboundProductionWorker(t, natsURL, schedulerKey, workerKey)
	expected := managedProductionRequest()
	raw := managedProductionRequestWire(t, schedulerKey, expected.GetTopic(), "message-id-00003")
	worker.handleManagedMessage(context.Background(), &nats.Msg{Subject: expected.GetTopic(), Data: raw}, func(
		ctx context.Context, request *agentv1.JobRequest,
	) (*agentv1.JobResult, error) {
		request.Identity.TenantId = "handler-mutated-tenant"
		request.Dispatch.DispatchId = "handler-mutated-dispatch"
		if err := PublishManagedProgress(ctx, &agentv1.JobProgress{Percent: 25, Message: "working"}); err != nil {
			return nil, err
		}
		return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	})
	if err := worker.conn.Flush(); err != nil {
		t.Fatalf("flush events: %v", err)
	}

	seenIDs := map[string]struct{}{}
	for range 2 {
		message := awaitProductionEvent(t, messages)
		packet := verifyManagedProductionEvent(t, message, workerKey)
		assertManagedProductionEcho(t, packet, expected)
		id := hex.EncodeToString(packet.GetSignatureMetadata().GetMessageId())
		if _, exists := seenIDs[id]; exists {
			t.Fatalf("reused production message id %s", id)
		}
		seenIDs[id] = struct{}{}
	}
	if len(seenIDs) != 2 {
		t.Fatalf("unique message IDs=%d, want 2", len(seenIDs))
	}
}

func TestManagedProductionEventRejectsConflictingHandlerAuthority(t *testing.T) {
	worker := managedProductionWorkerForTest(t, managedP256Key(t))
	ctx := withManagedEventPublisher(context.Background(), worker, "trace-production", managedProductionRequest())
	progress := &agentv1.JobProgress{
		Identity: &agentv1.IdentityBinding{TenantId: "attacker"},
	}
	if err := PublishManagedProgress(ctx, progress); !errors.Is(err, capsdk.ErrIdentityMismatch) {
		t.Fatalf("conflicting handler identity error=%v, want ErrIdentityMismatch", err)
	}
}

func TestManagedCompatibilityResultBindingRemainsUnchanged(t *testing.T) {
	result := &agentv1.JobResult{JobId: "handler-job"}
	identity, err := bindManagedResult(result, managedEventAuthority{jobID: "request-job"})
	if err != nil {
		t.Fatalf("compatibility result rejected: %v", err)
	}
	if identity != nil || result.GetJobId() != "handler-job" || result.GetIdentity() != nil || result.GetDispatch() != nil {
		t.Fatalf("compatibility result changed: %v", result)
	}
}

func TestManagedProductionEventVerificationRejectsTamperAndWrongAudience(t *testing.T) {
	_, natsURL := startTestNATS(t)
	observer := testNATSConn(t, natsURL)
	messages := make(chan *nats.Msg, 1)
	sub, err := observer.ChanSubscribe(capsdk.SubjectResult, messages)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	if err := observer.Flush(); err != nil {
		t.Fatal(err)
	}
	workerKey := managedP256Key(t)
	worker := managedOutboundProductionWorker(t, natsURL, managedP256Key(t), workerKey)
	if err := worker.publishManagedResultForRequest("trace-production", managedProductionRequest(), &agentv1.JobResult{
		WorkerId: "worker-production", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.conn.Flush(); err != nil {
		t.Fatal(err)
	}
	message := awaitProductionEvent(t, messages)
	trust := managedOutboundTrust(workerKey, capsdk.SubjectResult)
	if _, err := capsdk.VerifyProductionPacket(message.Data, trust); err != nil {
		t.Fatalf("verify untampered result: %v", err)
	}
	trust.Audience = capsdk.SubjectProgress
	if _, err := capsdk.VerifyProductionPacket(message.Data, trust); !errors.Is(err, capsdk.ErrAudienceMismatch) {
		t.Fatalf("wrong audience error=%v, want ErrAudienceMismatch", err)
	}
	tampered := append([]byte(nil), message.Data...)
	tampered[len(tampered)/2] ^= 0x01
	if _, err := capsdk.VerifyProductionPacket(tampered, managedOutboundTrust(workerKey, capsdk.SubjectResult)); err == nil {
		t.Fatal("tampered managed result verified")
	}
}

func managedOutboundProductionWorker(
	t *testing.T, natsURL string, schedulerKey, workerKey *ecdsa.PrivateKey,
) *ManagedWorker {
	t.Helper()
	worker := managedProductionWorkerForTest(t, schedulerKey)
	worker.cfg.PrivateKey = workerKey
	worker.conn = testNATSConn(t, natsURL)
	return worker
}

func managedProductionRequest() *agentv1.JobRequest {
	identity := &agentv1.IdentityBinding{TenantId: "tenant-trust", PrincipalId: "principal-1", ActorId: "actor-1"}
	return &agentv1.JobRequest{
		JobId: "job-production", Topic: "job.production.real", Identity: identity,
		Dispatch: &agentv1.DispatchIdentity{DispatchId: "dispatch-1", Attempt: 1, AssignedWorkerId: "worker-production"},
	}
}

func awaitProductionEvent(t *testing.T, messages <-chan *nats.Msg) *nats.Msg {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for managed production event")
		return nil
	}
}

func verifyManagedProductionEvent(t *testing.T, message *nats.Msg, key *ecdsa.PrivateKey) *agentv1.BusPacket {
	t.Helper()
	packet, err := capsdk.VerifyProductionPacket(message.Data, managedOutboundTrust(key, message.Subject))
	if err != nil {
		t.Fatalf("verify %s: %v", message.Subject, err)
	}
	metadata := packet.GetSignatureMetadata()
	if len(metadata.GetMessageId()) != 16 || metadata.GetKeyId() != "worker-key" {
		t.Fatalf("invalid signature metadata: %v", metadata)
	}
	if metadata.GetAudience() != message.Subject || !metadata.GetExpiresAt().AsTime().After(time.Now()) {
		t.Fatalf("metadata not bound to subject/expiry: %v", metadata)
	}
	return packet
}

func managedOutboundTrust(key *ecdsa.PrivateKey, audience string) capsdk.ProductionTrustStore {
	return capsdk.ProductionTrustStore{
		Audience: audience, Tenant: "tenant-trust", Sender: "worker-production",
		PublicKeys: map[string]*ecdsa.PublicKey{"worker-key": &key.PublicKey},
	}
}

func assertManagedProductionEcho(t *testing.T, packet *agentv1.BusPacket, request *agentv1.JobRequest) {
	t.Helper()
	var identity *agentv1.IdentityBinding
	var dispatch *agentv1.DispatchIdentity
	switch {
	case packet.GetJobResult() != nil:
		identity, dispatch = packet.GetJobResult().GetIdentity(), packet.GetJobResult().GetDispatch()
	case packet.GetJobProgress() != nil:
		identity, dispatch = packet.GetJobProgress().GetIdentity(), packet.GetJobProgress().GetDispatch()
	default:
		t.Fatalf("unexpected production payload %T", packet.GetPayload())
	}
	if !proto.Equal(identity, request.GetIdentity()) || !proto.Equal(dispatch, request.GetDispatch()) {
		t.Fatalf("event authority not echoed: identity=%v dispatch=%v", identity, dispatch)
	}
	if !proto.Equal(packet.GetIdentity(), request.GetIdentity()) {
		t.Fatalf("envelope identity not echoed: %v", packet.GetIdentity())
	}
}
