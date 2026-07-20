package worker

import (
	"crypto/ecdsa"
	"strings"
	"testing"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestManagedWorkerLegacyModeRequiresExplicitUnsignedOptIn(t *testing.T) {
	for _, keys := range []map[string]*ecdsa.PublicKey{nil, {}} {
		config := ManagedConfig{WorkerTrustMode: capsdk.WorkerTrustModeOff, PublicKeys: keys}
		if err := validateManagedTrustConfig(config, "worker-legacy"); err == nil || !strings.Contains(err.Error(), "unsigned") {
			t.Fatalf("validation error = %v, want explicit unsigned opt-in requirement", err)
		}
	}
	config := ManagedConfig{WorkerTrustMode: capsdk.WorkerTrustModeOff}
	config.AllowUnsigned = true
	if err := validateManagedTrustConfig(config, "worker-legacy"); err != nil {
		t.Fatalf("explicit unsigned opt-in rejected: %v", err)
	}
}
