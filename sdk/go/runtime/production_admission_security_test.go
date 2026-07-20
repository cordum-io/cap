package runtime

import (
	"crypto/ecdsa"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type recordingReplayStore struct {
	calls   int
	outcome capsdk.ReplayOutcome
	err     error
	expires time.Time
}

func (s *recordingReplayStore) Admit(_ string, _ string, _ string, _ []byte, _ []byte, expires time.Time) (capsdk.ReplayOutcome, error) {
	s.calls++
	s.expires = expires
	return s.outcome, s.err
}

func TestAdmitProductionPacket_RejectsInvalidEnvelopeBeforeReplay(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	replay := &recordingReplayStore{outcome: capsdk.ReplayOutcomeFirst}
	a.Production.Replay = replay

	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(admissionTestPacket(t, key, "0123456789abcdef"), packet); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	packet.TraceId = ""
	raw, err := capsdk.SignProductionPacket(packet, key)
	if err != nil {
		t.Fatalf("sign invalid envelope: %v", err)
	}
	if _, err := a.admitProductionPacket(raw); err == nil {
		t.Fatal("admission accepted an unsupported protocol version")
	}
	if replay.calls != 0 {
		t.Fatalf("invalid envelope reached replay store %d times", replay.calls)
	}
}

func TestAdmitProductionPacket_UsesSignedBodyDigest(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	first := admissionTestPacket(t, key, "0123456789abcdef")
	packet, err := capsdk.VerifyProductionPacket(first, a.Production.Trust)
	if err != nil {
		t.Fatalf("verify fixture: %v", err)
	}
	second, err := capsdk.SignProductionPacket(packet, key)
	if err != nil {
		t.Fatalf("re-sign identical body: %v", err)
	}
	if string(first) == string(second) {
		t.Skip("ECDSA produced an identical signature; retry is statistically unnecessary")
	}
	if _, err := a.admitProductionPacket(first); err != nil {
		t.Fatalf("first admit: %v", err)
	}
	duplicate, err := a.admitProductionPacket(second)
	if err != nil {
		t.Fatalf("same signed body with a fresh signature rejected: %v", err)
	}
	if duplicate != nil {
		t.Fatal("same signed body must be treated as an idempotent duplicate")
	}
}

func TestAdmitProductionPacket_MissingReplayStoreFailsClosed(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	a.Production.Replay = nil
	raw := admissionTestPacket(t, key, "0123456789abcdef")

	if _, err := a.admitProductionPacket(raw); !errors.Is(err, capsdk.ErrReplayStoreUnavailable) {
		t.Fatalf("admitProductionPacket error=%v, want ErrReplayStoreUnavailable", err)
	}
}

func TestAdmitProductionPacket_RejectsUnknownReplayOutcome(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	a.Production.Replay = &recordingReplayStore{outcome: 0}
	raw := admissionTestPacket(t, key, "0123456789abcdef")

	if _, err := a.admitProductionPacket(raw); !errors.Is(err, capsdk.ErrReplayStoreUnavailable) {
		t.Fatalf("admitProductionPacket error=%v, want ErrReplayStoreUnavailable", err)
	}
}

func TestAdmitProductionPacket_NormalizesReplayBackendError(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	a.Production.Replay = &recordingReplayStore{err: errors.New("secret backend detail")}
	raw := admissionTestPacket(t, key, "0123456789abcdef")

	_, err := a.admitProductionPacket(raw)
	if !errors.Is(err, capsdk.ErrReplayStoreUnavailable) {
		t.Fatalf("admitProductionPacket error=%v, want ErrReplayStoreUnavailable", err)
	}
	if strings.Contains(err.Error(), "secret backend detail") {
		t.Fatalf("admission error exposed replay backend detail: %v", err)
	}
}

func TestAdmitProductionPacket_RetainsReplayThroughClockSkew(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	a.Production.Trust.ClockSkew = 45 * time.Second
	replay := &recordingReplayStore{outcome: capsdk.ReplayOutcomeFirst}
	a.Production.Replay = replay
	raw := admissionTestPacket(t, key, "0123456789abcdef")
	packet, err := capsdk.VerifyProductionPacket(raw, a.Production.Trust)
	if err != nil {
		t.Fatalf("verify fixture: %v", err)
	}

	if _, err := a.admitProductionPacket(raw); err != nil {
		t.Fatalf("admitProductionPacket: %v", err)
	}
	want := packet.GetSignatureMetadata().GetExpiresAt().AsTime().Add(a.Production.Trust.ClockSkew)
	if !replay.expires.Equal(want) {
		t.Fatalf("replay expiry = %v, want expiry plus skew %v", replay.expires, want)
	}
}

func TestHandleMessage_ProductionRequiresAuthenticatedSession(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	replay := &recordingReplayStore{outcome: capsdk.ReplayOutcomeDuplicate}
	a.Production.Replay = replay
	a.Logger = silentLogger()
	msg := &nats.Msg{Subject: "worker-pool-a", Data: admissionTestPacket(t, key, "0123456789abcdef")}

	a.handleMessage(msg, handlerSpec{})
	if replay.calls != 0 {
		t.Fatalf("packet without authenticated session reached replay store %d times", replay.calls)
	}
}

func TestAdmitProductionPacket_BindsActualSubjectAudience(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	raw := admissionTestPacket(t, key, "0123456789abcdef")

	_, err := a.admitProductionPacketForAudience(raw, "other.subject")
	if !errors.Is(err, capsdk.ErrAudienceMismatch) {
		t.Fatalf("admitProductionPacketForAudience error=%v, want ErrAudienceMismatch", err)
	}
}

func TestAdmitProductionPacket_IdentityFailureDoesNotPoisonReplay(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	invalid := &agentv1.BusPacket{}
	if err := proto.Unmarshal(admissionTestPacket(t, key, "0123456789abcdef"), invalid); err != nil {
		t.Fatalf("unmarshal invalid fixture: %v", err)
	}
	invalid.GetJobRequest().Env = map[string]string{"tenant_id": "tenant-b"}
	invalidRaw, err := capsdk.SignProductionPacket(invalid, key)
	if err != nil {
		t.Fatalf("sign invalid identity: %v", err)
	}
	if _, err := a.admitProductionPacket(invalidRaw); err == nil {
		t.Fatal("identity mismatch was admitted")
	}

	validRaw := admissionTestPacket(t, key, "0123456789abcdef")
	if packet, err := a.admitProductionPacket(validRaw); err != nil || packet == nil {
		t.Fatalf("corrected same-message-id packet = (%v, %v), want packet, nil", packet, err)
	}
}

func TestAgentStart_RejectsIncompleteProductionConfiguration(t *testing.T) {
	key := admissionTestKey(t)
	a := &Agent{
		NATS: newMockNATS(), Store: NewInMemoryBlobStore(), SenderID: "worker-a", Logger: silentLogger(),
		HandshakeMode: HandshakeModeEnforce,
		Production: ProductionAdmission{Enabled: true, Trust: capsdk.ProductionTrustStore{
			Audience: "job.test", PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey},
		}},
	}
	Register(a, "job.test", func(Context, struct{}) (struct{}, error) { return struct{}{}, nil })

	if err := a.Start(); !errors.Is(err, capsdk.ErrReplayStoreUnavailable) {
		t.Fatalf("Start error=%v, want ErrReplayStoreUnavailable", err)
	}
}

