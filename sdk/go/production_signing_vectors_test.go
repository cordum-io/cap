package capsdk

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

// Go consumer for the cross-language conformance vectors in
// test/fixtures/production-signing-v1.json. Python (tests/test_production_signing.py)
// and Node (test/production-signing.test.ts) read the SAME file and must reach
// the same verdict on every vector; that agreement is what makes the fixture a
// conformance artifact rather than three parallel local test suites.

const productionFixturePath = "../../test/fixtures/production-signing-v1.json"

type productionVector struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Expect            string   `json:"expect"`
	RejectReason      string   `json:"reject_reason"`
	TrustKeyIDs       []string `json:"trust_key_ids"`
	RawBase64         string   `json:"raw_base64"`
	UnsignedBase64    string   `json:"unsigned_base64"`
	SignatureBase64   string   `json:"signature_base64"`
	PreimageDigestHex string   `json:"preimage_digest_hex"`
	BodyDigestHex     string   `json:"body_digest_hex"`
	MessageIDHex      string   `json:"message_id_hex"`
	Audience          string   `json:"audience"`
	KeyID             string   `json:"key_id"`
	ExpiresAtRFC3339  string   `json:"expires_at_rfc3339"`
}

type productionFixture struct {
	// Legacy flat keys, still read so a pre-schema-2 consumer stays valid.
	RawBase64         string `json:"raw_base64"`
	UnsignedBase64    string `json:"unsigned_base64"`
	SignatureBase64   string `json:"signature_base64"`
	PreimageDigestHex string `json:"preimage_digest_hex"`
	PublicKeyPEM      string `json:"public_key_pem"`

	SchemaVersion   int    `json:"schema_version"`
	DomainBase64    string `json:"domain_base64"`
	VerifyAtRFC3339 string `json:"verify_at_rfc3339"`
	Trust           struct {
		Audience   string            `json:"audience"`
		Tenant     string            `json:"tenant"`
		Sender     string            `json:"sender"`
		PublicKeys map[string]string `json:"public_keys"`
	} `json:"trust"`
	Vectors       []productionVector `json:"vectors"`
	ReplayVectors []struct {
		Name     string `json:"name"`
		Sequence []struct {
			Vector string `json:"vector"`
			Expect string `json:"expect"`
		} `json:"sequence"`
	} `json:"replay_vectors"`
	IdentityBindingVectors []struct {
		Name                string `json:"name"`
		Expect              string `json:"expect"`
		JobRequestBase64    string `json:"job_request_base64"`
		AuthoritativeBase64 string `json:"authoritative_base64"`
	} `json:"identity_binding_vectors"`
}

