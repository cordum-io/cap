// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	rs "github.com/cordum-io/cap/v2/internal/referencescheduler"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

// cfCanary stands in for a secret. It is in every fixture directory name and in
// the header line of every PEM fixture, so any error or log line that echoes a
// path or file content shows it.
const cfCanary = "CANARY-5ecr3t-7f3a9c1d"

// cfEnv is the fixture behind one cfValid config.
type cfEnv struct {
	dir            string
	caDER, certDER []byte
	keys           map[string]*ecdsa.PrivateKey // generated key pairs by name
}

type cfMut func(t *testing.T, c *rs.RuntimeConfig, e cfEnv)

type cfCase struct {
	name string
	mut  cfMut
	want error
}

func cfSet(f func(c *rs.RuntimeConfig)) cfMut {
	return func(_ *testing.T, c *rs.RuntimeConfig, _ cfEnv) { f(c) }
}

func cfGen(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func cfWrite(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	data := append([]byte("# "+cfCanary+"\n"), pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func cfJunk(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("# "+cfCanary+"\nnot a pem file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func cfRm(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func cfPriv(t *testing.T, path string, k *ecdsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	cfWrite(t, path, "EC PRIVATE KEY", der)
}

func cfPKCS8(t *testing.T, path string, k *ecdsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	cfWrite(t, path, "PRIVATE KEY", der)
}

func cfPub(t *testing.T, path string, k *ecdsa.PublicKey) {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		t.Fatal(err)
	}
	cfWrite(t, path, "PUBLIC KEY", der)
}

// cfCerts writes a P-256 test CA and a client certificate and key it signed.
func cfCerts(t *testing.T, e *cfEnv, caPath, certPath, keyPath string) {
	t.Helper()
	caKey, clientKey := cfGen(t, elliptic.P256()), cfGen(t, elliptic.P256())
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "cf-ca"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	var err error
	if e.caDER, err = x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey); err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(e.caDER)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "cf-client"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if e.certDER, err = x509.CreateCertificate(rand.Reader, leaf, caCert, &clientKey.PublicKey, caKey); err != nil {
		t.Fatal(err)
	}
	cfWrite(t, caPath, "CERTIFICATE", e.caDER)
	cfWrite(t, certPath, "CERTIFICATE", e.certDER)
	cfPriv(t, keyPath, clientKey)
}

// cfValid returns a complete config over fresh fixtures in a canary-named directory.
func cfValid(t *testing.T) (rs.RuntimeConfig, cfEnv) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), cfCanary)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	e := cfEnv{dir: dir, keys: map[string]*ecdsa.PrivateKey{}}
	p := func(name string) string { return filepath.Join(dir, name) }
	cfCerts(t, &e, p("ca.pem"), p("client.pem"), p("client.key"))
	e.keys["sched"], e.keys["trust"] = cfGen(t, elliptic.P256()), cfGen(t, elliptic.P256())
	cfPriv(t, p("sched.key"), e.keys["sched"])
	cfPKCS8(t, p("trust.key"), e.keys["trust"])
	pub := func(name string) string {
		k := cfGen(t, elliptic.P256())
		e.keys[name] = k
		cfPub(t, p(name+".pub"), &k.PublicKey)
		return p(name + ".pub")
	}
	worker := func(id, agent string) rs.WorkerEntry {
		return rs.WorkerEntry{
			WorkerID: id, TenantID: "t1", AgentID: agent, ProofKeyID: id + "-proof", ProofKeyFile: pub(id + "-proof"),
			SigningKeyFiles: map[string]string{id + "-sign": pub(id + "-sign")},
		}
	}
	c := rs.RuntimeConfig{
		NATSURL: "tls://nats.example.test:4222", TLSCAFile: p("ca.pem"), TLSCertFile: p("client.pem"), TLSKeyFile: p("client.key"),
		SchedulerID: "sched-1", SigningKeyID: "sched-sign-1", SigningKeyFile: p("sched.key"),
		TrustKeyID: "sched-trust-1", TrustKeyFile: p("trust.key"),
		Workers:      []rs.WorkerEntry{worker("w1", "a1"), worker("w2", "a2")},
		Clients:      []rs.ClientEntry{{SenderID: "c1", TenantID: "t1", SigningKeyFiles: map[string]string{"c1-sign": pub("c1-sign")}}},
		Allow:        []rs.AllowRule{{TenantID: "t1", Topic: "topicA", Capability: "capA"}},
		TopicWorkers: map[string][]string{"topicA": {"w1"}},
		Limits: rs.Limits{
			MaxInputBytes: 1 << 20, MaxOutputBytes: 1 << 20, MaxArtifacts: 8, MaxAttempts: 3,
			DispatchTimeout: time.Minute, RunTimeout: time.Minute, MaxRunTimeout: time.Hour,
		},
		MaxPacketBytes: 1 << 16, MaxConcurrency: 8, MaxLiveJobs: 100, MaxChallenges: 64,
		ChallengeTTL: 30 * time.Second, SessionTTL: 5 * time.Minute,
	}
	return c, e
}

