package capsdk

import (
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type expectedField struct {
	name   protoreflect.Name
	number protoreflect.FieldNumber
}

func assertFields(t *testing.T, message protoreflect.MessageDescriptor, expected []expectedField) {
	t.Helper()
	if message == nil {
		t.Fatal("message descriptor is missing")
	}
	fields := message.Fields()
	for _, want := range expected {
		field := fields.ByName(want.name)
		if field == nil || field.Number() != want.number {
			t.Errorf("%s.%s tag = %v, want %d", message.Name(), want.name, field, want.number)
		}
	}
}

func TestWorkerTrustHandshakeProtoContractExists(t *testing.T) {
	messages := agentv1.File_cordum_agent_v1_handshake_proto.Messages()
	for name, fields := range workerTrustMessageFields() {
		t.Run(string(name), func(t *testing.T) {
			assertFields(t, messages.ByName(name), fields)
		})
	}
	assertWorkerTrustEnums(t, agentv1.File_cordum_agent_v1_handshake_proto.Enums())
	assertAppendOnlyLegacyFields(t)
}

func workerTrustMessageFields() map[protoreflect.Name][]expectedField {
	return map[protoreflect.Name][]expectedField{
		"WorkerHandshakeChallengeRequest": fields(
			"request_id", "trace_id", "worker_id", "proof_key_id", "proof_algorithm",
			"audience", "purpose", "client_nonce", "protocol_version", "sdk_version"),
		"WorkerHandshakeChallenge": fields(
			"request_id", "challenge_id", "trace_id", "worker_id", "agent_id", "tenant_id",
			"proof_key_id", "proof_algorithm", "server_key_id", "audience", "purpose",
			"client_nonce", "server_nonce", "protocol_version", "sdk_version", "issued_at", "expires_at"),
		"WorkerHandshakeAuthenticate": fields("challenge", "capability_handshake"),
		"WorkerHandshakeResult":       fields("challenge", "accepted", "rejection_reason", "token_expires_at", "issued_at"),
	}
}

func fields(names ...protoreflect.Name) []expectedField {
	result := make([]expectedField, len(names))
	for index, name := range names {
		result[index] = expectedField{name: name, number: protoreflect.FieldNumber(index + 1)}
	}
	return result
}

func assertWorkerTrustEnums(t *testing.T, enums protoreflect.EnumDescriptors) {
	t.Helper()
	wants := map[protoreflect.Name][]protoreflect.Name{
		"WorkerHandshakePurpose":         {"WORKER_HANDSHAKE_PURPOSE_UNSPECIFIED", "WORKER_HANDSHAKE_PURPOSE_ISSUE", "WORKER_HANDSHAKE_PURPOSE_RENEW"},
		"WorkerHandshakeProofAlgorithm":  {"WORKER_HANDSHAKE_PROOF_ALGORITHM_UNSPECIFIED", "WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256"},
		"WorkerHandshakeRejectionReason": {"WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED", "WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED"},
	}
	for enumName, values := range wants {
		descriptor := enums.ByName(enumName)
		if descriptor == nil {
			t.Errorf("handshake proto missing enum %s", enumName)
			continue
		}
		for _, value := range values {
			if descriptor.Values().ByName(value) == nil {
				t.Errorf("%s missing %s", enumName, value)
			}
		}
	}
}

func assertAppendOnlyLegacyFields(t *testing.T) {
	t.Helper()
	handshake := agentv1.File_cordum_agent_v1_handshake_proto.Messages().ByName("Handshake")
	assertFields(t, handshake, fields("component_id", "role", "supported_versions", "capabilities", "sdk_version", "ready_topics", "agent_name"))
	packet := (&agentv1.BusPacket{}).ProtoReflect().Descriptor()
	assertFields(t, packet, []expectedField{
		{name: "trace_id", number: 1}, {name: "sender_id", number: 2},
		{name: "created_at", number: 3}, {name: "protocol_version", number: 4},
		{name: "signature_metadata", number: 5}, {name: "identity", number: 6},
		{name: "job_request", number: 10}, {name: "job_result", number: 11},
		{name: "heartbeat", number: 12}, {name: "alert", number: 13},
		{name: "signature", number: 14}, {name: "job_progress", number: 15},
		{name: "job_cancel", number: 16}, {name: "handshake", number: 17},
		{name: "auth_token", number: 18},
		{name: "worker_handshake_challenge_request", number: 19},
		{name: "worker_handshake_challenge", number: 20},
		{name: "worker_handshake_authenticate", number: 21},
		{name: "worker_handshake_result", number: 22},
	})
}
