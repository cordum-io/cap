package capsdk_test

import (
	"context"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

var resourceUnknownField = []byte{0xa0, 0x06, 0x01}

func TestResourceRegistryResolveRejectsUnknownFieldsBeforeBackend(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	for name, mutate := range resourceUnknownMutations() {
		t.Run(name, func(t *testing.T) {
			backend := &backendHarness{content: []byte("payload"), mediaType: "application/json"}
			registry := newRegistry(t, fixedClock(now), backend, 64)
			ref := validRef([]byte("payload"), now.Add(time.Hour))
			mutate(ref)

			_, err := registry.Resolve(context.Background(), ref, trustedContext())
			if !errors.Is(err, capsdk.ErrMalformedProductionWire) || backend.resolveCalls != 0 {
				t.Fatalf("Resolve() error=%v backend-calls=%d", err, backend.resolveCalls)
			}
		})
	}
}

func TestResourceRegistryExternalizeRejectsUnknownBackendFields(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	for name, mutate := range resourceUnknownMutations() {
		t.Run(name, func(t *testing.T) {
			backend := &backendHarness{externalize: func(request capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
				ref := refFromRequest("memory", "memory://objects/item", request)
				mutate(ref)
				return ref, nil
			}}
			registry := newRegistry(t, fixedClock(now), backend, 64)

			ref, err := registry.ExternalizeBytes(
				context.Background(), "memory", []byte("payload"),
				"application/json", "job-input", now.Add(time.Hour), trustedContext(),
			)
			if ref != nil || !errors.Is(err, capsdk.ErrMalformedProductionWire) {
				t.Fatalf("ExternalizeBytes() ref=%v error=%v", ref, err)
			}
		})
	}
}

func resourceUnknownMutations() map[string]func(*agentv1.ResourceRef) {
	return map[string]func(*agentv1.ResourceRef){
		"top-level": func(ref *agentv1.ResourceRef) {
			ref.ProtoReflect().SetUnknown(append([]byte(nil), resourceUnknownField...))
		},
		"nested-expiry": func(ref *agentv1.ResourceRef) {
			ref.GetExpiresAt().ProtoReflect().SetUnknown(append([]byte(nil), resourceUnknownField...))
		},
	}
}
