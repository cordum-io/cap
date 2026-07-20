// Command prodsigning generates test/fixtures/production-signing-v1.json, the
// cross-language conformance vector set for the CAP-PRODUCTION canonical
// signing preimage.
//
// It is separate from tools/conformance/generate_fixtures.go, which owns the
// disjoint legacy spec/conformance/fixtures/*.bin family.
//
// DETERMINISM. Running this twice in a row leaves the file byte-identical, but
// not by re-signing: since Go 1.24 ecdsa.SignASN1 is hedged and returns a
// different signature for the same key and digest on every call, so a
// re-signing generator could never produce a stable file. Instead the unsigned
// wire bytes are built deterministically (fixed keys, fixed timestamps, no
// clock, no randomness) and an existing signature is REUSED whenever the body
// it covers is unchanged and it still verifies. A changed body forces a fresh
// signature, so the file cannot go stale.
//
//	go run ./tools/conformance/prodsigning          # write
//	go run ./tools/conformance/prodsigning -check   # verify, never write
package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

const fixtureRelPath = "test/fixtures/production-signing-v1.json"

var productionDomain = []byte("CAP-PRODUCTION-SIGNATURE-V1\x00")

func main() {
	check := flag.Bool("check", false, "verify the committed fixture instead of writing it")
	flag.Parse()

	path := filepath.Join(repoRoot(), filepath.FromSlash(fixtureRelPath))
	previous := readPrevious(path)

	fixture, err := build(previous)
	if err != nil {
		fatal(err)
	}
	if err := CheckFixture(fixture); err != nil {
		fatal(fmt.Errorf("generated fixture failed self-check: %w", err))
	}

	encoded, err := encode(fixture)
	if err != nil {
		fatal(err)
	}
	if *check {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			fatal(readErr)
		}
		if string(existing) != string(encoded) {
			fatal(errors.New("fixture is stale: re-run `go run ./tools/conformance/prodsigning`"))
		}
		fmt.Printf("ok: %s is current (%d signing, %d replay, %d identity vectors)\n",
			fixtureRelPath, len(fixture.Vectors), len(fixture.ReplayVectors),
			len(fixture.IdentityBindingVectors))
		return
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (%d signing, %d replay, %d identity vectors)\n",
		fixtureRelPath, len(fixture.Vectors), len(fixture.ReplayVectors),
		len(fixture.IdentityBindingVectors))
}

func encode(fixture *Fixture) ([]byte, error) {
	encoded, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal fixture: %w", err)
	}
	return append(encoded, '\n'), nil
}

// readPrevious loads the committed fixture so unchanged signatures can be
// reused. A missing or unparsable file simply means "sign everything".
func readPrevious(path string) map[string]SigningVector {
	previous := map[string]SigningVector{}
	data, err := os.ReadFile(path)
	if err != nil {
		return previous
	}
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return previous
	}
	for _, vector := range fixture.Vectors {
		previous[vector.Name] = vector
	}
	return previous
}

func build(previous map[string]SigningVector) (*Fixture, error) {
	primary, next := fixedKey(scalarPrimary), fixedKey(scalarNext)
	primaryPEM, err := publicKeyPEM(&primary.PublicKey)
	if err != nil {
		return nil, err
	}
	nextPEM, err := publicKeyPEM(&next.PublicKey)
	if err != nil {
		return nil, err
	}

	vectors := make([]SigningVector, 0, len(packetSpecs(primary, next))+2)
	for _, spec := range packetSpecs(primary, next) {
		vector, err := signVector(spec, previous[spec.name])
		if err != nil {
			return nil, fmt.Errorf("vector %s: %w", spec.name, err)
		}
		vectors = append(vectors, vector)
	}

	baseline, err := vectorNamed(vectors, "accept/baseline")
	if err != nil {
		return nil, err
	}
	derived, err := derivedVectors(baseline)
	if err != nil {
		return nil, err
	}
	vectors = append(vectors, derived...)

	identity, err := identityVectors()
	if err != nil {
		return nil, err
	}

	return &Fixture{
		PreimageDigestHex: baseline.PreimageDigestHex,
		Producer:          "go",
		PublicKeyPEM:      primaryPEM,
		RawBase64:         baseline.RawBase64,
		SignatureBase64:   baseline.SignatureBase64,
		UnsignedBase64:    baseline.UnsignedBase64,

		SchemaVersion:   2,
		Generator:       "tools/conformance/prodsigning",
		ProfileVersion:  capsdk.ProductionProfileVersion,
		Algorithm:       capsdk.ProductionAlgorithm,
		DomainBase64:    base64.StdEncoding.EncodeToString(productionDomain),
		VerifyAtRFC3339: verifyAt.Format(time.RFC3339),
		Trust: TrustProfile{
			Audience: vectorAudience, Tenant: vectorTenant, Sender: vectorSender,
			PublicKeys: map[string]string{keyIDPrimary: primaryPEM, keyIDNext: nextPEM},
		},
		Vectors:                vectors,
		ReplayVectors:          replayVectors(),
		IdentityBindingVectors: identity,
	}, nil
}

