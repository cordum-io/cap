package capsdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
)

func TestParseWorkerTrustModeRequiresExplicitKnownValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  string
		want WorkerTrustMode
	}{
		{"off", WorkerTrustModeOff},
		{"warn", WorkerTrustModeWarn},
		{"enforce", WorkerTrustModeEnforce},
		{" WARN ", WorkerTrustModeWarn},
	}
	for _, test := range tests {
		test := test
		t.Run(test.raw, func(t *testing.T) {
			t.Parallel()
			got, err := ParseWorkerTrustMode(test.raw)
			if err != nil || got != test.want {
				t.Fatalf("ParseWorkerTrustMode(%q)=(%v,%v), want (%v,nil)", test.raw, got, err, test.want)
			}
		})
	}
	for _, raw := range []string{"", "optional", "true", "disabled"} {
		if _, err := ParseWorkerTrustMode(raw); !errors.Is(err, ErrWorkerTrustMode) || !strings.Contains(err.Error(), raw) {
			t.Errorf("ParseWorkerTrustMode(%q) error=%v, want typed error containing input", raw, err)
		}
	}
	if WorkerTrustModeOff.String() != "off" || WorkerTrustModeWarn.String() != "warn" ||
		WorkerTrustModeEnforce.String() != "enforce" || WorkerTrustMode(99).String() != "invalid" {
		t.Fatal("WorkerTrustMode.String returned a non-canonical value")
	}
}

func TestValidateWorkerTrustConfigRequiresCompletePinnedIdentity(t *testing.T) {
	t.Parallel()
	valid := validWorkerTrustConfig(t)
	tests := []struct {
		name   string
		mutate func(*WorkerTrustConfig)
	}{
		{"worker", func(c *WorkerTrustConfig) { c.WorkerID = "" }},
		{"agent", func(c *WorkerTrustConfig) { c.ExpectedAgentID = "" }},
		{"tenant", func(c *WorkerTrustConfig) { c.TenantID = "" }},
		{"audience", func(c *WorkerTrustConfig) { c.Audience = "" }},
		{"unexpected audience", func(c *WorkerTrustConfig) { c.Audience = "other" }},
		{"proof key id", func(c *WorkerTrustConfig) { c.ProofKeyID = "" }},
		{"proof key", func(c *WorkerTrustConfig) { c.ProofPrivateKey = nil }},
		{"scheduler id", func(c *WorkerTrustConfig) { c.ExpectedSchedulerID = "" }},
		{"scheduler keys", func(c *WorkerTrustConfig) { c.SchedulerPublicKeys = nil }},
		{"empty scheduler key id", func(c *WorkerTrustConfig) {
			var key *ecdsa.PublicKey
			for _, candidate := range c.SchedulerPublicKeys {
				key = candidate
			}
			c.SchedulerPublicKeys = map[string]*ecdsa.PublicKey{"": key}
		}},
		{"sdk version", func(c *WorkerTrustConfig) { c.SDKVersion = "" }},
		{"padded worker", func(c *WorkerTrustConfig) { c.WorkerID = " worker-1" }},
		{"padded sdk", func(c *WorkerTrustConfig) { c.SDKVersion = "v2.14.1 " }},
		{"embedded newline", func(c *WorkerTrustConfig) { c.WorkerID = "worker\nforged" }},
		{"embedded nul", func(c *WorkerTrustConfig) { c.ProofKeyID = "key\x00forged" }},
		{"oversized tenant", func(c *WorkerTrustConfig) { c.TenantID = strings.Repeat("t", WorkerHandshakeMaxIdentityLength+1) }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			config := cloneWorkerTrustConfig(valid)
			test.mutate(config)
			if err := ValidateWorkerTrustConfig(config); !errors.Is(err, ErrWorkerTrustConfig) {
				t.Fatalf("ValidateWorkerTrustConfig() error=%v, want %v", err, ErrWorkerTrustConfig)
			}
		})
	}
}

func validWorkerTrustConfig(t *testing.T) *WorkerTrustConfig {
	t.Helper()
	worker := generateWorkerTrustKey(t)
	scheduler := generateWorkerTrustKey(t)
	return &WorkerTrustConfig{
		WorkerID: "worker-1", ExpectedAgentID: "agent-1", TenantID: "tenant-1",
		Audience: WorkerHandshakeAudience, ProofKeyID: "worker-key-1", ProofPrivateKey: worker,
		ExpectedSchedulerID: "scheduler-1", SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"scheduler-key-1": &scheduler.PublicKey},
		SDKVersion: "v2.14.1",
	}
}

func cloneWorkerTrustConfig(config *WorkerTrustConfig) *WorkerTrustConfig {
	clone := *config
	clone.SchedulerPublicKeys = make(map[string]*ecdsa.PublicKey, len(config.SchedulerPublicKeys))
	for id, key := range config.SchedulerPublicKeys {
		clone.SchedulerPublicKeys[id] = key
	}
	return &clone
}

func generateWorkerTrustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	return key
}