func cfPlain(url string, dev bool) cfMut {
	return cfSet(func(c *rs.RuntimeConfig) {
		c.NATSURL, c.DevPlaintextLoopback = url, dev
		c.TLSCAFile, c.TLSCertFile, c.TLSKeyFile = "", "", ""
	})
}

func cfTLSCases() []cfCase {
	dir := func(get func(c *rs.RuntimeConfig) *string) cfMut {
		return func(_ *testing.T, c *rs.RuntimeConfig, e cfEnv) { *get(c) = e.dir }
	}
	ca := func(c *rs.RuntimeConfig) *string { return &c.TLSCAFile }
	cert := func(c *rs.RuntimeConfig) *string { return &c.TLSCertFile }
	key := func(c *rs.RuntimeConfig) *string { return &c.TLSKeyFile }
	rm := func(get func(c *rs.RuntimeConfig) *string) cfMut {
		return func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfRm(t, *get(c)) }
	}
	junk := func(get func(c *rs.RuntimeConfig) *string) cfMut {
		return func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfJunk(t, *get(c)) }
	}
	return []cfCase{
		{"ca missing", rm(ca), rs.ErrConfigTLSCA},
		{"ca unreadable", dir(ca), rs.ErrConfigTLSCA},
		{"ca without certificates", junk(ca), rs.ErrConfigTLSCA},
		{"cert missing", rm(cert), rs.ErrConfigTLSCert},
		{"cert unreadable", dir(cert), rs.ErrConfigTLSCert},
		{"cert not pem", junk(cert), rs.ErrConfigTLSCert},
		{"key missing", rm(key), rs.ErrConfigTLSKey},
		{"key unreadable", dir(key), rs.ErrConfigTLSKey},
		{"key not pem", junk(key), rs.ErrConfigTLSKey},
		{"key does not match cert", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) {
			cfPriv(t, c.TLSKeyFile, cfGen(t, elliptic.P256()))
		}, rs.ErrConfigTLSKey},
		{"partial tls: cert and key unset", cfSet(func(c *rs.RuntimeConfig) { c.TLSCertFile, c.TLSKeyFile = "", "" }), rs.ErrConfigTLSCert},
		{"partial tls: only key set", cfSet(func(c *rs.RuntimeConfig) { c.TLSCAFile, c.TLSCertFile = "", "" }), rs.ErrConfigTLSCA},
		{"plaintext to a remote host with the dev flag", cfPlain("nats://10.0.0.5:4222", true), rs.ErrConfigPlaintextRemote},
		{"plaintext to a remote host without the dev flag", cfPlain("nats://10.0.0.5:4222", false), rs.ErrConfigPlaintextRemote},
		{"loopback plaintext without the dev flag", cfPlain("nats://127.0.0.1:4222", false), rs.ErrConfigPlaintextDev},
		{"empty url", cfSet(func(c *rs.RuntimeConfig) { c.NATSURL = "" }), rs.ErrConfigNATSURL},
		{"url without scheme", cfSet(func(c *rs.RuntimeConfig) { c.NATSURL = "127.0.0.1:4222" }), rs.ErrConfigNATSURL},
		{"url with another scheme", cfSet(func(c *rs.RuntimeConfig) { c.NATSURL = "http://127.0.0.1:4222" }), rs.ErrConfigNATSURL},
		{"url without host", cfSet(func(c *rs.RuntimeConfig) { c.NATSURL = "nats://" }), rs.ErrConfigNATSURL},
		{"unparseable url", cfSet(func(c *rs.RuntimeConfig) { c.NATSURL = "nats://[::1" }), rs.ErrConfigNATSURL},
	}
}

