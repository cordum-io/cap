package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Fixed authority the vectors are verified against.
const (
	vectorAudience  = "sys.job.vector"
	vectorTenant    = "tenant-vector"
	vectorPrincipal = "principal-vector"
	vectorActor     = "actor-vector"
	vectorSender    = "worker-vector"

	keyIDPrimary = "vector-key"
	keyIDNext    = "vector-key-next"
	keyIDMissing = "vector-key-missing"
)

// Fixed scalars, so the derived public keys — and therefore the PEMs in the
// fixture — are byte-stable. ecdsa.GenerateKey cannot be used: since Go 1.24
// it draws from the FIPS DRBG and ignores a seeded reader, so it produces a
// different key on every run.
var (
	scalarPrimary = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
	scalarNext    = "1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f809"
)

// verifyAt is the instant every vector's outcome is defined at, and expiresAt
// sits 2 minutes later — inside the 5-minute maximum lifetime the profile
// allows. Fixed rather than relative to generation time so the file neither
// expires nor changes when regenerated.
var (
	verifyAt        = time.Date(2098, time.December, 31, 23, 58, 0, 0, time.UTC)
	expiresAt       = time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)
	expiredExpiry   = time.Date(2098, time.December, 31, 23, 57, 0, 0, time.UTC)
	baselineMsgID   = []byte("0123456789abcdef")
	rotationMsgID   = []byte("fedcba9876543210")
	conflictBodyMsg = baselineMsgID // deliberately shared: drives the conflict vector
)

func fixedKey(hexScalar string) *ecdsa.PrivateKey {
	d, ok := new(big.Int).SetString(hexScalar, 16)
	if !ok {
		panic("invalid fixed scalar")
	}
	curve := elliptic.P256()
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}
}

// packetSpec is a vector that must actually be signed.
type packetSpec struct {
	name, description string
	expect, reason    string
	trustKeyIDs       []string
	signWith          *ecdsa.PrivateKey
	packet            *agentv1.BusPacket
}

func basePacket(msgID []byte, audience, keyID string, expiry time.Time, tenant, jobID string) *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId:         "vector-go-v1",
		SenderId:        vectorSender,
		ProtocolVersion: 1,
		SignatureMetadata: &agentv1.SignatureMetadata{
			ProfileVersion: capsdk.ProductionProfileVersion,
			Algorithm:      capsdk.ProductionAlgorithm,
			MessageId:      msgID,
			Audience:       audience,
			ExpiresAt:      timestamppb.New(expiry),
			KeyId:          keyID,
		},
		Identity: &agentv1.IdentityBinding{TenantId: tenant, PrincipalId: vectorPrincipal},
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
			JobId: jobID, Topic: "fixture",
		}},
	}
}

func packetSpecs(primary, next *ecdsa.PrivateKey) []packetSpec {
	return []packetSpec{
		{
			name:        "accept/baseline",
			description: "Canonical CAP-PRODUCTION packet: message id, audience, expiry and key id all bound into the signed preimage.",
			expect:      "accept",
			signWith:    primary,
			packet:      basePacket(baselineMsgID, vectorAudience, keyIDPrimary, expiresAt, vectorTenant, "job-vector"),
		},
		{
			name:        "accept/rotation-overlap-next-key",
			description: "Signed by the incoming key during a rotation overlap window; accepted because both key ids are installed.",
			expect:      "accept",
			signWith:    next,
			packet:      basePacket(rotationMsgID, vectorAudience, keyIDNext, expiresAt, vectorTenant, "job-vector-rotated"),
		},
		{
			name:        "accept/replay-conflict-body",
			description: "Valid signature reusing accept/baseline's message id over a DIFFERENT body. Verifies standalone; drives the replay conflict sequence.",
			expect:      "accept",
			signWith:    primary,
			packet:      basePacket(conflictBodyMsg, vectorAudience, keyIDPrimary, expiresAt, vectorTenant, "job-vector-conflict"),
		},
		{
			name:        "reject/wrong-audience",
			description: "Correctly signed, but the audience bound at signing is not the subject it was received on.",
			expect:      "reject",
			reason:      ReasonAudienceMismatch,
			signWith:    primary,
			packet:      basePacket(rotationMsgID, "sys.job.other", keyIDPrimary, expiresAt, vectorTenant, "job-vector"),
		},
		{
			name:        "reject/expired",
			description: "Correctly signed but the bound expiry precedes verify_at, so the packet is outside its lifetime.",
			expect:      "reject",
			reason:      ReasonExpired,
			signWith:    primary,
			packet:      basePacket(rotationMsgID, vectorAudience, keyIDPrimary, expiredExpiry, vectorTenant, "job-vector"),
		},
		{
			name:        "reject/unknown-key-id",
			description: "Correctly signed by the primary key but names a key id absent from the trust store.",
			expect:      "reject",
			reason:      ReasonUnknownKeyID,
			signWith:    primary,
			packet:      basePacket(rotationMsgID, vectorAudience, keyIDMissing, expiresAt, vectorTenant, "job-vector"),
		},
		{
			name:        "reject/tenant-mismatch",
			description: "Correctly signed, but the envelope tenant is not the authenticated tenant. Go collapses this into unknown-key-id so the error cannot be used as an oracle; Python and Node report a tenant mismatch.",
			expect:      "reject",
			reason:      ReasonIdentityMismatch,
			signWith:    primary,
			packet:      basePacket(rotationMsgID, vectorAudience, keyIDPrimary, expiresAt, "tenant-other", "job-vector"),
		},
	}
}