func loadProductionFixture(t *testing.T) productionFixture {
	t.Helper()
	data, err := os.ReadFile(productionFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture productionFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion < 2 {
		t.Fatalf("fixture schema_version %d, want >= 2", fixture.SchemaVersion)
	}
	// A shrinking fixture silently weakens all three SDKs at once, so the
	// counts are asserted rather than assumed.
	if len(fixture.Vectors) < 9 || len(fixture.ReplayVectors) < 3 ||
		len(fixture.IdentityBindingVectors) < 3 {
		t.Fatalf("fixture lost coverage: %d signing, %d replay, %d identity vectors",
			len(fixture.Vectors), len(fixture.ReplayVectors), len(fixture.IdentityBindingVectors))
	}
	return fixture
}

func parseVectorKey(t *testing.T, pemText string) *ecdsa.PublicKey {
	t.Helper()
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		t.Fatal("public key is not PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("public key is not ECDSA")
	}
	return key
}

func trustForVector(t *testing.T, fixture productionFixture, vector productionVector) ProductionTrustStore {
	t.Helper()
	installed := vector.TrustKeyIDs
	if len(installed) == 0 {
		for keyID := range fixture.Trust.PublicKeys {
			installed = append(installed, keyID)
		}
	}
	keys := map[string]*ecdsa.PublicKey{}
	for _, keyID := range installed {
		pemText, ok := fixture.Trust.PublicKeys[keyID]
		if !ok {
			t.Fatalf("vector %s names unknown trust key %q", vector.Name, keyID)
		}
		keys[keyID] = parseVectorKey(t, pemText)
	}
	verifyAt, err := time.Parse(time.RFC3339, fixture.VerifyAtRFC3339)
	if err != nil {
		t.Fatal(err)
	}
	return ProductionTrustStore{
		Audience: fixture.Trust.Audience, Tenant: fixture.Trust.Tenant,
		Sender: fixture.Trust.Sender, PublicKeys: keys,
		Now: func() time.Time { return verifyAt },
	}
}

// goErrorForRejectReason maps the fixture's language-neutral reason onto a Go
// sentinel. Go answers an identity mismatch with ErrUnknownKeyID by design:
// resolveProductionKey checks tenant and sender before key lookup and reports
// one error for all three so the failure cannot be used as an oracle. Python
// and Node distinguish them; the fixture records the outcome, not the wording.
func goErrorForRejectReason(t *testing.T, reason string) error {
	t.Helper()
	switch reason {
	case "invalid_signature":
		return ErrInvalidSignature
	case "audience_mismatch":
		return ErrAudienceMismatch
	case "signature_expired":
		return ErrSignatureExpired
	case "unknown_key_id", "identity_mismatch":
		return ErrUnknownKeyID
	default:
		t.Fatalf("unknown reject_reason %q", reason)
		return nil
	}
}

func TestProductionSigningVectorsMatchExpectedVerdicts(t *testing.T) {
	fixture := loadProductionFixture(t)
	for _, vector := range fixture.Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			raw, err := base64.StdEncoding.DecodeString(vector.RawBase64)
			if err != nil {
				t.Fatal(err)
			}
			_, verifyErr := VerifyProductionPacket(raw, trustForVector(t, fixture, vector))
			if vector.Expect == "accept" {
				if verifyErr != nil {
					t.Fatalf("labeled accept but rejected: %v", verifyErr)
				}
				return
			}
			if verifyErr == nil {
				t.Fatal("labeled reject but the verifier accepted it")
			}
			if want := goErrorForRejectReason(t, vector.RejectReason); !errors.Is(verifyErr, want) {
				t.Fatalf("reason %q wants %v, got %v", vector.RejectReason, want, verifyErr)
			}
		})
	}
}

// TestProductionSigningVectorsPinBothDigests is the guard for a defect that
// already shipped once: the domain-separated SIGNATURE preimage and the
// undomained signed-BODY digest are different values over identical bytes, and
// an admission path that swaps them makes a valid redelivery look like a
// conflict to every other SDK.
func TestProductionSigningVectorsPinBothDigests(t *testing.T) {
	fixture := loadProductionFixture(t)
	domain, err := base64.StdEncoding.DecodeString(fixture.DomainBase64)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture declares the domain separator; the SDK must be using that
	// exact prefix, not merely some prefix of its own.
	if !bytes.Equal(domain, productionDomain) {
		t.Fatalf("fixture domain %q != SDK domain %q", domain, productionDomain)
	}
	for _, vector := range fixture.Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			raw, err := base64.StdEncoding.DecodeString(vector.RawBase64)
			if err != nil {
				t.Fatal(err)
			}
			unsigned, signature, err := extractSignatureField(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := base64.StdEncoding.EncodeToString(unsigned); got != vector.UnsignedBase64 {
				t.Fatal("extraction changed the fixture's unsigned bytes")
			}
			if got := base64.StdEncoding.EncodeToString(signature); got != vector.SignatureBase64 {
				t.Fatal("extraction changed the fixture's signature bytes")
			}
			preimage := productionDigest(unsigned)
			if hex.EncodeToString(preimage[:]) != vector.PreimageDigestHex {
				t.Fatal("preimage digest mismatch")
			}
			body := sha256.Sum256(unsigned)
			if hex.EncodeToString(body[:]) != vector.BodyDigestHex {
				t.Fatal("body digest mismatch")
			}
			if vector.PreimageDigestHex == vector.BodyDigestHex {
				t.Fatal("preimage and body digests are equal; domain separation lost")
			}
			sdkBody, err := ProductionSignedBodyDigest(raw)
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(sdkBody[:]) != vector.BodyDigestHex {
				t.Fatal("ProductionSignedBodyDigest disagrees with the fixture")
			}
		})
	}
}

