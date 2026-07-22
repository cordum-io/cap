package capsdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"reflect"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

// Resolve fetches through the exact installed resolver selected by ref, then
// rechecks expiry and verifies the declared media type, size, and SHA-256.
func (registry *ResourceRegistry) Resolve(
	ctx context.Context,
	ref *agentv1.ResourceRef,
	trusted TrustedResourceContext,
) (ResolvedResource, error) {
	if !registry.valid() {
		return ResolvedResource{}, ErrInvalidResourceRegistry
	}
	if ctx == nil {
		return ResolvedResource{}, ErrResourceContextRequired
	}
	if err := ctx.Err(); err != nil {
		return ResolvedResource{}, err
	}
	if err := validateTrustedResourceContext(trusted); err != nil {
		return ResolvedResource{}, err
	}
	snapshot, backend, expiresAt, err := registry.resolveSnapshot(ref)
	if err != nil {
		return ResolvedResource{}, err
	}
	result, err := backend.resolver.Resolve(ctx, resolveRequest(snapshot, trusted))
	if err != nil {
		closeResourceBody(result.Body)
		return ResolvedResource{}, collapseBackendError(ctx)
	}
	content, err := readAndCloseBounded(ctx, result.Body, snapshot.GetSizeBytes())
	if err != nil {
		return ResolvedResource{}, err
	}
	if err := ctx.Err(); err != nil {
		return ResolvedResource{}, err
	}
	if !expiresAt.After(registry.currentTime()) {
		return ResolvedResource{}, ErrResourceExpired
	}
	if err := verifyResolvedContent(snapshot, result.MediaType, content); err != nil {
		return ResolvedResource{}, err
	}
	return newResolvedResource(content, result.MediaType), nil
}

func (registry *ResourceRegistry) resolveSnapshot(
	ref *agentv1.ResourceRef,
) (*agentv1.ResourceRef, resourceBackend, time.Time, error) {
	if ref == nil {
		return nil, resourceBackend{}, time.Time{}, ErrInvalidResourceRef
	}
	snapshot := proto.Clone(ref).(*agentv1.ResourceRef)
	now := registry.currentTime()
	if err := validateGenericResourceRef(snapshot, now); err != nil {
		return nil, resourceBackend{}, time.Time{}, err
	}
	backend, installed := registry.backends[snapshot.GetResolverId()]
	if !installed {
		return nil, resourceBackend{}, time.Time{}, ErrResourceResolverUnavailable
	}
	if snapshot.GetSizeBytes() > backend.maxBytes {
		return nil, resourceBackend{}, time.Time{}, ErrResourceTooLarge
	}
	return snapshot, backend, snapshot.GetExpiresAt().AsTime(), nil
}

func validateGenericResourceRef(ref *agentv1.ResourceRef, now time.Time) error {
	installed := []string{ref.GetResolverId()}
	if err := ValidateResourceRefAt(ref, installed, now); err != nil {
		if resourceRefExpiredAt(ref, now) {
			return ErrResourceExpired
		}
		return err
	}
	return nil
}

func resourceRefExpiredAt(ref *agentv1.ResourceRef, now time.Time) bool {
	if ref == nil || ref.GetExpiresAt() == nil || ref.GetExpiresAt().CheckValid() != nil {
		return false
	}
	return !ref.GetExpiresAt().AsTime().After(now)
}

func resolveRequest(ref *agentv1.ResourceRef, trusted TrustedResourceContext) ResourceResolveRequest {
	return ResourceResolveRequest{
		TrustedContext: trusted, URI: ref.GetUri(), MediaType: ref.GetMediaType(),
		Purpose: ref.GetPurpose(), DeclaredSizeBytes: ref.GetSizeBytes(),
	}
}

func readAndCloseBounded(ctx context.Context, body io.ReadCloser, limit uint64) ([]byte, error) {
	if readCloserIsNil(body) {
		return nil, ErrResourceBackendUnavailable
	}
	content, readErr := readBounded(ctx, body, limit)
	closeErr := body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, ErrResourceBackendUnavailable
	}
	return content, nil
}

func readBounded(ctx context.Context, reader io.Reader, limit uint64) ([]byte, error) {
	if readerIsNil(reader) || limit == 0 {
		return nil, ErrResourceBackendUnavailable
	}
	var output bytes.Buffer
	buffer := make([]byte, 32*1024)
	for uint64(output.Len()) <= limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining := limit + 1 - uint64(output.Len())
		chunk := buffer
		if uint64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		count, err := reader.Read(chunk)
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		if count < 0 || count > len(chunk) {
			return nil, ErrResourceBackendUnavailable
		}
		if count > 0 {
			_, _ = output.Write(chunk[:count])
			if uint64(output.Len()) > limit {
				return nil, ErrResourceTooLarge
			}
		}
		if err == io.EOF {
			return output.Bytes(), nil
		}
		if err != nil || count == 0 {
			return nil, ErrResourceBackendUnavailable
		}
	}
	return nil, ErrResourceTooLarge
}

func verifyResolvedContent(ref *agentv1.ResourceRef, mediaType string, content []byte) error {
	if mediaType != ref.GetMediaType() {
		return ErrResourceMediaTypeMismatch
	}
	if uint64(len(content)) != ref.GetSizeBytes() {
		return ErrResourceSizeMismatch
	}
	digest := sha256.Sum256(content)
	if subtle.ConstantTimeCompare(digest[:], ref.GetSha256()) != 1 {
		return ErrResourceDigestMismatch
	}
	return nil
}

func collapseBackendError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrResourceBackendUnavailable
}

func closeResourceBody(body io.ReadCloser) {
	if !readCloserIsNil(body) {
		_ = body.Close()
	}
}

func readerIsNil(reader io.Reader) bool {
	if reader == nil {
		return true
	}
	return nilReflectValue(reflect.ValueOf(reader))
}

func readCloserIsNil(reader io.ReadCloser) bool {
	if reader == nil {
		return true
	}
	return nilReflectValue(reflect.ValueOf(reader))
}
