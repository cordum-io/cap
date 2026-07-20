package capsdk

// RED evidence for task-a13f83fa (CAP-PRODUCTION). These tests reference the
// not-yet-generated wire types (SignatureMetadata, IdentityBinding,
// DispatchIdentity, ResourceRef) and not-yet-implemented verification/replay
// APIs. They MUST fail to compile until step-4 regenerates the proto and
// steps 6-8 add the implementation. Do not make these pass by relaxing the
// assertions — only by implementing the real behavior.

import (
	"crypto/ecdsa"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type alwaysUnavailableReplayStore struct{}

func (*alwaysUnavailableReplayStore) Admit(string, string, string, []byte, []byte, time.Time) (ReplayOutcome, error) {
	return 0, ErrReplayStoreUnavailable
}

func appendDuplicateSignatureField(raw, signature []byte) []byte {
	return appendSignatureField(raw, signature)
}

// --- Threat 1: missing signature metadata is rejected in production ---

func TestProductionVerify_RejectsMissingSignatureMetadata(t *testing.T) {
	key := testPrivateSigningKey()
	packet := authTokenPayloadPacket()
	if err := SignPacket(packet, key); err != nil {
		t.Fatalf("SignPacket: %v", err)
	}
	packet.SignatureMetadata = nil // no metadata at all

	raw, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, err = VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a"))
	if !errors.Is(err, ErrMissingSignatureMetadata) {
		t.Fatalf("VerifyProductionPacket error = %v, want ErrMissingSignatureMetadata", err)
	}
}

// --- Threat 2: expired / wrong-audience / unknown-key are rejected ---

func TestProductionVerify_RejectsExpiredAudienceUnknownKey(t *testing.T) {
	key := testPrivateSigningKey()

	cases := []struct {
		name    string
		mutate  func(*agentv1.SignatureMetadata)
		wantErr error
	}{
		{"expired", func(m *agentv1.SignatureMetadata) { m.ExpiresAt = pastTimestamp() }, ErrSignatureExpired},
		{"wrong-audience", func(m *agentv1.SignatureMetadata) { m.Audience = "not-the-real-tenant" }, ErrAudienceMismatch},
		{"unknown-key", func(m *agentv1.SignatureMetadata) { m.KeyId = "no-such-key" }, ErrUnknownKeyID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packet := productionPacketSignedForTest(t, key, "tenant-a")
			tc.mutate(packet.SignatureMetadata)
			var raw []byte
			if tc.name == "expired" {
				unsigned, err := proto.Marshal(packet)
				if err != nil {
					t.Fatalf("proto.Marshal: %v", err)
				}
				raw = signProductionWireForTest(t, unsigned, key)
			} else {
				var err error
				raw, err = SignProductionPacket(packet, key)
				if err != nil {
					t.Fatalf("SignProductionPacket: %v", err)
				}
			}
			_, err := VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a"))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyProductionPacket error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestProductionSignerRejectsExpiredMetadata(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.SignatureMetadata.ExpiresAt = pastTimestamp()
	if _, err := SignProductionPacket(packet, key); !errors.Is(err, ErrSignatureExpired) {
		t.Fatalf("SignProductionPacket error=%v, want ErrSignatureExpired", err)
	}
}

// --- Threat 3: duplicate/malformed signature field is rejected (parser-differential defense) ---

func TestProductionVerify_RejectsDuplicateOrMalformedSignatureField(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	raw, err := SignProductionPacket(packet, key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	tampered := appendDuplicateSignatureField(raw, []byte("bogus"))
	_, err = VerifyProductionPacket(tampered, strictProductionTrust(key, "tenant-a"))
	if !errors.Is(err, ErrDuplicateSignatureField) {
		t.Fatalf("VerifyProductionPacket error = %v, want ErrDuplicateSignatureField", err)
	}
}

// --- Threat 4: replay defense distinguishes identical redelivery from a same-id/different-body conflict ---

func TestReplayStore_IdenticalRedeliveryIsHarmlessDifferentBodyConflicts(t *testing.T) {
	store := NewInMemoryReplayStore()
	msgID := []byte("0123456789abcdef")
	digestA := []byte("digest-a-32-bytes-aaaaaaaaaaaaaa")
	digestB := []byte("digest-b-32-bytes-bbbbbbbbbbbbbb")

	outcome1, err := store.Admit("tenant-a", "audience-a", "sender-a", msgID, digestA, time.Now().Add(time.Minute))
	if err != nil || outcome1 != ReplayOutcomeFirst {
		t.Fatalf("first admit = (%v, %v), want (ReplayOutcomeFirst, nil)", outcome1, err)
	}
	outcome2, err := store.Admit("tenant-a", "audience-a", "sender-a", msgID, digestA, time.Now().Add(time.Minute))
	if err != nil || outcome2 != ReplayOutcomeDuplicate {
		t.Fatalf("identical redelivery = (%v, %v), want (ReplayOutcomeDuplicate, nil)", outcome2, err)
	}
	_, err = store.Admit("tenant-a", "audience-a", "sender-a", msgID, digestB, time.Now().Add(time.Minute))
	if !errors.Is(err, ErrReplayConflict) {
		t.Fatalf("different-body same-id = %v, want ErrReplayConflict", err)
	}
}

func TestReplayStore_UnavailableStoreFailsClosed(t *testing.T) {
	store := &alwaysUnavailableReplayStore{}
	_, err := store.Admit("tenant-a", "audience-a", "sender-a", []byte("0123456789abcdef"), []byte("d"), time.Now().Add(time.Minute))
	if !errors.Is(err, ErrReplayStoreUnavailable) {
		t.Fatalf("Admit on unavailable store = %v, want ErrReplayStoreUnavailable", err)
	}
}

// --- Threat 5: identity disagreement across mirrors is rejected ---

func TestIdentityBinding_RejectsMirrorMismatch(t *testing.T) {
	authoritative := &agentv1.IdentityBinding{TenantId: "tenant-a", PrincipalId: "principal-a"}
	req := &agentv1.JobRequest{
		TenantId:    "tenant-a",
		PrincipalId: "principal-a",
		Identity:    authoritative,
		Env:         map[string]string{"tenant_id": "tenant-B-DIFFERENT"},
	}
	if err := ValidateIdentityBinding(req, authoritative); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("ValidateIdentityBinding = %v, want ErrIdentityMismatch", err)
	}
}

// --- Threat 6: stale/future/wrong-worker Result/Progress/Cancel never mutate a different attempt ---

func TestDispatchFencing_RejectsStaleFutureWrongWorkerEvents(t *testing.T) {
	current := &agentv1.DispatchIdentity{DispatchId: "d-current", Attempt: 2, AssignedWorkerId: "worker-1"}

	cases := []struct {
		name  string
		event *agentv1.DispatchIdentity
	}{
		{"stale-attempt", &agentv1.DispatchIdentity{DispatchId: "d-current", Attempt: 1, AssignedWorkerId: "worker-1"}},
		{"future-attempt", &agentv1.DispatchIdentity{DispatchId: "d-current", Attempt: 3, AssignedWorkerId: "worker-1"}},
		{"wrong-worker", &agentv1.DispatchIdentity{DispatchId: "d-current", Attempt: 2, AssignedWorkerId: "worker-EVIL"}},
		{"wrong-dispatch-id", &agentv1.DispatchIdentity{DispatchId: "d-OTHER", Attempt: 2, AssignedWorkerId: "worker-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateDispatchFencing(current, tc.event); !errors.Is(err, ErrStaleDispatchEvent) {
				t.Fatalf("ValidateDispatchFencing(%s) = %v, want ErrStaleDispatchEvent", tc.name, err)
			}
		})
	}
}

// --- Threat 7: compensation cannot escalate identity, capability, or risk beyond the parent ---

func TestCompensation_RejectsPrivilegeEscalation(t *testing.T) {
	parent := &agentv1.JobRequest{
		TenantId: "tenant-a",
		Identity: &agentv1.IdentityBinding{TenantId: "tenant-a", PrincipalId: "principal-a"},
		Meta:     &agentv1.JobMetadata{Capability: "read-only", RiskTags: []string{"read"}},
	}
	escalated := &agentv1.Compensation{
		Identity: &agentv1.IdentityBinding{TenantId: "tenant-a", PrincipalId: "principal-ADMIN"},
		Meta:     &agentv1.JobMetadata{Capability: "delete-prod", RiskTags: []string{"write", "prod"}},
	}
	if err := ValidateCompensationMonotonicity(parent, escalated); !errors.Is(err, ErrCompensationEscalation) {
		t.Fatalf("ValidateCompensationMonotonicity = %v, want ErrCompensationEscalation", err)
	}
}

// --- Threat 8: ResourceRef rejects credentials, traversal, unknown resolver, expiry, digest/type/size conflict ---

func TestResourceRef_RejectsUnsafeReferences(t *testing.T) {
	cases := []struct {
		name string
		ref  *agentv1.ResourceRef
	}{
		{"embedded-credential", &agentv1.ResourceRef{ResolverId: "redis", Uri: "redis://user:pass@host/0/key", Sha256: valid32Bytes(), SizeBytes: 10, MediaType: "application/json"}},
		{"path-traversal", &agentv1.ResourceRef{ResolverId: "redis", Uri: "redis://host/0/../../etc/passwd", Sha256: valid32Bytes(), SizeBytes: 10, MediaType: "application/json"}},
		{"unknown-resolver", &agentv1.ResourceRef{ResolverId: "not-installed", Uri: "redis://host/0/key", Sha256: valid32Bytes(), SizeBytes: 10, MediaType: "application/json"}},
		{"bad-digest-length", &agentv1.ResourceRef{ResolverId: "redis", Uri: "redis://host/0/key", Sha256: []byte("short"), SizeBytes: 10, MediaType: "application/json"}},
		{"expired", &agentv1.ResourceRef{ResolverId: "redis", Uri: "redis://host/0/key", Sha256: valid32Bytes(), SizeBytes: 10, MediaType: "application/json", ExpiresAt: pastTimestamp()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResourceRef(tc.ref, []string{"redis"}); err == nil {
				t.Fatalf("ValidateResourceRef(%s) = nil, want rejection", tc.name)
			}
		})
	}
}

// --- test helpers referencing not-yet-existing symbols (intentional RED) ---

func productionPacketSignedForTest(t *testing.T, key *ecdsa.PrivateKey, audience string) *agentv1.BusPacket {
	t.Helper()
	packet := authTokenPayloadPacket()
	packet.Identity = &agentv1.IdentityBinding{
		TenantId: "tenant-a", PrincipalId: "principal-a", ActorId: "actor-a",
	}
	packet.SignatureMetadata = &agentv1.SignatureMetadata{
		ProfileVersion: "cap-production-v1",
		Algorithm:      "ECDSA-P256-SHA256",
		MessageId:      []byte("0123456789abcdef"),
		Audience:       audience,
		ExpiresAt:      futureTimestamp(),
		KeyId:          "k1",
	}
	return packet
}

func strictProductionTrust(key *ecdsa.PrivateKey, audience string) ProductionTrustStore {
	return ProductionTrustStore{
		Audience: audience, Tenant: "tenant-a", Sender: "worker-1",
		PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey},
	}
}

func pastTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Now().Add(-time.Hour))
}

func futureTimestamp() *timestamppb.Timestamp {
	return futureTimestampAfter(2 * time.Minute)
}

func futureTimestampAfter(duration time.Duration) *timestamppb.Timestamp {
	return timestamppb.New(time.Now().Add(duration))
}

func valid32Bytes() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}
