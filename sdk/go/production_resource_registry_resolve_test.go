package capsdk_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestNewResourceRegistryRejectsUnsafeRegistrations(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{}
	valid := capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: 64, Resolver: backend.resolver(), Externalizer: backend.externalizer(),
	}
	cases := map[string][]capsdk.ResourceBackendRegistration{
		"empty":                  nil,
		"nil resolver":           {{ID: "memory", MaxBytes: 64, Externalizer: backend.externalizer()}},
		"typed nil resolver":     {{ID: "memory", MaxBytes: 64, Resolver: resolverFunc(nil), Externalizer: backend.externalizer()}},
		"nil writer":             {{ID: "memory", MaxBytes: 64, Resolver: backend.resolver()}},
		"typed nil externalizer": {{ID: "memory", MaxBytes: 64, Resolver: backend.resolver(), Externalizer: externalizerFunc(nil)}},
		"invalid id":             {{ID: "Memory", MaxBytes: 64, Resolver: backend.resolver(), Externalizer: backend.externalizer()}},
		"zero bound":             {{ID: "memory", Resolver: backend.resolver(), Externalizer: backend.externalizer()}},
		"global bound":           {{ID: "memory", MaxBytes: capsdk.MaxResourceSizeBytes + 1, Resolver: backend.resolver(), Externalizer: backend.externalizer()}},
		"duplicate id":           {valid, valid},
	}
	for name, registrations := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := capsdk.NewResourceRegistry(fixedClock(now), registrations...)
			if !errors.Is(err, capsdk.ErrInvalidResourceRegistry) {
				t.Fatalf("NewResourceRegistry() error = %v", err)
			}
		})
	}
}

func TestNewResourceRegistrySnapshotsRegistrations(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("payload")
	backend := &backendHarness{content: content, mediaType: "application/json"}
	registrations := []capsdk.ResourceBackendRegistration{{
		ID: "memory", MaxBytes: 64, Resolver: backend.resolver(), Externalizer: backend.externalizer(),
	}}
	registry, err := capsdk.NewResourceRegistry(fixedClock(now), registrations...)
	if err != nil {
		t.Fatal(err)
	}
	registrations[0] = capsdk.ResourceBackendRegistration{ID: "changed"}
	if _, err := registry.Resolve(context.Background(), validRef(content, now.Add(time.Hour)), trustedContext()); err != nil {
		t.Fatalf("Resolve() after caller mutation error = %v", err)
	}
}

