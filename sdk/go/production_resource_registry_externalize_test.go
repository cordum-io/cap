package capsdk_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestResourceRegistryExternalizeValidatesAndClonesBackendRef(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	content := []byte("payload")
	var request capsdk.ResourceExternalizeRequest
	var backendRef = validRef(content, expires)
	backend := &backendHarness{externalize: func(got capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		request = got
		backendRef = refFromRequest("memory", "memory://objects/item", got)
		return backendRef, nil
	}}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	ref, err := registry.Externalize(context.Background(), "memory", bytes.NewReader(content), "application/json", "job-input", expires, trustedContext())
	if err != nil {
		t.Fatalf("Externalize() error = %v", err)
	}
	if string(request.Content) != string(content) || request.TrustedContext != trustedContext() || request.SizeBytes != 7 {
		t.Fatalf("backend request = %#v", request)
	}
	backendRef.Uri = "memory://objects/mutated"
	backendRef.Sha256[0] ^= 0xff
	request.Content[0] = 'X'
	if ref.GetUri() != "memory://objects/item" || string(content) != "payload" {
		t.Fatalf("Externalize() returned aliased data: %#v content=%q", ref, content)
	}
}

func TestResourceRegistryExternalizeBytes(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	ref, err := registry.ExternalizeBytes(context.Background(), "memory", []byte("payload"), "application/json", "job-input", now.Add(time.Hour), trustedContext())
	if err != nil || ref.GetSizeBytes() != 7 {
		t.Fatalf("ExternalizeBytes() ref=%v error=%v", ref, err)
	}
}

func TestResourceRegistryExternalizeRejectsUnknownWithoutFallback(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	called := false
	backend := &backendHarness{externalize: func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		called = true
		return nil, nil
	}}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	_, err := registry.Externalize(context.Background(), "missing", strings.NewReader("payload"), "application/json", "job-input", now.Add(time.Hour), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceExternalizerUnavailable) || called {
		t.Fatalf("Externalize() error=%v called=%v", err, called)
	}
}

func TestResourceRegistryExternalizeBoundsInput(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	reader := &countingReader{remaining: 1024}
	called := false
	backend := &backendHarness{externalize: func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		called = true
		return nil, nil
	}}
	registry := newRegistry(t, fixedClock(now), backend, 4)
	_, err := registry.Externalize(context.Background(), "memory", reader, "application/json", "job-input", now.Add(time.Hour), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceTooLarge) || called || reader.readBytes != 5 {
		t.Fatalf("Externalize() error=%v called=%v bytes=%d", err, called, reader.readBytes)
	}
}

func TestResourceRegistryExternalizeRejectsUnsafeBackendReference(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	content := []byte("payload")
	cases := map[string]func(*agentv1.ResourceRef){
		"resolver": func(ref *agentv1.ResourceRef) { ref.ResolverId = "other" },
		"digest":   func(ref *agentv1.ResourceRef) { ref.Sha256[0] ^= 0xff },
		"type":     func(ref *agentv1.ResourceRef) { ref.MediaType = "text/plain" },
		"size":     func(ref *agentv1.ResourceRef) { ref.SizeBytes++ },
		"purpose":  func(ref *agentv1.ResourceRef) { ref.Purpose = "other" },
		"expiry":   func(ref *agentv1.ResourceRef) { ref.ExpiresAt.Seconds++ },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			backend := &backendHarness{externalize: func(request capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
				ref := refFromRequest("memory", "memory://objects/item", request)
				mutate(ref)
				return ref, nil
			}}
			registry := newRegistry(t, fixedClock(now), backend, 64)
			_, err := registry.Externalize(context.Background(), "memory", bytes.NewReader(content), "application/json", "job-input", now.Add(time.Hour), trustedContext())
			if !errors.Is(err, capsdk.ErrResourceMetadataMismatch) {
				t.Fatalf("Externalize() error = %v", err)
			}
		})
	}
}

func TestResourceRegistryExternalizeRejectsCredentialURI(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{externalize: func(request capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		return refFromRequest("memory", "memory://user:secret@objects/item", request), nil
	}}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	_, err := registry.Externalize(context.Background(), "memory", strings.NewReader("payload"), "application/json", "job-input", now.Add(time.Hour), trustedContext())
	if !errors.Is(err, capsdk.ErrInvalidResourceRef) {
		t.Fatalf("Externalize() error = %v", err)
	}
}

func TestResourceRegistryExternalizeBoundsBackendErrors(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	secret := strings.Repeat("secret", 1000)
	backend := &backendHarness{externalize: func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		return nil, errors.New(secret)
	}}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	_, err := registry.Externalize(context.Background(), "memory", strings.NewReader("payload"), "application/json", "job-input", now.Add(time.Hour), trustedContext())
	if !errors.Is(err, capsdk.ErrResourceBackendUnavailable) || strings.Contains(err.Error(), secret) || len(err.Error()) > 100 {
		t.Fatalf("Externalize() leaked backend error: %q", err)
	}
}

func TestResourceRegistryExternalizeRejectsCancellationAndExpiry(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	backend := &backendHarness{}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := registry.Externalize(ctx, "memory", strings.NewReader("payload"), "application/json", "job-input", now.Add(time.Hour), trustedContext())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Externalize() error = %v", err)
	}
	_, err = registry.Externalize(context.Background(), "memory", strings.NewReader("payload"), "application/json", "job-input", now, trustedContext())
	if !errors.Is(err, capsdk.ErrResourceExpired) {
		t.Fatalf("expired Externalize() error = %v", err)
	}
}

func TestResourceRegistryExternalizeRejectsCancellationAtEndOfRead(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	called := false
	backend := &backendHarness{externalize: func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		called = true
		return nil, nil
	}}
	registry := newRegistry(t, fixedClock(now), backend, 64)
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelOnReadCloser{Reader: bytes.NewReader([]byte("payload")), cancel: cancel}
	_, err := registry.Externalize(ctx, "memory", reader, "application/json", "job-input", now.Add(time.Hour), trustedContext())
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("Externalize() error=%v backend-called=%v", err, called)
	}
}

func TestResourceRegistryExternalizeRejectsExpiryAfterReadBeforeBackend(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	times := []time.Time{base, base.Add(2 * time.Second)}
	index := 0
	clock := func() time.Time { value := times[index]; index++; return value }
	called := false
	backend := &backendHarness{externalize: func(capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
		called = true
		return nil, nil
	}}
	registry := newRegistry(t, clock, backend, 64)
	_, err := registry.Externalize(
		context.Background(), "memory", strings.NewReader("payload"),
		"application/json", "job-input", base.Add(time.Second), trustedContext(),
	)
	if !errors.Is(err, capsdk.ErrResourceExpired) || called {
		t.Fatalf("Externalize() error=%v backend-called=%v", err, called)
	}
}

func TestResourceRegistryExternalizeRejectsExpiryDuringBackendCall(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	times := []time.Time{base, base, base.Add(2 * time.Second)}
	index := 0
	clock := func() time.Time { value := times[index]; index++; return value }
	backend := &backendHarness{}
	registry := newRegistry(t, clock, backend, 64)

	_, err := registry.Externalize(
		context.Background(), "memory", strings.NewReader("payload"),
		"application/json", "job-input", base.Add(time.Second), trustedContext(),
	)
	if !errors.Is(err, capsdk.ErrResourceExpired) {
		t.Fatalf("Externalize() error=%v", err)
	}
}
