package capsdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Externalize reads content through a strict installed bound and asks the
// explicitly selected backend to store it. Every returned reference field is
// independently checked before a deep clone is returned.
func (registry *ResourceRegistry) Externalize(
	ctx context.Context,
	backendID string,
	reader io.Reader,
	mediaType string,
	purpose string,
	expiresAt time.Time,
	trusted TrustedResourceContext,
) (*agentv1.ResourceRef, error) {
	backend, now, err := registry.externalizeInputs(ctx, backendID, mediaType, purpose, expiresAt, trusted)
	if err != nil {
		return nil, err
	}
	content, err := readBounded(ctx, reader, backend.maxBytes)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, ErrResourceSizeMismatch
	}
	request := externalizeRequest(content, mediaType, purpose, expiresAt, trusted)
	ref, err := backend.externalizer.Externalize(ctx, request)
	if err != nil {
		return nil, collapseBackendError(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !expiresAt.After(registry.currentTime()) {
		return nil, ErrResourceExpired
	}
	return registry.validateExternalizedRef(ref, backendID, request, now)
}

// ExternalizeBytes is a convenience wrapper over Externalize.
func (registry *ResourceRegistry) ExternalizeBytes(
	ctx context.Context,
	backendID string,
	content []byte,
	mediaType string,
	purpose string,
	expiresAt time.Time,
	trusted TrustedResourceContext,
) (*agentv1.ResourceRef, error) {
	return registry.Externalize(ctx, backendID, bytes.NewReader(content), mediaType, purpose, expiresAt, trusted)
}

func (registry *ResourceRegistry) externalizeInputs(
	ctx context.Context,
	backendID string,
	mediaType string,
	purpose string,
	expiresAt time.Time,
	trusted TrustedResourceContext,
) (resourceBackend, time.Time, error) {
	if !registry.valid() {
		return resourceBackend{}, time.Time{}, ErrInvalidResourceRegistry
	}
	if ctx == nil {
		return resourceBackend{}, time.Time{}, ErrInvalidTrustedResourceContext
	}
	if err := ctx.Err(); err != nil {
		return resourceBackend{}, time.Time{}, err
	}
	if err := validateTrustedResourceContext(trusted); err != nil {
		return resourceBackend{}, time.Time{}, err
	}
	if !validResourceIdentifier(backendID, MaxResourceIdentifierBytes, resourceIDPattern) {
		return resourceBackend{}, time.Time{}, ErrResourceExternalizerUnavailable
	}
	backend, installed := registry.backends[backendID]
	if !installed {
		return resourceBackend{}, time.Time{}, ErrResourceExternalizerUnavailable
	}
	if !validResourceIdentifier(mediaType, MaxResourceMediaTypeBytes, resourceMediaPattern) ||
		!validResourceIdentifier(purpose, MaxResourcePurposeBytes, resourcePurposePattern) {
		return resourceBackend{}, time.Time{}, ErrInvalidResourceRef
	}
	expiresAt = expiresAt.UTC()
	timestamp := timestamppb.New(expiresAt)
	now := registry.currentTime()
	if timestamp.CheckValid() != nil {
		return resourceBackend{}, time.Time{}, ErrInvalidResourceRef
	}
	if !expiresAt.After(now) {
		return resourceBackend{}, time.Time{}, ErrResourceExpired
	}
	return backend, now, nil
}

func externalizeRequest(
	content []byte,
	mediaType string,
	purpose string,
	expiresAt time.Time,
	trusted TrustedResourceContext,
) ResourceExternalizeRequest {
	digest := sha256.Sum256(content)
	return ResourceExternalizeRequest{
		TrustedContext: trusted, Content: append([]byte(nil), content...),
		MediaType: mediaType, Purpose: purpose, ExpiresAt: expiresAt.UTC(),
		SizeBytes: uint64(len(content)), SHA256: digest,
	}
}

func (registry *ResourceRegistry) validateExternalizedRef(
	ref *agentv1.ResourceRef,
	backendID string,
	request ResourceExternalizeRequest,
	validatedAt time.Time,
) (*agentv1.ResourceRef, error) {
	if ref == nil {
		return nil, ErrResourceMetadataMismatch
	}
	snapshot := proto.Clone(ref).(*agentv1.ResourceRef)
	if !externalizedMetadataMatches(snapshot, backendID, request) {
		return nil, ErrResourceMetadataMismatch
	}
	if err := ValidateResourceRefAt(snapshot, registry.ids, validatedAt); err != nil {
		return nil, err
	}
	return proto.Clone(snapshot).(*agentv1.ResourceRef), nil
}

func externalizedMetadataMatches(
	ref *agentv1.ResourceRef,
	backendID string,
	request ResourceExternalizeRequest,
) bool {
	if ref.GetResolverId() != backendID || ref.GetMediaType() != request.MediaType {
		return false
	}
	if ref.GetPurpose() != request.Purpose || ref.GetSizeBytes() != request.SizeBytes {
		return false
	}
	if subtle.ConstantTimeCompare(ref.GetSha256(), request.SHA256[:]) != 1 {
		return false
	}
	if ref.GetExpiresAt() == nil || ref.GetExpiresAt().CheckValid() != nil {
		return false
	}
	return ref.GetExpiresAt().AsTime().Equal(request.ExpiresAt)
}