func TestResourceRegistryResolveUsesTrustedContextAndClonesBytes(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte(`{"ok":true}`)
	backendContent := append([]byte(nil), content...)
	var got capsdk.ResourceResolveRequest
	backend := &backendHarness{content: backendContent, mediaType: "application/json"}
	backendResolver := resolverFunc(func(_ context.Context, request capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		got = request
		return capsdk.ResourceResolveResult{Body: io.NopCloser(bytes.NewReader(backendContent)), MediaType: "application/json"}, nil
	})
	registry, err := capsdk.NewResourceRegistry(fixedClock(now), capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: 64, Resolver: backendResolver, Externalizer: backend.externalizer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := registry.Resolve(context.Background(), validRef(content, now.Add(time.Hour)), trustedContext())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.TrustedContext != trustedContext() || got.URI != "memory://objects/item" {
		t.Fatalf("backend request = %#v", got)
	}
	backendContent[0] = 'X'
	if string(resolved.Content) != string(content) || resolved.MediaType != "application/json" {
		t.Fatalf("Resolve() = %#v", resolved)
	}
}

func TestResourceRegistryResolveRejectsUnknownWithoutFallback(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("payload")
	backend := &backendHarness{content: content, mediaType: "application/octet-stream"}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	ref := validRef(content, now.Add(time.Hour))
	ref.ResolverId = "missing"
	ref.Uri = "memory://objects/item"
	ref.MediaType = "application/octet-stream"
	_, err := registry.Resolve(context.Background(), ref, trustedContext())
	if !errors.Is(err, capsdk.ErrResourceResolverUnavailable) {
		t.Fatalf("Resolve() error = %v", err)
	}
	if backend.resolveCalls != 0 {
		t.Fatalf("registered fallback called %d times", backend.resolveCalls)
	}
}

func TestResourceRegistryResolveRejectsBeforeBackend(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{content: []byte("x"), mediaType: "application/json"}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	ref := validRef([]byte("x"), now.Add(time.Hour))
	if _, err := registry.Resolve(cancelled, ref, trustedContext()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Resolve() error = %v", err)
	}
	ref.Sha256 = []byte("short")
	if _, err := registry.Resolve(context.Background(), ref, trustedContext()); !errors.Is(err, capsdk.ErrInvalidResourceRef) {
		t.Fatalf("invalid Resolve() error = %v", err)
	}
	if backend.resolveCalls != 0 {
		t.Fatalf("backend called %d times", backend.resolveCalls)
	}
}

func TestResourceRegistryResolveRejectsCancellationAtEndOfRead(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("payload")
	ctx, cancel := context.WithCancel(context.Background())
	resolver := resolverFunc(func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		body := &cancelOnReadCloser{Reader: bytes.NewReader(content), cancel: cancel}
		return capsdk.ResourceResolveResult{Body: body, MediaType: "application/json"}, nil
	})
	backend := &backendHarness{}
	registry, err := capsdk.NewResourceRegistry(fixedClock(now), capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: 64, Resolver: resolver, Externalizer: backend.externalizer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Resolve(ctx, validRef(content, now.Add(time.Hour)), trustedContext())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResourceRegistryResolveRejectsExpiryBeforeAndAfterFetch(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("payload")
	backend := &backendHarness{content: content, mediaType: "application/json"}
	expiredRegistry := newRegistry(t, fixedClock(base), backend, 64)
	if _, err := expiredRegistry.Resolve(context.Background(), validRef(content, base), trustedContext()); !errors.Is(err, capsdk.ErrResourceExpired) {
		t.Fatalf("pre-fetch expiry error = %v", err)
	}
	times := []time.Time{base, base.Add(2 * time.Second)}
	index := 0
	raceClock := func() time.Time { value := times[index]; index++; return value }
	raceRegistry := newRegistry(t, raceClock, backend, 64)
	if _, err := raceRegistry.Resolve(context.Background(), validRef(content, base.Add(time.Second)), trustedContext()); !errors.Is(err, capsdk.ErrResourceExpired) {
		t.Fatalf("post-fetch expiry error = %v", err)
	}
}

func TestResourceRegistryResolveRejectsContentMismatches(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	want := []byte("payload")
	cases := map[string]struct {
		content []byte
		media   string
		err     error
	}{
		"short":  {[]byte("pay"), "application/json", capsdk.ErrResourceSizeMismatch},
		"digest": {[]byte("PAYLOAD"), "application/json", capsdk.ErrResourceDigestMismatch},
		"type":   {want, "text/plain", capsdk.ErrResourceMediaTypeMismatch},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			backend := &backendHarness{content: tc.content, mediaType: tc.media}
			registry := newRegistry(t, fixedClock(now), backend, 64)
			_, err := registry.Resolve(context.Background(), validRef(want, now.Add(time.Hour)), trustedContext())
			if !errors.Is(err, tc.err) {
				t.Fatalf("Resolve() error = %v", err)
			}
		})
	}
}

func TestResourceRegistryResolveBoundsOversizeRead(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	reader := &countingReader{remaining: 1024}
	resolver := resolverFunc(func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		return capsdk.ResourceResolveResult{Body: io.NopCloser(reader), MediaType: "application/json"}, nil
	})
	backend := &backendHarness{}
	registry, err := capsdk.NewResourceRegistry(fixedClock(now), capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: 64, Resolver: resolver, Externalizer: backend.externalizer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Resolve(context.Background(), validRef([]byte("1234"), now.Add(time.Hour)), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceTooLarge) || reader.readBytes != 5 {
		t.Fatalf("Resolve() error = %v, bytes read = %d", err, reader.readBytes)
	}
}

func TestResourceRegistryResolveBoundsBackendErrors(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	secret := strings.Repeat("secret", 1000)
	resolver := resolverFunc(func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
		return capsdk.ResourceResolveResult{}, errors.New(secret)
	})
	backend := &backendHarness{}
	registry, err := capsdk.NewResourceRegistry(fixedClock(now), capsdk.ResourceBackendRegistration{
		ID: "memory", MaxBytes: 64, Resolver: resolver, Externalizer: backend.externalizer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Resolve(context.Background(), validRef([]byte("x"), now.Add(time.Hour)), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceBackendUnavailable) || strings.Contains(err.Error(), secret) || len(err.Error()) > 100 {
		t.Fatalf("Resolve() leaked backend error: %q", err)
	}
}

type countingReader struct {
	remaining int
	readBytes int
}

type cancelOnReadCloser struct {
	*bytes.Reader
	cancel context.CancelFunc
}

func (reader *cancelOnReadCloser) Read(buffer []byte) (int, error) {
	count, _ := reader.Reader.Read(buffer)
	reader.cancel()
	return count, io.EOF
}

func (*cancelOnReadCloser) Close() error { return nil }

func (reader *countingReader) Read(buffer []byte) (int, error) {
	if reader.remaining == 0 {
		return 0, io.EOF
	}
	count := len(buffer)
	if count > reader.remaining {
		count = reader.remaining
	}
	for index := 0; index < count; index++ {
		buffer[index] = 'x'
	}
	reader.remaining -= count
	reader.readBytes += count
	return count, nil
}
