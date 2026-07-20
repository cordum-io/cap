package capsdk

import (
	"bytes"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestValidateResourceRefAtAcceptsBoundedCanonicalReference(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	ref := validResourceRef(now)
	if err := ValidateResourceRefAt(ref, []string{"redis"}, now); err != nil {
		t.Fatalf("ValidateResourceRefAt() error = %v", err)
	}
}

func TestValidateResourceRefAtRejectsInvalidFields(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	cases := map[string]func(*agentv1.ResourceRef){
		"empty resolver":       func(r *agentv1.ResourceRef) { r.ResolverId = "" },
		"trimmed resolver":     func(r *agentv1.ResourceRef) { r.ResolverId = " redis" },
		"invalid resolver":     func(r *agentv1.ResourceRef) { r.ResolverId = "redis/primary" },
		"resolver too long":    func(r *agentv1.ResourceRef) { r.ResolverId = strings.Repeat("r", MaxResourceIdentifierBytes+1) },
		"short digest":         func(r *agentv1.ResourceRef) { r.Sha256 = bytes.Repeat([]byte{1}, 31) },
		"long digest":          func(r *agentv1.ResourceRef) { r.Sha256 = bytes.Repeat([]byte{1}, 33) },
		"zero size":            func(r *agentv1.ResourceRef) { r.SizeBytes = 0 },
		"oversize":             func(r *agentv1.ResourceRef) { r.SizeBytes = MaxResourceSizeBytes + 1 },
		"empty media type":     func(r *agentv1.ResourceRef) { r.MediaType = "" },
		"trimmed media type":   func(r *agentv1.ResourceRef) { r.MediaType = " application/json" },
		"noncanonical media":   func(r *agentv1.ResourceRef) { r.MediaType = "Application/JSON" },
		"media parameters":     func(r *agentv1.ResourceRef) { r.MediaType = "application/json; charset=utf-8" },
		"empty purpose":        func(r *agentv1.ResourceRef) { r.Purpose = "" },
		"trimmed purpose":      func(r *agentv1.ResourceRef) { r.Purpose = " input" },
		"invalid purpose":      func(r *agentv1.ResourceRef) { r.Purpose = "job input" },
		"missing expiry":       func(r *agentv1.ResourceRef) { r.ExpiresAt = nil },
		"invalid expiry":       func(r *agentv1.ResourceRef) { r.ExpiresAt = &timestamppb.Timestamp{Seconds: 1, Nanos: -1} },
		"expired":              func(r *agentv1.ResourceRef) { r.ExpiresAt = timestamppb.New(now.Add(-time.Nanosecond)) },
		"expiry at validation": func(r *agentv1.ResourceRef) { r.ExpiresAt = timestamppb.New(now) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			ref := validResourceRef(now)
			mutate(ref)
			if err := ValidateResourceRefAt(ref, []string{"redis"}, now); err == nil {
				t.Fatal("ValidateResourceRefAt() error = nil")
			}
		})
	}
}

func TestValidateResourceRefAtRejectsUninstalledResolverExactly(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	ref := validResourceRef(now)
	for _, installed := range [][]string{nil, {}, {"Redis"}, {"redis "}, {"other"}, {"redis", " invalid"}} {
		if err := ValidateResourceRefAt(ref, installed, now); err == nil {
			t.Fatalf("ValidateResourceRefAt(installed=%q) error = nil", installed)
		}
	}
}

