package capsdk_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestResourceRegistryRejectsInvalidTrustedContextBeforeBackend(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("payload")
	cases := invalidTrustedResourceContexts()
	for name, trusted := range cases {
		t.Run(name, func(t *testing.T) {
			externalizeCalls := 0
			backend := &backendHarness{content: content, mediaType: "application/json"}
			backend.externalize = func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
				externalizeCalls++
				return nil, nil
			}
			registry := newRegistry(t, fixedClock(now), backend, 64)
			_, resolveErr := registry.Resolve(context.Background(), validRef(content, now.Add(time.Hour)), trusted)
			_, externalizeErr := registry.ExternalizeBytes(
				context.Background(), "memory", content, "application/json", "job-input", now.Add(time.Hour), trusted,
			)
			assertTrustedContextRejected(t, resolveErr, externalizeErr, backend.resolveCalls, externalizeCalls)
		})
	}
}

func TestResourceRegistryRejectsMissingOperationContext(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("payload")
	backend := &backendHarness{content: content, mediaType: "application/json"}
	externalizeCalls := 0
	backend.externalize = func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		externalizeCalls++
		return nil, nil
	}
	registry := newRegistry(t, fixedClock(now), backend, 64)

	_, resolveErr := registry.Resolve(nil, validRef(content, now.Add(time.Hour)), trustedContext())
	_, externalizeErr := registry.ExternalizeBytes(
		nil, "memory", content, "application/json", "job-input", now.Add(time.Hour), trustedContext(),
	)
	if !errors.Is(resolveErr, capsdk.ErrResourceContextRequired) ||
		!errors.Is(externalizeErr, capsdk.ErrResourceContextRequired) {
		t.Fatalf("Resolve error=%v Externalize error=%v", resolveErr, externalizeErr)
	}
	if backend.resolveCalls != 0 || externalizeCalls != 0 {
		t.Fatalf("backend calls resolve=%d externalize=%d", backend.resolveCalls, externalizeCalls)
	}
}

func invalidTrustedResourceContexts() map[string]capsdk.TrustedResourceContext {
	return map[string]capsdk.TrustedResourceContext{
		"empty tenant":   {JobID: "job-7"},
		"empty job":      {TenantID: "tenant-a"},
		"invalid tenant": {TenantID: "tenant/a", JobID: "job-7"},
		"invalid job":    {TenantID: "tenant-a", JobID: "job 7"},
		"tenant too long": {
			TenantID: strings.Repeat("t", capsdk.MaxTrustedTenantIDBytes+1), JobID: "job-7",
		},
		"job too long": {
			TenantID: "tenant-a", JobID: strings.Repeat("j", capsdk.MaxTrustedJobIDBytes+1),
		},
	}
}

func assertTrustedContextRejected(t *testing.T, resolveErr, externalizeErr error, resolveCalls, externalizeCalls int) {
	t.Helper()
	if !errors.Is(resolveErr, capsdk.ErrInvalidTrustedResourceContext) ||
		!errors.Is(externalizeErr, capsdk.ErrInvalidTrustedResourceContext) {
		t.Fatalf("Resolve error=%v Externalize error=%v", resolveErr, externalizeErr)
	}
	if resolveCalls != 0 || externalizeCalls != 0 {
		t.Fatalf("backend calls resolve=%d externalize=%d", resolveCalls, externalizeCalls)
	}
}

func TestResourceRegistryResolveClosesBodyReturnedWithError(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	body := &closeCountingReader{Reader: strings.NewReader("payload")}
	registry := newErroringResolverRegistry(t, now, body)
	_, err := registry.Resolve(context.Background(), validRef([]byte("payload"), now.Add(time.Hour)), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceBackendUnavailable) || body.closeCalls != 1 {
		t.Fatalf("Resolve() error=%v close-calls=%d", err, body.closeCalls)
	}
}

func TestResourceRegistryResolveIgnoresTypedNilBodyReturnedWithError(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	var body *closeCountingReader
	registry := newErroringResolverRegistry(t, now, body)
	_, err := registry.Resolve(context.Background(), validRef([]byte("payload"), now.Add(time.Hour)), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceBackendUnavailable) {
		t.Fatalf("Resolve() error=%v", err)
	}
}

func newErroringResolverRegistry(t *testing.T, now time.Time, body io.ReadCloser) *capsdk.ResourceRegistry {
	t.Helper()
	resolver := resolverFunc(func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		return capsdk.ResourceResolveResult{Body: body}, errors.New("backend secret")
	})
	backend := &backendHarness{}
	registry, err := capsdk.NewResourceRegistry(fixedClock(now), capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: 64, Resolver: resolver, Externalizer: backend.externalizer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

type closeCountingReader struct {
	io.Reader
	closeCalls int
}

func (reader *closeCountingReader) Close() error {
	reader.closeCalls++
	return nil
}
