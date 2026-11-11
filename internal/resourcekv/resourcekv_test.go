package resourcekv_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/internal/resourcekv"
	"github.com/cordum-io/cap/v2/internal/tck"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

var update = flag.Bool("update", false, "regenerate the cross-language golden entry")

const bucket = "cap-resources"
const media = "application/json"

type fixture struct {
	ctx      context.Context
	js       jetstream.JetStream
	kv       jetstream.KeyValue
	backend  *resourcekv.Backend
	registry *capsdk.ResourceRegistry
	now      time.Time
	expires  time.Time
	trusted  capsdk.TrustedResourceContext
}

func startKV(t *testing.T) *fixture {
	t.Helper()
	server, err := tck.StartEmbeddedNATS()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL())
	if err != nil || u.Hostname() != "127.0.0.1" {
		t.Fatalf("not loopback: %v", u)
	}
	nc, err := nats.Connect(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket, MaxValueSize: 65536, History: 1, Storage: jetstream.MemoryStorage})
	if err != nil {
		t.Fatal(err)
	}
	b, err := resourcekv.New(ctx, kv)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{ctx: ctx, js: js, kv: kv, backend: b, now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), expires: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), trusted: capsdk.TrustedResourceContext{TenantID: "tenant-a", JobID: "job-1"}}
	f.registry = newRegistry(t, f, b.Registration())
	return f
}