func cfSchedulerCases() []cfCase {
	key := func(get func(c *rs.RuntimeConfig) string, write func(t *testing.T, path string, k *ecdsa.PrivateKey)) cfMut {
		return func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { write(t, get(c), cfGen(t, elliptic.P384())) }
	}
	sign := func(c *rs.RuntimeConfig) string { return c.SigningKeyFile }
	trust := func(c *rs.RuntimeConfig) string { return c.TrustKeyFile }
	return []cfCase{
		{"scheduler id blank", cfSet(func(c *rs.RuntimeConfig) { c.SchedulerID = " " }), rs.ErrConfigScheduler},
		{"scheduler id untrimmed", cfSet(func(c *rs.RuntimeConfig) { c.SchedulerID = " sched-1" }), rs.ErrConfigScheduler},
		{"signing key id blank", cfSet(func(c *rs.RuntimeConfig) { c.SigningKeyID = "" }), rs.ErrConfigScheduler},
		{"trust key id blank", cfSet(func(c *rs.RuntimeConfig) { c.TrustKeyID = "" }), rs.ErrConfigScheduler},
		{"signing key path empty", cfSet(func(c *rs.RuntimeConfig) { c.SigningKeyFile = "" }), rs.ErrConfigScheduler},
		{"signing key missing", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfRm(t, c.SigningKeyFile) }, rs.ErrConfigScheduler},
		{"signing key P-384", key(sign, cfPriv), rs.ErrConfigScheduler},
		{"signing key not pem", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfJunk(t, c.SigningKeyFile) }, rs.ErrConfigScheduler},
		{"trust key missing", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfRm(t, c.TrustKeyFile) }, rs.ErrConfigScheduler},
		{"trust key P-384 pkcs8", key(trust, cfPKCS8), rs.ErrConfigScheduler},
		{"trust key not pem", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfJunk(t, c.TrustKeyFile) }, rs.ErrConfigScheduler},
	}
}

// cfBadIDs are not single NATS subject tokens.
var cfBadIDs = []string{"", "a.b", "*", ">", "w 1", "w\t1", "w 1", "w\x001", "w\xff1", strings.Repeat("w", 257)}

func cfWorkerCases() []cfCase {
	w0 := func(f func(w *rs.WorkerEntry)) cfMut { return cfSet(func(c *rs.RuntimeConfig) { f(&c.Workers[0]) }) }
	p384 := func(get func(w *rs.WorkerEntry) string) cfMut {
		return func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) {
			cfPub(t, get(&c.Workers[0]), &cfGen(t, elliptic.P384()).PublicKey)
		}
	}
	cases := []cfCase{
		{"no workers", cfSet(func(c *rs.RuntimeConfig) { c.Workers = nil }), rs.ErrConfigEmptyTrustStore},
		{"duplicate worker id", cfSet(func(c *rs.RuntimeConfig) { c.Workers[1].WorkerID = "w1" }), rs.ErrConfigWorker},
		{"worker tenant blank", w0(func(w *rs.WorkerEntry) { w.TenantID = "" }), rs.ErrConfigWorker},
		{"worker tenant untrimmed", w0(func(w *rs.WorkerEntry) { w.TenantID = "t1 " }), rs.ErrConfigWorker},
		{"worker agent blank", w0(func(w *rs.WorkerEntry) { w.AgentID = "" }), rs.ErrConfigWorker},
		{"worker proof key id blank", w0(func(w *rs.WorkerEntry) { w.ProofKeyID = "" }), rs.ErrConfigWorker},
		{"worker proof key P-384", p384(func(w *rs.WorkerEntry) string { return w.ProofKeyFile }), rs.ErrConfigWorker},
		{"worker proof key missing", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfRm(t, c.Workers[0].ProofKeyFile) }, rs.ErrConfigWorker},
		{"worker proof key not pem", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfJunk(t, c.Workers[0].ProofKeyFile) }, rs.ErrConfigWorker},
		{"worker signing key P-384", p384(func(w *rs.WorkerEntry) string { return w.SigningKeyFiles["w1-sign"] }), rs.ErrConfigWorker},
		{"worker without signing keys", w0(func(w *rs.WorkerEntry) { w.SigningKeyFiles = nil }), rs.ErrConfigWorker},
		{"worker signing key id blank", w0(func(w *rs.WorkerEntry) { w.SigningKeyFiles = map[string]string{"": w.SigningKeyFiles["w1-sign"]} }), rs.ErrConfigWorker},
	}
	for i, id := range cfBadIDs {
		cases = append(cases,
			cfCase{fmt.Sprintf("worker id #%d in the trust store", i), w0(func(w *rs.WorkerEntry) { w.WorkerID = id }), rs.ErrConfigWorkerID},
			cfCase{fmt.Sprintf("worker id #%d in a topic list", i), cfSet(func(c *rs.RuntimeConfig) { c.TopicWorkers = map[string][]string{"topicA": {id}} }), rs.ErrConfigWorkerID})
	}
	return cases
}

