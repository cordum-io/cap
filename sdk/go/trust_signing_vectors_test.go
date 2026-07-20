package capsdk

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

func TestTrustHandshakePositiveVectors(t *testing.T) {
	manifest, root := readHandshakeVectorManifest(t)
	keys := map[string]*ecdsa.PublicKey{}
	for _, signer := range []string{"worker", "scheduler"} {
		metadata := manifest.Keys[signer]
		keys[metadata.ID] = readTrustVectorPublicKey(t, filepath.Join(root, metadata.PublicKey))
	}

	for phase, vector := range manifest.Positive {
		t.Run(phase, func(t *testing.T) {
			packet := readTrustVectorPacket(t, filepath.Join(root, vector.Packet))
			_, domain, err := SelectTrustHandshakePhase(packet)
			if err != nil || domain != vector.Domain {
				t.Fatalf("domain=%q error=%v, want %q,nil", domain, err, vector.Domain)
			}
			digest, err := TrustHandshakeDigest(packet)
			if err != nil || hex.EncodeToString(digest[:]) != vector.DigestSHA256 {
				t.Fatalf("digest=%x error=%v, want %s,nil", digest, err, vector.DigestSHA256)
			}
			if err := VerifyTrustHandshake(packet, keys); err != nil {
				t.Fatalf("verify reference signature: %v", err)
			}
		})
	}
}

func readTrustVectorPacket(t *testing.T, path string) *agentv1.BusPacket {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vector packet: %v", err)
	}
	var packet agentv1.BusPacket
	if err := proto.Unmarshal(data, &packet); err != nil {
		t.Fatalf("decode vector packet: %v", err)
	}
	return &packet
}

func readTrustVectorPublicKey(t *testing.T, path string) *ecdsa.PublicKey {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vector public key: %v", err)
	}
	block, rest := pem.Decode(data)
	if block == nil || len(rest) != 0 || block.Type != "PUBLIC KEY" {
		t.Fatalf("invalid public key PEM %s", path)
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("public key type=%T, want ECDSA", parsed)
	}
	return key
}
