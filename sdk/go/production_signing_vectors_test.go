package capsdk

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"
	"time"
)

type productionVector struct {
	RawBase64         string `json:"raw_base64"`
	UnsignedBase64    string `json:"unsigned_base64"`
	SignatureBase64   string `json:"signature_base64"`
	PreimageDigestHex string `json:"preimage_digest_hex"`
	PublicKeyPEM      string `json:"public_key_pem"`
}

func TestProductionSigningVectorVerifiesExactGoWire(t *testing.T) {
	data, err := os.ReadFile("../../test/fixtures/production-signing-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture productionVector
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(fixture.RawBase64)
	unsigned, signature, err := extractSignatureField(raw)
	if err != nil {
		t.Fatal(err)
	}
	if base64.StdEncoding.EncodeToString(unsigned) != fixture.UnsignedBase64 || base64.StdEncoding.EncodeToString(signature) != fixture.SignatureBase64 {
		t.Fatal("raw extraction changed fixture bytes")
	}
	digest := productionDigest(unsigned)
	if hex.EncodeToString(digest[:]) != fixture.PreimageDigestHex {
		t.Fatal("production preimage digest mismatch")
	}
	block, _ := pem.Decode([]byte(fixture.PublicKeyPEM))
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	key := parsed.(*ecdsa.PublicKey)
	vectorNow := time.Date(2098, 12, 31, 23, 58, 0, 0, time.UTC)
	trust := ProductionTrustStore{
		Audience: "sys.job.vector", Tenant: "tenant-vector", Sender: "worker-vector",
		Now: func() time.Time { return vectorNow }, PublicKeys: map[string]*ecdsa.PublicKey{"vector-key": key},
	}
	if _, err := VerifyProductionPacket(raw, trust); err != nil {
		t.Fatalf("verify vector: %v", err)
	}

	tampered := append([]byte(nil), raw...)
	tampered[1] ^= 1
	if _, err := VerifyProductionPacket(tampered, trust); err == nil {
		t.Fatal("tampered raw packet was accepted")
	}
}
