package capsdk_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type resolverFunc func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error)

func (fn resolverFunc) Resolve(
	ctx context.Context,
	request capsdk.ResourceResolveRequest,
) (capsdk.ResourceResolveResult, error) {
	return fn(ctx, request)
}

type externalizerFunc func(context.Context, capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error)

func (fn externalizerFunc) Externalize(
	ctx context.Context,
	request capsdk.ResourceExternalizeRequest,
) (*agentv1.ResourceRef, error) {
	return fn(ctx, request)
}

type backendHarness struct {
	mu           sync.Mutex
	content      []byte
	mediaType    string
	resolveCalls int
	externalize  func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error)
}

func (backend *backendHarness) resolver() capsdk.ResourceResolverBackend {
	return resolverFunc(func(_ context.Context, _ capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		backend.mu.Lock()
		defer backend.mu.Unlock()
		backend.resolveCalls++
		body := append([]byte(nil), backend.content...)
		return capsdk.ResourceResolveResult{Body: io.NopCloser(bytes.NewReader(body)), MediaType: backend.mediaType}, nil
	})
}

func (backend *backendHarness) externalizer() capsdk.ResourceExternalizerBackend {
	return externalizerFunc(func(_ context.Context, request capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		if backend.externalize != nil {
			return backend.externalize(request)
		}
		return refFromRequest("memory", "memory://objects/item", request), nil
	})
}

func newRegistry(t *testing.T, now func() time.Time, backend *backendHarness, maxBytes uint64) *capsdk.ResourceRegistry {
	t.Helper()
	registry, err := capsdk.NewResourceRegistry(now, capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: maxBytes,
		Resolver: backend.resolver(), Externalizer: backend.externalizer(),
	})
	if err != nil {
		t.Fatalf("NewResourceRegistry() error = %v", err)
	}
	return registry
}

func validRef(content []byte, expires time.Time) *agentv1.ResourceRef {
	digest := sha256.Sum256(content)
	return &agentv1.ResourceRef{
		ResolverId: "memory", Uri: "memory://objects/item", Sha256: digest[:],
		MediaType: "application/json", SizeBytes: uint64(len(content)),
		ExpiresAt: timestamppb.New(expires), Purpose: "job-input",
	}
}

func refFromRequest(id, uri string, request capsdk.ResourceExternalizeRequest) *agentv1.ResourceRef {
	digest := append([]byte(nil), request.SHA256[:]...)
	return &agentv1.ResourceRef{
		ResolverId: id, Uri: uri, Sha256: digest, MediaType: request.MediaType,
		SizeBytes: request.SizeBytes, ExpiresAt: timestamppb.New(request.ExpiresAt),
		Purpose: request.Purpose,
	}
}

func trustedContext() capsdk.TrustedResourceContext {
	return capsdk.TrustedResourceContext{TenantID: "tenant-a", JobID: "job-7"}
}

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func TestResolvedResourceAPIIsOpaque(t *testing.T) {
	resourceType := reflect.TypeOf(capsdk.ResolvedResource{})
	for index := 0; index < resourceType.NumField(); index++ {
		field := resourceType.Field(index)
		if field.IsExported() {
			t.Fatalf("ResolvedResource field %q is externally forgeable", field.Name)
		}
	}
}

func TestResolvedResourceZeroValueExposesNoVerifiedData(t *testing.T) {
	var resource capsdk.ResolvedResource
	if content := resource.Content(); content != nil {
		t.Fatalf("zero-value Content() = %q", content)
	}
	if mediaType := resource.MediaType(); mediaType != "" {
		t.Fatalf("zero-value MediaType() = %q", mediaType)
	}
}