func TestAgentStart_ProductionRequiresEnforceHandshakeMode(t *testing.T) {
	key := admissionTestKey(t)
	a := &Agent{
		NATS: newMockNATS(), Store: NewInMemoryBlobStore(), SenderID: "worker-a", Logger: silentLogger(),
		HandshakeMode: HandshakeModeOff,
		Production: ProductionAdmission{Enabled: true, Replay: capsdk.NewInMemoryReplayStore(),
			Trust: capsdk.ProductionTrustStore{Audience: "job.test", PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey}}},
	}
	Register(a, "job.test", func(Context, struct{}) (struct{}, error) { return struct{}{}, nil })

	if err := a.Start(); !errors.Is(err, ErrProductionSessionRequired) {
		t.Fatalf("Start error=%v, want ErrProductionSessionRequired", err)
	}
}

func TestAdmitProductionPacket_BindsAuthenticatedSessionAuthority(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	a.trustMode = HandshakeModeEnforce
	a.trustConfig = &capsdk.WorkerTrustConfig{TenantID: "tenant-a", ExpectedSchedulerID: "scheduler-1"}
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(admissionTestPacket(t, key, "0123456789abcdef"), packet); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	packet.Identity.TenantId = "tenant-b"
	packet.GetJobRequest().TenantId = "tenant-b"
	packet.GetJobRequest().Identity.TenantId = "tenant-b"
	raw, err := capsdk.SignProductionPacket(packet, key)
	if err != nil {
		t.Fatalf("sign mismatched authority: %v", err)
	}

	if _, err := a.admitProductionPacket(raw); !errors.Is(err, capsdk.ErrUnknownKeyID) {
		t.Fatalf("admitProductionPacket error=%v, want session-scoped trust rejection", err)
	}
}

func TestAgentStart_RejectsProductionTrustSessionConflict(t *testing.T) {
	key := admissionTestKey(t)
	a := &Agent{
		NATS: newMockNATS(), Store: NewInMemoryBlobStore(), SenderID: "worker-a", Logger: silentLogger(),
		HandshakeMode: HandshakeModeEnforce,
		WorkerTrust:   capsdk.WorkerTrustConfig{TenantID: "tenant-a", ExpectedSchedulerID: "scheduler-1"},
		Production: ProductionAdmission{Enabled: true, Replay: capsdk.NewInMemoryReplayStore(),
			Trust: capsdk.ProductionTrustStore{Tenant: "tenant-b", PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey}}},
	}
	Register(a, "job.test", func(Context, struct{}) (struct{}, error) { return struct{}{}, nil })

	if err := a.Start(); !errors.Is(err, ErrProductionTrustConflict) {
		t.Fatalf("Start error=%v, want ErrProductionTrustConflict", err)
	}
}

func TestProductionAdmission_UsesFrozenStartupConfiguration(t *testing.T) {
	key := admissionTestKey(t)
	a := newProductionTestAgent(key)
	raw := admissionTestPacket(t, key, "0123456789abcdef")
	a.freezeProductionAdmission()
	frozenKey := a.activeProductionAdmission().Trust.PublicKeys["k1"]
	if frozenKey == nil || frozenKey == &key.PublicKey || frozenKey.X == key.X || frozenKey.Y == key.Y {
		t.Fatal("frozen production key did not deep-copy coordinates")
	}
	a.Production.Enabled = false
	a.Production.Replay = nil
	a.Production.Trust.PublicKeys["k1"] = nil
	key.X.SetInt64(1)
	key.Y.SetInt64(1)

	if !a.productionAdmissionEnabled() {
		t.Fatal("mutating exported Production disabled frozen production mode")
	}
	if packet, err := a.admitProductionPacket(raw); err != nil || packet == nil {
		t.Fatalf("frozen production admission = (%v, %v), want packet, nil", packet, err)
	}
}
