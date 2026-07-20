package worker

import (
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestManagedProductionRejectsUnsafeConfigBeforeConnect(t *testing.T) {
	workerKey := managedP256Key(t)
	trust := managedTrustConfig(t, "worker-production")
	valid := ManagedConfig{
		WorkerID: "worker-production", Subjects: []string{"job.production"}, NatsURL: "nats://127.0.0.1:1",
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: trust, PrivateKey: workerKey,
		Production: ManagedProductionConfig{
			Enabled: true, KeyID: "worker-key", Replay: capsdk.NewInMemoryReplayStore(),
			Trust: capsdk.ProductionTrustStore{PublicKeys: trust.SchedulerPublicKeys},
		},
	}
	tests := map[string]func(*ManagedConfig){
		"unsigned compatibility": func(c *ManagedConfig) { c.AllowUnsigned = true },
		"trust not enforced":     func(c *ManagedConfig) { c.WorkerTrustMode = capsdk.WorkerTrustModeWarn },
		"missing signer":         func(c *ManagedConfig) { c.PrivateKey = nil },
		"mismatched signer": func(c *ManagedConfig) {
			c.PrivateKey = cloneECDSAPrivateKey(workerKey)
			c.PrivateKey.D = big.NewInt(1)
		},
		"missing key id": func(c *ManagedConfig) { c.Production.KeyID = "" },
		"missing replay": func(c *ManagedConfig) { c.Production.Replay = nil },
		"negative lifetime": func(c *ManagedConfig) {
			c.Production.MessageLifetime = -time.Second
		},
		"excessive lifetime": func(c *ManagedConfig) {
			c.Production.MessageLifetime = capsdk.DefaultProductionMaxLifetime + time.Second
		},
		"missing scheduler keys": func(c *ManagedConfig) {
			c.Production.Trust.PublicKeys = nil
		},
		"padded key id": func(c *ManagedConfig) { c.Production.KeyID = " worker-key" },
	}
	if _, err := resolveManagedConfig(valid); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := valid
			mutate(&config)
			if _, err := resolveManagedConfig(config); err == nil {
				t.Fatal("config resolution accepted unsafe production config")
			}
		})
	}
}

func TestManagedProductionRejectsPlaintextNATS(t *testing.T) {
	for _, name := range []string{"NATS_TLS_CA", "NATS_TLS_CERT", "NATS_TLS_KEY", "NATS_TLS_SERVER_NAME", "NATS_TLS_INSECURE"} {
		t.Setenv(name, "")
	}
	_, err := connectManagedNATS(
		ManagedConfig{Production: ManagedProductionConfig{Enabled: true}},
		resolvedManagedConfig{workerID: "worker-production", natsURL: "nats://127.0.0.1:1"},
	)
	if err == nil || !strings.Contains(err.Error(), "NATS TLS required") {
		t.Fatalf("plaintext production NATS error=%v, want TLS rejection", err)
	}
	_, err = connectManagedNATS(
		ManagedConfig{
			Production:    ManagedProductionConfig{Enabled: true},
			NATSTLSConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // deliberate rejection fixture
		},
		resolvedManagedConfig{workerID: "worker-production", natsURL: "nats://127.0.0.1:1"},
	)
	if err == nil || !strings.Contains(err.Error(), "TLS verification required") {
		t.Fatalf("unverified production TLS error=%v, want verification rejection", err)
	}
}

func TestManagedProductionRecognizesOnlyTLSNATSEndpoints(t *testing.T) {
	tests := map[string]bool{
		"tls://nats.example:4222":                        true,
		"wss://nats.example:443":                         true,
		"tls://one.example:4222,tls://two.example:4222":  true,
		"nats://nats.example:4222":                       false,
		"tls://one.example:4222,nats://two.example:4222": false,
		"": false,
	}
	for endpoint, want := range tests {
		if got := managedNATSTransportSecure(endpoint); got != want {
			t.Errorf("managedNATSTransportSecure(%q)=%v, want %v", endpoint, got, want)
		}
	}
}

