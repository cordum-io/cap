package main

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

// CheckFixture proves every labeled outcome against the real SDK verifier.
//
// The generator builds the signing preimage itself, because
// capsdk.SignProductionPacket refuses a far-future expiry (correctly — a live
// signer must not mint one). This check is what stops that second construction
// from drifting: if the generator's preimage, digest or wire layout diverged
// from the SDK's by even a byte, every accept vector would fail here.
func CheckFixture(fixture *Fixture) error {
	if err := checkLegacyAlias(fixture); err != nil {
		return err
	}
	verifyAt, err := time.Parse(time.RFC3339, fixture.VerifyAtRFC3339)
	if err != nil {
		return fmt.Errorf("parse verify_at: %w", err)
	}
	byName := map[string]SigningVector{}
	for _, vector := range fixture.Vectors {
		if _, duplicate := byName[vector.Name]; duplicate {
			return fmt.Errorf("duplicate vector name %q", vector.Name)
		}
		byName[vector.Name] = vector
		if err := checkSigningVector(fixture, vector, verifyAt); err != nil {
			return fmt.Errorf("vector %s: %w", vector.Name, err)
		}
	}
	if err := checkRotationPair(byName); err != nil {
		return err
	}
	if err := checkReplayVectors(fixture, byName); err != nil {
		return err
	}
	return checkIdentityVectors(fixture)
}

// checkLegacyAlias pins the pre-schema-2 flat keys to the baseline vector so
// the two copies cannot drift.
func checkLegacyAlias(fixture *Fixture) error {
	baseline, err := vectorNamed(fixture.Vectors, "accept/baseline")
	if err != nil {
		return err
	}
	if fixture.RawBase64 != baseline.RawBase64 ||
		fixture.UnsignedBase64 != baseline.UnsignedBase64 ||
		fixture.SignatureBase64 != baseline.SignatureBase64 ||
		fixture.PreimageDigestHex != baseline.PreimageDigestHex {
		return errors.New("legacy top-level keys drifted from accept/baseline")
	}
	if fixture.PublicKeyPEM != fixture.Trust.PublicKeys[baseline.KeyID] {
		return errors.New("legacy public_key_pem is not the baseline signing key")
	}
	return nil
}

func checkSigningVector(fixture *Fixture, vector SigningVector, verifyAt time.Time) error {
	raw, err := base64.StdEncoding.DecodeString(vector.RawBase64)
	if err != nil {
		return fmt.Errorf("decode raw: %w", err)
	}
	unsigned, err := base64.StdEncoding.DecodeString(vector.UnsignedBase64)
	if err != nil {
		return fmt.Errorf("decode unsigned: %w", err)
	}
	if hex.EncodeToString(preimageDigest(unsigned)) != vector.PreimageDigestHex {
		return errors.New("preimage_digest_hex does not match unsigned bytes")
	}
	if hex.EncodeToString(bodyDigest(unsigned)) != vector.BodyDigestHex {
		return errors.New("body_digest_hex does not match unsigned bytes")
	}
	if vector.PreimageDigestHex == vector.BodyDigestHex {
		return errors.New("preimage and body digests are equal; domain separation lost")
	}
	// The SDK must derive the same replay body digest from the raw wire.
	sdkDigest, err := capsdk.ProductionSignedBodyDigest(raw)
	if err != nil {
		return fmt.Errorf("sdk body digest: %w", err)
	}
	if hex.EncodeToString(sdkDigest[:]) != vector.BodyDigestHex {
		return errors.New("body_digest_hex disagrees with capsdk.ProductionSignedBodyDigest")
	}

	trust, err := trustFor(fixture, vector, verifyAt)
	if err != nil {
		return err
	}
	_, verifyErr := capsdk.VerifyProductionPacket(raw, trust)
	return checkOutcome(vector, verifyErr)
}

func checkOutcome(vector SigningVector, verifyErr error) error {
	if vector.Expect == "accept" {
		if verifyErr != nil {
			return fmt.Errorf("labeled accept but verifier rejected: %w", verifyErr)
		}
		return nil
	}
	if verifyErr == nil {
		return errors.New("labeled reject but verifier accepted")
	}
	want, ok := goErrorForReason(vector.RejectReason)
	if !ok {
		return fmt.Errorf("unknown reject_reason %q", vector.RejectReason)
	}
	if !errors.Is(verifyErr, want) {
		return fmt.Errorf("reject_reason %q maps to %v, verifier gave %v",
			vector.RejectReason, want, verifyErr)
	}
	return nil
}

// goErrorForReason maps an abstract reason to the Go sentinel. Go reports an
// identity mismatch as ErrUnknownKeyID on purpose: resolveProductionKey checks
// tenant and sender before key lookup and returns one error for all three, so
// a prober cannot tell which of them was wrong.
func goErrorForReason(reason string) (error, bool) {
	switch reason {
	case ReasonInvalidSignature:
		return capsdk.ErrInvalidSignature, true
	case ReasonAudienceMismatch:
		return capsdk.ErrAudienceMismatch, true
	case ReasonExpired:
		return capsdk.ErrSignatureExpired, true
	case ReasonUnknownKeyID, ReasonIdentityMismatch:
		return capsdk.ErrUnknownKeyID, true
	default:
		return nil, false
	}
}