func TestValidateResourceRefAtRejectsUnsafeURI(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	unsafe := []string{
		"", " redis://ctx:job-1", "redis://ctx:job-1 ",
		"redis:ctx:job-1", "Redis://ctx:job-1", "1redis://ctx:job-1",
		"redis://", "redis:///job", "redis://" + strings.Repeat("a", MaxResourceAuthorityBytes+1),
		"redis://user:secret@ctx/job", "redis://ctx/job?token=secret",
		"redis://ctx/job#secret", "redis://ctx/../secret", "redis://ctx/%2e%2e/secret",
		"redis://ctx/%252e%252e/secret", "redis://ctx/a\\b", "redis://ctx/%00",
		"redis://ctx/", "redis://ctx//job", "redis://ctx/./job", "redis://ctx/%2Fjob",
		"redis://ctx/" + strings.Repeat("a", MaxResourceURIBytes),
	}
	for _, uri := range unsafe {
		t.Run(uri, func(t *testing.T) {
			ref := validResourceRef(now)
			ref.Uri = uri
			if err := ValidateResourceRefAt(ref, []string{"redis"}, now); err == nil {
				t.Fatal("ValidateResourceRefAt() error = nil")
			}
		})
	}
}

func TestCanonicalLegacyRedisKeyReturnsExactBytes(t *testing.T) {
	for _, key := range []string{"res:job-1[2]", "res:run:loop[0]@2"} {
		got, err := CanonicalLegacyRedisKey("redis://" + key)
		if err != nil {
			t.Fatalf("CanonicalLegacyRedisKey(%q) error = %v", key, err)
		}
		if !bytes.Equal(got, []byte(key)) {
			t.Fatalf("CanonicalLegacyRedisKey() = %q, want %q", got, key)
		}
	}
}

func TestCanonicalLegacyRedisKeyRejectsAmbiguousPointers(t *testing.T) {
	invalid := []string{
		"", "Redis://res:job", "redis://", " redis://res:job", "redis://res:job ",
		"redis://res/job", "redis://res\\job", "redis://res..job", "redis://res%3Ajob",
		"redis://user@res:job", "redis://res:job?token=x", "redis://res:job#part",
		"redis://res:\x00job", "redis://" + strings.Repeat("k", MaxLegacyRedisKeyBytes+1),
	}
	for _, pointer := range invalid {
		if _, err := CanonicalLegacyRedisKey(pointer); err == nil {
			t.Errorf("CanonicalLegacyRedisKey(%q) error = nil", pointer)
		}
	}
}

func TestValidateResourceRefCompatibilityRequiresSameRedisKey(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	ref := validResourceRef(now)
	if err := ValidateResourceRefCompatibility("redis://ctx:job-1", ref); err != nil {
		t.Fatalf("matching reference error = %v", err)
	}
	for name, tc := range rangeCompatibilityCases() {
		t.Run(name, func(t *testing.T) {
			candidate := validResourceRef(now)
			tc.mutate(candidate)
			if err := ValidateResourceRefCompatibility(tc.legacy, candidate); err == nil {
				t.Fatal("ValidateResourceRefCompatibility() error = nil")
			}
		})
	}
	if err := ValidateResourceRefCompatibility("", ref); err != nil {
		t.Fatalf("single structured field error = %v", err)
	}
	if err := ValidateResourceRefCompatibility("redis://ctx:job-1", nil); err != nil {
		t.Fatalf("single legacy field error = %v", err)
	}
}

func rangeCompatibilityCases() map[string]struct {
	legacy string
	mutate func(*agentv1.ResourceRef)
} {
	return map[string]struct {
		legacy string
		mutate func(*agentv1.ResourceRef)
	}{
		"different key":     {"redis://ctx:other", func(*agentv1.ResourceRef) {}},
		"other resolver":    {"redis://ctx:job-1", func(r *agentv1.ResourceRef) { r.ResolverId = "blob" }},
		"unsafe legacy":     {"redis://ctx/../job-1", func(*agentv1.ResourceRef) {}},
		"unsafe structured": {"redis://ctx:job-1", func(r *agentv1.ResourceRef) { r.Uri = "redis://ctx:%6aob-1" }},
	}
}

func validResourceRef(now time.Time) *agentv1.ResourceRef {
	return &agentv1.ResourceRef{
		ResolverId: "redis", Uri: "redis://ctx:job-1", Sha256: bytes.Repeat([]byte{1}, 32),
		MediaType: "application/json", SizeBytes: 128, ExpiresAt: timestamppb.New(now.Add(time.Hour)),
		Purpose: "job-input",
	}
}