func cfPolicyCases() []cfCase {
	cl0 := func(f func(e *rs.ClientEntry)) cfMut { return cfSet(func(c *rs.RuntimeConfig) { f(&c.Clients[0]) }) }
	rule := func(f func(r *rs.AllowRule)) cfMut { return cfSet(func(c *rs.RuntimeConfig) { f(&c.Allow[0]) }) }
	topics := func(m map[string][]string) cfMut { return cfSet(func(c *rs.RuntimeConfig) { c.TopicWorkers = m }) }
	bound := func(name string, f func(c *rs.RuntimeConfig)) cfCase {
		return cfCase{name, cfSet(f), rs.ErrConfigBounds}
	}
	cases := []cfCase{
		{"no clients", cfSet(func(c *rs.RuntimeConfig) { c.Clients = nil }), rs.ErrConfigEmptyClients},
		{"duplicate client sender", cfSet(func(c *rs.RuntimeConfig) { c.Clients = append(c.Clients, c.Clients[0]) }), rs.ErrConfigClient},
		{"client sender blank", cl0(func(e *rs.ClientEntry) { e.SenderID = "" }), rs.ErrConfigClient},
		{"client tenant blank", cl0(func(e *rs.ClientEntry) { e.TenantID = " " }), rs.ErrConfigClient},
		{"client without signing keys", cl0(func(e *rs.ClientEntry) { e.SigningKeyFiles = nil }), rs.ErrConfigClient},
		{"client signing key not pem", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) { cfJunk(t, c.Clients[0].SigningKeyFiles["c1-sign"]) }, rs.ErrConfigClient},
		{"client signing key P-384", func(t *testing.T, c *rs.RuntimeConfig, _ cfEnv) {
			cfPub(t, c.Clients[0].SigningKeyFiles["c1-sign"], &cfGen(t, elliptic.P384()).PublicKey)
		}, rs.ErrConfigClient},
		{"no allow rules", cfSet(func(c *rs.RuntimeConfig) { c.Allow = nil }), rs.ErrConfigEmptyAllowlist},
		{"allow tenant blank", rule(func(r *rs.AllowRule) { r.TenantID = "" }), rs.ErrConfigAllowRule},
		{"allow topic blank", rule(func(r *rs.AllowRule) { r.Topic = "" }), rs.ErrConfigAllowRule},
		{"allow capability blank", rule(func(r *rs.AllowRule) { r.Capability = "" }), rs.ErrConfigAllowRule},
		{"allow capability untrimmed", rule(func(r *rs.AllowRule) { r.Capability = "capA " }), rs.ErrConfigAllowRule},
		{"topic worker unknown", topics(map[string][]string{"topicA": {"ghost"}}), rs.ErrConfigTopicWorker},
		{"allowed topic without a worker list", topics(nil), rs.ErrConfigTopicWorker},
		{"allowed topic with an empty worker list", topics(map[string][]string{"topicA": {}}), rs.ErrConfigTopicWorker},
	}
	return append(cases,
		bound("max packet bytes zero", func(c *rs.RuntimeConfig) { c.MaxPacketBytes = 0 }),
		bound("max packet bytes negative", func(c *rs.RuntimeConfig) { c.MaxPacketBytes = -1 }),
		bound("max packet bytes above the production raw limit", func(c *rs.RuntimeConfig) { c.MaxPacketBytes = capsdk.DefaultProductionMaxRawBytes + 1 }),
		bound("max concurrency zero", func(c *rs.RuntimeConfig) { c.MaxConcurrency = 0 }),
		bound("max live jobs zero", func(c *rs.RuntimeConfig) { c.MaxLiveJobs = 0 }))
}

