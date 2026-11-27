package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cordum-io/cap/v2/internal/referencescheduler"
	"github.com/cordum-io/cap/v2/internal/tck"
)

// envRunMain makes the test binary run main, so a test can signal the real
// command process.
const envRunMain = "CAP_REFERENCE_SCHEDULER_TEST_MAIN"

// canary sits in files and config values that the command must never print.
const canary = "CANARY-7f3c9a1e-never-printed"

func TestMain(m *testing.M) {
	if os.Getenv(envRunMain) == "1" {
		main()
	}
	os.Exit(m.Run())
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// writeKeyPair writes a P-256 private key (mode 0600) and its public key.
func writeKeyPair(t *testing.T, dir, name string) (priv, pub string) {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	priv, pub = filepath.Join(dir, name+".key"), filepath.Join(dir, name+".pub")
	writeFile(t, priv, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600)
	writeFile(t, pub, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o644)
	return priv, pub
}

// validConfig writes a configuration that Resolve accepts, with plaintext
// NATS to natsURL on loopback, and returns its path and its signing key.
func validConfig(t *testing.T, natsURL string) (cfgPath, signingKey string) {
	t.Helper()
	dir := t.TempDir()
	signingKey, _ = writeKeyPair(t, dir, "signing")
	trustKey, _ := writeKeyPair(t, dir, "trust")
	_, proof := writeKeyPair(t, dir, "worker-proof")
	_, workerSign := writeKeyPair(t, dir, "worker-sign")
	_, clientSign := writeKeyPair(t, dir, "client-sign")
	cfg := map[string]any{
		"NATSURL": natsURL, "DevPlaintextLoopback": true, "SchedulerID": "refsched-test",
		"SigningKeyID": "s1", "SigningKeyFile": signingKey, "TrustKeyID": "t1", "TrustKeyFile": trustKey,
		"Workers": []map[string]any{{"WorkerID": "worker-1", "TenantID": "tenant-a", "AgentID": "agent-1",
			"ProofKeyID": "p1", "ProofKeyFile": proof, "SigningKeyFiles": map[string]string{"w1": workerSign}}},
		"Clients": []map[string]any{{"SenderID": "client-1", "TenantID": "tenant-a",
			"SigningKeyFiles": map[string]string{"c1": clientSign}}},
		"Allow":        []map[string]string{{"TenantID": "tenant-a", "Topic": "job.echo", "Capability": "echo"}},
		"TopicWorkers": map[string][]string{"job.echo": {"worker-1"}},
		"Limits": map[string]any{"MaxInputBytes": 1 << 20, "MaxOutputBytes": 1 << 20, "MaxArtifacts": 8,
			"MaxAttempts": 1, "DispatchTimeout": "5s", "RunTimeout": "30s", "MaxRunTimeout": "1m"},
		"MaxPacketBytes": 1 << 16, "MaxConcurrency": 4, "MaxLiveJobs": 64, "MaxChallenges": 16,
		"ChallengeTTL": "30s", "SessionTTL": "10m",
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(dir, "scheduler.json")
	writeFile(t, cfgPath, data, 0o600)
	return cfgPath, signingKey
}

// TestRunConfigErrors: every usage or configuration error exits 2 with its
// own message and never prints the canary, which sits in a key file, an
// unknown field, a duration and data after the configuration.
func TestRunConfigErrors(t *testing.T) {
	// Port 1 refuses connections, so a case that wrongly reaches the runtime
	// fails fast instead of serving.
	good, signingKey := validConfig(t, "nats://127.0.0.1:1")
	dir := filepath.Dir(good)
	file := func(name, body string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		writeFile(t, p, []byte(body), mode)
		return p
	}
	key, err := os.ReadFile(signingKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		args  []string
		want  string
		posix bool // needs POSIX file modes or an unprivileged symlink
	}{
		{"unknown flag", []string{"-config", good, "-nope"}, errUsage.Error(), false},
		{"extra argument", []string{"-config", good, "extra"}, errUsage.Error(), false},
		{"missing config file", []string{"-config", filepath.Join(dir, "absent.json")}, errConfigFile.Error(), false},
		{"unknown field", []string{"-config", file("unknown.json", `{"Canary":"`+canary+`"}`, 0o600)}, errConfigFile.Error(), false},
		{"invalid duration", []string{"-config", file("duration.json", `{"SessionTTL":"`+canary+`"}`, 0o600)}, errConfigFile.Error(), false},
		{"trailing data", []string{"-config", file("trailing.json", `{} {}`, 0o600)}, errConfigFile.Error(), false},
		{"closing bracket after the configuration", []string{"-config", file("bracket.json", string(cfg)+"]", 0o600)}, errConfigFile.Error(), false},
		{"closing brace after the configuration", []string{"-config", file("brace.json", string(cfg)+"}", 0o600)}, errConfigFile.Error(), false},
		{"data after a closing delimiter", []string{"-config", file("after.json", string(cfg)+"]"+canary, 0o600)}, errConfigFile.Error(), false},
		{"no configuration", nil, referencescheduler.ErrConfigNATSURL.Error(), false},
		{"plaintext to a remote host", []string{"-config", good, "-nats-url", "nats://192.0.2.1:4222"}, referencescheduler.ErrConfigPlaintextRemote.Error(), false},
		{"missing key file", []string{"-config", good, "-signing-key-file", filepath.Join(dir, "absent.key")}, "signing-key-file: " + errKeyFile.Error(), false},
		{"key file that is no key", []string{"-config", good, "-signing-key-file", file("canary.key", "-----BEGIN EC PRIVATE KEY-----\n"+canary+"\n-----END EC PRIVATE KEY-----\n", 0o600)}, referencescheduler.ErrConfigScheduler.Error(), false},
		{"group-readable key", []string{"-config", good, "-signing-key-file", file("group.key", string(key), 0o640)}, "signing-key-file: " + errKeyMode.Error(), true},
		{"world-readable key", []string{"-config", good, "-trust-key-file", file("world.key", string(key), 0o604)}, "trust-key-file: " + errKeyMode.Error(), true},
		{"symlinked key", []string{"-config", good, "-signing-key-file", symlink(t, signingKey, filepath.Join(dir, "link.key"))}, "signing-key-file: " + errKeySymlink.Error(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.posix && runtime.GOOS == "windows" {
				t.Skip("POSIX file modes and symlinks")
			}
			var out bytes.Buffer
			if code := run(context.Background(), tc.args, &out); code != exitConfig {
				t.Fatalf("exit %d, want %d; stderr %q", code, exitConfig, out.String())
			}
			if !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), canary) {
				t.Fatalf("stderr %q, want %q and no canary", out.String(), tc.want)
			}
		})
	}
}

