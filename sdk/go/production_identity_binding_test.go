package capsdk

import (
	"errors"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

// authoritativeBinding is a fully-populated authoritative identity: every field
// that a nested mirror could contradict is non-empty.
func authoritativeBinding() *agentv1.IdentityBinding {
	return &agentv1.IdentityBinding{
		TenantId:     "tenant-a",
		PrincipalId:  "principal-a",
		ActorId:      "actor-a",
		DelegationId: "delegation-a",
	}
}

// TestValidateIdentityBindingRejectsPartiallyBlankNestedIdentity pins the rule
// that a PRESENT nested IdentityBinding must match the authoritative binding on
// every field, including the ones the request left blank.
//
// This is a regression guard, not a new feature. The skip-if-empty rule that
// governs the flat string mirrors (env.*, labels.*, meta.*) is correct for those
// -- a blank env["actor_id"] genuinely means "not mirrored here". It is NOT
// correct for a nested IdentityBinding: once the sub-message is present, a blank
// ActorId is an affirmative claim of "no actor", not an omission. Letting it pass
// hands downstream code an identity that is missing exactly the fields an
// attacker would want dropped.
//
// Cross-language contract: sdk/python production_validation._same_identity and
// sdk/node production-validation.sameIdentity both compare all four fields
// unconditionally. Go must not diverge -- these three implementations are the
// same normative check in the CAP-PRODUCTION profile.
func TestValidateIdentityBindingRejectsPartiallyBlankNestedIdentity(t *testing.T) {
	cases := []struct {
		name    string
		request *agentv1.JobRequest
	}{
		{
			name: "request identity omits principal actor and delegation",
			request: &agentv1.JobRequest{
				TenantId:    "tenant-a",
				PrincipalId: "principal-a",
				// Only the tenant is echoed. Under the mirror loop's
				// skip-if-empty rule the other three fields are never compared.
				Identity: &agentv1.IdentityBinding{TenantId: "tenant-a"},
			},
		},
		{
			name: "request identity omits delegation only",
			request: &agentv1.JobRequest{
				TenantId:    "tenant-a",
				PrincipalId: "principal-a",
				Identity: &agentv1.IdentityBinding{
					TenantId:    "tenant-a",
					PrincipalId: "principal-a",
					ActorId:     "actor-a",
				},
			},
		},
		{
			name: "compensation identity omits actor and delegation",
			request: &agentv1.JobRequest{
				TenantId:    "tenant-a",
				PrincipalId: "principal-a",
				Identity:    authoritativeBinding(),
				Compensation: &agentv1.Compensation{
					Identity: &agentv1.IdentityBinding{
						TenantId:    "tenant-a",
						PrincipalId: "principal-a",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIdentityBinding(tc.request, authoritativeBinding())
			if !errors.Is(err, ErrIdentityMismatch) {
				t.Fatalf(
					"blank nested identity field was accepted: got err=%v, want ErrIdentityMismatch", err,
				)
			}
		})
	}
}

// TestValidateIdentityBindingAcceptsFullyMatchingIdentity is the paired positive
// case. Without it the test above could be satisfied by a validator that rejects
// everything, which would be a different bug rather than a fix.
func TestValidateIdentityBindingAcceptsFullyMatchingIdentity(t *testing.T) {
	request := &agentv1.JobRequest{
		TenantId:    "tenant-a",
		PrincipalId: "principal-a",
		Identity:    authoritativeBinding(),
		Compensation: &agentv1.Compensation{
			TenantId:    "tenant-a",
			PrincipalId: "principal-a",
			Identity:    authoritativeBinding(),
		},
	}
	if err := ValidateIdentityBinding(request, authoritativeBinding()); err != nil {
		t.Fatalf("fully matching identity was rejected: %v", err)
	}
}

// TestValidateIdentityBindingAllowsAbsentNestedIdentity keeps the nested check
// scoped to sub-messages that are actually present. An absent IdentityBinding is
// a legitimate migration/compat shape and must stay distinguishable from a
// present-but-blank one -- that distinction is the whole point of the fix.
func TestValidateIdentityBindingAllowsAbsentNestedIdentity(t *testing.T) {
	request := &agentv1.JobRequest{TenantId: "tenant-a", PrincipalId: "principal-a"}
	if err := ValidateIdentityBinding(request, authoritativeBinding()); err != nil {
		t.Fatalf("absent nested identity was rejected: %v", err)
	}
}

// TestValidateIdentityBindingStillRejectsPopulatedMismatch guards the case the
// mirror loop already covered, so a fix for the blank-field hole cannot
// accidentally drop the non-blank comparison it was built on.
func TestValidateIdentityBindingStillRejectsPopulatedMismatch(t *testing.T) {
	request := &agentv1.JobRequest{
		TenantId:    "tenant-a",
		PrincipalId: "principal-a",
		Identity: &agentv1.IdentityBinding{
			TenantId:     "tenant-a",
			PrincipalId:  "principal-a",
			ActorId:      "actor-IMPOSTER",
			DelegationId: "delegation-a",
		},
	}
	if err := ValidateIdentityBinding(request, authoritativeBinding()); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("populated mismatching actor was accepted: got err=%v", err)
	}
}
