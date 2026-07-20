package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestLoadEnvironmentRejectsMissingValue(t *testing.T) {
	for _, name := range []string{
		"CAP_HANDSHAKE_WORKER_PRIVATE_KEY", "CAP_HANDSHAKE_SCHEDULER_PUBLIC_KEY",
		"CAP_HANDSHAKE_SCHEDULER_KEY_ID", "CAP_HANDSHAKE_WORKER_ID",
		"CAP_HANDSHAKE_AGENT_ID", "CAP_HANDSHAKE_TENANT_ID", "CAP_HANDSHAKE_PROOF_KEY_ID",
		"CAP_HANDSHAKE_SCHEDULER_ID", "CAP_HANDSHAKE_SDK_VERSION",
		"CAP_HANDSHAKE_CASE", "CAP_HANDSHAKE_NATS_URL",
	} {
		t.Setenv(name, "test")
	}
	t.Setenv("CAP_HANDSHAKE_WORKER_ID", "   ")
	_, err := loadEnvironment()
	if err == nil || !strings.Contains(err.Error(), "CAP_HANDSHAKE_WORKER_ID") {
		t.Fatalf("loadEnvironment() error=%v", err)
	}
}

func TestMutateRequestAppliesRequiredNegative(t *testing.T) {
	tests := []struct {
		name   string
		assert func(*testing.T, *agentv1.BusPacket)
	}{
		{"wrong_audience", func(t *testing.T, packet *agentv1.BusPacket) {
			if packet.GetWorkerHandshakeChallengeRequest().GetAudience() != "other-scheduler" {
				t.Fatal("audience not mutated")
			}
		}},
		{"missing_identity", func(t *testing.T, packet *agentv1.BusPacket) {
			if packet.GetSenderId() != "" {
				t.Fatal("sender_id not cleared")
			}
		}},
		{"missing_trace", func(t *testing.T, packet *agentv1.BusPacket) {
			if packet.GetTraceId() != "" {
				t.Fatal("trace_id not cleared")
			}
		}},
		{"unsupported_version", func(t *testing.T, packet *agentv1.BusPacket) {
			if packet.GetProtocolVersion() != 2 {
				t.Fatal("protocol version not mutated")
			}
		}},
		{"tamper", func(t *testing.T, packet *agentv1.BusPacket) {
			if packet.GetWorkerHandshakeChallengeRequest().GetAudience() != "tampered-after-signing" {
				t.Fatal("tamper not applied")
			}
		}},
		{"skew", func(t *testing.T, packet *agentv1.BusPacket) {
			if packet.GetProtocolVersion() != 1 {
				t.Fatal("skew mutated packet shape")
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			packet := mutationFixture()
			if err := mutateRequest(test.name, packet); err != nil {
				t.Fatalf("mutate: %v", err)
			}
			test.assert(t, packet)
		})
	}
}

func TestMutateRequestRejectsUnknownCase(t *testing.T) {
	if err := mutateRequest("invented", mutationFixture()); err == nil {
		t.Fatal("unknown mutation case accepted")
	}
}

func TestMutationProofMatchesInteropJSONContract(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	trust := mutationTrust(key)
	for _, test := range []struct {
		name                  string
		mutate                bool
		wantValid, wantTamper bool
	}{
		{"wrong_audience", true, true, false},
		{"tamper", true, false, true},
		{"impersonation", false, false, false},
		{"replay", false, false, false},
		{"skew", true, false, false},
		{"missing_identity", true, false, false},
		{"missing_trace", true, false, false},
		{"unsupported_version", true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			packet, buildErr := buildRequest(trust, issuePurpose(), time.Now().UTC())
			if buildErr != nil {
				t.Fatalf("build request: %v", buildErr)
			}
			config := &clientConfig{testCase: test.name, trust: trust}
			if test.mutate {
				if mutationErr := applyNegativeMutation(config, packet); mutationErr != nil {
					t.Fatalf("apply mutation: %v", mutationErr)
				}
			}
			proof, proofErr := proveMutationSignature(test.name, packet, trust)
			if proofErr != nil {
				t.Fatalf("prove mutation signature: %v", proofErr)
			}
			if proof.signatureValid != test.wantValid || proof.tamperRejected != test.wantTamper {
				t.Fatalf("proof=(%t,%t) want=(%t,%t)", proof.signatureValid, proof.tamperRejected, test.wantValid, test.wantTamper)
			}
		})
	}
}

func TestClientResultIncludesMutationProofFields(t *testing.T) {
	result := clientResult{MutationSignatureValid: true, TamperSignatureRejected: true}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal client result: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode client result: %v", err)
	}
	if fields["mutation_signature_valid"] != true || fields["tamper_signature_rejected"] != true {
		t.Fatalf("mutation proof JSON fields missing: %s", encoded)
	}
}

func mutationTrust(key *ecdsa.PrivateKey) *capsdk.WorkerTrustConfig {
	return &capsdk.WorkerTrustConfig{
		WorkerID: "worker", ExpectedAgentID: "agent", TenantID: "tenant",
		Audience: capsdk.WorkerHandshakeAudience, ProofKeyID: "proof-key",
		ProofPrivateKey: key, ExpectedSchedulerID: "scheduler",
		SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"scheduler-key": &key.PublicKey},
		SDKVersion:          "cap-go/test",
	}
}

func mutationFixture() *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId: "trace", SenderId: "worker", ProtocolVersion: 1,
		Payload: &agentv1.BusPacket_WorkerHandshakeChallengeRequest{
			WorkerHandshakeChallengeRequest: &agentv1.WorkerHandshakeChallengeRequest{Audience: "cordum-scheduler"},
		},
	}
}
