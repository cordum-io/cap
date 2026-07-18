package worker

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type managedTrustScheduler struct {
	t                *testing.T
	config           *capsdk.WorkerTrustConfig
	schedulerKey     *ecdsa.PrivateKey
	schedulerKeyID   string
	tokenTTL         time.Duration
	invalidChallenge bool
	failAuthFrom     int

	mu             sync.Mutex
	challengeCalls int
	authenticates  []*agentv1.BusPacket
	requests       []*agentv1.BusPacket
}

func newManagedTrustScheduler(t *testing.T, natsURL, workerID string) *managedTrustScheduler {
	t.Helper()
	schedulerKey := managedP256Key(t)
	config := managedTrustConfig(t, workerID)
	config.SchedulerPublicKeys = map[string]*ecdsa.PublicKey{"scheduler-trust-key": &schedulerKey.PublicKey}
	server := &managedTrustScheduler{
		t: t, config: config, schedulerKey: schedulerKey,
		schedulerKeyID: "scheduler-trust-key", tokenTTL: time.Hour,
	}
	server.subscribe(natsURL)
	return server
}

func (s *managedTrustScheduler) subscribe(natsURL string) {
	nc := testNATSConn(s.t, natsURL)
	challenge, err := nc.Subscribe(capsdk.WorkerHandshakeChallengeSubject, s.handleChallenge)
	if err != nil {
		s.t.Fatalf("subscribe challenge: %v", err)
	}
	s.t.Cleanup(func() { _ = challenge.Unsubscribe() })
	authenticate, err := nc.Subscribe(capsdk.WorkerHandshakeAuthenticateSubject, s.handleAuthenticate)
	if err != nil {
		s.t.Fatalf("subscribe authenticate: %v", err)
	}
	s.t.Cleanup(func() { _ = authenticate.Unsubscribe() })
	if err := nc.Flush(); err != nil {
		s.t.Fatalf("flush trust responders: %v", err)
	}
}

func (s *managedTrustScheduler) handleChallenge(message *nats.Msg) {
	s.mu.Lock()
	s.challengeCalls++
	s.mu.Unlock()
	if s.invalidChallenge {
		_ = message.Respond([]byte("invalid challenge"))
		return
	}
	request, err := capsdk.UnmarshalWorkerTrustPacket(message.Data)
	if err != nil {
		s.t.Errorf("decode challenge request: %v", err)
		return
	}
	workerKeys := map[string]*ecdsa.PublicKey{s.config.ProofKeyID: &s.config.ProofPrivateKey.PublicKey}
	if err := capsdk.VerifyTrustHandshake(request, workerKeys); err != nil {
		s.t.Errorf("verify challenge request: %v", err)
		return
	}
	s.mu.Lock()
	s.requests = append(s.requests, proto.Clone(request).(*agentv1.BusPacket))
	sequence := len(s.requests)
	s.mu.Unlock()
	response := s.challengeResponse(request, sequence)
	if err := message.Respond(s.mustMarshal(response)); err != nil {
		s.t.Errorf("respond challenge: %v", err)
	}
}

func (s *managedTrustScheduler) challengeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.challengeCalls
}

func (s *managedTrustScheduler) challengeResponse(request *agentv1.BusPacket, sequence int) *agentv1.BusPacket {
	now := time.Now().UTC()
	source := request.GetWorkerHandshakeChallengeRequest()
	challenge := &agentv1.WorkerHandshakeChallenge{
		RequestId: source.RequestId, ChallengeId: fmt.Sprintf("challenge-%d", sequence), TraceId: source.TraceId,
		WorkerId: source.WorkerId, AgentId: s.config.ExpectedAgentID, TenantId: s.config.TenantID,
		ProofKeyId: source.ProofKeyId, ProofAlgorithm: source.ProofAlgorithm, ServerKeyId: s.schedulerKeyID,
		Audience: source.Audience, Purpose: source.Purpose, ClientNonce: append([]byte(nil), source.ClientNonce...),
		ServerNonce:     bytes.Repeat([]byte{byte(sequence)}, capsdk.WorkerHandshakeNonceSize),
		ProtocolVersion: source.ProtocolVersion, SdkVersion: source.SdkVersion,
		IssuedAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(30 * time.Second)),
	}
	packet := &agentv1.BusPacket{
		TraceId: challenge.TraceId, SenderId: s.config.ExpectedSchedulerID,
		CreatedAt: timestamppb.New(now), ProtocolVersion: capsdk.WorkerHandshakeProtocolVersion,
		Payload: &agentv1.BusPacket_WorkerHandshakeChallenge{WorkerHandshakeChallenge: challenge},
	}
	s.sign(packet)
	return packet
}