func cfCases() []cfCase {
	return append(append(append(cfTLSCases(), cfSchedulerCases()...), cfWorkerCases()...), cfPolicyCases()...)
}

func cfApply(t *testing.T, tc cfCase) rs.RuntimeConfig {
	t.Helper()
	c, e := cfValid(t)
	tc.mut(t, &c, e)
	return c
}

func TestRuntimeConfig_ResolveFailureClasses(t *testing.T) {
	valid, _ := cfValid(t)
	if got, err := valid.Resolve(); err != nil || got == nil {
		t.Errorf("complete config: got (%v, %v), want a resolved config", got, err)
	}
	for _, tc := range cfCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cfApply(t, tc).Resolve()
			if got != nil {
				t.Fatalf("got a resolved config alongside err %v", err)
			}
			if !errors.Is(err, tc.want) || errors.Unwrap(err) != nil {
				t.Fatalf("err = %v, want the bare sentinel %v", err, tc.want)
			}
		})
	}
}

type cfLogBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *cfLogBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *cfLogBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestRuntimeConfig_NoSecretInErrorsOrLogs(t *testing.T) {
	logs := &cfLogBuf{}
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.Level(-8)})))
	slog.Info("probe")
	probe := logs.String()
	if !strings.Contains(probe, "probe") {
		t.Fatal("the capturing slog handler is not installed")
	}
	c, _ := cfValid(t)
	body, err := os.ReadFile(c.SigningKeyFile)
	if err != nil || !strings.Contains(string(body), cfCanary) || !strings.Contains(c.SigningKeyFile, cfCanary) {
		t.Fatalf("the canary must be in the scheduler key file and in its path (read err %v)", err)
	}
	for _, tc := range cfCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cfApply(t, tc).Resolve()
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("got (%v, %v), want %v", got, err, tc.want)
			}
			if strings.Contains(err.Error(), cfCanary) {
				t.Fatalf("the error carries the canary: %q", err.Error())
			}
		})
	}
	if strings.Contains(logs.String(), cfCanary) {
		t.Fatalf("captured log output carries the canary:\n%s", logs.String())
	}
	if logs.String() != probe {
		t.Fatalf("Resolve logged: %s", logs.String())
	}
}

func TestRuntimeConfig_TLS(t *testing.T) {
	c, e := cfValid(t)
	got, err := c.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.TLS == nil || got.TLS.MinVersion < tls.VersionTLS12 {
		t.Fatalf("tls config = %+v, want MinVersion >= TLS 1.2", got.TLS)
	}
	caCert, err := x509.ParseCertificate(e.caDER)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if got.TLS.RootCAs == nil || !got.TLS.RootCAs.Equal(pool) {
		t.Fatal("RootCAs is not the pool holding the test CA")
	}
	if len(got.TLS.Certificates) != 1 || !bytes.Equal(got.TLS.Certificates[0].Certificate[0], e.certDER) {
		t.Fatal("Certificates[0] is not the generated client certificate")
	}
}