// TestReadConfigFileAllowsTrailingWhitespace: a hand-written file that ends
// in a newline is still one JSON object.
func TestReadConfigFileAllowsTrailingWhitespace(t *testing.T) {
	good, _ := validConfig(t, "nats://127.0.0.1:1")
	cfg, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(good), "newline.json")
	writeFile(t, path, append(cfg, " \r\n\t\n"...), 0o600)
	if _, err := readConfigFile(path); err != nil {
		t.Fatalf("readConfigFile = %v, want nil", err)
	}
}

// symlink links name to target, or returns target where symlinks need
// privileges (the case is skipped there).
func symlink(t *testing.T, target, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return target
	}
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
	return name
}

// TestRunDrainsOnSIGTERM: the scheduler process connects to NATS, reports
// ready, and on SIGTERM drains and exits 0 within 10 seconds.
func TestRunDrainsOnSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signals")
	}
	ns, err := tck.StartEmbeddedNATS()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ns.Close)
	cfg, _ := validConfig(t, ns.URL())
	cmd := exec.Command(os.Args[0], "-config", cfg)
	cmd.Env = append(os.Environ(), envRunMain+"=1")
	pipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	for ready, timeout := false, time.After(20*time.Second); !ready; {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatal("stderr closed before the ready line")
			}
			ready = strings.Contains(l, "cap-reference-scheduler: ready")
		case <-timeout:
			_ = cmd.Process.Kill()
			t.Fatal("no ready line within 20s")
		}
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	drained := false
	for open, timeout := true, time.After(10*time.Second); open; {
		select {
		case l, ok := <-lines:
			open = ok
			drained = drained || strings.Contains(l, "cap-reference-scheduler: drained")
		case <-timeout:
			_ = cmd.Process.Kill()
			t.Fatal("still running 10s after SIGTERM")
		}
	}
	if err := cmd.Wait(); err != nil || !drained {
		t.Fatalf("after SIGTERM: exit %v, drained line %v; want exit 0 after a drain", err, drained)
	}
}
