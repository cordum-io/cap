package capsdk

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var workerCapabilityKey = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

// ValidateWorkerTrustPacket enforces structural trust-packet semantics.
// Cryptographic verification remains an explicit caller responsibility.
func ValidateWorkerTrustPacket(packet *agentv1.BusPacket) error {
	if _, _, err := SelectTrustHandshakePhase(packet); err != nil {
		return err
	}
	return ValidateBusPacket(packet)
}

func validateWorkerTrustPayload(packet *agentv1.BusPacket) error {
	if hasUnknownProtoFields(packet) {
		return workerTrustValidation("unknown_fields", "must be empty recursively")
	}
	if _, ok := checkedProtoTime(packet.GetCreatedAt()); !ok {
		return workerTrustValidation("created_at", "must be a valid timestamp")
	}
	switch packet.GetPayload().(type) {
	case *agentv1.BusPacket_WorkerHandshakeChallengeRequest:
		return validateChallengeRequestPayload(packet)
	case *agentv1.BusPacket_WorkerHandshakeChallenge:
		return validateChallengePayload(packet, packet.GetWorkerHandshakeChallenge(), true)
	case *agentv1.BusPacket_WorkerHandshakeAuthenticate:
		return validateAuthenticatePayload(packet)
	case *agentv1.BusPacket_WorkerHandshakeResult:
		return validateResultPayload(packet)
	default:
		return workerTrustValidation("payload", "must be a worker trust phase")
	}
}

func validateChallengeRequestPayload(packet *agentv1.BusPacket) error {
	request := packet.GetWorkerHandshakeChallengeRequest()
	if request == nil || missingStrings(request.GetRequestId(), request.GetTraceId(), request.GetWorkerId(), request.GetProofKeyId(), request.GetAudience(), request.GetSdkVersion()) {
		return workerTrustValidation("challenge_request", "required field is empty")
	}
	if packet.GetTraceId() != request.GetTraceId() || packet.GetSenderId() != request.GetWorkerId() {
		return workerTrustValidation("challenge_request", "trace or worker does not match envelope")
	}
	if request.GetProtocolVersion() != WorkerHandshakeProtocolVersion || !validWorkerHandshakePurpose(request.GetPurpose()) {
		return workerTrustValidation("challenge_request", "version or purpose is unsupported")
	}
	if request.GetProofAlgorithm() != p256WorkerProofAlgorithm() || len(request.GetClientNonce()) != WorkerHandshakeNonceSize {
		return workerTrustValidation("challenge_request", "proof algorithm or nonce is invalid")
	}
	if packet.GetAuthToken() != "" || len(packet.GetSignature()) == 0 {
		return workerTrustValidation("challenge_request", "must be signed without a session token")
	}
	return nil
}

func validateChallengePayload(packet *agentv1.BusPacket, challenge *agentv1.WorkerHandshakeChallenge, topLevel bool) error {
	if challenge == nil || missingChallengeStrings(challenge) {
		return workerTrustValidation("challenge", "required field is empty")
	}
	if packet.GetTraceId() != challenge.GetTraceId() {
		return workerTrustValidation("challenge.trace_id", "must equal envelope trace_id")
	}
	if topLevel && packet.GetAuthToken() != "" {
		return workerTrustValidation("auth_token", "challenge must not carry a session")
	}
	if challenge.GetProtocolVersion() != WorkerHandshakeProtocolVersion || !validWorkerHandshakePurpose(challenge.GetPurpose()) {
		return workerTrustValidation("challenge", "version or purpose is unsupported")
	}
	if challenge.GetProofAlgorithm() != p256WorkerProofAlgorithm() || len(challenge.GetClientNonce()) != WorkerHandshakeNonceSize || len(challenge.GetServerNonce()) != WorkerHandshakeNonceSize {
		return workerTrustValidation("challenge", "proof algorithm or nonce is invalid")
	}
	if err := validateChallengeTimes(challenge); err != nil {
		return err
	}
	if topLevel && len(packet.GetSignature()) == 0 {
		return workerTrustValidation("signature", "challenge must be signed")
	}
	return nil
}