func trustFor(fixture *Fixture, vector SigningVector, verifyAt time.Time) (capsdk.ProductionTrustStore, error) {
	keys := map[string]*ecdsa.PublicKey{}
	installed := vector.TrustKeyIDs
	if len(installed) == 0 {
		for keyID := range fixture.Trust.PublicKeys {
			installed = append(installed, keyID)
		}
	}
	for _, keyID := range installed {
		pemText, ok := fixture.Trust.PublicKeys[keyID]
		if !ok {
			return capsdk.ProductionTrustStore{}, fmt.Errorf("trust_key_ids names unknown key %q", keyID)
		}
		key, err := parsePublicKeyPEM(pemText)
		if err != nil {
			return capsdk.ProductionTrustStore{}, err
		}
		keys[keyID] = key
	}
	return capsdk.ProductionTrustStore{
		Audience: fixture.Trust.Audience, Tenant: fixture.Trust.Tenant,
		Sender: fixture.Trust.Sender, PublicKeys: keys,
		Now: func() time.Time { return verifyAt },
	}, nil
}

// checkRotationPair asserts the overlap window is expressed by the trust store
// alone: identical bytes, opposite outcomes.
func checkRotationPair(byName map[string]SigningVector) error {
	baseline, ok := byName["accept/baseline"]
	if !ok {
		return errors.New("missing accept/baseline")
	}
	retired, ok := byName["reject/rotation-retired-key"]
	if !ok {
		return errors.New("missing reject/rotation-retired-key")
	}
	if retired.RawBase64 != baseline.RawBase64 {
		return errors.New("rotation-retired-key must reuse the exact baseline bytes")
	}
	return nil
}

func checkReplayVectors(fixture *Fixture, byName map[string]SigningVector) error {
	for _, replay := range fixture.ReplayVectors {
		if len(replay.Sequence) < 2 {
			return fmt.Errorf("replay vector %s: needs at least two steps", replay.Name)
		}
		store := capsdk.NewInMemoryReplayStore()
		for index, step := range replay.Sequence {
			vector, ok := byName[step.Vector]
			if !ok {
				return fmt.Errorf("replay vector %s: unknown signing vector %q", replay.Name, step.Vector)
			}
			if err := admitStep(store, fixture, vector, step); err != nil {
				return fmt.Errorf("replay vector %s step %d: %w", replay.Name, index, err)
			}
		}
	}
	return nil
}

func admitStep(
	store *capsdk.InMemoryReplayStore, fixture *Fixture, vector SigningVector, step ReplayStep,
) error {
	messageID, err := hex.DecodeString(vector.MessageIDHex)
	if err != nil {
		return fmt.Errorf("decode message id: %w", err)
	}
	digest, err := hex.DecodeString(vector.BodyDigestHex)
	if err != nil {
		return fmt.Errorf("decode body digest: %w", err)
	}
	expiry, err := time.Parse(time.RFC3339, vector.ExpiresAtRFC3339)
	if err != nil {
		return fmt.Errorf("parse expiry: %w", err)
	}
	outcome, admitErr := store.Admit(
		fixture.Trust.Tenant, vector.Audience, fixture.Trust.Sender, messageID, digest, expiry,
	)
	switch step.Expect {
	case ReplayFirst:
		if admitErr != nil || outcome != capsdk.ReplayOutcomeFirst {
			return fmt.Errorf("expected first, got outcome=%v err=%v", outcome, admitErr)
		}
	case ReplayDuplicate:
		if admitErr != nil || outcome != capsdk.ReplayOutcomeDuplicate {
			return fmt.Errorf("expected duplicate, got outcome=%v err=%v", outcome, admitErr)
		}
	case ReplayConflict:
		if !errors.Is(admitErr, capsdk.ErrReplayConflict) {
			return fmt.Errorf("expected replay conflict, got outcome=%v err=%v", outcome, admitErr)
		}
	default:
		return fmt.Errorf("unknown replay expectation %q", step.Expect)
	}
	return nil
}

func checkIdentityVectors(fixture *Fixture) error {
	for _, vector := range fixture.IdentityBindingVectors {
		requestBytes, err := base64.StdEncoding.DecodeString(vector.JobRequestBase64)
		if err != nil {
			return fmt.Errorf("identity vector %s: decode request: %w", vector.Name, err)
		}
		request := &agentv1.JobRequest{}
		if err := proto.Unmarshal(requestBytes, request); err != nil {
			return fmt.Errorf("identity vector %s: parse request: %w", vector.Name, err)
		}
		authoritative := &agentv1.IdentityBinding{}
		if vector.AuthoritativeBase64 != "" {
			decoded, err := base64.StdEncoding.DecodeString(vector.AuthoritativeBase64)
			if err != nil {
				return fmt.Errorf("identity vector %s: decode authoritative: %w", vector.Name, err)
			}
			if err := proto.Unmarshal(decoded, authoritative); err != nil {
				return fmt.Errorf("identity vector %s: parse authoritative: %w", vector.Name, err)
			}
		}
		err = capsdk.ValidateIdentityBinding(request, authoritative)
		if vector.Expect == "accept" && err != nil {
			return fmt.Errorf("identity vector %s: labeled accept but rejected: %w", vector.Name, err)
		}
		if vector.Expect == "reject" && err == nil {
			return fmt.Errorf("identity vector %s: labeled reject but accepted", vector.Name)
		}
	}
	return nil
}
