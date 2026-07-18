package capsdk

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// VerifiedWorkerHandshakeChallenge is an opaque, scheduler-authenticated
// challenge. Its contents can only be obtained as a defensive copy.
type VerifiedWorkerHandshakeChallenge struct {
	request  *agentv1.BusPacket
	response *agentv1.BusPacket
}

// Message returns a copy safe for embedding in an authenticate packet.
func (v *VerifiedWorkerHandshakeChallenge) Message() *agentv1.WorkerHandshakeChallenge {
	if v == nil || v.response == nil || v.response.GetWorkerHandshakeChallenge() == nil {
		return nil
	}
	return proto.Clone(v.response.GetWorkerHandshakeChallenge()).(*agentv1.WorkerHandshakeChallenge)
}

// BuildWorkerHandshakeChallengeRequest creates and signs one protobuf request.
func BuildWorkerHandshakeChallengeRequest(config *WorkerTrustConfig, options WorkerHandshakeRequestOptions) (*agentv1.BusPacket, error) {
	if err := ValidateWorkerTrustConfig(config); err != nil {
		return nil, err
	}
	if err := validateRequestOptions(options); err != nil {
		return nil, err
	}
	request := &agentv1.WorkerHandshakeChallengeRequest{
		RequestId: options.RequestID, TraceId: options.TraceID, WorkerId: config.WorkerID,
		ProofKeyId: config.ProofKeyID, ProofAlgorithm: p256WorkerProofAlgorithm(), Audience: config.Audience,
		Purpose: options.Purpose, ClientNonce: append([]byte(nil), options.ClientNonce...),
		ProtocolVersion: WorkerHandshakeProtocolVersion, SdkVersion: config.SDKVersion,
	}
	packet := newWorkerTrustPacket(options.TraceID, config.WorkerID, options.CreatedAt)
	packet.Payload = &agentv1.BusPacket_WorkerHandshakeChallengeRequest{WorkerHandshakeChallengeRequest: request}
	if err := SignTrustHandshake(packet, config.ProofPrivateKey); err != nil {
		return nil, fmt.Errorf("build worker handshake challenge request: %w", err)
	}
	if err := ValidateWorkerTrustPacket(packet); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkerHandshakePacket, err)
	}
	return packet, nil
}

// BuildWorkerHandshakeAuthenticate signs the verified challenge and capability.
func BuildWorkerHandshakeAuthenticate(config *WorkerTrustConfig, verified *VerifiedWorkerHandshakeChallenge, capability *agentv1.Handshake, currentSessionToken string, createdAt time.Time) (*agentv1.BusPacket, error) {
	if err := ValidateWorkerTrustConfig(config); err != nil {
		return nil, err
	}
	challenge := verified.Message()
	if err := validateVerifiedChallengeConfig(config, challenge); err != nil {
		return nil, err
	}
	if err := validateWorkerCapability(capability, challenge); err != nil {
		return nil, err
	}
	if err := validateSessionForPurpose(challenge.GetPurpose(), currentSessionToken); err != nil {
		return nil, err
	}
	if createdAt.IsZero() {
		return nil, fmt.Errorf("%w: created_at is empty", ErrWorkerHandshakePacket)
	}
	packet := newWorkerTrustPacket(challenge.GetTraceId(), config.WorkerID, createdAt)
	packet.AuthToken = currentSessionToken
	packet.Payload = &agentv1.BusPacket_WorkerHandshakeAuthenticate{WorkerHandshakeAuthenticate: &agentv1.WorkerHandshakeAuthenticate{
		Challenge: challenge, CapabilityHandshake: proto.Clone(capability).(*agentv1.Handshake),
	}}
	if err := SignTrustHandshake(packet, config.ProofPrivateKey); err != nil {
		return nil, fmt.Errorf("build worker handshake authenticate: %w", err)
	}
	if err := ValidateWorkerTrustPacket(packet); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkerHandshakePacket, err)
	}
	return packet, nil
}

func validateRequestOptions(options WorkerHandshakeRequestOptions) error {
	if !canonicalTrustString(options.RequestID) || !canonicalTrustString(options.TraceID) {
		return fmt.Errorf("%w: request_id and trace_id are required", ErrWorkerHandshakePacket)
	}
	if !validWorkerHandshakePurpose(options.Purpose) {
		return fmt.Errorf("%w: unsupported purpose %d", ErrWorkerHandshakePacket, options.Purpose)
	}
	if len(options.ClientNonce) != WorkerHandshakeNonceSize {
		return fmt.Errorf("%w: client nonce must be %d bytes", ErrWorkerHandshakePacket, WorkerHandshakeNonceSize)
	}
	if options.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is empty", ErrWorkerHandshakePacket)
	}
	return nil
}

func newWorkerTrustPacket(traceID, senderID string, createdAt time.Time) *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId: traceID, SenderId: senderID, CreatedAt: timestamppb.New(createdAt.UTC()),
		ProtocolVersion: WorkerHandshakeProtocolVersion,
	}
}

func validateVerifiedChallengeConfig(config *WorkerTrustConfig, challenge *agentv1.WorkerHandshakeChallenge) error {
	if challenge == nil {
		return fmt.Errorf("%w: challenge is unverified", ErrWorkerHandshakeBinding)
	}
	if challenge.GetWorkerId() != config.WorkerID || challenge.GetAgentId() != config.ExpectedAgentID ||
		challenge.GetTenantId() != config.TenantID || challenge.GetProofKeyId() != config.ProofKeyID ||
		challenge.GetAudience() != config.Audience || challenge.GetSdkVersion() != config.SDKVersion {
		return fmt.Errorf("%w: challenge identity changed", ErrWorkerHandshakeBinding)
	}
	return nil
}

func validateSessionForPurpose(purpose agentv1.WorkerHandshakePurpose, token string) error {
	if purpose == issueWorkerHandshakePurpose() && token != "" {
		return fmt.Errorf("%w: issue must not carry a session", ErrWorkerHandshakePacket)
	}
	if purpose == renewWorkerHandshakePurpose() && !validSessionToken(token) {
		return fmt.Errorf("%w: renew requires the current session", ErrWorkerHandshakePacket)
	}
	return nil
}

func validSessionToken(token string) bool {
	if token == "" || token != strings.TrimSpace(token) || len(token) > WorkerHandshakeMaxSessionTokenSize {
		return false
	}
	for index := 0; index < len(token); index++ {
		if token[index] < 0x21 || token[index] > 0x7e {
			return false
		}
	}
	return true
}

func sameNonce(left, right []byte) bool {
	return bytes.Equal(left, right)
}