func TestRuntimeConfig_LoopbackIsParsedHost(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want error
	}{
		{"nats://127.0.0.1:4222", nil}, {"nats://[::1]:4222", nil}, {"nats://localhost:4222", nil},
		{"nats://127.0.0.1.evil.example:4222", rs.ErrConfigPlaintextRemote},
		{"nats://localhost.evil.example:4222", rs.ErrConfigPlaintextRemote},
		{"nats://127.0.0.1@evil.example:4222", rs.ErrConfigPlaintextRemote},
		{"nats://evil.example:4222/localhost", rs.ErrConfigPlaintextRemote},
		{"nats://0.0.0.0:4222", rs.ErrConfigPlaintextRemote},
		{"nats://10.0.0.5:4222", rs.ErrConfigPlaintextRemote},
	} {
		t.Run(tc.url, func(t *testing.T) {
			c, e := cfValid(t)
			cfPlain(tc.url, true)(t, &c, e)
			got, err := c.Resolve()
			if tc.want == nil {
				if err != nil || got == nil || got.TLS != nil {
					t.Fatalf("got (%+v, %v), want a resolved config with nil TLS", got, err)
				}
				return
			}
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("got (%v, %v), want %v", got, err, tc.want)
			}
		})
	}
}

func TestRuntimeConfig_WorkerIDTokens(t *testing.T) {
	for _, id := range []string{"w1", "W-1_x", "wörker", strings.Repeat("w", 256)} {
		c, _ := cfValid(t)
		c.Workers[0].WorkerID = id
		c.TopicWorkers = map[string][]string{"topicA": {id}}
		got, err := c.Resolve()
		if err != nil || got.Workers[id].WorkerID != id {
			t.Fatalf("worker id of %d bytes: got (%v, %v), want it resolved", len(id), got, err)
		}
	}
}

func TestRuntimeConfig_ResolveCompleteAndCopies(t *testing.T) {
	c, e := cfValid(t)
	got, err := c.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	w1, c1 := got.Workers["w1"], got.Clients["c1"]
	switch {
	case got.NATSURL != c.NATSURL || got.SchedulerID != "sched-1" || got.SigningKeyID != "sched-sign-1" || got.TrustKeyID != "sched-trust-1":
		t.Fatalf("scalar identity fields differ: %+v", got)
	case !got.SigningKey.Equal(e.keys["sched"]) || !got.TrustKey.Equal(e.keys["trust"]):
		t.Fatal("scheduler keys are not the fixture keys (EC PRIVATE KEY and PKCS8)")
	case len(got.Workers) != 2 || w1.TenantID != "t1" || w1.AgentID != "a1" || w1.ProofKeyID != "w1-proof":
		t.Fatalf("worker w1 = %+v", w1)
	case !w1.ProofKey.Equal(&e.keys["w1-proof"].PublicKey) || !w1.SigningKeys["w1-sign"].Equal(&e.keys["w1-sign"].PublicKey):
		t.Fatal("worker keys are not the fixture keys")
	case c1.TenantID != "t1" || !c1.SigningKeys["c1-sign"].Equal(&e.keys["c1-sign"].PublicKey):
		t.Fatalf("client c1 = %+v", c1)
	case got.Limits != c.Limits || got.MaxPacketBytes != c.MaxPacketBytes || got.MaxConcurrency != 8 || got.MaxLiveJobs != 100 ||
		got.MaxChallenges != 64 || got.ChallengeTTL != 30*time.Second || got.SessionTTL != 5*time.Minute:
		t.Fatalf("limits, bounds or TTLs differ: %+v", got)
	}
	// The result must not alias the caller's slices or maps.
	c.Allow[0].Topic = "mutated"
	c.TopicWorkers["topicA"][0] = "w2"
	c.TopicWorkers["topicB"] = []string{"w2"}
	c.Workers[0].SigningKeyFiles["extra"] = "x"
	delete(c.Clients[0].SigningKeyFiles, "c1-sign")
	if got.Allow[0].Topic != "topicA" || got.TopicWorkers["topicA"][0] != "w1" || len(got.TopicWorkers) != 1 ||
		len(got.Workers["w1"].SigningKeys) != 1 || len(got.Clients["c1"].SigningKeys) != 1 {
		t.Fatalf("the resolved config changed when the input was mutated: %+v", got)
	}
}
