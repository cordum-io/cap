package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

const testAPIKey = "bridge-test-key"

func clearTLSEnv(t *testing.T) {
	t.Helper()
	for _, p := range []string{"NATS", "REDIS"} {
		for _, s := range []string{"CA", "CERT", "KEY", "INSECURE", "SERVER_NAME"} {
			t.Setenv(p+"_TLS_"+s, "")
		}
	}
}

func newKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func writePEM(t *testing.T, path, typ string, der []byte) string {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePriv(t *testing.T, path string, k *ecdsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return writePEM(t, path, "PRIVATE KEY", der)
}

func writePub(t *testing.T, path string, k *ecdsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return writePEM(t, path, "PUBLIC KEY", der)
}

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func hashOf(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// validEnv is a complete TLS configuration with keys in a fresh temp dir.
func validEnv(t *testing.T) map[string]string {
	t.Helper()
	clearTLSEnv(t)
	dir := t.TempDir()
	certKey := newKey(t, elliptic.P256())
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"bridge.example"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &certKey.PublicKey, certKey)
	if err != nil {
		t.Fatal(err)
	}
	keysDir := filepath.Join(dir, "result-keys")
	if err := os.Mkdir(keysDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writePub(t, filepath.Join(keysDir, "a2a-demo-worker.pem"), newKey(t, elliptic.P256()))
	writeFile(t, filepath.Join(keysDir, "README.txt"), "ignored")
	return map[string]string{
		"CAP_A2A_PUBLIC_URL":    "https://bridge.example/a2a",
		"CAP_A2A_TLS_CERT_FILE": writePEM(t, filepath.Join(dir, "tls.crt"), "CERTIFICATE", der),
		"CAP_A2A_TLS_KEY_FILE":  writePriv(t, filepath.Join(dir, "tls.key"), certKey),
		"CAP_NATS_URL":          "tls://nats-a.example:4222, tls://nats-b.example:4222",
		"CAP_REDIS_URL":         "rediss://redis.example:6380/0",
		"CAP_A2A_TOPIC":         "job.a2a.demo",
		"CAP_A2A_API_KEYS_FILE": writeFile(t, filepath.Join(dir, "api-keys.json"),
			`[{"sha256":"`+hashOf(testAPIKey)+`","tenant":"acme","principal":"alice"}]`),
		"CAP_A2A_SIGNING_KEY_FILE": writePriv(t, filepath.Join(dir, "bridge.key"), newKey(t, elliptic.P256())),
		"CAP_A2A_RESULT_KEYS_DIR":  keysDir,
	}
}

func load(env map[string]string) (config, []string, error) {
	return loadConfig(func(k string) string { return env[k] })
}

func plaintext(env map[string]string) {
	delete(env, "CAP_A2A_TLS_CERT_FILE")
	delete(env, "CAP_A2A_TLS_KEY_FILE")
	env["CAP_NATS_URL"] = "nats://127.0.0.1:4222"
	env["CAP_REDIS_URL"] = "redis://:hunter2@127.0.0.1:6379"
	env["CAP_A2A_PUBLIC_URL"] = "http://127.0.0.1:8080/"
}

func TestConfigRefusesPlaintextWithoutDevFlag(t *testing.T) {
	env := validEnv(t)
	plaintext(env)
	_, _, err := load(env)
	if err == nil {
		t.Fatal("plaintext configuration accepted without CAP_A2A_DEV_INSECURE")
	}
	for _, want := range []string{"HTTP listener", "CAP_A2A_TLS_CERT_FILE", "NATS", "tls://", "NATS_TLS_CA",
		"Redis", "rediss://", "REDIS_TLS_CA", "public URL", "https", "CAP_A2A_DEV_INSECURE=1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("error echoes the Redis password: %v", err)
	}
}

func TestConfigDevInsecureWarnsPerSurface(t *testing.T) {
	env := validEnv(t)
	plaintext(env)
	env["CAP_A2A_DEV_INSECURE"] = "1"
	cfg, warnings, err := load(env)
	if err != nil {
		t.Fatalf("dev insecure refused: %v", err)
	}
	if len(warnings) != 4 {
		t.Fatalf("want one warning per plaintext surface (4), got %d: %q", len(warnings), warnings)
	}
	for i, want := range []string{"HTTP listener", "NATS", "Redis", "public URL"} {
		if !strings.Contains(warnings[i], want) || !strings.Contains(warnings[i], "development only") {
			t.Errorf("warning %d = %q, want %q and \"development only\"", i, warnings[i], want)
		}
	}
	if !cfg.devInsecure || cfg.cert != nil {
		t.Errorf("devInsecure = %v, cert = %v; want plaintext listener", cfg.devInsecure, cfg.cert)
	}
}

func TestConfigRejectsInsecureSkipVerify(t *testing.T) {
	for _, v := range []string{"NATS_TLS_INSECURE", "REDIS_TLS_INSECURE"} {
		t.Run(v, func(t *testing.T) {
			env := validEnv(t)
			t.Setenv(v, "1")
			if _, _, err := load(env); err == nil || !strings.Contains(err.Error(), v) {
				t.Fatalf("%s=1 without the dev flag: err = %v, want it named", v, err)
			}
			env["CAP_A2A_DEV_INSECURE"] = "1"
			_, warnings, err := load(env)
			if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], v) {
				t.Fatalf("%s=1 with the dev flag: err = %v, warnings = %q", v, err, warnings)
			}
		})
	}
}

