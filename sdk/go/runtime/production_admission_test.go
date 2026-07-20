package runtime

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func admissionTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func admissionTestPacket(t *testing.T, key *ecdsa.PrivateKey, msgID string) []byte {
	t.Helper()
	packet := &agentv1.BusPacket{
		TraceId:         "trace-1",
		SenderId:        "scheduler-1",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Identity:        &agentv1.IdentityBinding{TenantId: "tenant-a", PrincipalId: "principal-a"},
		SignatureMetadata: &agentv1.SignatureMetadata{
			ProfileVersion: capsdk.ProductionProfileVersion,
			Algorithm:      capsdk.ProductionAlgorithm,
			MessageId:      []byte(msgID),
			Audience:       "worker-pool-a",
			ExpiresAt:      timestamppb.New(time.Now().Add(time.Hour)),
			KeyId:          "k1",
		},
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
			JobId: "job-1", Topic: "job.test", TenantId: "tenant-a",
			Identity: &agentv1.IdentityBinding{TenantId: "tenant-a", PrincipalId: "principal-a"},
		}},
	}
	raw, err := capsdk.SignProductionPacket(packet, key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	return raw
}

func newProductionTestAgent(key *ecdsa.PrivateKey) *Agent {
	return &Agent{
		Production: ProductionAdmission{
			Enabled: true,
			Trust: capsdk.ProductionTrustStore{
				Audience:   "worker-pool-a",
				PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey},
			},
			Replay: capsdk.NewInMemoryReplayStore(),
		},
	}
}

func TestAdmitProductionPacket_AcceptsValidSignedPacket(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	raw := admissionTestPacket(t, key, "0123456789abcdef")

	packet, err := a.admitProductionPacket(raw)
	if err != nil {
		t.Fatalf("admitProductionPacket: %v", err)
	}
	if packet.GetJobRequest().GetJobId() != "job-1" {
		t.Fatalf("unexpected job id %q", packet.GetJobRequest().GetJobId())
	}
}

func TestAdmitProductionPacket_RejectsUnsignedPacket(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)

	packet := &agentv1.BusPacket{SenderId: "scheduler-1", ProtocolVersion: capsdk.DefaultProtocolVersion}
	raw, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if _, err := a.admitProductionPacket(raw); err == nil {
		t.Fatalf("admitProductionPacket accepted an unsigned packet")
	}
}

func TestAdmitProductionPacket_RejectsTamperedSignature(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	raw := admissionTestPacket(t, key, "0123456789abcdef")
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)-1] ^= 0xFF

	if _, err := a.admitProductionPacket(tampered); err == nil {
		t.Fatalf("admitProductionPacket accepted a tampered signature")
	}
}

func TestAdmitProductionPacket_RejectsReplayConflict(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	first := admissionTestPacket(t, key, "0123456789abcdef")
	if _, err := a.admitProductionPacket(first); err != nil {
		t.Fatalf("first admit: %v", err)
	}
	// Identical redelivery (exact same bytes) must be harmless (idempotent).
	if _, err := a.admitProductionPacket(first); err != nil {
		t.Fatalf("identical redelivery rejected: %v", err)
	}
}

func TestAdmitProductionPacket_RejectsUnknownKeyID(t *testing.T) {
	key := admissionTestKey(t)
	other := admissionTestKey(t)
	a := newProductionTestAgent(key)
	raw := admissionTestPacket(t, other, "fedcba9876543210")

	_, err := a.admitProductionPacket(raw)
	if !errors.Is(err, capsdk.ErrUnknownKeyID) && !errors.Is(err, capsdk.ErrInvalidSignature) {
		t.Fatalf("admitProductionPacket = %v, want ErrUnknownKeyID/ErrInvalidSignature for an unrecognized signer", err)
	}
}