func (s *managedTrustScheduler) handleAuthenticate(message *nats.Msg) {
	packet, err := capsdk.UnmarshalWorkerTrustPacket(message.Data)
	if err != nil {
		s.t.Errorf("decode authenticate: %v", err)
		return
	}
	workerKeys := map[string]*ecdsa.PublicKey{s.config.ProofKeyID: &s.config.ProofPrivateKey.PublicKey}
	if err := capsdk.VerifyTrustHandshake(packet, workerKeys); err != nil {
		s.t.Errorf("verify authenticate: %v", err)
		return
	}
	s.mu.Lock()
	s.authenticates = append(s.authenticates, proto.Clone(packet).(*agentv1.BusPacket))
	sequence := len(s.authenticates)
	s.mu.Unlock()
	if s.failAuthFrom > 0 && sequence >= s.failAuthFrom {
		_ = message.Respond([]byte("invalid authenticate result"))
		return
	}
	response := s.resultResponse(packet.GetWorkerHandshakeAuthenticate().GetChallenge(), sequence)
	if err := message.Respond(s.mustMarshal(response)); err != nil {
		s.t.Errorf("respond authenticate: %v", err)
	}
}

func (s *managedTrustScheduler) resultResponse(challenge *agentv1.WorkerHandshakeChallenge, sequence int) *agentv1.BusPacket {
	now := time.Now().UTC()
	packet := &agentv1.BusPacket{
		TraceId: challenge.TraceId, SenderId: s.config.ExpectedSchedulerID,
		CreatedAt: timestamppb.New(now), ProtocolVersion: capsdk.WorkerHandshakeProtocolVersion,
		AuthToken: fmt.Sprintf("session-%d", sequence),
		Payload: &agentv1.BusPacket_WorkerHandshakeResult{WorkerHandshakeResult: &agentv1.WorkerHandshakeResult{
			Challenge: proto.Clone(challenge).(*agentv1.WorkerHandshakeChallenge), Accepted: true,
			IssuedAt: timestamppb.New(now), TokenExpiresAt: timestamppb.New(now.Add(s.tokenTTL)),
		}},
	}
	s.sign(packet)
	return packet
}

func (s *managedTrustScheduler) sign(packet *agentv1.BusPacket) {
	if err := capsdk.SignTrustHandshake(packet, s.schedulerKey); err != nil {
		s.t.Fatalf("sign scheduler packet: %v", err)
	}
}

func (s *managedTrustScheduler) mustMarshal(packet *agentv1.BusPacket) []byte {
	data, err := capsdk.MarshalWorkerTrustPacket(packet)
	if err != nil {
		s.t.Fatalf("marshal scheduler packet: %v", err)
	}
	return data
}

func (s *managedTrustScheduler) authenticateAt(index int) *agentv1.BusPacket {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= len(s.authenticates) {
		return nil
	}
	return proto.Clone(s.authenticates[index]).(*agentv1.BusPacket)
}

func awaitManagedCondition(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for managed worker condition")
}

func subscribeManagedCapture(t *testing.T, connection *nats.Conn, subject string, output chan<- *agentv1.BusPacket) {
	t.Helper()
	subscription, err := connection.Subscribe(subject, captureManagedPacket(output))
	if err != nil {
		t.Fatalf("subscribe %s: %v", subject, err)
	}
	t.Cleanup(func() { _ = subscription.Unsubscribe() })
	if err := connection.Flush(); err != nil {
		t.Fatalf("flush %s subscription: %v", subject, err)
	}
}

func runManagedWorker(t *testing.T, worker *ManagedWorker, ctx context.Context, handler Handler) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx, handler) }()
	return done
}

func awaitManagedAuthenticate(t *testing.T, scheduler *managedTrustScheduler, index int) *agentv1.BusPacket {
	t.Helper()
	var packet *agentv1.BusPacket
	awaitManagedCondition(t, func() bool {
		packet = scheduler.authenticateAt(index)
		return packet != nil
	})
	return packet
}

func assertManagedSessionEnvelope(t *testing.T, packet *agentv1.BusPacket, token string) {
	t.Helper()
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		t.Fatalf("managed worker emitted invalid packet: %v", err)
	}
	if packet.GetTraceId() == "" {
		t.Fatal("managed worker emitted empty trace_id")
	}
	if packet.GetAuthToken() != token {
		t.Fatalf("auth_token=%q, want %q", packet.GetAuthToken(), token)
	}
}

func publishManagedJob(t *testing.T, connection *nats.Conn, subject string, packet *agentv1.BusPacket) {
	t.Helper()
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	if err := connection.Publish(subject, data); err != nil {
		t.Fatalf("publish job: %v", err)
	}
	if err := connection.Flush(); err != nil {
		t.Fatalf("flush job: %v", err)
	}
}

func successfulManagedHandler(_ context.Context, request *agentv1.JobRequest) (*agentv1.JobResult, error) {
	return &agentv1.JobResult{
		JobId: request.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
	}, nil
}