func TestConfigTLSAccepted(t *testing.T) {
	env := validEnv(t)
	cfg, warnings, err := load(env)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("valid TLS configuration: err = %v, warnings = %q", err, warnings)
	}
	id, ok := cfg.apiKeys[hashOf(testAPIKey)]
	switch {
	case !ok || id.Tenant != "acme" || id.Principal != "alice":
		t.Errorf("api keys = %+v", cfg.apiKeys)
	case cfg.resultKeys["a2a-demo-worker"] == nil || len(cfg.resultKeys) != 1:
		t.Errorf("result keys = %v, want only a2a-demo-worker", cfg.resultKeys)
	case cfg.signingKey == nil || cfg.unsigned || cfg.cert == nil:
		t.Errorf("signing key %v, unsigned %v, cert %v", cfg.signingKey != nil, cfg.unsigned, cfg.cert != nil)
	case cfg.listen != ":8443" || cfg.senderID != "cap-a2a-bridge" || cfg.topic != "job.a2a.demo":
		t.Errorf("defaults: listen %q sender %q topic %q", cfg.listen, cfg.senderID, cfg.topic)
	case cfg.inputTTL != 24*time.Hour || cfg.shutdown != 10*time.Second:
		t.Errorf("defaults: input TTL %v, shutdown %v", cfg.inputTTL, cfg.shutdown)
	}
}