func TestProductionSigningVectorsReplaySemantics(t *testing.T) {
	fixture := loadProductionFixture(t)
	byName := map[string]productionVector{}
	for _, vector := range fixture.Vectors {
		byName[vector.Name] = vector
	}
	for _, replay := range fixture.ReplayVectors {
		t.Run(replay.Name, func(t *testing.T) {
			store := NewInMemoryReplayStore()
			for index, step := range replay.Sequence {
				vector, ok := byName[step.Vector]
				if !ok {
					t.Fatalf("step %d names unknown vector %q", index, step.Vector)
				}
				outcome, err := admitVectorForTest(t, fixture, vector, store)
				assertReplayStep(t, index, step.Expect, outcome, err)
			}
		})
	}
}

func admitVectorForTest(
	t *testing.T, fixture productionFixture, vector productionVector, store *InMemoryReplayStore,
) (ReplayOutcome, error) {
	t.Helper()
	messageID, err := hex.DecodeString(vector.MessageIDHex)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := hex.DecodeString(vector.BodyDigestHex)
	if err != nil {
		t.Fatal(err)
	}
	expiry, err := time.Parse(time.RFC3339, vector.ExpiresAtRFC3339)
	if err != nil {
		t.Fatal(err)
	}
	return store.Admit(
		fixture.Trust.Tenant, vector.Audience, fixture.Trust.Sender, messageID, digest, expiry,
	)
}

func assertReplayStep(t *testing.T, index int, expect string, outcome ReplayOutcome, err error) {
	t.Helper()
	switch expect {
	case "first":
		if err != nil || outcome != ReplayOutcomeFirst {
			t.Fatalf("step %d: want first, got outcome=%v err=%v", index, outcome, err)
		}
	case "duplicate":
		if err != nil || outcome != ReplayOutcomeDuplicate {
			t.Fatalf("step %d: want duplicate, got outcome=%v err=%v", index, outcome, err)
		}
	case "conflict":
		if !errors.Is(err, ErrReplayConflict) {
			t.Fatalf("step %d: want replay conflict, got outcome=%v err=%v", index, outcome, err)
		}
	default:
		t.Fatalf("step %d: unknown expectation %q", index, expect)
	}
}

func TestProductionIdentityBindingVectors(t *testing.T) {
	fixture := loadProductionFixture(t)
	for _, vector := range fixture.IdentityBindingVectors {
		t.Run(vector.Name, func(t *testing.T) {
			requestBytes, err := base64.StdEncoding.DecodeString(vector.JobRequestBase64)
			if err != nil {
				t.Fatal(err)
			}
			request := &agentv1.JobRequest{}
			if err := proto.Unmarshal(requestBytes, request); err != nil {
				t.Fatal(err)
			}
			authoritative := &agentv1.IdentityBinding{}
			if vector.AuthoritativeBase64 != "" {
				decoded, err := base64.StdEncoding.DecodeString(vector.AuthoritativeBase64)
				if err != nil {
					t.Fatal(err)
				}
				if err := proto.Unmarshal(decoded, authoritative); err != nil {
					t.Fatal(err)
				}
			}
			err = ValidateIdentityBinding(request, authoritative)
			if vector.Expect == "accept" && err != nil {
				t.Fatalf("labeled accept but rejected: %v", err)
			}
			if vector.Expect == "reject" && err == nil {
				t.Fatal("labeled reject but accepted")
			}
		})
	}
}

// TestProductionSigningLegacyKeysAliasBaseline keeps the pre-schema-2 flat keys
// honest: they are an alias of accept/baseline, not a second hand-maintained
// copy that can drift.
func TestProductionSigningLegacyKeysAliasBaseline(t *testing.T) {
	fixture := loadProductionFixture(t)
	var baseline productionVector
	for _, vector := range fixture.Vectors {
		if vector.Name == "accept/baseline" {
			baseline = vector
		}
	}
	if baseline.Name == "" {
		t.Fatal("fixture has no accept/baseline vector")
	}
	if fixture.RawBase64 != baseline.RawBase64 ||
		fixture.UnsignedBase64 != baseline.UnsignedBase64 ||
		fixture.SignatureBase64 != baseline.SignatureBase64 ||
		fixture.PreimageDigestHex != baseline.PreimageDigestHex {
		t.Fatal("legacy top-level keys drifted from accept/baseline")
	}
	if fixture.PublicKeyPEM != fixture.Trust.PublicKeys[baseline.KeyID] {
		t.Fatal("legacy public_key_pem is not the baseline signing key")
	}
}
