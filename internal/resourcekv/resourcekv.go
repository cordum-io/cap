// Package resourcekv installs the nats-kv backend in capsdk.NewResourceRegistry.
//
// The URI is nats-kv://<bucket>/<tenant>/<sha256hex>, with the trusted tenant
// verbatim and a 64-character lowercase hexadecimal SHA-256 digest. The KV key
// is hex(tenant) + "." + sha256hex + "." + hex(media_type), using lowercase
// hexadecimal ASCII bytes. For example, tenant-a and application/json give
// 74656e616e742d61.<sha256hex>.6170706c69636174696f6e2f6a736f6e.
// The value is raw content, with no header or envelope. Media types have separate
// keys, so storing the same bytes with different media types never overwrites.
// cap-sdk-python's cap.resource_nats_kv must match this format byte for byte;
// testdata/golden-entry.json is the cross-language parity fixture.
//
// New requires bucket max_value_size in 1..capsdk.MaxResourceSizeBytes (1 GiB)
// and uses it as the registration's MaxBytes. Resolve checks the exact scheme,
// bucket, two path segments, tenant equality and lowercase digest, then derives
// the key from the trusted tenant. The registry validates trusted context, media
// type, bounded size, digest, and expiry before and after fetching. Externalize
// rechecks content size and digest before Put and refuses tenants containing '..'.
//
// Operators provision the bucket (History 1 suffices), set max_age to at least
// the longest reference lifetime, keep server max_payload at or above
// max_value_size, and keep NATS credentials local. Entries have no per-entry TTL.
// No packet supplies credentials or chooses a connection or bucket.
package resourcekv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"reflect"
	"strings"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const ResolverID = "nats-kv"

var (
	ErrInvalidBucket  = errors.New("resourcekv: invalid or unbounded bucket")
	ErrInvalidURI     = errors.New("resourcekv: invalid URI")
	ErrTenantMismatch = errors.New("resourcekv: tenant mismatch")
	ErrNotFound       = errors.New("resourcekv: entry not found")
)

// Backend is immutable after New; the supplied KV handle must support concurrent
// use, as jetstream's handles do. Callers must not mutate the bucket's bound.
type Backend struct {
	kv       jetstream.KeyValue
	bucket   string
	maxBytes uint64
}

// storageError preserves errors.Is/As without disclosing backend error text,
// which can contain a key, URI or content from a transport implementation.
type storageError struct{ cause error }

func (storageError) Error() string   { return "resourcekv: storage unavailable" }
func (e storageError) Unwrap() error { return e.cause }

// New validates the operator-provisioned bucket and snapshots its value bound.
func New(ctx context.Context, kv jetstream.KeyValue) (*Backend, error) {
	if nilKV(kv) {
		return nil, ErrInvalidBucket
	}
	if ctx == nil {
		return nil, capsdk.ErrResourceContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bucket := kv.Bucket()
	if bucket == "" || len(bucket) > capsdk.MaxResourceAuthorityBytes || strings.Trim(bucket, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
		return nil, ErrInvalidBucket
	}
	status, err := kv.Status(ctx)
	if err != nil {
		return nil, storageError{err}
	}
	bound := status.Config().MaxValueSize
	if bound <= 0 || uint64(bound) > capsdk.MaxResourceSizeBytes {
		return nil, ErrInvalidBucket
	}
	return &Backend{kv: kv, bucket: bucket, maxBytes: uint64(bound)}, nil
}

func nilKV(kv jetstream.KeyValue) bool {
	if kv == nil {
		return true
	}
	v := reflect.ValueOf(kv)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// Registration returns the paired resolver and externalizer for the registry.
func (b *Backend) Registration() capsdk.ResourceBackendRegistration {
	return capsdk.ResourceBackendRegistration{ID: ResolverID, MaxBytes: b.maxBytes, Resolver: b, Externalizer: b}
}

// Resolve fetches only from this bucket and the authenticated tenant's key.
func (b *Backend) Resolve(ctx context.Context, req capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
	if ctx == nil {
		return capsdk.ResourceResolveResult{}, capsdk.ErrResourceContextRequired
	}
	if err := ctx.Err(); err != nil {
		return capsdk.ResourceResolveResult{}, err
	}
	digest, err := b.uriDigest(req.URI, req.TrustedContext.TenantID)
	if err != nil {
		return capsdk.ResourceResolveResult{}, err
	}
	entry, err := b.kv.Get(ctx, entryKey(req.TrustedContext.TenantID, digest, req.MediaType))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return capsdk.ResourceResolveResult{}, ctxErr
	}
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return capsdk.ResourceResolveResult{}, ErrNotFound
	}
	if err != nil {
		return capsdk.ResourceResolveResult{}, storageError{err}
	}
	return capsdk.ResourceResolveResult{Body: io.NopCloser(bytes.NewReader(bytes.Clone(entry.Value()))), MediaType: req.MediaType}, nil
}

func (b *Backend) uriDigest(uri, tenant string) (string, error) {
	if !strings.HasPrefix(uri, "nats-kv://") {
		return "", ErrInvalidURI
	}
	parts := strings.Split(strings.TrimPrefix(uri, "nats-kv://"), "/")
	if len(parts) != 3 || parts[0] != b.bucket || parts[1] == "" {
		return "", ErrInvalidURI
	}
	if parts[1] != tenant {
		return "", ErrTenantMismatch
	}
	if len(parts[2]) != 64 || strings.Trim(parts[2], "0123456789abcdef") != "" {
		return "", ErrInvalidURI
	}
	return parts[2], nil
}

// Externalize writes content-addressed raw bytes and returns caller-owned metadata.
func (b *Backend) Externalize(ctx context.Context, req capsdk.ResourceExternalizeRequest) (*agentv1.ResourceRef, error) {
	if ctx == nil {
		return nil, capsdk.ErrResourceContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content := bytes.Clone(req.Content)
	if uint64(len(content)) != req.SizeBytes {
		return nil, capsdk.ErrResourceSizeMismatch
	}
	if sha256.Sum256(content) != req.SHA256 {
		return nil, capsdk.ErrResourceDigestMismatch
	}
	if req.SizeBytes > b.maxBytes {
		return nil, capsdk.ErrResourceTooLarge
	}
	if strings.Contains(req.TrustedContext.TenantID, "..") {
		return nil, ErrInvalidURI
	}
	digest := hex.EncodeToString(req.SHA256[:])
	_, err := b.kv.Put(ctx, entryKey(req.TrustedContext.TenantID, digest, req.MediaType), content)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, storageError{err}
	}
	return &agentv1.ResourceRef{
		ResolverId: ResolverID, Uri: "nats-kv://" + b.bucket + "/" + req.TrustedContext.TenantID + "/" + digest,
		Sha256: bytes.Clone(req.SHA256[:]), MediaType: req.MediaType, SizeBytes: req.SizeBytes,
		ExpiresAt: timestamppb.New(req.ExpiresAt), Purpose: req.Purpose,
	}, nil
}

func entryKey(tenant, digest, mediaType string) string {
	return hex.EncodeToString([]byte(tenant)) + "." + digest + "." + hex.EncodeToString([]byte(mediaType))
}