func TestConfigKeysAndUnsigned(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(t *testing.T, env map[string]string)
	}{
		{"P-384 signing key", "P-256", func(t *testing.T, env map[string]string) {
			env["CAP_A2A_SIGNING_KEY_FILE"] = writePriv(t, filepath.Join(t.TempDir(), "k"), newKey(t, elliptic.P384()))
		}},
		{"garbage PEM", "PEM", func(t *testing.T, env map[string]string) {
			env["CAP_A2A_SIGNING_KEY_FILE"] = writeFile(t, filepath.Join(t.TempDir(), "k"), "SECRET-MATERIAL")
		}},
		{"empty result keys dir", "CAP_A2A_RESULT_KEYS_DIR", func(t *testing.T, env map[string]string) {
			env["CAP_A2A_RESULT_KEYS_DIR"] = t.TempDir()
		}},
		{"bad sender file name", "sender", func(t *testing.T, env map[string]string) {
			writePub(t, filepath.Join(env["CAP_A2A_RESULT_KEYS_DIR"], "bad sender.pem"), newKey(t, elliptic.P256()))
		}},
		{"unsigned with result keys", "CAP_A2A_DEV_UNSIGNED", func(_ *testing.T, env map[string]string) {
			env["CAP_A2A_DEV_UNSIGNED"] = "1"
		}},
		{"bad API key hash", "sha256", func(t *testing.T, env map[string]string) {
			env["CAP_A2A_API_KEYS_FILE"] = writeFile(t, filepath.Join(t.TempDir(), "k.json"),
				`[{"sha256":"`+strings.ToUpper(hashOf("x"))+`","tenant":"t","principal":"p"}]`)
		}},
		{"duplicate API key hash", "duplicate", func(t *testing.T, env map[string]string) {
			e := `{"sha256":"` + hashOf("x") + `","tenant":"t","principal":"p"}`
			env["CAP_A2A_API_KEYS_FILE"] = writeFile(t, filepath.Join(t.TempDir(), "k.json"), "["+e+","+e+"]")
		}},
		{"cert without key", "must be set together", func(_ *testing.T, env map[string]string) {
			delete(env, "CAP_A2A_TLS_KEY_FILE")
		}},
		{"key without cert", "must be set together", func(_ *testing.T, env map[string]string) {
			delete(env, "CAP_A2A_TLS_CERT_FILE")
		}},
		{"two topics", "CAP_A2A_TOPIC", func(_ *testing.T, env map[string]string) { env["CAP_A2A_TOPIC"] = "job.a,job.b" }},
		{"input TTL range", "CAP_A2A_INPUT_TTL", func(_ *testing.T, env map[string]string) { env["CAP_A2A_INPUT_TTL"] = "30s" }},
		{"shutdown range", "CAP_A2A_SHUTDOWN_TIMEOUT", func(_ *testing.T, env map[string]string) {
			env["CAP_A2A_SHUTDOWN_TIMEOUT"] = "2m"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv(t)
			tc.mutate(t, env)
			_, _, err := load(env)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to name %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "SECRET-MATERIAL") {
				t.Fatalf("error echoes file contents: %v", err)
			}
		})
	}
	t.Run("missing required are all named", func(t *testing.T) {
		clearTLSEnv(t)
		_, _, err := load(map[string]string{})
		for _, want := range []string{"CAP_A2A_PUBLIC_URL", "CAP_NATS_URL", "CAP_REDIS_URL", "CAP_A2A_TOPIC",
			"CAP_A2A_API_KEYS_FILE", "CAP_A2A_SIGNING_KEY_FILE", "CAP_A2A_RESULT_KEYS_DIR"} {
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("missing %s is not named: %v", want, err)
			}
		}
	})
	t.Run("dev unsigned", func(t *testing.T) {
		env := validEnv(t)
		delete(env, "CAP_A2A_RESULT_KEYS_DIR")
		env["CAP_A2A_DEV_UNSIGNED"] = "1"
		cfg, warnings, err := load(env)
		if err != nil || !cfg.unsigned || cfg.signingKey == nil || len(warnings) != 1 ||
			!strings.Contains(warnings[0], "CAP_A2A_DEV_UNSIGNED") || !strings.Contains(warnings[0], "development only") {
			t.Fatalf("err = %v, unsigned = %v, signing key kept = %v, warnings = %q", err, cfg.unsigned, cfg.signingKey != nil, warnings)
		}
		delete(env, "CAP_A2A_SIGNING_KEY_FILE")
		if cfg, _, err = load(env); err != nil || cfg.signingKey != nil {
			t.Fatalf("unsigned without a signing key: err = %v", err)
		}
	})
}

// fakeRedis implements only Set and GetRange; it has no Get, so redisIO
// cannot read an unbounded value.
type fakeRedis struct {
	val    string
	ranges [][2]int64
	keys   []string
	ttls   []time.Duration
}

func (f *fakeRedis) Set(_ context.Context, key string, _ any, ttl time.Duration) *redis.StatusCmd {
	f.keys, f.ttls = append(f.keys, key), append(f.ttls, ttl)
	return redis.NewStatusResult("OK", nil)
}