func validateAuthenticatePayload(packet *agentv1.BusPacket) error {
	authenticate := packet.GetWorkerHandshakeAuthenticate()
	if authenticate == nil || authenticate.GetChallenge() == nil || authenticate.GetCapabilityHandshake() == nil {
		return workerTrustValidation("authenticate", "challenge and capability are required")
	}
	challenge := authenticate.GetChallenge()
	if err := validateChallengePayload(packet, challenge, false); err != nil {
		return err
	}
	if packet.GetSenderId() != challenge.GetWorkerId() || len(packet.GetSignature()) == 0 {
		return workerTrustValidation("authenticate", "worker binding or signature is invalid")
	}
	if err := validateCapabilityShape(authenticate.GetCapabilityHandshake(), challenge); err != nil {
		return err
	}
	return validateSessionForPurpose(challenge.GetPurpose(), packet.GetAuthToken())
}

func validateResultPayload(packet *agentv1.BusPacket) error {
	result := packet.GetWorkerHandshakeResult()
	if result == nil || result.GetChallenge() == nil || len(packet.GetSignature()) == 0 {
		return workerTrustValidation("result", "challenge and signature are required")
	}
	if err := validateChallengePayload(packet, result.GetChallenge(), false); err != nil {
		return err
	}
	issuedAt, ok := checkedProtoTime(result.GetIssuedAt())
	if !ok {
		return workerTrustValidation("result.issued_at", "must be a valid timestamp")
	}
	if result.GetAccepted() {
		return validateAcceptedResult(result, packet.GetAuthToken(), issuedAt)
	}
	if packet.GetAuthToken() != "" || !validWorkerHandshakeRejection(result.GetRejectionReason()) || result.GetTokenExpiresAt() != nil {
		return workerTrustValidation("result", "rejection must not carry token metadata")
	}
	return nil
}

func validWorkerHandshakeRejection(reason agentv1.WorkerHandshakeRejectionReason) bool {
	switch reason {
	case agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_INVALID_REQUEST,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_REPLAY_DETECTED,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_CLOCK_SKEW,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_CHALLENGE_EXPIRED,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_SESSION_REQUIRED,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_SESSION_INVALID,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_UNSUPPORTED_VERSION,
		agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_INTERNAL_ERROR:
		return true
	default:
		return false
	}
}

func validateAcceptedResult(result *agentv1.WorkerHandshakeResult, token string, issuedAt time.Time) error {
	expiresAt, ok := checkedProtoTime(result.GetTokenExpiresAt())
	if !validSessionToken(token) || !ok || !expiresAt.After(issuedAt) {
		return workerTrustValidation("result", "accepted result requires a future token expiry")
	}
	if result.GetRejectionReason() != unspecifiedWorkerHandshakeReason() {
		return workerTrustValidation("result.rejection_reason", "must be unspecified when accepted")
	}
	return nil
}

func validateChallengeTimes(challenge *agentv1.WorkerHandshakeChallenge) error {
	issuedAt, issuedOK := checkedProtoTime(challenge.GetIssuedAt())
	expiresAt, expiresOK := checkedProtoTime(challenge.GetExpiresAt())
	if !issuedOK || !expiresOK || !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > WorkerHandshakeMaxLifetime {
		return workerTrustValidation("challenge.timestamps", "invalid order or lifetime")
	}
	return nil
}

