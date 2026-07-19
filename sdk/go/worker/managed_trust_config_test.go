package worker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestManagedWorkerRejectsInvalidTrustConfigBeforeConnect(t *testing.T) {
	valid := managedTrustConfig(t, "worker-trust")
	tests := []struct {
		name    string
		mode    capsdk.WorkerTrustMode
		trust   *capsdk.WorkerTrustConfig
		worker  string
		timeout time.Duration
	}{
		{name: "unknown mode", mode: capsdk.WorkerTrustMode(99), worker: "worker-trust"},
		{name: "warn missing config", mode: capsdk.WorkerTrustModeWarn, worker: "worker-trust"},
		{name: "enforce missing config", mode: capsdk.WorkerTrustModeEnforce, worker: "worker-trust"},
		{name: "off rejects dormant trust", mode: capsdk.WorkerTrustModeOff, trust: valid, worker: "worker-trust"},
		{name: "identity mismatch", mode: capsdk.WorkerTrustModeWarn, trust: valid, worker: "other-worker"},
		{name: "unbounded timeout", mode: capsdk.WorkerTrustModeEnforce, trust: valid, worker: "worker-trust", timeout: 2 * capsdk.WorkerHandshakeMaxLifetime},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			timeout := test.timeout
			if timeout == 0 {
				timeout = 20 * time.Millisecond
			}
			_, err := NewManagedWorker(ManagedConfig{
				WorkerID: test.worker, Subjects: []string{"job.secure"}, NatsURL: "nats://127.0.0.1:1",
				WorkerTrustMode: test.mode, WorkerTrust: test.trust, WorkerTrustTimeout: timeout,
			})
			if err == nil || !strings.Contains(err.Error(), "worker trust") {
				t.Fatalf("NewManagedWorker() error=%v, want pre-connect worker trust error", err)
			}
		})
	}
}

func TestManagedWorkerExplicitOffConnectsWithoutTrustMaterial(t *testing.T) {
	_, natsURL := startTestNATS(t)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-off", Subjects: []string{"job.off"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
	})
	if err != nil {
		t.Fatalf("explicit off must connect without trust material: %v", err)
	}
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}

func TestManagedWorkerClonesCallerOwnedTrustMaterial(t *testing.T) {
	_, natsURL := startTestNATS(t)
	trust := managedTrustConfig(t, "worker-clone")
	wantProofD := trust.ProofPrivateKey.D.String()
	wantSchedulerX := trust.SchedulerPublicKeys["scheduler-trust-key"].X.String()
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-clone", Subjects: []string{"job.clone"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: trust,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Close() })
	trust.ExpectedSchedulerID = "scheduler-attacker"
	trust.ProofPrivateKey.D.SetInt64(1)
	delete(trust.SchedulerPublicKeys, "scheduler-trust-key")
	got := worker.trust.config
	if got.ExpectedSchedulerID != "scheduler-trust" || got.ProofPrivateKey.D.String() != wantProofD {
		t.Fatal("caller mutation changed worker proof identity")
	}
	key := got.SchedulerPublicKeys["scheduler-trust-key"]
	if key == nil || key.X.String() != wantSchedulerX {
		t.Fatal("caller mutation changed pinned scheduler keys")
	}
}

func managedTrustConfig(t *testing.T, workerID string) *capsdk.WorkerTrustConfig {
	t.Helper()
	proof := managedP256Key(t)
	scheduler := managedP256Key(t)
	return &capsdk.WorkerTrustConfig{
		WorkerID: workerID, ExpectedAgentID: "agent-trust", TenantID: "tenant-trust",
		Audience: capsdk.WorkerHandshakeAudience, ProofKeyID: "proof-trust", ProofPrivateKey: proof,
		ExpectedSchedulerID: "scheduler-trust",
		SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"scheduler-trust-key": &scheduler.PublicKey},
		SDKVersion:          "cap-go/v2",
	}
}

func managedP256Key(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	return key
}
