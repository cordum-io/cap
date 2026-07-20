package capsdk

import (
	"errors"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

var (
	ErrIdentityMismatch       = errors.New("capsdk: authoritative identity mismatch")
	ErrStaleDispatchEvent     = errors.New("capsdk: stale or unauthorized dispatch event")
	ErrCompensationEscalation = errors.New("capsdk: compensation privilege escalation")
)

func ValidateIdentityBinding(request *agentv1.JobRequest, authoritative *agentv1.IdentityBinding) error {
	if request == nil || authoritative == nil || authoritative.GetTenantId() == "" {
		return ErrIdentityMismatch
	}
	for _, mirror := range productionIdentityMirrors(request, authoritative) {
		if mirror.actual != "" && mirror.actual != mirror.expected {
			return fmt.Errorf("%w: %s", ErrIdentityMismatch, mirror.path)
		}
	}
	// A nested IdentityBinding is not a partial mirror. The skip-if-empty rule
	// above is correct for the flat env/labels/meta strings -- a blank
	// env["actor_id"] means "not mirrored here" -- but wrong for a sub-message,
	// where a present binding with a blank ActorId is an affirmative claim of
	// "no actor" rather than an omission. Comparing only the non-blank fields
	// would admit an identity stripped of exactly the fields an attacker wants
	// dropped. Keeps Go aligned with sdk/python production_validation's
	// _same_identity and sdk/node production-validation's sameIdentity, which
	// both compare all four fields unconditionally.
	if err := strictBindingMatch("identity", request.GetIdentity(), authoritative); err != nil {
		return err
	}
	if compensation := request.GetCompensation(); compensation != nil {
		if err := strictBindingMatch(
			"compensation.identity", compensation.GetIdentity(), authoritative,
		); err != nil {
			return err
		}
	}
	return nil
}

// strictBindingMatch compares every field of a present nested binding, blank
// ones included. An absent binding is allowed: that is a legitimate
// migration/compat shape, and keeping it distinguishable from a present-but-blank
// binding is the point.
func strictBindingMatch(prefix string, actual, expected *agentv1.IdentityBinding) error {
	if actual == nil {
		return nil
	}
	for _, mirror := range bindingIdentityMirrors(prefix, actual, expected) {
		if mirror.actual != mirror.expected {
			return fmt.Errorf("%w: %s", ErrIdentityMismatch, mirror.path)
		}
	}
	return nil
}

type identityMirror struct {
	path, actual, expected string
}

func productionIdentityMirrors(
	request *agentv1.JobRequest, identity *agentv1.IdentityBinding,
) []identityMirror {
	mirrors := []identityMirror{
		{"tenant", request.GetTenantId(), identity.GetTenantId()},
		{"principal", request.GetPrincipalId(), identity.GetPrincipalId()},
	}
	mirrors = append(mirrors, metadataIdentityMirrors("meta", request.GetMeta(), identity)...)
	mirrors = append(mirrors, mapIdentityMirrors("env", request.GetEnv(), identity)...)
	mirrors = append(mirrors, mapIdentityMirrors("labels", request.GetLabels(), identity)...)
	// request.Identity is deliberately absent here: nested bindings are checked
	// strictly by strictBindingMatch, not by the skip-if-empty loop.
	if compensation := request.GetCompensation(); compensation != nil {
		mirrors = append(mirrors, compensationIdentityMirrors(compensation, identity)...)
	}
	return mirrors
}

func compensationIdentityMirrors(
	compensation *agentv1.Compensation, identity *agentv1.IdentityBinding,
) []identityMirror {
	mirrors := []identityMirror{
		{"compensation.tenant", compensation.GetTenantId(), identity.GetTenantId()},
		{"compensation.principal", compensation.GetPrincipalId(), identity.GetPrincipalId()},
	}
	mirrors = append(mirrors, metadataIdentityMirrors("compensation.meta", compensation.GetMeta(), identity)...)
	mirrors = append(mirrors, mapIdentityMirrors("compensation.env", compensation.GetEnv(), identity)...)
	mirrors = append(mirrors, mapIdentityMirrors("compensation.labels", compensation.GetLabels(), identity)...)
	// compensation.Identity: see the note in productionIdentityMirrors.
	return mirrors
}

func metadataIdentityMirrors(
	prefix string, metadata *agentv1.JobMetadata, identity *agentv1.IdentityBinding,
) []identityMirror {
	if metadata == nil {
		return nil
	}
	mirrors := []identityMirror{
		{prefix + ".tenant", metadata.GetTenantId(), identity.GetTenantId()},
		{prefix + ".actor", metadata.GetActorId(), identity.GetActorId()},
	}
	return append(mirrors, mapIdentityMirrors(prefix+".labels", metadata.GetLabels(), identity)...)
}

func mapIdentityMirrors(
	prefix string, values map[string]string, identity *agentv1.IdentityBinding,
) []identityMirror {
	if values == nil {
		return nil
	}
	return []identityMirror{
		{prefix + ".tenant_id", values["tenant_id"], identity.GetTenantId()},
		{prefix + ".principal_id", values["principal_id"], identity.GetPrincipalId()},
		{prefix + ".actor_id", values["actor_id"], identity.GetActorId()},
		{prefix + ".delegation_id", values["delegation_id"], identity.GetDelegationId()},
	}
}

func bindingIdentityMirrors(
	prefix string, actual, expected *agentv1.IdentityBinding,
) []identityMirror {
	if actual == nil {
		return nil
	}
	return []identityMirror{
		{prefix + ".tenant_id", actual.GetTenantId(), expected.GetTenantId()},
		{prefix + ".principal_id", actual.GetPrincipalId(), expected.GetPrincipalId()},
		{prefix + ".actor_id", actual.GetActorId(), expected.GetActorId()},
		{prefix + ".delegation_id", actual.GetDelegationId(), expected.GetDelegationId()},
	}
}

func ValidateDispatchFencing(current, event *agentv1.DispatchIdentity) error {
	if current == nil || event == nil || current.GetDispatchId() == "" ||
		current.GetDispatchId() != event.GetDispatchId() || current.GetAttempt() != event.GetAttempt() ||
		current.GetAssignedWorkerId() != event.GetAssignedWorkerId() {
		return ErrStaleDispatchEvent
	}
	return nil
}

func ValidateCompensationMonotonicity(parent *agentv1.JobRequest, compensation *agentv1.Compensation) error {
	if parent == nil || compensation == nil {
		return ErrCompensationEscalation
	}
	if compensation.GetIdentity() != nil && !sameIdentity(parent.GetIdentity(), compensation.GetIdentity()) {
		return ErrCompensationEscalation
	}
	if valueDiffers(compensation.GetTenantId(), parent.GetTenantId()) || valueDiffers(compensation.GetPrincipalId(), parent.GetPrincipalId()) {
		return ErrCompensationEscalation
	}
	parentMeta, childMeta := parent.GetMeta(), compensation.GetMeta()
	if childMeta != nil {
		if parentMeta == nil || valueDiffers(childMeta.GetCapability(), parentMeta.GetCapability()) || !subset(childMeta.GetRiskTags(), parentMeta.GetRiskTags()) {
			return ErrCompensationEscalation
		}
	}
	return nil
}

func sameIdentity(a, b *agentv1.IdentityBinding) bool {
	return a != nil && b != nil && a.GetTenantId() == b.GetTenantId() && a.GetPrincipalId() == b.GetPrincipalId() &&
		a.GetActorId() == b.GetActorId() && a.GetDelegationId() == b.GetDelegationId()
}

func valueDiffers(child, parent string) bool { return child != "" && child != parent }

func subset(values, allowed []string) bool {
	for _, value := range values {
		if !contains(allowed, value) {
			return false
		}
	}
	return true
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
