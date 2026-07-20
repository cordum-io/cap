package capsdk

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

const (
	MaxResourceIdentifierBytes = 128
	MaxResourceAuthorityBytes  = 255
	MaxResourceURIBytes        = 2048
	MaxResourceMediaTypeBytes  = 127
	MaxResourcePurposeBytes    = 128
	MaxResourceSizeBytes       = uint64(1 << 30)
	MaxLegacyRedisKeyBytes     = 1024
)

var (
	ErrInvalidResourceRef  = errors.New("capsdk: invalid production resource reference")
	resourceIDPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	resourcePurposePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]*$`)
	resourceMediaPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9!#$&^_.+-]*/[a-z0-9][a-z0-9!#$&^_.+-]*$`)
	resourceURIPattern     = regexp.MustCompile(`^([a-z][a-z0-9+.-]{0,31})://(.+)$`)
	legacyRedisKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._@\-\[\]]*$`)
)

// ValidateResourceRef validates a production reference at the current time.
func ValidateResourceRef(ref *agentv1.ResourceRef, installedResolvers []string) error {
	return ValidateResourceRefAt(ref, installedResolvers, time.Now().UTC())
}

// ValidateResourceResolvers rejects an empty, duplicate, or noncanonical
// production resolver allowlist.
func ValidateResourceResolvers(installedResolvers []string) error {
	if !validInstalledResolvers(installedResolvers) {
		return resourceError("resolver configuration is empty or noncanonical")
	}
	seen := make(map[string]struct{}, len(installedResolvers))
	for _, resolverID := range installedResolvers {
		if _, exists := seen[resolverID]; exists {
			return resourceError("resolver configuration contains duplicates")
		}
		seen[resolverID] = struct{}{}
	}
	return nil
}

// ValidateResourceRefAt validates all generic CAP-PRODUCTION reference bounds.
// Resolver-specific scheme, authority, tenant, media, and size allowlists remain
// mandatory at the operator-installed resolver boundary.
func ValidateResourceRefAt(ref *agentv1.ResourceRef, installedResolvers []string, now time.Time) error {
	if ref == nil {
		return resourceError("missing reference")
	}
	if err := ValidateResourceResolvers(installedResolvers); err != nil {
		return err
	}
	if err := validateResourceIdentifiers(ref, installedResolvers); err != nil {
		return err
	}
	if len(ref.GetSha256()) != 32 {
		return resourceError("SHA-256 must contain exactly 32 bytes")
	}
	if ref.GetSizeBytes() == 0 || ref.GetSizeBytes() > MaxResourceSizeBytes {
		return resourceError("declared size is outside production bounds")
	}
	if err := validateResourceURI(ref.GetUri()); err != nil {
		return err
	}
	return validateResourceExpiry(ref, now)
}

func validInstalledResolvers(installed []string) bool {
	if len(installed) == 0 {
		return false
	}
	for _, resolverID := range installed {
		if !validResourceIdentifier(resolverID, MaxResourceIdentifierBytes, resourceIDPattern) {
			return false
		}
	}
	return true
}

func validateResourceIdentifiers(ref *agentv1.ResourceRef, installed []string) error {
	if !validResourceIdentifier(ref.GetResolverId(), MaxResourceIdentifierBytes, resourceIDPattern) {
		return resourceError("resolver ID is not canonical")
	}
	if !contains(installed, ref.GetResolverId()) {
		return resourceError("resolver is not installed")
	}
	if !validResourceIdentifier(ref.GetMediaType(), MaxResourceMediaTypeBytes, resourceMediaPattern) {
		return resourceError("media type is not canonical")
	}
	if !validResourceIdentifier(ref.GetPurpose(), MaxResourcePurposeBytes, resourcePurposePattern) {
		return resourceError("purpose is not canonical")
	}
	return nil
}

func validResourceIdentifier(value string, limit int, pattern *regexp.Regexp) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value && pattern.MatchString(value)
}

func validateResourceURI(uri string) error {
	if uri == "" || len(uri) > MaxResourceURIBytes || strings.TrimSpace(uri) != uri || !printableASCII(uri) {
		return resourceError("URI is empty, non-ASCII, untrimmed, or too long")
	}
	match := resourceURIPattern.FindStringSubmatch(uri)
	if len(match) != 3 || match[2] == "" {
		return resourceError("URI scheme or authority is not canonical")
	}
	if strings.ContainsAny(match[2], "@?#\\%") {
		return resourceError("URI contains credentials, metadata, or escapes")
	}
	if err := validateResourcePath(match[2]); err != nil {
		return err
	}
	return nil
}

func validateResourcePath(authorityAndPath string) error {
	authority, pathValue, hasPath := strings.Cut(authorityAndPath, "/")
	if authority == "" || len(authority) > MaxResourceAuthorityBytes {
		return resourceError("URI authority is empty or too long")
	}
	if !hasPath {
		return nil
	}
	for _, segment := range strings.Split(pathValue, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.Contains(segment, "..") {
			return resourceError("URI path is not normalized")
		}
	}
	return nil
}

func validateResourceExpiry(ref *agentv1.ResourceRef, now time.Time) error {
	expiresAt := ref.GetExpiresAt()
	if expiresAt == nil {
		return resourceError("expiry is required")
	}
	if err := expiresAt.CheckValid(); err != nil {
		return resourceError("expiry timestamp is invalid")
	}
	if !expiresAt.AsTime().After(now) {
		return resourceError("reference is expired")
	}
	return nil
}

func printableASCII(value string) bool {
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

// CanonicalLegacyRedisKey derives the exact Redis key bytes from the narrow
// migration pointer grammar. It never uses a URL parser or normalization.
func CanonicalLegacyRedisKey(pointer string) ([]byte, error) {
	if len(pointer) > len("redis://")+MaxLegacyRedisKeyBytes || strings.TrimSpace(pointer) != pointer {
		return nil, resourceError("legacy Redis pointer is untrimmed or too long")
	}
	if !strings.HasPrefix(pointer, "redis://") {
		return nil, resourceError("legacy Redis pointer has the wrong scheme")
	}
	key := strings.TrimPrefix(pointer, "redis://")
	if !legacyRedisKeyPattern.MatchString(key) || strings.Contains(key, "..") || hasLegacyUserinfo(key) {
		return nil, resourceError("legacy Redis key is empty or ambiguous")
	}
	return []byte(key), nil
}

func hasLegacyUserinfo(key string) bool {
	at, colon := strings.IndexByte(key, '@'), strings.IndexByte(key, ':')
	return at >= 0 && (colon < 0 || at < colon)
}

// ValidateResourceRefCompatibility rejects ambiguity when migration code
// receives both a legacy Redis pointer and a structured reference.
func ValidateResourceRefCompatibility(legacy string, ref *agentv1.ResourceRef) error {
	if legacy == "" || ref == nil {
		return nil
	}
	if ref.GetResolverId() != "redis" {
		return resourceError("legacy Redis pointer uses a different resolver")
	}
	legacyKey, err := CanonicalLegacyRedisKey(legacy)
	if err != nil {
		return err
	}
	structuredKey, err := CanonicalLegacyRedisKey(ref.GetUri())
	if err != nil || !bytes.Equal(legacyKey, structuredKey) {
		return resourceError("legacy and structured references differ")
	}
	return nil
}

func resourceError(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidResourceRef, detail)
}
