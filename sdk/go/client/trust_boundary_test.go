package client

import (
	"crypto/ecdsa"
	"strings"
	"testing"
)

func TestClientLegacyTrustRequiresExplicitUnsignedOptIn(t *testing.T) {
	for _, keys := range []map[string]*ecdsa.PublicKey{nil, {}} {
		client := &Client{PublicKeys: keys}
		if err := client.validateLegacyTrust(); err == nil || !strings.Contains(err.Error(), "unsigned") {
			t.Fatalf("validation error = %v, want explicit unsigned opt-in requirement", err)
		}
	}
	client := &Client{}
	client.AllowUnsigned = true
	if err := client.validateLegacyTrust(); err != nil {
		t.Fatalf("explicit unsigned opt-in rejected: %v", err)
	}
}
