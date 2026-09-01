package capsdk_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestResourceRegistryZeroValuesFailClosed(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	var nilRegistry *capsdk.ResourceRegistry
	zeroRegistry := &capsdk.ResourceRegistry{}
	for name, registry := range map[string]*capsdk.ResourceRegistry{
		"nil": nilRegistry, "empty": zeroRegistry,
	} {
		t.Run(name, func(t *testing.T) {
			_, resolveErr := registry.Resolve(context.Background(), validRef([]byte("x"), now.Add(time.Hour)), trustedContext())
			_, externalizeErr := registry.ExternalizeBytes(
				context.Background(), "memory", []byte("x"), "application/json", "job-input", now.Add(time.Hour), trustedContext(),
			)
			if !errors.Is(resolveErr, capsdk.ErrInvalidResourceRegistry) ||
				!errors.Is(externalizeErr, capsdk.ErrInvalidResourceRegistry) {
				t.Fatalf("Resolve error=%v Externalize error=%v", resolveErr, externalizeErr)
			}
		})
	}
}

func TestResourceRegistryResolveRejectsNilReference(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	_, err := registry.Resolve(context.Background(), nil, trustedContext())
	if !errors.Is(err, capsdk.ErrInvalidResourceRef) || backend.resolveCalls != 0 {
		t.Fatalf("Resolve() error=%v backend-calls=%d", err, backend.resolveCalls)
	}
}

func TestResourceRegistryResolveRejectsNilBodies(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	var typedNil *closeCountingReader
	for _, tc := range []struct {
		name string
		body io.ReadCloser
	}{{"nil", nil}, {"typed-nil", typedNil}} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			resolver := resolverFunc(func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
				calls++
				return capsdk.ResourceResolveResult{Body: tc.body, MediaType: "application/json"}, nil
			})
			registry := registryWithResolver(t, now, resolver)
			_, err := registry.Resolve(context.Background(), validRef([]byte("x"), now.Add(time.Hour)), trustedContext())
			if !errors.Is(err, capsdk.ErrResourceBackendUnavailable) || calls != 1 {
				t.Fatalf("Resolve() error=%v backend-calls=%d", err, calls)
			}
		})
	}
}

func TestResourceRegistryExternalizeRejectsNilReaders(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	var typedNil *nilResourceReader
	for _, tc := range []struct {
		name   string
		reader io.Reader
	}{{"nil", nil}, {"typed-nil", typedNil}} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			backend := &backendHarness{externalize: func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
				called = true
				return nil, nil
			}}
			registry := newRegistry(t, fixedClock(now), backend, 64)
			_, err := registry.Externalize(
				context.Background(), "memory", tc.reader, "application/json", "job-input", now.Add(time.Hour), trustedContext(),
			)
			if !errors.Is(err, capsdk.ErrResourceBackendUnavailable) || called {
				t.Fatalf("Externalize() error=%v backend-called=%v", err, called)
			}
		})
	}
}

func TestResourceRegistryRejectsZeroProgressReaders(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	body := &closeCountingReader{Reader: zeroProgressReader{}}
	resolver := resolverFunc(func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		return capsdk.ResourceResolveResult{Body: body, MediaType: "application/json"}, nil
	})
	registry := registryWithResolver(t, now, resolver)
	resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer resolveCancel()
	_, resolveErr := registry.Resolve(resolveCtx, validRef([]byte("x"), now.Add(time.Hour)), trustedContext())
	externalizeCtx, externalizeCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer externalizeCancel()
	_, externalizeErr := registry.Externalize(
		externalizeCtx, "memory", zeroProgressReader{}, "application/json", "job-input", now.Add(time.Hour), trustedContext(),
	)
	if !errors.Is(resolveErr, capsdk.ErrResourceBackendUnavailable) || body.closeCalls != 1 ||
		!errors.Is(externalizeErr, capsdk.ErrResourceBackendUnavailable) {
		t.Fatalf("Resolve error=%v close-calls=%d Externalize error=%v", resolveErr, body.closeCalls, externalizeErr)
	}
}