func validateCapabilityShape(capability *agentv1.Handshake, challenge *agentv1.WorkerHandshakeChallenge) error {
	if capability.GetComponentId() != challenge.GetWorkerId() || capability.GetRole() != agentv1.ComponentRole_COMPONENT_ROLE_WORKER || capability.GetSdkVersion() != challenge.GetSdkVersion() {
		return workerTrustValidation("capability_handshake", "identity does not match challenge")
	}
	versions := capability.GetSupportedVersions()
	if len(versions) != 1 || versions[0] != WorkerHandshakeProtocolVersion {
		return workerTrustValidation("capability_handshake.supported_versions", "must contain exactly protocol v1")
	}
	for key, enabled := range capability.GetCapabilities() {
		if !enabled || !workerCapabilityKey.MatchString(key) {
			return workerTrustValidation("capability_handshake.capabilities", "key is invalid or disabled")
		}
	}
	seen := make(map[string]struct{}, len(capability.GetReadyTopics()))
	for _, topic := range capability.GetReadyTopics() {
		if strings.TrimSpace(topic) == "" {
			return workerTrustValidation("capability_handshake.ready_topics", "topic is empty")
		}
		if _, exists := seen[topic]; exists {
			return workerTrustValidation("capability_handshake.ready_topics", "topic is duplicated")
		}
		seen[topic] = struct{}{}
	}
	return nil
}

func validateWorkerCapability(capability *agentv1.Handshake, challenge *agentv1.WorkerHandshakeChallenge) error {
	if capability == nil {
		return workerTrustValidation("capability_handshake", "must not be nil")
	}
	return validateCapabilityShape(capability, challenge)
}

func hasUnknownProtoFields(message proto.Message) bool {
	if message == nil {
		return false
	}
	return reflectedMessageHasUnknown(message.ProtoReflect())
}

func reflectedMessageHasUnknown(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}
	found := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		found = nestedValueHasUnknown(field, value)
		return !found
	})
	return found
}

func nestedValueHasUnknown(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
	if field.IsList() && field.Kind() == protoreflect.MessageKind {
		list := value.List()
		for index := 0; index < list.Len(); index++ {
			if reflectedMessageHasUnknown(list.Get(index).Message()) {
				return true
			}
		}
	}
	if field.IsMap() && field.MapValue().Kind() == protoreflect.MessageKind {
		found := false
		value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
			found = reflectedMessageHasUnknown(item.Message())
			return !found
		})
		return found
	}
	if !field.IsList() && !field.IsMap() && field.Kind() == protoreflect.MessageKind {
		return reflectedMessageHasUnknown(value.Message())
	}
	return false
}

func checkedProtoTime(timestamp *timestamppb.Timestamp) (time.Time, bool) {
	if timestamp == nil || timestamp.CheckValid() != nil {
		return time.Time{}, false
	}
	return timestamp.AsTime().UTC(), true
}

func workerTrustValidation(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

func missingChallengeStrings(challenge *agentv1.WorkerHandshakeChallenge) bool {
	return missingStrings(
		challenge.GetRequestId(), challenge.GetChallengeId(), challenge.GetTraceId(), challenge.GetWorkerId(),
		challenge.GetAgentId(), challenge.GetTenantId(), challenge.GetProofKeyId(), challenge.GetServerKeyId(),
		challenge.GetAudience(), challenge.GetSdkVersion(),
	)
}

func missingStrings(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func validWorkerHandshakePurpose(purpose agentv1.WorkerHandshakePurpose) bool {
	return purpose == issueWorkerHandshakePurpose() || purpose == renewWorkerHandshakePurpose()
}

func p256WorkerProofAlgorithm() agentv1.WorkerHandshakeProofAlgorithm {
	return agentv1.WorkerHandshakeProofAlgorithm_WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256
}

func issueWorkerHandshakePurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE
}

func renewWorkerHandshakePurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW
}

func unspecifiedWorkerHandshakeReason() agentv1.WorkerHandshakeRejectionReason {
	return agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED
}
func validateTimestampSkew(timestamp *timestamppb.Timestamp, now time.Time, field string) error {
	value, ok := checkedProtoTime(timestamp)
	if !ok {
		return workerTrustValidation(field, "must be a valid timestamp")
	}
	delta := value.Sub(now.UTC())
	if delta > WorkerHandshakeMaxSkew || delta < -WorkerHandshakeMaxSkew {
		return fmt.Errorf("%w: %s outside allowed skew", ErrWorkerHandshakeExpired, field)
	}
	return nil
}
