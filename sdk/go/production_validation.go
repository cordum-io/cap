package capsdk

import (
	"errors"
	"fmt"
	"strings"

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
	wants := map[string]string{
		"tenant": request.GetTenantId(), "principal": request.GetPrincipalId(),
		"meta.tenant": request.GetMeta().GetTenantId(), "meta.actor": request.GetMeta().GetActorId(),
		"env.tenant": request.GetEnv()["tenant_id"], "env.principal": request.GetEnv()["principal_id"],
	}
	for name, actual := range wants {
		expected := authoritative.GetTenantId()
		if strings.Contains(name, "principal") {
			expected = authoritative.GetPrincipalId()
		} else if strings.Contains(name, "actor") {
			expected = authoritative.GetActorId()
		}
		if actual != "" && actual != expected {
			return fmt.Errorf("%w: %s", ErrIdentityMismatch, name)
		}
	}
	if request.GetIdentity() != nil && !sameIdentity(request.GetIdentity(), authoritative) {
		return ErrIdentityMismatch
	}
	return nil
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