func (f *fakeRedis) GetRange(_ context.Context, key string, start, end int64) *redis.StringCmd {
	f.keys, f.ranges = append(f.keys, key), append(f.ranges, [2]int64{start, end})
	v := f.val
	if int64(len(v)) > end+1 {
		v = v[:end+1] // GETRANGE's end is inclusive
	}
	return redis.NewStringResult(v, nil)
}

func TestRedisIOBoundedRead(t *testing.T) {
	ctx := context.Background()
	f := &fakeRedis{}
	io := redisIO{c: f, ttl: 7 * time.Minute}
	const maxBytes = 8

	f.val = strings.Repeat("x", maxBytes+1)
	if _, err := io.ReadResult(ctx, "res:j1", maxBytes); err == nil {
		t.Error("a value of maxBytes+1 bytes was accepted")
	}
	f.val = strings.Repeat("y", maxBytes)
	if got, err := io.ReadResult(ctx, "res:j1", maxBytes); err != nil || string(got) != f.val {
		t.Errorf("exactly maxBytes: got %q, %v", got, err)
	}
	f.val = ""
	if _, err := io.ReadResult(ctx, "res:j1", maxBytes); err == nil {
		t.Error("a missing or empty value was accepted")
	}
	for _, r := range f.ranges {
		if r != [2]int64{0, maxBytes} {
			t.Errorf("GETRANGE %v, want [0 %d]", r, maxBytes)
		}
	}
	reads := len(f.ranges)
	if _, err := io.ReadResult(ctx, "ctx:j1", maxBytes); err == nil {
		t.Error("ReadResult accepted a ctx: key")
	}
	if _, err := io.ReadResult(ctx, "res:j1", 0); err == nil {
		t.Error("ReadResult accepted maxBytes 0")
	}
	if err := io.PutTurnInput(ctx, "res:j1", []byte("{}")); err == nil {
		t.Error("PutTurnInput accepted a res: key")
	}
	if len(f.ranges) != reads || len(f.ttls) != 0 {
		t.Fatalf("a refused call reached Redis: %d reads, %d writes", len(f.ranges)-reads, len(f.ttls))
	}
	if err := io.PutTurnInput(ctx, "ctx:j1", []byte("{}")); err != nil || len(f.ttls) != 1 || f.ttls[0] != 7*time.Minute {
		t.Fatalf("PutTurnInput: err = %v, ttls = %v", err, f.ttls)
	}
}

type countCloser struct{ n int }

func (c *countCloser) Close() error { c.n++; return nil }

func TestShutdownIsBounded(t *testing.T) {
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	ns.Start()
	t.Cleanup(ns.Shutdown)
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats-server not ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	if _, err := nc.Subscribe("sys.job.result", func(*nats.Msg) {}); err != nil {
		t.Fatal(err)
	}
	entered, exited := make(chan struct{}), make(chan struct{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done() // like a blocking SendMessage; the body is left unread
		close(exited)
	})}
	stops, rdb := 0, &countCloser{}
	s := newBridge(srv, func() error { stops++; return nil }, nc, rdb)
	go func() { _ = srv.Serve(ln) }()
	go func() {
		if resp, err := http.Post("http://"+ln.Addr().String()+"/", "application/json", strings.NewReader("{}")); err == nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request never reached the handler")
	}
	began := time.Now()
	s.shutdown(200 * time.Millisecond)
	if d := time.Since(began); d > 2*time.Second {
		t.Fatalf("shutdown took %v with a request in flight", d)
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("the in-flight handler did not exit")
	}
	if !nc.IsClosed() || stops != 1 || rdb.n != 1 {
		t.Fatalf("after shutdown: nats closed %v, egress stops %d, redis closes %d", nc.IsClosed(), stops, rdb.n)
	}
	s.shutdown(200 * time.Millisecond)
	if stops != 1 || rdb.n != 1 {
		t.Fatalf("second shutdown was not a no-op: egress stops %d, redis closes %d", stops, rdb.n)
	}
}
