package capsdk

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWorkerTrustClientIssueLifecycle(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	request := fixture.buildRequest(t, issuePurpose())
	challengePacket := fixture.buildChallenge(t, request)
	verified, err := VerifyWorkerHandshakeChallenge(fixture.config, request, challengePacket, fixture.now)
	if err != nil {
		t.Fatalf("VerifyWorkerHandshakeChallenge: %v", err)
	}
	capability := validWorkerCapability(fixture.config)
	authenticate, err := BuildWorkerHandshakeAuthenticate(fixture.config, verified, capability, "", fixture.now)
	if err != nil {
		t.Fatalf("BuildWorkerHandshakeAuthenticate: %v", err)
	}
	resultPacket := fixture.buildResult(t, verified.Message(), "session-token", true)
	session, err := VerifyWorkerHandshakeResult(fixture.config, verified, authenticate, resultPacket, fixture.now)
	if err != nil {
		t.Fatalf("VerifyWorkerHandshakeResult: %v", err)
	}
	if session.Token != "session-token" || !session.ExpiresAt.Equal(fixture.now.Add(time.Hour)) {
		t.Fatalf("unexpected session: %+v", session)
	}
}

func TestWorkerTrustClientRenewCoversCurrentSession(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	request := fixture.buildRequest(t, renewPurpose())
	verified, err := VerifyWorkerHandshakeChallenge(fixture.config, request, fixture.buildChallenge(t, request), fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	authenticate, err := BuildWorkerHandshakeAuthenticate(
		fixture.config, verified, validWorkerCapability(fixture.config), "current-session", fixture.now,
	)
	if err != nil {
		t.Fatalf("BuildWorkerHandshakeAuthenticate: %v", err)
	}
	if authenticate.GetAuthToken() != "current-session" {
		t.Fatalf("auth_token=%q, want current session", authenticate.GetAuthToken())
	}
	if err := VerifyTrustHandshake(authenticate, map[string]*ecdsa.PublicKey{fixture.config.ProofKeyID: &fixture.config.ProofPrivateKey.PublicKey}); err != nil {
		t.Fatalf("VerifyTrustHandshake: %v", err)
	}
}

func TestVerifyWorkerHandshakeChallengeRejectsCorrelationAndUnknowns(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	request := fixture.buildRequest(t, issuePurpose())
	tests := []struct {
		name   string
		mutate func(*agentv1.BusPacket)
	}{
		{"scheduler sender", func(p *agentv1.BusPacket) { p.SenderId = "scheduler-2" }},
		{"request", func(p *agentv1.BusPacket) { p.GetWorkerHandshakeChallenge().RequestId = "other" }},
		{"agent", func(p *agentv1.BusPacket) { p.GetWorkerHandshakeChallenge().AgentId = "attacker" }},
		{"tenant", func(p *agentv1.BusPacket) { p.GetWorkerHandshakeChallenge().TenantId = "attacker" }},
		{"audience", func(p *agentv1.BusPacket) { p.GetWorkerHandshakeChallenge().Audience = "other" }},
		{"client nonce", func(p *agentv1.BusPacket) { p.GetWorkerHandshakeChallenge().ClientNonce[0] ^= 1 }},
		{"nested unknown", func(p *agentv1.BusPacket) {
			p.GetWorkerHandshakeChallenge().ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			response := fixture.buildChallenge(t, request)
			test.mutate(response)
			if test.name != "nested unknown" {
				fixture.resignScheduler(t, response)
			}
			if _, err := VerifyWorkerHandshakeChallenge(fixture.config, request, response, fixture.now); err == nil {
				t.Fatal("VerifyWorkerHandshakeChallenge accepted invalid response")
			}
		})
	}
}

func TestVerifyWorkerHandshakeResultRejectsTamperAndOpaqueRejection(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	request := fixture.buildRequest(t, issuePurpose())
	verified, err := VerifyWorkerHandshakeChallenge(fixture.config, request, fixture.buildChallenge(t, request), fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	authenticate, err := BuildWorkerHandshakeAuthenticate(fixture.config, verified, validWorkerCapability(fixture.config), "", fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	rejected := fixture.buildResult(t, verified.Message(), "", false)
	_, err = VerifyWorkerHandshakeResult(fixture.config, verified, authenticate, rejected, fixture.now)
	var rejection *WorkerHandshakeRejectionError
	if !errors.As(err, &rejection) || rejection.Reason != agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED {
		t.Fatalf("rejection error=%v", err)
	}
	tampered := fixture.buildResult(t, verified.Message(), "session-token", true)
	tampered.GetWorkerHandshakeResult().Challenge.TraceId = "other"
	fixture.resignScheduler(t, tampered)
	if _, err := VerifyWorkerHandshakeResult(fixture.config, verified, authenticate, tampered, fixture.now); err == nil {
		t.Fatal("accepted mismatched challenge")
	}
}

func TestVerifyWorkerHandshakeResultRejectsExpiredToken(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	verified, authenticate := fixture.issueAuthenticate(t)
	response := fixture.buildResult(t, verified.Message(), "session-token", true)
	result := response.GetWorkerHandshakeResult()
	result.IssuedAt = timestamppb.New(fixture.now.Add(-30 * time.Second))
	result.TokenExpiresAt = timestamppb.New(fixture.now.Add(-time.Second))
	fixture.resignScheduler(t, response)
	if _, err := VerifyWorkerHandshakeResult(fixture.config, verified, authenticate, response, fixture.now); !errors.Is(err, ErrWorkerHandshakeExpired) {
		t.Fatalf("VerifyWorkerHandshakeResult error=%v, want %v", err, ErrWorkerHandshakeExpired)
	}
}

func TestVerifyWorkerHandshakeResultRequiresRenewalRotation(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	request := fixture.buildRequest(t, renewPurpose())
	verified, err := VerifyWorkerHandshakeChallenge(fixture.config, request, fixture.buildChallenge(t, request), fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	authenticate, err := BuildWorkerHandshakeAuthenticate(fixture.config, verified, validWorkerCapability(fixture.config), "same-token", fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	response := fixture.buildResult(t, verified.Message(), "same-token", true)
	if _, err := VerifyWorkerHandshakeResult(fixture.config, verified, authenticate, response, fixture.now); !errors.Is(err, ErrWorkerHandshakeBinding) {
		t.Fatalf("VerifyWorkerHandshakeResult error=%v, want %v", err, ErrWorkerHandshakeBinding)
	}
}

func TestVerifyWorkerHandshakeResultRejectsUnknownReason(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	verified, authenticate := fixture.issueAuthenticate(t)
	response := fixture.buildResult(t, verified.Message(), "", false)
	response.GetWorkerHandshakeResult().RejectionReason = agentv1.WorkerHandshakeRejectionReason(999)
	fixture.resignScheduler(t, response)
	_, err := VerifyWorkerHandshakeResult(fixture.config, verified, authenticate, response, fixture.now)
	var rejection *WorkerHandshakeRejectionError
	if err == nil || errors.As(err, &rejection) {
		t.Fatalf("unknown rejection reason error=%v", err)
	}
}

type workerTrustClientFixture struct {
	config       *WorkerTrustConfig
	schedulerKey *ecdsa.PrivateKey
	now          time.Time
}

func newWorkerTrustClientFixture(t *testing.T) *workerTrustClientFixture {
	t.Helper()
	config := validWorkerTrustConfig(t)
	scheduler := generateWorkerTrustKey(t)
	config.SchedulerPublicKeys = map[string]*ecdsa.PublicKey{"scheduler-key-1": &scheduler.PublicKey}
	return &workerTrustClientFixture{config: config, schedulerKey: scheduler, now: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)}
}

func (f *workerTrustClientFixture) buildRequest(t *testing.T, purpose agentv1.WorkerHandshakePurpose) *agentv1.BusPacket {
	t.Helper()
	nonce := bytes.Repeat([]byte{0x42}, WorkerHandshakeNonceSize)
	packet, err := BuildWorkerHandshakeChallengeRequest(f.config, WorkerHandshakeRequestOptions{
		RequestID: "request-1", TraceID: "trace-1", Purpose: purpose, ClientNonce: nonce, CreatedAt: f.now,
	})
	if err != nil {
		t.Fatalf("BuildWorkerHandshakeChallengeRequest: %v", err)
	}
	return packet
}

func (f *workerTrustClientFixture) issueAuthenticate(t *testing.T) (*VerifiedWorkerHandshakeChallenge, *agentv1.BusPacket) {
	t.Helper()
	request := f.buildRequest(t, issuePurpose())
	verified, err := VerifyWorkerHandshakeChallenge(f.config, request, f.buildChallenge(t, request), f.now)
	if err != nil {
		t.Fatal(err)
	}
	authenticate, err := BuildWorkerHandshakeAuthenticate(f.config, verified, validWorkerCapability(f.config), "", f.now)
	if err != nil {
		t.Fatal(err)
	}
	return verified, authenticate
}

func (f *workerTrustClientFixture) buildChallenge(t *testing.T, request *agentv1.BusPacket) *agentv1.BusPacket {
	t.Helper()
	r := request.GetWorkerHandshakeChallengeRequest()
	challenge := &agentv1.WorkerHandshakeChallenge{
		RequestId: r.RequestId, ChallengeId: "challenge-1", TraceId: r.TraceId,
		WorkerId: r.WorkerId, AgentId: f.config.ExpectedAgentID, TenantId: f.config.TenantID,
		ProofKeyId: r.ProofKeyId, ProofAlgorithm: r.ProofAlgorithm, ServerKeyId: "scheduler-key-1",
		Audience: r.Audience, Purpose: r.Purpose, ClientNonce: append([]byte(nil), r.ClientNonce...),
		ServerNonce: bytes.Repeat([]byte{0x24}, WorkerHandshakeNonceSize), ProtocolVersion: r.ProtocolVersion,
		SdkVersion: r.SdkVersion, IssuedAt: timestamppb.New(f.now), ExpiresAt: timestamppb.New(f.now.Add(30 * time.Second)),
	}
	packet := &agentv1.BusPacket{
		TraceId: r.TraceId, SenderId: f.config.ExpectedSchedulerID, CreatedAt: timestamppb.New(f.now),
		ProtocolVersion: WorkerHandshakeProtocolVersion,
		Payload:         &agentv1.BusPacket_WorkerHandshakeChallenge{WorkerHandshakeChallenge: challenge},
	}
	f.resignScheduler(t, packet)
	return packet
}

func (f *workerTrustClientFixture) buildResult(t *testing.T, challenge *agentv1.WorkerHandshakeChallenge, token string, accepted bool) *agentv1.BusPacket {
	t.Helper()
	reason := agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED
	if accepted {
		reason = agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED
	}
	result := &agentv1.WorkerHandshakeResult{
		Challenge: proto.Clone(challenge).(*agentv1.WorkerHandshakeChallenge), Accepted: accepted, RejectionReason: reason,
		IssuedAt: timestamppb.New(f.now),
	}
	if accepted {
		result.TokenExpiresAt = timestamppb.New(f.now.Add(time.Hour))
	}
	packet := &agentv1.BusPacket{
		TraceId: challenge.TraceId, SenderId: f.config.ExpectedSchedulerID, CreatedAt: timestamppb.New(f.now),
		ProtocolVersion: WorkerHandshakeProtocolVersion, AuthToken: token,
		Payload: &agentv1.BusPacket_WorkerHandshakeResult{WorkerHandshakeResult: result},
	}
	f.resignScheduler(t, packet)
	return packet
}

func (f *workerTrustClientFixture) resignScheduler(t *testing.T, packet *agentv1.BusPacket) {
	t.Helper()
	packet.Signature = nil
	if err := SignTrustHandshake(packet, f.schedulerKey); err != nil {
		t.Fatalf("sign scheduler packet: %v", err)
	}
}

func validWorkerCapability(config *WorkerTrustConfig) *agentv1.Handshake {
	return &agentv1.Handshake{
		ComponentId: config.WorkerID, Role: agentv1.ComponentRole_COMPONENT_ROLE_WORKER,
		SupportedVersions: []int32{WorkerHandshakeProtocolVersion}, Capabilities: map[string]bool{"progress": true},
		SdkVersion: config.SDKVersion, ReadyTopics: []string{"job.allowed"},
	}
}

func issuePurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE
}

func renewPurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW
}