func newRegistry(t *testing.T, f *fixture, registration capsdk.ResourceBackendRegistration) *capsdk.ResourceRegistry {
	t.Helper()
	r, err := capsdk.NewResourceRegistry(func() time.Time { return f.now }, registration)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func expectedKey(tenant string, digest []byte, mt string) string {
	return hex.EncodeToString([]byte(tenant)) + "." + hex.EncodeToString(digest) + "." + hex.EncodeToString([]byte(mt))
}

func (f *fixture) store(t *testing.T, content []byte, mt string) *agentv1.ResourceRef {
	t.Helper()
	ref, err := f.registry.ExternalizeBytes(f.ctx, "nats-kv", content, mt, "job-input", f.expires, f.trusted)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func keys(t *testing.T, f *fixture) []string {
	t.Helper()
	ks, err := f.kv.Keys(f.ctx)
	if errors.Is(err, jetstream.ErrNoKeysFound) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return ks
}

func assertResolved(t *testing.T, f *fixture, ref *agentv1.ResourceRef, content []byte) {
	t.Helper()
	got, err := f.registry.Resolve(f.ctx, ref, f.trusted)
	if err != nil || !bytes.Equal(got.Content(), content) || got.MediaType() != ref.MediaType {
		t.Fatalf("resolve: %v, %q, %q", err, got.Content(), got.MediaType())
	}
}

func TestNewRequiresBoundedBucket(t *testing.T) {
	f := startKV(t)
	if _, err := resourcekv.New(f.ctx, nil); !errors.Is(err, resourcekv.ErrInvalidBucket) {
		t.Fatal(err)
	}
	kv, err := f.js.CreateKeyValue(f.ctx, jetstream.KeyValueConfig{Bucket: "unbounded", Storage: jetstream.MemoryStorage})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = resourcekv.New(f.ctx, kv); !errors.Is(err, resourcekv.ErrInvalidBucket) {
		t.Fatalf("unbounded: %v", err)
	}
	r := f.backend.Registration()
	if r.ID != "nats-kv" || r.MaxBytes != 65536 || r.Resolver != f.backend || r.Externalizer != f.backend {
		t.Fatalf("registration: %+v", r)
	}
	newRegistry(t, f, r)
}

func TestRegistryRoundTrip(t *testing.T) {
	for _, size := range []int{16, 65536, 65537} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			f := startKV(t)
			content := bytes.Repeat([]byte("x"), size)
			ref, err := f.registry.ExternalizeBytes(f.ctx, "nats-kv", content, media, "job-input", f.expires, f.trusted)
			if size > 65536 {
				if !errors.Is(err, capsdk.ErrResourceTooLarge) || ref != nil || len(keys(t, f)) != 0 {
					t.Fatalf("oversize: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(content)
			if ref.ResolverId != "nats-kv" || ref.Uri != "nats-kv://"+bucket+"/tenant-a/"+hex.EncodeToString(digest[:]) || !bytes.Equal(ref.Sha256, digest[:]) || ref.SizeBytes != uint64(size) || ref.MediaType != media || ref.Purpose != "job-input" || !ref.ExpiresAt.AsTime().Equal(f.expires) {
				t.Fatalf("ref: %v", ref)
			}
			ks := keys(t, f)
			if len(ks) != 1 || ks[0] != expectedKey("tenant-a", digest[:], media) {
				t.Fatalf("keys: %v", ks)
			}
			entry, err := f.kv.Get(f.ctx, ks[0])
			if err != nil || !bytes.Equal(entry.Value(), content) {
				t.Fatalf("stored bytes: %v", err)
			}
			assertResolved(t, f, ref, content)
		})
	}
}

type resolverFunc func(context.Context, capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error)

func (fn resolverFunc) Resolve(ctx context.Context, req capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
	return fn(ctx, req)
}

type rejectCase struct {
	name   string
	want   error
	modify func(*testing.T, *fixture, *agentv1.ResourceRef)
}

func rejectCases() []rejectCase {
	return []rejectCase{
		{"tenant_mismatch", capsdk.ErrResourceBackendUnavailable, func(t *testing.T, f *fixture, r *agentv1.ResourceRef) {
			f.trusted.TenantID = "tenant-b"
			f.store(t, []byte("payload"), media)
			if len(keys(t, f)) != 2 {
				t.Fatal("both tenant entries required")
			}
		}},
		{"tampered_value", capsdk.ErrResourceDigestMismatch, overwrite("payloae")},
		{"oversize_value", capsdk.ErrResourceTooLarge, overwrite("payloadx")},
		{"short_value", capsdk.ErrResourceSizeMismatch, overwrite("payloa")},
		{"expired_before_fetch", capsdk.ErrResourceExpired, func(_ *testing.T, f *fixture, _ *agentv1.ResourceRef) { f.now = f.expires }},
		{"expired_during_fetch", capsdk.ErrResourceExpired, func(t *testing.T, f *fixture, _ *agentv1.ResourceRef) {
			reg := f.backend.Registration()
			reg.Resolver = resolverFunc(func(ctx context.Context, req capsdk.ResourceResolveRequest) (capsdk.ResourceResolveResult, error) {
				res, err := f.backend.Resolve(ctx, req)
				f.now = f.expires
				return res, err
			})
			f.registry = newRegistry(t, f, reg)
		}},
		{"missing_entry", capsdk.ErrResourceBackendUnavailable, func(_ *testing.T, _ *fixture, r *agentv1.ResourceRef) {
			d := sha256.Sum256([]byte("missing"))
			r.Sha256 = d[:]
			r.Uri = "nats-kv://" + bucket + "/tenant-a/" + hex.EncodeToString(d[:])
		}},
		{"canceled_context", context.Canceled, func(_ *testing.T, f *fixture, _ *agentv1.ResourceRef) {
			ctx, cancel := context.WithCancel(f.ctx)
			cancel()
			f.ctx = ctx
		}},
	}
}

func overwrite(value string) func(*testing.T, *fixture, *agentv1.ResourceRef) {
	return func(t *testing.T, f *fixture, r *agentv1.ResourceRef) {
		t.Helper()
		if _, err := f.kv.Put(f.ctx, expectedKey("tenant-a", r.Sha256, media), []byte(value)); err != nil {
			t.Fatal(err)
		}
	}
}

func uriCases() []rejectCase {
	d := sha256.Sum256([]byte("payload"))
	h := hex.EncodeToString(d[:])
	prefix := "nats-kv://" + bucket + "/tenant-a/"
	bad := map[string]string{
		"unknown_bucket":               "nats-kv://other-bucket/tenant-a/" + h,
		"malformed_uri/missing_digest": "nats-kv://" + bucket + "/tenant-a",
		"malformed_uri/uppercase_hex":  prefix + strings.ToUpper(h),
		"malformed_uri/short_hex":      prefix + h[:63],
		"malformed_uri/extra_segment":  prefix + h + "/x",
		"malformed_uri/wrong_scheme":   "nats://" + bucket + "/tenant-a/" + h,
		"malformed_uri/traversal":      "nats-kv://" + bucket + "/../" + h,
	}
	var cases []rejectCase
	for name, uri := range bad {
		want := capsdk.ErrResourceBackendUnavailable
		if name == "malformed_uri/traversal" {
			want = capsdk.ErrInvalidResourceRef
		}
		cases = append(cases, rejectCase{name, want, func(t *testing.T, f *fixture, r *agentv1.ResourceRef) {
			r.Uri = uri
			if name == "malformed_uri/uppercase_hex" {
				key := hex.EncodeToString([]byte("tenant-a")) + "." + strings.ToUpper(h) + "." + hex.EncodeToString([]byte(media))
				if _, err := f.kv.Put(f.ctx, key, []byte("payload")); err != nil {
					t.Fatal(err)
				}
			}
		}})
	}
	return cases
}

func TestResolveRejects(t *testing.T) {
	for _, tc := range append(rejectCases(), uriCases()...) {
		t.Run(tc.name, func(t *testing.T) {
			f := startKV(t)
			ref := f.store(t, []byte("payload"), media)
			tc.modify(t, f, ref)
			got, err := f.registry.Resolve(f.ctx, ref, f.trusted)
			if !errors.Is(err, tc.want) || len(got.Content()) != 0 {
				t.Fatalf("got %v, %d bytes; want %v and no bytes", err, len(got.Content()), tc.want)
			}
		})
	}
}

func directRequest(f *fixture, ref *agentv1.ResourceRef) capsdk.ResourceResolveRequest {
	return capsdk.ResourceResolveRequest{TrustedContext: f.trusted, URI: ref.Uri, MediaType: ref.MediaType, Purpose: ref.Purpose, DeclaredSizeBytes: ref.SizeBytes}
}

func externalizeRequest(f *fixture) capsdk.ResourceExternalizeRequest {
	content := []byte("payload")
	return capsdk.ResourceExternalizeRequest{TrustedContext: f.trusted, Content: content, SHA256: sha256.Sum256(content), SizeBytes: uint64(len(content)), MediaType: media, Purpose: "job-input", ExpiresAt: f.expires}
}

func TestBackendSentinels(t *testing.T) {
	f := startKV(t)
	ref := f.store(t, []byte("payload"), media)
	for _, tc := range []struct {
		name   string
		want   error
		modify func(*capsdk.ResourceResolveRequest)
	}{
		{"tenant", resourcekv.ErrTenantMismatch, func(r *capsdk.ResourceResolveRequest) { r.TrustedContext.TenantID = "tenant-b" }},
		{"bucket", resourcekv.ErrInvalidURI, func(r *capsdk.ResourceResolveRequest) { r.URI = strings.Replace(r.URI, bucket, "other", 1) }},
		{"grammar", resourcekv.ErrInvalidURI, func(r *capsdk.ResourceResolveRequest) { r.URI += "/x" }},
		{"missing", resourcekv.ErrNotFound, func(r *capsdk.ResourceResolveRequest) {
			r.URI = "nats-kv://" + bucket + "/tenant-a/" + strings.Repeat("0", 64)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := directRequest(f, ref)
			tc.modify(&req)
			res, err := f.backend.Resolve(f.ctx, req)
			if !errors.Is(err, tc.want) || res.Body != nil {
				t.Fatalf("%v: %+v", err, res)
			}
		})
	}
	for _, name := range []string{"tenant_dots", "size", "digest"} {
		t.Run(name, func(t *testing.T) {
			req := externalizeRequest(f)
			switch name {
			case "tenant_dots":
				req.TrustedContext.TenantID = "a..b"
			case "size":
				req.SizeBytes++
			case "digest":
				req.SHA256[0] ^= 1
			}
			before := len(keys(t, f))
			got, err := f.backend.Externalize(f.ctx, req)
			if err == nil || got != nil || len(keys(t, f)) != before {
				t.Fatalf("invalid write: %v %v", got, err)
			}
			if name == "tenant_dots" && !errors.Is(err, resourcekv.ErrInvalidURI) {
				t.Fatal(err)
			}
		})
	}
	if err := f.kv.Delete(f.ctx, expectedKey("tenant-a", ref.Sha256, media)); err != nil {
		t.Fatal(err)
	}
	res, err := f.backend.Resolve(f.ctx, directRequest(f, ref))
	if !errors.Is(err, resourcekv.ErrNotFound) || res.Body != nil {
		t.Fatalf("deleted: %v", err)
	}
}

func TestMediaTypesAreSeparateEntries(t *testing.T) {
	f := startKV(t)
	content := []byte("payload")
	a := f.store(t, content, "text/plain")
	b := f.store(t, content, media)
	if len(keys(t, f)) != 2 {
		t.Fatal("media types must have separate keys")
	}
	assertResolved(t, f, a, content)
	assertResolved(t, f, b, content)
}

func TestConcurrentExternalize(t *testing.T) {
	f := startKV(t)
	content := []byte("payload")
	refs := make([]*agentv1.ResourceRef, 8)
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range refs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			refs[i], errs[i] = f.registry.ExternalizeBytes(f.ctx, "nats-kv", content, media, "job-input", f.expires, f.trusted)
		}()
	}
	wg.Wait()
	for i := range refs {
		if errs[i] != nil || !proto.Equal(refs[0], refs[i]) {
			t.Fatalf("writer %d: %v", i, errs[i])
		}
	}
	if len(keys(t, f)) != 1 {
		t.Fatal("expected one key")
	}
	assertResolved(t, f, refs[0], content)
}

