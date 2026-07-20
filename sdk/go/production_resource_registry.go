package capsdk

import (
	"context"
	"errors"
	"io"
	"reflect"
	"regexp"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

const (
	MaxTrustedTenantIDBytes = 128
	MaxTrustedJobIDBytes    = 256
)

var (
	ErrInvalidResourceRegistry         = errors.New("capsdk: invalid resource registry")
	ErrInvalidTrustedResourceContext   = errors.New("capsdk: invalid trusted resource context")
	ErrResourceResolverUnavailable     = errors.New("capsdk: resource resolver unavailable")
	ErrResourceExternalizerUnavailable = errors.New("capsdk: resource externalizer unavailable")
	ErrResourceBackendUnavailable      = errors.New("capsdk: resource backend unavailable")
	ErrResourceExpired                 = errors.New("capsdk: resource expired")
	ErrResourceTooLarge                = errors.New("capsdk: resource exceeds installed limit")
	ErrResourceSizeMismatch            = errors.New("capsdk: resource size mismatch")
	ErrResourceMediaTypeMismatch       = errors.New("capsdk: resource media type mismatch")
	ErrResourceDigestMismatch          = errors.New("capsdk: resource digest mismatch")
	ErrResourceMetadataMismatch        = errors.New("capsdk: externalized resource metadata mismatch")

	trustedTenantPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	trustedJobPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@\-\[\]]*$`)
)

// TrustedResourceContext comes from authenticated local authority. A
// ResourceRef never supplies or overrides these values.
type TrustedResourceContext struct {
	TenantID string
	JobID    string
}

// ResourceResolveRequest is the validated, immutable request sent to an
// explicitly installed resolver.
type ResourceResolveRequest struct {
	TrustedContext    TrustedResourceContext
	URI               string
	MediaType         string
	Purpose           string
	DeclaredSizeBytes uint64
}

// ResourceResolveResult is the streaming result returned by a resolver.
type ResourceResolveResult struct {
	Body      io.ReadCloser
	MediaType string
}

// ResolvedResource contains bytes that passed expiry, type, size, and digest
// checks. Content is owned by the caller.
type ResolvedResource struct {
	Content   []byte
	MediaType string
}

// ResourceExternalizeRequest contains bounded content and trusted local
// authority for an explicitly installed externalizer.
type ResourceExternalizeRequest struct {
	TrustedContext TrustedResourceContext
	Content        []byte
	MediaType      string
	Purpose        string
	ExpiresAt      time.Time
	SizeBytes      uint64
	SHA256         [32]byte
}

// ResourceResolverBackend fetches only from its operator-configured store. It
// must honor ctx and must not interpret ResourceRef values as credentials.
type ResourceResolverBackend interface {
	Resolve(context.Context, ResourceResolveRequest) (ResourceResolveResult, error)
}

// ResourceExternalizerBackend stores bounded content and returns a complete
// ResourceRef. The registry independently validates every returned field.
type ResourceExternalizerBackend interface {
	Externalize(context.Context, ResourceExternalizeRequest) (*agentv1.ResourceRef, error)
}

// ResourceBackendRegistration installs one paired resolver/externalizer under
// an exact canonical ID.
type ResourceBackendRegistration struct {
	ID           string
	MaxBytes     uint64
	Resolver     ResourceResolverBackend
	Externalizer ResourceExternalizerBackend
}

type resourceBackend struct {
	maxBytes     uint64
	resolver     ResourceResolverBackend
	externalizer ResourceExternalizerBackend
}

// ResourceRegistry is an immutable snapshot of operator-installed backends.
type ResourceRegistry struct {
	now      func() time.Time
	backends map[string]resourceBackend
	ids      []string
}

// NewResourceRegistry snapshots registrations. A nil clock uses time.Now.
func NewResourceRegistry(now func() time.Time, registrations ...ResourceBackendRegistration) (*ResourceRegistry, error) {
	if len(registrations) == 0 {
		return nil, ErrInvalidResourceRegistry
	}
	if now == nil {
		now = time.Now
	}
	registry := &ResourceRegistry{
		now: now, backends: make(map[string]resourceBackend, len(registrations)),
		ids: make([]string, 0, len(registrations)),
	}
	for _, registration := range registrations {
		if err := registry.addRegistration(registration); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (registry *ResourceRegistry) addRegistration(registration ResourceBackendRegistration) error {
	if !validResourceIdentifier(registration.ID, MaxResourceIdentifierBytes, resourceIDPattern) {
		return ErrInvalidResourceRegistry
	}
	if registration.MaxBytes == 0 || registration.MaxBytes > MaxResourceSizeBytes {
		return ErrInvalidResourceRegistry
	}
	if resolverBackendIsNil(registration.Resolver) || externalizerBackendIsNil(registration.Externalizer) {
		return ErrInvalidResourceRegistry
	}
	if _, duplicate := registry.backends[registration.ID]; duplicate {
		return ErrInvalidResourceRegistry
	}
	registry.backends[registration.ID] = resourceBackend{
		maxBytes: registration.MaxBytes,
		resolver: registration.Resolver, externalizer: registration.Externalizer,
	}
	registry.ids = append(registry.ids, registration.ID)
	return nil
}

func (registry *ResourceRegistry) valid() bool {
	return registry != nil && registry.now != nil && len(registry.backends) > 0 && len(registry.ids) > 0
}

func (registry *ResourceRegistry) currentTime() time.Time {
	return registry.now().UTC()
}

func validateTrustedResourceContext(trusted TrustedResourceContext) error {
	if !validTrustedValue(trusted.TenantID, MaxTrustedTenantIDBytes, trustedTenantPattern) {
		return ErrInvalidTrustedResourceContext
	}
	if !validTrustedValue(trusted.JobID, MaxTrustedJobIDBytes, trustedJobPattern) {
		return ErrInvalidTrustedResourceContext
	}
	return nil
}

func validTrustedValue(value string, limit int, pattern *regexp.Regexp) bool {
	return value != "" && len(value) <= limit && pattern.MatchString(value)
}

func resolverBackendIsNil(backend ResourceResolverBackend) bool {
	if backend == nil {
		return true
	}
	return nilReflectValue(reflect.ValueOf(backend))
}

func externalizerBackendIsNil(backend ResourceExternalizerBackend) bool {
	if backend == nil {
		return true
	}
	return nilReflectValue(reflect.ValueOf(backend))
}

func nilReflectValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