// signVector marshals the spec's unsigned wire and attaches a signature,
// reusing the previous one when it still covers exactly these bytes.
func signVector(spec packetSpec, previous SigningVector) (SigningVector, error) {
	unsignedPacket := proto.Clone(spec.packet).(*agentv1.BusPacket)
	unsignedPacket.Signature = nil
	unsigned, err := proto.Marshal(unsignedPacket)
	if err != nil {
		return SigningVector{}, fmt.Errorf("marshal unsigned: %w", err)
	}

	signature, err := reuseOrSign(unsigned, previous, spec.signWith)
	if err != nil {
		return SigningVector{}, err
	}
	raw := appendSignatureField(unsigned, signature)
	metadata := spec.packet.GetSignatureMetadata()
	return SigningVector{
		Name: spec.name, Description: spec.description,
		Expect: spec.expect, RejectReason: spec.reason, TrustKeyIDs: spec.trustKeyIDs,
		RawBase64:         base64.StdEncoding.EncodeToString(raw),
		UnsignedBase64:    base64.StdEncoding.EncodeToString(unsigned),
		SignatureBase64:   base64.StdEncoding.EncodeToString(signature),
		PreimageDigestHex: hex.EncodeToString(preimageDigest(unsigned)),
		BodyDigestHex:     hex.EncodeToString(bodyDigest(unsigned)),
		MessageIDHex:      hex.EncodeToString(metadata.GetMessageId()),
		Audience:          metadata.GetAudience(),
		KeyID:             metadata.GetKeyId(),
		ExpiresAtRFC3339:  metadata.GetExpiresAt().AsTime().Format(time.RFC3339),
	}, nil
}

func reuseOrSign(unsigned []byte, previous SigningVector, key *ecdsa.PrivateKey) ([]byte, error) {
	if previous.UnsignedBase64 == base64.StdEncoding.EncodeToString(unsigned) {
		if stored, err := base64.StdEncoding.DecodeString(previous.SignatureBase64); err == nil {
			if ecdsa.VerifyASN1(&key.PublicKey, preimageDigest(unsigned), stored) {
				return stored, nil
			}
		}
	}
	signature, err := ecdsa.SignASN1(rand.Reader, key, preimageDigest(unsigned))
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return signature, nil
}