type goldenRef struct {
	ResolverID string `json:"resolver_id"`
	URI        string `json:"uri"`
	SHA256Hex  string `json:"sha256_hex"`
	MediaType  string `json:"media_type"`
	SizeBytes  uint64 `json:"size_bytes"`
	ExpiresAt  string `json:"expires_at"`
	Purpose    string `json:"purpose"`
}
type goldenEntry struct {
	Bucket      string    `json:"bucket"`
	TenantID    string    `json:"tenant_id"`
	JobID       string    `json:"job_id"`
	Key         string    `json:"key"`
	ValueBase64 string    `json:"value_base64"`
	Ref         goldenRef `json:"ref"`
}

func TestGoldenEntry(t *testing.T) {
	f := startKV(t)
	content := []byte(`{"cap":"golden"}`)
	ref := f.store(t, content, media)
	ks := keys(t, f)
	if len(ks) != 1 || ks[0] != expectedKey("tenant-a", ref.Sha256, media) {
		t.Fatalf("keys: %v", ks)
	}
	entry, err := f.kv.Get(f.ctx, ks[0])
	if err != nil {
		t.Fatal(err)
	}
	v := goldenEntry{bucket, "tenant-a", "job-1", ks[0], base64.StdEncoding.EncodeToString(entry.Value()), goldenRef{ref.ResolverId, ref.Uri, hex.EncodeToString(ref.Sha256), ref.MediaType, ref.SizeBytes, ref.ExpiresAt.AsTime().Format(time.RFC3339), ref.Purpose}}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := "testdata/golden-entry.json"
	if *update {
		if err = os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("golden mismatch\ngot:\n%s\nwant:\n%s", data, want)
	}
	assertResolved(t, f, ref, content)
}

// Keep io in the acceptance file for the direct-body ownership tests below.
var _ io.Reader
