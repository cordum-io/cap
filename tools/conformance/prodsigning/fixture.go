package main

// Fixture schema for test/fixtures/production-signing-v1.json.
//
// Emitted with encoding/json over structs (never maps) so field order — and
// therefore the file's bytes — is stable across runs.

// Fixture is the whole conformance file.
type Fixture struct {
	// Legacy flat keys, retained so any pre-schema-2 reader keeps working.
	// Generated from the same source as Vectors[0]; CheckFixture asserts they
	// cannot drift apart.
	PreimageDigestHex string `json:"preimage_digest_hex"`
	Producer          string `json:"producer"`
	PublicKeyPEM      string `json:"public_key_pem"`
	RawBase64         string `json:"raw_base64"`
	SignatureBase64   string `json:"signature_base64"`
	UnsignedBase64    string `json:"unsigned_base64"`

	SchemaVersion  int    `json:"schema_version"`
	Generator      string `json:"generator"`
	ProfileVersion string `json:"profile_version"`
	Algorithm      string `json:"algorithm"`
	// DomainBase64 is the domain-separation prefix of the SIGNATURE preimage.
	// It is deliberately NOT part of the replay body digest; see BodyDigestHex.
	DomainBase64 string `json:"domain_base64"`
	// VerifyAtRFC3339 is the instant every vector's expected outcome is
	// evaluated at. Consumers MUST pin their trust clock to it; using wall
	// time would make the expiry vectors flip meaning in the year 2099.
	VerifyAtRFC3339 string `json:"verify_at_rfc3339"`

	Trust                  TrustProfile            `json:"trust"`
	Vectors                []SigningVector         `json:"vectors"`
	ReplayVectors          []ReplayVector          `json:"replay_vectors"`
	IdentityBindingVectors []IdentityBindingVector `json:"identity_binding_vectors"`
}

// TrustProfile is the authenticated transport authority every vector is
// verified against. Tenant/sender are the AUTHENTICATED values, not values
// read out of the packet.
type TrustProfile struct {
	Audience   string            `json:"audience"`
	Tenant     string            `json:"tenant"`
	Sender     string            `json:"sender"`
	PublicKeys map[string]string `json:"public_keys"`
}

// Reject reasons. A reason is an abstract outcome, not an error string:
// the three SDKs word their errors differently and Go deliberately collapses
// tenant/sender mismatch into "unknown key id" so a prober cannot use the
// error as an oracle. Each consumer maps reason -> its own error identity.
const (
	ReasonNone             = ""
	ReasonInvalidSignature = "invalid_signature"
	ReasonAudienceMismatch = "audience_mismatch"
	ReasonExpired          = "signature_expired"
	ReasonUnknownKeyID     = "unknown_key_id"
	ReasonIdentityMismatch = "identity_mismatch"
)

// SigningVector is one raw wire packet plus the outcome a conforming verifier
// MUST produce for it at VerifyAtRFC3339.
type SigningVector struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Expect is "accept" or "reject".
	Expect string `json:"expect"`
	// RejectReason is set iff Expect == "reject".
	RejectReason string `json:"reject_reason,omitempty"`
	// TrustKeyIDs is the subset of Trust.PublicKeys installed for this vector.
	// Empty means all of them. This is how the key-rotation overlap window and
	// its closure are expressed without a second trust profile.
	TrustKeyIDs []string `json:"trust_key_ids,omitempty"`

	RawBase64       string `json:"raw_base64"`
	UnsignedBase64  string `json:"unsigned_base64"`
	SignatureBase64 string `json:"signature_base64"`

	// PreimageDigestHex is sha256(DOMAIN || unsigned) — what the signature
	// covers.
	PreimageDigestHex string `json:"preimage_digest_hex"`
	// BodyDigestHex is sha256(unsigned) with NO domain prefix — the signed-body
	// digest replay stores retain (spec/19 "Replay and at-least-once
	// delivery"). These two digests are different values over the same bytes
	// and conflating them silently breaks replay interop, so both are pinned.
	BodyDigestHex string `json:"body_digest_hex"`

	MessageIDHex     string `json:"message_id_hex"`
	Audience         string `json:"audience"`
	KeyID            string `json:"key_id"`
	ExpiresAtRFC3339 string `json:"expires_at_rfc3339"`
}

// ReplayOutcome values a replay store must produce for a step.
const (
	ReplayFirst     = "first"
	ReplayDuplicate = "duplicate"
	ReplayConflict  = "conflict"
)

// ReplayVector is an ordered sequence of admissions against ONE store,
// asserting at-least-once delivery semantics.
type ReplayVector struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Sequence    []ReplayStep `json:"sequence"`
}

// ReplayStep admits the named signing vector and expects a given outcome.
type ReplayStep struct {
	Vector string `json:"vector"`
	Expect string `json:"expect"`
}

// IdentityBindingVector pins JobRequest-mirror validation against the
// authoritative envelope IdentityBinding.
//
// Present because a P1 regression on this branch (a nested present-but-blank
// binding being skipped by a skip-if-empty mirror rule) shipped green in every
// SDK test: no fixture covered it. See the "identity/present-blank-actor"
// vector.
type IdentityBindingVector struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Expect is "accept" or "reject".
	Expect string `json:"expect"`
	// JobRequestBase64 is a serialized cordum.agent.v1.JobRequest.
	JobRequestBase64 string `json:"job_request_base64"`
	// AuthoritativeBase64 is a serialized cordum.agent.v1.IdentityBinding.
	// Empty means "no authoritative binding supplied".
	AuthoritativeBase64 string `json:"authoritative_base64,omitempty"`
}