// derivedVectors are byte-level derivations of the baseline. They are never
// signed, so they stay stable for free.
func derivedVectors(baseline SigningVector) ([]SigningVector, error) {
	baseUnsigned, err := base64.StdEncoding.DecodeString(baseline.UnsignedBase64)
	if err != nil {
		return nil, fmt.Errorf("decode baseline unsigned: %w", err)
	}
	signature, err := base64.StdEncoding.DecodeString(baseline.SignatureBase64)
	if err != nil {
		return nil, fmt.Errorf("decode baseline signature: %w", err)
	}
	// Tamper the body only, then re-attach the ORIGINAL signature, so the packet
	// still parses and fails specifically at signature verification.
	unsigned, err := tamperJobID(baseUnsigned)
	if err != nil {
		return nil, err
	}
	tampered := appendSignatureField(unsigned, signature)

	// Same bytes as the baseline, but the key that signed them has been retired
	// from the trust store: the closing half of the rotation window.
	retired := baseline
	retired.Name = "reject/rotation-retired-key"
	retired.Description = "The baseline bytes after its signing key id was retired; only the incoming key remains installed."
	retired.Expect, retired.RejectReason = "reject", ReasonUnknownKeyID
	retired.TrustKeyIDs = []string{keyIDNext}

	return []SigningVector{
		{
			Name:        "reject/tampered-body",
			Description: "One byte of the job id flipped after signing. The wire still parses, so this must fail at signature verification rather than at decode.",
			Expect:      "reject", RejectReason: ReasonInvalidSignature,
			RawBase64:         base64.StdEncoding.EncodeToString(tampered),
			UnsignedBase64:    base64.StdEncoding.EncodeToString(unsigned),
			SignatureBase64:   base64.StdEncoding.EncodeToString(signature),
			PreimageDigestHex: hex.EncodeToString(preimageDigest(unsigned)),
			BodyDigestHex:     hex.EncodeToString(bodyDigest(unsigned)),
			MessageIDHex:      baseline.MessageIDHex,
			Audience:          baseline.Audience,
			KeyID:             baseline.KeyID,
			ExpiresAtRFC3339:  baseline.ExpiresAtRFC3339,
		},
		retired,
	}, nil
}

// tamperJobID flips one byte of the ASCII job id, keeping every length prefix
// intact so the packet still decodes.
func tamperJobID(raw []byte) ([]byte, error) {
	needle := []byte("job-vector")
	index := indexOf(raw, needle)
	if index < 0 {
		return nil, errors.New("baseline job id not found in raw wire")
	}
	tampered := append([]byte(nil), raw...)
	tampered[index] = 'J'
	return tampered, nil
}

func identityVectors() ([]IdentityBindingVector, error) {
	vectors := make([]IdentityBindingVector, 0, len(identityBindingSpecs()))
	for _, spec := range identityBindingSpecs() {
		request, err := marshalMessage(spec.request)
		if err != nil {
			return nil, fmt.Errorf("vector %s: %w", spec.name, err)
		}
		authoritative, err := marshalMessage(spec.authoritative)
		if err != nil {
			return nil, fmt.Errorf("vector %s: %w", spec.name, err)
		}
		vectors = append(vectors, IdentityBindingVector{
			Name: spec.name, Description: spec.description, Expect: spec.expect,
			JobRequestBase64:    base64.StdEncoding.EncodeToString(request),
			AuthoritativeBase64: base64.StdEncoding.EncodeToString(authoritative),
		})
	}
	return vectors, nil
}

func marshalMessage(message proto.Message) ([]byte, error) {
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return encoded, nil
}

// preimageDigest is what the signature covers: domain-separated.
func preimageDigest(unsigned []byte) []byte {
	sum := sha256.Sum256(append(append([]byte(nil), productionDomain...), unsigned...))
	return sum[:]
}

// bodyDigest is what a replay store retains: NO domain prefix. Distinct from
// preimageDigest on purpose — see SigningVector.BodyDigestHex.
func bodyDigest(unsigned []byte) []byte {
	sum := sha256.Sum256(unsigned)
	return sum[:]
}

func appendSignatureField(unsigned, signature []byte) []byte {
	result := append([]byte(nil), unsigned...)
	result = append(result, 0x72) // field 14, wire type 2
	result = append(result, encodeVarint(uint64(len(signature)))...)
	return append(result, signature...)
}

func encodeVarint(value uint64) []byte {
	var out []byte
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func publicKeyPEM(key *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

func parsePublicKeyPEM(pemText string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("public key is not PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ECDSA")
	}
	return key, nil
}

func vectorNamed(vectors []SigningVector, name string) (SigningVector, error) {
	for _, vector := range vectors {
		if vector.Name == name {
			return vector, nil
		}
	}
	return SigningVector{}, fmt.Errorf("vector %q not built", name)
}

func indexOf(haystack, needle []byte) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func repoRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fatal(errors.New("resolve caller path"))
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filename))))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
