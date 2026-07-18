package capsdk

import (
	"crypto/ecdsa"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

// VerifyWorkerHandshakeChallenge authenticates and correlates a scheduler reply.
func VerifyWorkerHandshakeChallenge(config *WorkerTrustConfig, request, response *agentv1.BusPacket, now time.Time) (*VerifiedWorkerHandshakeChallenge, error) {
	if err := ValidateWorkerTrustConfig(config); err != nil {
		return nil, err
	}
	if err := validateChallengePackets(config, request, response, now); err != nil {
		return nil, err
	}
	return &VerifiedWorkerHandshakeChallenge{
		request: proto.Clone(request).(*agentv1.BusPacket), response: proto.Clone(response).(*agentv1.BusPacket),
	}, nil
}

// VerifyWorkerHandshakeResult authenticates a correlated signed result.
func VerifyWorkerHandshakeResult(config *WorkerTrustConfig, verified *VerifiedWorkerHandshakeChallenge, authenticate, response *agentv1.BusPacket, now time.Time) (*WorkerHandshakeSession, error) {
	if err := ValidateWorkerTrustConfig(config); err != nil {
		return nil, err
	}
	if err := validateResultPackets(config, verified, authenticate, response, now); err != nil {
		return nil, err
	}
	result := response.GetWorkerHandshakeResult()
	if !result.GetAccepted() {
		return nil, &WorkerHandshakeRejectionError{Reason: result.GetRejectionReason()}
	}
	issuedAt, _ := checkedProtoTime(result.GetIssuedAt())
	expiresAt, _ := checkedProtoTime(result.GetTokenExpiresAt())
	return &WorkerHandshakeSession{Token: response.GetAuthToken(), IssuedAt: issuedAt, ExpiresAt: expiresAt}, nil
}

func validateChallengePackets(config *WorkerTrustConfig, request, response *agentv1.BusPacket, now time.Time) error {
	if err := ValidateWorkerTrustPacket(request); err != nil {
		return fmt.Errorf("%w: request: %w", ErrWorkerHandshakePacket, err)
	}
	workerKeys := map[string]*ecdsa.PublicKey{config.ProofKeyID: &config.ProofPrivateKey.PublicKey}
	if err := VerifyTrustHandshake(request, workerKeys); err != nil {
		return fmt.Errorf("%w: request signature: %w", ErrWorkerHandshakePacket, err)
	}
	if err := validateSignedSchedulerPacket(config, response, now); err != nil {
		return err
	}
	challenge := response.GetWorkerHandshakeChallenge()
	if err := correlateChallengeRequest(config, request.GetWorkerHandshakeChallengeRequest(), challenge); err != nil {
		return err
	}
	if err := validateChallengeFreshness(challenge, now); err != nil {
		return err
	}
	return nil
}

func validateResultPackets(config *WorkerTrustConfig, verified *VerifiedWorkerHandshakeChallenge, authenticate, response *agentv1.BusPacket, now time.Time) error {
	if verified == nil || verified.request == nil || verified.response == nil {
		return fmt.Errorf("%w: challenge is unverified", ErrWorkerHandshakeBinding)
	}
	if err := ValidateWorkerTrustPacket(authenticate); err != nil {
		return fmt.Errorf("%w: authenticate: %w", ErrWorkerHandshakePacket, err)
	}
	workerKeys := map[string]*ecdsa.PublicKey{config.ProofKeyID: &config.ProofPrivateKey.PublicKey}
	if err := VerifyTrustHandshake(authenticate, workerKeys); err != nil {
		return fmt.Errorf("%w: authenticate signature: %w", ErrWorkerHandshakePacket, err)
	}
	if err := validateSignedSchedulerPacket(config, response, now); err != nil {
		return err
	}
	want := verified.response.GetWorkerHandshakeChallenge()
	got := response.GetWorkerHandshakeResult().GetChallenge()
	if !proto.Equal(want, got) || !proto.Equal(want, authenticate.GetWorkerHandshakeAuthenticate().GetChallenge()) {
		return fmt.Errorf("%w: result challenge differs", ErrWorkerHandshakeBinding)
	}
	if err := validateChallengeFreshness(got, now); err != nil {
		return err
	}
	result := response.GetWorkerHandshakeResult()
	if err := validateTimestampSkew(result.GetIssuedAt(), now, "result.issued_at"); err != nil {
		return err
	}
	if result.GetAccepted() {
		expiresAt, _ := checkedProtoTime(result.GetTokenExpiresAt())
		if !expiresAt.After(now.UTC()) {
			return fmt.Errorf("%w: result token is already expired", ErrWorkerHandshakeExpired)
		}
		if got.GetPurpose() == renewWorkerHandshakePurpose() && response.GetAuthToken() == authenticate.GetAuthToken() {
			return fmt.Errorf("%w: renewal did not rotate session token", ErrWorkerHandshakeBinding)
		}
	}
	return nil
}

func validateSignedSchedulerPacket(config *WorkerTrustConfig, packet *agentv1.BusPacket, now time.Time) error {
	if err := ValidateWorkerTrustPacket(packet); err != nil {
		return fmt.Errorf("%w: scheduler packet: %w", ErrWorkerHandshakePacket, err)
	}
	if packet.GetSenderId() != config.ExpectedSchedulerID {
		return fmt.Errorf("%w: scheduler sender differs", ErrWorkerHandshakeBinding)
	}
	if err := validateTimestampSkew(packet.GetCreatedAt(), now, "created_at"); err != nil {
		return err
	}
	if err := VerifyTrustHandshake(packet, config.SchedulerPublicKeys); err != nil {
		return fmt.Errorf("%w: scheduler signature: %w", ErrWorkerHandshakePacket, err)
	}
	return nil
}

func correlateChallengeRequest(config *WorkerTrustConfig, request *agentv1.WorkerHandshakeChallengeRequest, challenge *agentv1.WorkerHandshakeChallenge) error {
	if request == nil || challenge == nil {
		return fmt.Errorf("%w: request or challenge missing", ErrWorkerHandshakeBinding)
	}
	matches := challenge.GetRequestId() == request.GetRequestId() && challenge.GetTraceId() == request.GetTraceId() &&
		challenge.GetWorkerId() == request.GetWorkerId() && challenge.GetProofKeyId() == request.GetProofKeyId() &&
		challenge.GetProofAlgorithm() == request.GetProofAlgorithm() && challenge.GetAudience() == request.GetAudience() &&
		challenge.GetPurpose() == request.GetPurpose() && sameNonce(challenge.GetClientNonce(), request.GetClientNonce()) &&
		challenge.GetProtocolVersion() == request.GetProtocolVersion() && challenge.GetSdkVersion() == request.GetSdkVersion()
	if !matches || challenge.GetAgentId() != config.ExpectedAgentID || challenge.GetTenantId() != config.TenantID {
		return fmt.Errorf("%w: challenge does not match request or expected identity", ErrWorkerHandshakeBinding)
	}
	return nil
}

func validateChallengeFreshness(challenge *agentv1.WorkerHandshakeChallenge, now time.Time) error {
	if err := validateTimestampSkew(challenge.GetIssuedAt(), now, "challenge.issued_at"); err != nil {
		return err
	}
	expiresAt, ok := checkedProtoTime(challenge.GetExpiresAt())
	if !ok || !now.UTC().Before(expiresAt) {
		return fmt.Errorf("%w: challenge expired", ErrWorkerHandshakeExpired)
	}
	return nil
}
