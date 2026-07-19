package main

import (
	"crypto/ecdsa"
	"errors"
	"path/filepath"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

type vectorEvidence string

const (
	evidencePacketValidation vectorEvidence = "sdk-packet-validation"
	evidenceSignature        vectorEvidence = "sdk-signature-validation"
	evidenceKeySelection     vectorEvidence = "sdk-key-selection"
	evidenceServerRequired   vectorEvidence = "server-required-not-local-evidence"
	expectedLocalVectors                    = 19
	expectedServerVectors                   = 19
)

var manifestVectorEvidence = map[string]vectorEvidence{
	"impersonation_worker_id":        evidencePacketValidation,
	"replay_exact":                   evidenceServerRequired,
	"replay_concurrent":              evidenceServerRequired,
	"skew_positive_boundary":         evidenceServerRequired,
	"skew_negative_boundary":         evidenceServerRequired,
	"skew_positive_over_1ns":         evidenceServerRequired,
	"skew_negative_over_1ns":         evidenceServerRequired,
	"audience_wrong":                 evidenceServerRequired,
	"audience_tampered":              evidenceSignature,
	"tenant_tampered":                evidenceSignature,
	"agent_tampered":                 evidenceSignature,
	"client_nonce_tampered":          evidenceSignature,
	"server_nonce_tampered":          evidenceSignature,
	"trace_tampered":                 evidencePacketValidation,
	"capabilities_tampered":          evidencePacketValidation,
	"protocol_version_tampered":      evidencePacketValidation,
	"missing_trace_id":               evidencePacketValidation,
	"missing_sender_id":              evidencePacketValidation,
	"missing_component_id":           evidencePacketValidation,
	"missing_key_id":                 evidencePacketValidation,
	"missing_signature":              evidencePacketValidation,
	"missing_worker_id":              evidencePacketValidation,
	"unsupported_version":            evidencePacketValidation,
	"challenge_expired":              evidenceServerRequired,
	"response_request_id_mismatch":   evidenceServerRequired,
	"response_trace_mismatch":        evidenceServerRequired,
	"response_challenge_id_mismatch": evidenceServerRequired,
	"unknown_scheduler_key":          evidenceKeySelection,
	"token_worker_mismatch":          evidenceServerRequired,
	"token_agent_mismatch":           evidenceServerRequired,
	"token_tenant_mismatch":          evidenceServerRequired,
	"token_audience_mismatch":        evidenceServerRequired,
	"token_proof_key_mismatch":       evidenceServerRequired,
	"renew_a_to_b":                   evidenceServerRequired,
	"renew_superseded_token":         evidenceServerRequired,
	"state_store_unavailable":        evidenceServerRequired,
	"unknown_envelope_field":         evidencePacketValidation,
	"unknown_payload_field":          evidencePacketValidation,
}

func loadManifestPublicKeys(t *testing.T, directory string, manifest mutationManifest) map[string]*ecdsa.PublicKey {
	t.Helper()
	keys := make(map[string]*ecdsa.PublicKey, len(manifest.Keys))
	for name, spec := range manifest.Keys {
		if name == "" || spec.ID == "" || filepath.Base(spec.PublicKey) != spec.PublicKey {
			t.Fatalf("invalid manifest key %q", name)
		}
		key, err := loadPublicKey(filepath.Join(directory, spec.PublicKey))
		if err != nil {
			t.Fatalf("load manifest key %q: %v", name, err)
		}
		if _, exists := keys[spec.ID]; exists {
			t.Fatalf("duplicate manifest key ID %q", spec.ID)
		}
		keys[spec.ID] = key
	}
	return keys
}

func assertPositiveFixturesValidate(t *testing.T, directory string, manifest mutationManifest, keys map[string]*ecdsa.PublicKey) {
	t.Helper()
	for name, fixture := range manifest.Positive {
		packet := loadBasePacket(t, directory, manifest, name)
		if _, ok := manifest.Keys[fixture.Signer]; !ok {
			t.Fatalf("positive fixture %q has unknown signer %q", name, fixture.Signer)
		}
		if err := capsdk.ValidateWorkerTrustPacket(packet); err != nil {
			t.Fatalf("positive fixture %q failed packet validation: %v", name, err)
		}
		if err := capsdk.VerifyTrustHandshake(packet, keys); err != nil {
			t.Fatalf("positive fixture %q failed signature validation: %v", name, err)
		}
	}
}

func assertVectorContractsComplete(t *testing.T, manifest mutationManifest) {
	t.Helper()
	if len(manifestVectorEvidence) != expectedHandshakeNegativeVectors {
		t.Fatalf("vector evidence count=%d want=%d", len(manifestVectorEvidence), expectedHandshakeNegativeVectors)
	}
	manifestIDs := make(map[string]struct{}, len(manifest.NegativeVectors))
	localCount, serverCount := 0, 0
	for _, vector := range manifest.NegativeVectors {
		missingValue := len(vector.Mutation.Value) == 0 && vector.Mutation.Kind != "clear"
		if vector.ID == "" || vector.Base == "" || vector.Mutation.Kind == "" || vector.Mutation.Path == "" || missingValue {
			t.Fatal("manifest vector has incomplete mutation metadata")
		}
		manifestIDs[vector.ID] = struct{}{}
		if _, ok := manifestVectorEvidence[vector.ID]; !ok {
			t.Fatalf("vector %q has no evidence classification", vector.ID)
		}
		if manifestVectorEvidence[vector.ID] == evidenceServerRequired {
			serverCount++
		} else {
			localCount++
		}
	}
	if localCount != expectedLocalVectors || serverCount != expectedServerVectors {
		t.Fatalf("evidence classification local=%d server=%d want=%d/%d", localCount, serverCount, expectedLocalVectors, expectedServerVectors)
	}
	t.Logf("evidence classification local=%d server_required=%d", localCount, serverCount)
	for id := range manifestVectorEvidence {
		if _, ok := manifestIDs[id]; !ok {
			t.Fatalf("evidence classification %q has no manifest vector", id)
		}
	}
}

func assertVectorEvidence(t *testing.T, directory string, manifest mutationManifest, vector negativeVector, keys map[string]*ecdsa.PublicKey) {
	t.Helper()
	evidence := manifestVectorEvidence[vector.ID]
	packet := loadBasePacket(t, directory, manifest, vector.Base)
	if evidence == evidenceServerRequired {
		t.Logf("SERVER_REQUIRED: %s is not counted as local executable evidence", vector.ID)
		return
	}
	original := marshalManifestPacket(t, packet)
	if err := applyManifestPacketMutation(packet, vector.Mutation, manifest.State.NowUnixNanos); err != nil {
		t.Fatalf("apply %s mutation: %v", vector.Mutation.Kind, err)
	}
	if proto.Equal(loadBasePacket(t, directory, manifest, vector.Base), packet) {
		t.Fatalf("mutation %s/%s made no packet change", vector.Mutation.Kind, vector.Mutation.Path)
	}
	if mutated := marshalManifestPacket(t, packet); string(original) == string(mutated) {
		t.Fatalf("mutation %s/%s preserved wire bytes", vector.Mutation.Kind, vector.Mutation.Path)
	}
	assertSDKRejection(t, packet, keys, evidence)
}

func marshalManifestPacket(t *testing.T, packet proto.Message) []byte {
	t.Helper()
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal manifest packet: %v", err)
	}
	return data
}

func assertSDKRejection(t *testing.T, packet *agentv1.BusPacket, keys map[string]*ecdsa.PublicKey, evidence vectorEvidence) {
	t.Helper()
	switch evidence {
	case evidencePacketValidation:
		var validationError *capsdk.ValidationError
		if err := capsdk.ValidateWorkerTrustPacket(packet); !errors.As(err, &validationError) {
			t.Fatalf("public packet validator error=%v, want *capsdk.ValidationError", err)
		}
	case evidenceSignature:
		if err := capsdk.VerifyTrustHandshake(packet, keys); !errors.Is(err, capsdk.ErrInvalidSignature) {
			t.Fatalf("public signature validator error=%v, want ErrInvalidSignature", err)
		}
	case evidenceKeySelection:
		if err := capsdk.VerifyTrustHandshake(packet, keys); !errors.Is(err, capsdk.ErrTrustHandshakeKeyID) {
			t.Fatalf("public signature validator error=%v, want ErrTrustHandshakeKeyID", err)
		}
	default:
		t.Fatalf("unsupported local evidence %q", evidence)
	}
}