func TestManagedProductionRawAdmissionRejectsTamperAndActualAudienceMismatch(t *testing.T) {
	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "message-id-00001")

	if _, _, err := worker.decodeManagedRequest(raw, "job.production.other"); !errors.Is(err, capsdk.ErrAudienceMismatch) {
		t.Fatalf("wrong actual subject error=%v, want ErrAudienceMismatch", err)
	}
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)/2] ^= 0x01
	if _, _, err := worker.decodeManagedRequest(tampered, "job.production.real"); err == nil {
		t.Fatal("tampered production request reached managed dispatch")
	}
	tests := []struct {
		name   string
		mutate func(*agentv1.BusPacket)
		want   error
	}{
		{"identity conflict", func(p *agentv1.BusPacket) { p.GetJobRequest().Identity.TenantId = "attacker" }, capsdk.ErrIdentityMismatch},
		{"incomplete identity", func(p *agentv1.BusPacket) { p.Identity.ActorId, p.GetJobRequest().Identity.ActorId = "", "" }, capsdk.ErrIdentityMismatch},
		{"topic mismatch", func(p *agentv1.BusPacket) { p.GetJobRequest().Topic = "job.production.other" }, capsdk.ErrAudienceMismatch},
		{"wrong worker", func(p *agentv1.BusPacket) { p.GetJobRequest().Dispatch.AssignedWorkerId = "worker-other" }, capsdk.ErrStaleDispatchEvent},
	}
	for _, test := range tests {
		mutated := mutateManagedProductionRequestWire(t, raw, schedulerKey, test.mutate)
		if _, _, err := worker.decodeManagedRequest(mutated, "job.production.real"); !errors.Is(err, test.want) {
			t.Fatalf("%s error=%v, want %v", test.name, err, test.want)
		}
	}
}

func TestManagedProductionDuplicateIsDroppedBeforeDispatch(t *testing.T) {
	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "message-id-00002")
	if _, _, err := worker.decodeManagedRequest(raw, "job.production.real"); err != nil {
		t.Fatalf("first admission: %v", err)
	}
	if _, _, err := worker.decodeManagedRequest(raw, "job.production.real"); !errors.Is(err, errManagedProductionDuplicate) {
		t.Fatalf("redelivery error=%v, want duplicate drop", err)
	}
}

func managedProductionWorkerForTest(t *testing.T, schedulerKey *ecdsa.PrivateKey) *ManagedWorker {
	t.Helper()
	trust := managedTrustConfig(t, "worker-production")
	trust.SchedulerPublicKeys = map[string]*ecdsa.PublicKey{"scheduler-key": &schedulerKey.PublicKey}
	worker := &ManagedWorker{
		workerID: "worker-production",
		cfg: ManagedConfig{
			WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: trust, PrivateKey: managedP256Key(t),
			Production: ManagedProductionConfig{
				Enabled: true, KeyID: "worker-key", Replay: capsdk.NewInMemoryReplayStore(),
				Trust: capsdk.ProductionTrustStore{PublicKeys: trust.SchedulerPublicKeys},
			},
		},
		trust: newManagedTrustState(ManagedConfig{WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: trust}),
	}
	worker.trust.setSession(&capsdk.WorkerHandshakeSession{Token: "session-live", ExpiresAt: time.Now().Add(time.Hour)})
	return worker
}

func managedProductionRequestWire(t *testing.T, key *ecdsa.PrivateKey, audience, messageID string) []byte {
	t.Helper()
	identity := &agentv1.IdentityBinding{TenantId: "tenant-trust", PrincipalId: "principal-1", ActorId: "actor-1"}
	dispatch := &agentv1.DispatchIdentity{DispatchId: "dispatch-1", Attempt: 1, AssignedWorkerId: "worker-production"}
	packet := &agentv1.BusPacket{
		TraceId: "trace-production", SenderId: "scheduler-trust", ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt: timestamppb.Now(), Identity: identity,
		SignatureMetadata: &agentv1.SignatureMetadata{
			ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm,
			MessageId: []byte(messageID), Audience: audience, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
			KeyId: "scheduler-key",
		},
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
			JobId: "job-production", Topic: audience, TenantId: identity.TenantId, PrincipalId: identity.PrincipalId,
			Meta:     &agentv1.JobMetadata{TenantId: identity.TenantId, ActorId: identity.ActorId},
			Identity: identity, Dispatch: dispatch,
		}},
	}
	raw, err := capsdk.SignProductionPacket(packet, key)
	if err != nil {
		t.Fatalf("sign production request: %v", err)
	}
	return raw
}

func mutateManagedProductionRequestWire(
	t *testing.T, raw []byte, key *ecdsa.PrivateKey, mutate func(*agentv1.BusPacket),
) []byte {
	t.Helper()
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(raw, packet); err != nil {
		t.Fatal(err)
	}
	mutate(packet)
	mutated, err := capsdk.SignProductionPacket(packet, key)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}