// identityBindingSpecs pin JobRequest-mirror validation. The middle case is the
// regression that shipped green on this branch: a present nested binding whose
// actor_id was blanked. A skip-if-empty mirror rule accepts it; the profile
// requires rejection, because a present binding claiming "no actor" is an
// assertion, not an omission.
func identityBindingSpecs() []struct {
	name, description, expect string
	request                   *agentv1.JobRequest
	authoritative             *agentv1.IdentityBinding
} {
	authoritative := &agentv1.IdentityBinding{
		TenantId: vectorTenant, PrincipalId: vectorPrincipal, ActorId: vectorActor,
	}
	return []struct {
		name, description, expect string
		request                   *agentv1.JobRequest
		authoritative             *agentv1.IdentityBinding
	}{
		{
			name:        "identity/full-match",
			description: "Nested binding mirrors every authoritative field exactly.",
			expect:      "accept",
			request: &agentv1.JobRequest{
				JobId: "job-vector", TenantId: vectorTenant, PrincipalId: vectorPrincipal,
				Identity: &agentv1.IdentityBinding{
					TenantId: vectorTenant, PrincipalId: vectorPrincipal, ActorId: vectorActor,
				},
			},
			authoritative: authoritative,
		},
		{
			name:        "identity/present-blank-actor",
			description: "Present nested binding with the actor_id stripped. MUST reject: a present sub-message is a full claim, not a partial mirror.",
			expect:      "reject",
			request: &agentv1.JobRequest{
				JobId: "job-vector", TenantId: vectorTenant, PrincipalId: vectorPrincipal,
				Identity: &agentv1.IdentityBinding{
					TenantId: vectorTenant, PrincipalId: vectorPrincipal,
				},
			},
			authoritative: authoritative,
		},
		{
			name:        "identity/absent-binding",
			description: "No nested binding at all. Accepted: a legitimate compat/migration shape that must stay distinguishable from present-but-blank.",
			expect:      "accept",
			request: &agentv1.JobRequest{
				JobId: "job-vector", TenantId: vectorTenant, PrincipalId: vectorPrincipal,
			},
			authoritative: authoritative,
		},
	}
}

func replayVectors() []ReplayVector {
	return []ReplayVector{
		{
			Name:        "replay/idempotent-redelivery",
			Description: "At-least-once redelivery of identical wire bytes is a duplicate, not a conflict: the digest matches, so the second copy is acknowledged without a second side effect.",
			Sequence: []ReplayStep{
				{Vector: "accept/baseline", Expect: ReplayFirst},
				{Vector: "accept/baseline", Expect: ReplayDuplicate},
			},
		},
		{
			Name:        "replay/conflict-different-digest",
			Description: "Same message id over a different signed body MUST fail closed rather than be treated as a redelivery.",
			Sequence: []ReplayStep{
				{Vector: "accept/baseline", Expect: ReplayFirst},
				{Vector: "accept/replay-conflict-body", Expect: ReplayConflict},
			},
		},
		{
			Name:        "replay/distinct-messages-both-admitted",
			Description: "Control: distinct message ids are both first-seen. Fails an implementation that conflicts or dedupes unconditionally.",
			Sequence: []ReplayStep{
				{Vector: "accept/baseline", Expect: ReplayFirst},
				{Vector: "accept/rotation-overlap-next-key", Expect: ReplayFirst},
			},
		},
	}
}
