package worker

import (
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestManagedProductionRejectsNestedIdentityMirrors(t *testing.T) {
	tests := map[string]func(*agentv1.JobRequest){
		"environment actor": func(request *agentv1.JobRequest) {
			request.Env = map[string]string{"actor_id": "actor-attacker"}
		},
		"environment delegation": func(request *agentv1.JobRequest) {
			request.Env = map[string]string{"delegation_id": "delegation-attacker"}
		},
		"request label tenant": func(request *agentv1.JobRequest) {
			request.Labels = map[string]string{"tenant_id": "tenant-attacker"}
		},
		"metadata label principal": func(request *agentv1.JobRequest) {
			request.Meta.Labels = map[string]string{"principal_id": "principal-attacker"}
		},
		"compensation identity": func(request *agentv1.JobRequest) {
			request.Compensation = &agentv1.Compensation{
				Identity: &agentv1.IdentityBinding{TenantId: "tenant-attacker"},
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			assertManagedProductionRequestRejected(t, mutate, capsdk.ErrIdentityMismatch)
		})
	}
}

func TestManagedProductionRejectsCompensationEscalation(t *testing.T) {
	assertManagedProductionRequestRejected(t, func(request *agentv1.JobRequest) {
		request.Meta.Capability = "read"
		request.Meta.RiskTags = []string{"read"}
		request.Compensation = &agentv1.Compensation{
			TenantId: request.Identity.TenantId, PrincipalId: request.Identity.PrincipalId,
			Identity: &agentv1.IdentityBinding{
				TenantId: request.Identity.TenantId, PrincipalId: request.Identity.PrincipalId, ActorId: request.Identity.ActorId,
			},
			Meta: &agentv1.JobMetadata{Capability: "admin", RiskTags: []string{"prod", "write"}},
		}
	}, capsdk.ErrCompensationEscalation)
}

func TestManagedProductionRejectsUnsafeResourceBeforeHandler(t *testing.T) {
	assertManagedProductionRequestRejected(t, func(request *agentv1.JobRequest) {
		request.ContextRef = &agentv1.ResourceRef{
			Uri:       "https://user:secret@example.test/input?token=secret",
			Sha256:    []byte{1},
			ExpiresAt: timestamppb.New(time.Now().Add(-time.Hour)),
		}
	}, capsdk.ErrInvalidResourceRef)
}

func TestManagedProductionRejectsInvalidStaticTrustAtStartup(t *testing.T) {
	config := managedProductionConfigForValidation(t)
	config.Production.Trust.PublicKeys = map[string]*ecdsa.PublicKey{" bad-key ": nil}
	if _, err := resolveManagedConfig(config); err == nil {
		t.Fatal("production config accepted a noncanonical nil scheduler key")
	}
}

func TestManagedProductionRequiresResourceResolversAtStartup(t *testing.T) {
	config := managedProductionConfigForValidation(t)
	config.Production.ResourceResolvers = nil
	if _, err := resolveManagedConfig(config); err == nil {
		t.Fatal("production config accepted an empty resource-resolver allowlist")
	}
}

func TestManagedProductionRequiresDurableSettlementStoreAtStartup(t *testing.T) {
	config := managedProductionConfigForValidation(t)
	store := newManagedReplayTestStore()
	store.durable = false
	config.Production.Replay = store
	if _, err := resolveManagedConfig(config); err == nil {
		t.Fatal("production config accepted an admit-only in-memory replay store")
	}
}

func TestManagedProductionRejectsOversizedKeyIDAtStartup(t *testing.T) {
	config := managedProductionConfigForValidation(t)
	config.Production.KeyID = strings.Repeat("x", capsdk.WorkerHandshakeMaxIdentityLength+1)
	if _, err := resolveManagedConfig(config); err == nil {
		t.Fatal("production config accepted an oversized signing key id")
	}
}

func TestManagedProductionRequiresClientAuthenticatedTLS(t *testing.T) {
	_, err := connectManagedNATS(
		ManagedConfig{
			Production:    ManagedProductionConfig{Enabled: true},
			NATSTLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
		resolvedManagedConfig{workerID: "worker-production", natsURL: "tls://127.0.0.1:1"},
	)
	if err == nil || !strings.Contains(err.Error(), "client certificate") {
		t.Fatalf("anonymous TLS error=%v, want client-certificate rejection", err)
	}
}

func assertManagedProductionRequestRejected(
	t *testing.T, mutate func(*agentv1.JobRequest), want error,
) {
	t.Helper()
	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "adversarial-id01")
	raw = mutateManagedProductionRequestWire(t, raw, schedulerKey, func(packet *agentv1.BusPacket) {
		mutate(packet.GetJobRequest())
	})
	if _, _, err := worker.decodeManagedRequest(raw, "job.production.real"); !errors.Is(err, want) {
		t.Fatalf("decode error=%v, want %v", err, want)
	}
}

func managedProductionConfigForValidation(t *testing.T) ManagedConfig {
	t.Helper()
	trust := managedTrustConfig(t, "worker-production")
	return ManagedConfig{
		WorkerID: "worker-production", Subjects: []string{"job.production"},
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: trust,
		PrivateKey: managedP256Key(t),
		Production: ManagedProductionConfig{
			Enabled: true, KeyID: "worker-key", Replay: newManagedReplayTestStore(),
			Stream:            "CAP_PRODUCTION",
			ResourceResolvers: []string{"redis"},
			Trust:             capsdk.ProductionTrustStore{PublicKeys: trust.SchedulerPublicKeys},
		},
	}
}