func TestResourceRegistryRejectsInvalidReaderCounts(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	for _, invalid := range []invalidCountReader{{negative: true}, {}} {
		_, err := registry.Externalize(
			context.Background(), "memory", invalid, "application/json", "job-input", now.Add(time.Hour), trustedContext(),
		)
		if !errors.Is(err, capsdk.ErrResourceBackendUnavailable) {
			t.Fatalf("Externalize(%+v) error=%v", invalid, err)
		}
	}
}

func TestResourceRegistryRejectsCancellationDuringRead(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	resolveCtx, resolveCancel := context.WithCancel(context.Background())
	body := &closeCountingReader{Reader: &cancelAfterDataReader{cancel: resolveCancel}}
	resolver := resolverFunc(func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		return capsdk.ResourceResolveResult{Body: body, MediaType: "application/json"}, nil
	})
	registry := registryWithResolver(t, now, resolver)
	_, resolveErr := registry.Resolve(resolveCtx, validRef([]byte("x"), now.Add(time.Hour)), trustedContext())
	externalizeCtx, externalizeCancel := context.WithCancel(context.Background())
	_, externalizeErr := registry.Externalize(
		externalizeCtx, "memory", &cancelAfterDataReader{cancel: externalizeCancel},
		"application/json", "job-input", now.Add(time.Hour), trustedContext(),
	)
	if !errors.Is(resolveErr, context.Canceled) || body.closeCalls != 1 || !errors.Is(externalizeErr, context.Canceled) {
		t.Fatalf("Resolve error=%v close-calls=%d Externalize error=%v", resolveErr, body.closeCalls, externalizeErr)
	}
}

func TestResourceRegistryResolveRejectsInstalledBoundPlusOne(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("12345")
	backend := &backendHarness{content: content, mediaType: "application/json"}
	registry := newRegistry(t, fixedClock(now), backend, 4)
	_, err := registry.Resolve(context.Background(), validRef(content, now.Add(time.Hour)), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceTooLarge) || backend.resolveCalls != 0 {
		t.Fatalf("Resolve() error=%v backend-calls=%d", err, backend.resolveCalls)
	}
}

func TestResourceRegistryResolveRejectsUnsafeURIsBeforeBackend(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{content: []byte("x"), mediaType: "application/json"}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	for _, uri := range []string{
		"memory://user:secret@objects/item", "memory://objects/item?token=x",
		"memory://objects/item#fragment", "memory://objects/../secret",
	} {
		ref := validRef([]byte("x"), now.Add(time.Hour))
		ref.Uri = uri
		_, err := registry.Resolve(context.Background(), ref, trustedContext())
		if !errors.Is(err, capsdk.ErrInvalidResourceRef) {
			t.Fatalf("Resolve(%q) error=%v", uri, err)
		}
	}
	if backend.resolveCalls != 0 {
		t.Fatalf("backend called %d times", backend.resolveCalls)
	}
}

func TestResourceRegistryExternalizeRejectsNilBackendReference(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{externalize: func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		return nil, nil
	}}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	_, err := registry.ExternalizeBytes(
		context.Background(), "memory", []byte("x"), "application/json", "job-input", now.Add(time.Hour), trustedContext(),
	)
	if !errors.Is(err, capsdk.ErrResourceMetadataMismatch) {
		t.Fatalf("ExternalizeBytes() error=%v", err)
	}
}

func registryWithResolver(t *testing.T, now time.Time, resolver capsdk.ResourceResolverBackend) *capsdk.ResourceRegistry {
	t.Helper()
	backend := &backendHarness{}
	registry, err := capsdk.NewResourceRegistry(fixedClock(now), capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: 64, Resolver: resolver, Externalizer: backend.externalizer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

type nilResourceReader struct{}

func (*nilResourceReader) Read([]byte) (int, error) { return 0, io.EOF }

type zeroProgressReader struct{}

func (zeroProgressReader) Read([]byte) (int, error) { return 0, nil }

type invalidCountReader struct{ negative bool }

func (reader invalidCountReader) Read(buffer []byte) (int, error) {
	if reader.negative {
		return -1, nil
	}
	return len(buffer) + 1, nil
}

type cancelAfterDataReader struct {
	cancel context.CancelFunc
	read   bool
}

func (reader *cancelAfterDataReader) Read(buffer []byte) (int, error) {
	if reader.read {
		return 0, io.EOF
	}
	reader.read = true
	buffer[0] = 'x'
	reader.cancel()
	return 1, nil
}
