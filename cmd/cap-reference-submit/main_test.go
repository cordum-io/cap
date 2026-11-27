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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/internal/referencescheduler"
	"github.com/cordum-io/cap/v2/internal/resourcekv"
	"github.com/cordum-io/cap/v2/internal/tck"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// canary sits in files the command reads; it must never be printed.
const canary = "CANARY-5d1e8b2a-never-printed"

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
func writeKeyPair(t *testing.T, dir, name string) (priv, pub string, key *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	priv, pub = filepath.Join(dir, name+".key"), filepath.Join(dir, name+".pub")
	writeFile(t, priv, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600)
	writeFile(t, pub, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o644)
	return priv, pub, key
}

// clientArgs is a complete command line for client-1 of tenant-a.
func clientArgs(natsURL, key, schedulerPub, input string) []string {
	return []string{"-nats-url", natsURL, "-dev-plaintext-loopback", "-sender-id", "client-1", "-tenant-id", "tenant-a",
		"-signing-key-id", "c1", "-signing-key-file", key, "-scheduler-id", "refsched-e2e", "-scheduler-key-id", "s1",
		"-scheduler-key-file", schedulerPub, "-topic", "job.echo", "-capability", "echo", "-kv-bucket", "refsched-inputs", "-input", input}
}

// TestRunConfigErrors: every usage or configuration error exits 2 with its
// own message, prints no summary and never prints the canary.
func TestRunConfigErrors(t *testing.T) {
	dir := t.TempDir()
	key, pub, _ := writeKeyPair(t, dir, "client")
	file := func(name, body string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		writeFile(t, p, []byte(body), mode)
		return p
	}
	keyPEM, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	// Port 1 refuses connections, so a case that wrongly gets past load fails fast.
	base := clientArgs("nats://127.0.0.1:1", key, pub, file("input.json", `{}`, 0o600))
	with := func(extra ...string) []string { return append(slices.Clone(base), extra...) }
	notKey := "-----BEGIN EC PRIVATE KEY-----\n" + canary + "\n-----END EC PRIVATE KEY-----\n"
	cases := []struct {
		name  string
		args  []string
		want  string
		posix bool // needs POSIX file modes or an unprivileged symlink
	}{
		{"unknown flag", with("-nope"), errUsage.Error(), false},
		{"extra argument", with("extra"), errUsage.Error(), false},
		{"missing sender id", with("-sender-id", ""), "-sender-id: " + errRequired.Error(), false},
		{"untrimmed topic", with("-topic", " job.echo"), "-topic: " + errRequired.Error(), false},
		{"deadline not positive", with("-deadline", "0s"), errDeadline.Error(), false},
		{"plaintext to a remote host", with("-nats-url", "nats://192.0.2.1:4222"), referencescheduler.ErrConfigPlaintextRemote.Error(), false},
		{"plaintext without the dev flag", with("-dev-plaintext-loopback=false"), referencescheduler.ErrConfigPlaintextDev.Error(), false},
		{"missing TLS CA", with("-tls-ca", filepath.Join(dir, "absent.pem")), referencescheduler.ErrConfigTLSCA.Error(), false},
		{"missing key file", with("-signing-key-file", filepath.Join(dir, "absent.key")), "signing-key-file: " + errKeyFile.Error(), false},
		{"key file that is no key", with("-signing-key-file", file("canary.key", notKey, 0o600)), errSigningKey.Error(), false},
		{"scheduler key that is no key", with("-scheduler-key-file", file("canary.pub", canary, 0o644)), errSchedulerKey.Error(), false},
		{"missing input", with("-input", filepath.Join(dir, "absent.json")), errInput.Error(), false},
		{"group-readable key", with("-signing-key-file", file("group.key", string(keyPEM), 0o640)), "signing-key-file: " + errKeyMode.Error(), true},
		{"world-readable key", with("-signing-key-file", file("world.key", string(keyPEM), 0o604)), "signing-key-file: " + errKeyMode.Error(), true},
		{"group-readable TLS key", with("-tls-key", file("tls.key", string(keyPEM), 0o640)), "tls-key: " + errKeyMode.Error(), true},
		{"symlinked key", with("-signing-key-file", symlink(t, key, filepath.Join(dir, "link.key"))), "signing-key-file: " + errKeySymlink.Error(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.posix && runtime.GOOS == "windows" {
				t.Skip("POSIX file modes and symlinks")
			}
			var stdout, stderr bytes.Buffer
			if code := run(context.Background(), tc.args, &stdout, &stderr); code != exitConfig {
				t.Fatalf("exit %d, want %d; stderr %q", code, exitConfig, stderr.String())
			}
			out := stdout.String() + stderr.String()
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) || strings.Contains(out, canary) {
				t.Fatalf("stdout %q stderr %q, want only %q and no canary", stdout.String(), stderr.String(), tc.want)
			}
		})
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

// e2e holds the keys and connection the test worker signs and talks with.
type e2e struct {
	nc                          *nats.Conn
	registry                    *capsdk.ResourceRegistry
	input                       []byte
	proof, workerSign, sign, tr *ecdsa.PrivateKey
}

// TestSubmitE2E builds cap-reference-scheduler and cap-reference-submit, runs
// them with a Go test worker over embedded NATS, and checks each exit code.
func TestSubmitE2E(t *testing.T) {
	bin, dir := t.TempDir(), t.TempDir()
	schedulerBin, submitBin := build(t, bin, "cap-reference-scheduler"), build(t, bin, "cap-reference-submit")
	ns, err := tck.StartEmbeddedNATS()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ns.Close)
	f := &e2e{input: []byte(`{"question":"ping"}`)}
	if f.nc, err = nats.Connect(ns.URL()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.nc.Close)
	ctx := context.Background()
	js, err := jetstream.New(f.nc)
	if err != nil {
		t.Fatal(err)
	}
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "refsched-inputs", MaxValueSize: 65536, History: 1, Storage: jetstream.MemoryStorage})
	if err != nil {
		t.Fatal(err)
	}
	backend, err := resourcekv.New(ctx, kv)
	if err != nil {
		t.Fatal(err)
	}
	if f.registry, err = capsdk.NewResourceRegistry(nil, backend.Registration()); err != nil {
		t.Fatal(err)
	}
	clientKey, clientPub, _ := writeKeyPair(t, dir, "client")
	cfg, signPub := f.schedulerConfig(t, dir, ns.URL(), clientPub)
	startScheduler(t, schedulerBin, cfg)
	f.startWorker(t)
	input := filepath.Join(dir, "input.json")
	writeFile(t, input, f.input, 0o600)
	for _, tc := range []struct {
		name, capability, deadline string
		code                       int
		status                     string
	}{
		{"succeeded", "echo", "20s", exitSucceeded, "JOB_STATUS_SUCCEEDED"},
		{"failed", "fail", "20s", exitFailed, "JOB_STATUS_FAILED"},
		{"denied", "forbidden", "20s", exitFailed, "JOB_STATUS_DENIED"},
		{"deadline", "silent", "2s", exitDeadline, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(clientArgs(ns.URL(), clientKey, signPub, input), "-capability", tc.capability, "-deadline", tc.deadline, "-job-id", "job-"+tc.name)
			cmd := exec.Command(submitBin, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			code := 0
			if err := cmd.Run(); err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			var got summary
			if tc.status != "" {
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
					t.Fatalf("summary %q: %v", stdout.String(), err)
				}
			}
			if code != tc.code || got.Status != tc.status || (tc.status != "" && got.JobID != "job-"+tc.name) {
				t.Fatalf("exit %d summary %+v, want exit %d status %q; stderr %q", code, got, tc.code, tc.status, stderr.String())
			}
			if tc.code == exitSucceeded && (!strings.HasPrefix(got.ResultURI, "nats-kv://refsched-inputs/tenant-a/") || got.ResultSizeBytes != uint64(len(f.input))) {
				t.Fatalf("summary %+v does not name the worker's output", got)
			}
		})
	}
}

// build compiles one of this module's commands into dir.
func build(t *testing.T, dir, name string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	if b, err := exec.Command("go", "build", "-o", out, "github.com/cordum-io/cap/v2/cmd/"+name).CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", name, err, b)
	}
	return out
}

// schedulerConfig writes the scheduler's key files and configuration. Its
// allowlist has echo, fail and silent but not forbidden, and DispatchTimeout
// outlasts the deadline case.
func (f *e2e) schedulerConfig(t *testing.T, dir, natsURL, clientPub string) (cfgPath, signPub string) {
	t.Helper()
	var signKey, trustKey, proofPub, workerPub string
	signKey, signPub, f.sign = writeKeyPair(t, dir, "signing")
	trustKey, _, f.tr = writeKeyPair(t, dir, "trust")
	_, proofPub, f.proof = writeKeyPair(t, dir, "worker-proof")
	_, workerPub, f.workerSign = writeKeyPair(t, dir, "worker-sign")
	allow := []map[string]string{}
	for _, c := range []string{"echo", "fail", "silent"} {
		allow = append(allow, map[string]string{"TenantID": "tenant-a", "Topic": "job.echo", "Capability": c})
	}
	cfg := map[string]any{
		"NATSURL": natsURL, "DevPlaintextLoopback": true, "SchedulerID": "refsched-e2e",
		"SigningKeyID": "s1", "SigningKeyFile": signKey, "TrustKeyID": "t1", "TrustKeyFile": trustKey,
		"Workers": []map[string]any{{"WorkerID": "worker-1", "TenantID": "tenant-a", "AgentID": "agent-1",
			"ProofKeyID": "p1", "ProofKeyFile": proofPub, "SigningKeyFiles": map[string]string{"w1": workerPub}}},
		"Clients": []map[string]any{{"SenderID": "client-1", "TenantID": "tenant-a",
			"SigningKeyFiles": map[string]string{"c1": clientPub}}},
		"Allow": allow, "TopicWorkers": map[string][]string{"job.echo": {"worker-1"}},
		"Limits": map[string]any{"MaxInputBytes": 65536, "MaxOutputBytes": 65536, "MaxArtifacts": 8,
			"MaxAttempts": 1, "DispatchTimeout": "60s", "RunTimeout": "60s", "MaxRunTimeout": "2m"},
		"MaxPacketBytes": 1 << 16, "MaxConcurrency": 4, "MaxLiveJobs": 64, "MaxChallenges": 16,
		"ChallengeTTL": "30s", "SessionTTL": "10m",
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(dir, "scheduler.json")
	writeFile(t, cfgPath, data, 0o600)
	return cfgPath, signPub
}

// startScheduler runs the scheduler until its ready line and kills it at cleanup.
func startScheduler(t *testing.T, bin, cfg string) {
	t.Helper()
	cmd := exec.Command(bin, "-config", cfg)
	pipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	ready := make(chan struct{})
	go func() {
		for sc, seen := bufio.NewScanner(pipe), false; sc.Scan(); {
			if !seen && strings.Contains(sc.Text(), "cap-reference-scheduler: ready") {
				seen = true
				close(ready)
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(20 * time.Second):
		t.Fatal("no scheduler ready line within 20s")
	}
}

// startWorker authenticates worker-1 through the scheduler's handshake, then
// answers dispatches by capability: echo resolves the input and succeeds when
// it matches, fail publishes two terminals the client must ignore and fails,
// and silent never answers.
func (f *e2e) startWorker(t *testing.T) {
	t.Helper()
	c := capsdk.WorkerTrustConfig{WorkerID: "worker-1", ExpectedAgentID: "agent-1", TenantID: "tenant-a",
		Audience: capsdk.WorkerHandshakeAudience, ProofKeyID: "p1", ProofPrivateKey: f.proof, SDKVersion: "e2e",
		ExpectedSchedulerID: "refsched-e2e", SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"t1": &f.tr.PublicKey}}
	nonce := make([]byte, 32)
	_, _ = rand.Read(nonce)
	req, err := capsdk.BuildWorkerHandshakeChallengeRequest(&c, capsdk.WorkerHandshakeRequestOptions{RequestID: "e2e-request",
		TraceID: "e2e-trace", Purpose: agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE, ClientNonce: nonce, CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	v, err := capsdk.VerifyWorkerHandshakeChallenge(&c, req, f.exchange(t, capsdk.WorkerHandshakeChallengeSubject, req), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	auth, err := capsdk.BuildWorkerHandshakeAuthenticate(&c, v, &agentv1.Handshake{ComponentId: "worker-1",
		Role: agentv1.ComponentRole_COMPONENT_ROLE_WORKER, SdkVersion: "e2e", SupportedVersions: []int32{1}}, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s, err := capsdk.VerifyWorkerHandshakeResult(&c, v, auth, f.exchange(t, capsdk.WorkerHandshakeAuthenticateSubject, auth), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	sub, err := f.nc.Subscribe(referencescheduler.WorkerSubject("worker-1"), func(m *nats.Msg) { f.answer(m, s.Token) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	if err := f.nc.Flush(); err != nil {
		t.Fatal(err)
	}
}

func (f *e2e) exchange(t *testing.T, subject string, p *agentv1.BusPacket) *agentv1.BusPacket {
	t.Helper()
	raw, err := capsdk.MarshalWorkerTrustPacket(p)
	if err != nil {
		t.Fatal(err)
	}
	m, err := f.nc.Request(subject, raw, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if p, err = capsdk.UnmarshalWorkerTrustPacket(m.Data); err != nil {
		t.Fatal(err)
	}
	return p
}

// answer verifies one dispatch and replies on sys.job.result as worker-1.
func (f *e2e) answer(m *nats.Msg, token string) {
	v, err := capsdk.VerifyTrustedProductionPacket(m.Data, capsdk.ProductionTrustStore{Audience: m.Subject, Tenant: "tenant-a",
		Sender: "refsched-e2e", PublicKeys: map[string]*ecdsa.PublicKey{"s1": &f.sign.PublicKey}})
	if err != nil {
		return
	}
	d := v.Packet()
	req := d.GetJobRequest()
	trusted := capsdk.TrustedResourceContext{TenantID: "tenant-a", JobID: req.GetJobId()}
	r := &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED, WorkerId: "worker-1", Dispatch: req.GetDispatch()}
	switch req.GetMeta().GetCapability() {
	case "silent":
		return
	case "fail":
		// A terminal under the scheduler's name but the wrong key, and a genuine one for another job.
		f.publish(referencescheduler.SubjectTerminal, "refsched-e2e", "s1", f.workerSign, "", d.Identity, &agentv1.JobResult{JobId: req.GetJobId(), Status: r.Status})
		f.publish(referencescheduler.SubjectTerminal, "refsched-e2e", "s1", f.sign, "", d.Identity, &agentv1.JobResult{JobId: "job-other", Status: r.Status})
		r.Status = agentv1.JobStatus_JOB_STATUS_FAILED
	default:
		got, err := f.registry.Resolve(context.Background(), req.GetContextRef(), trusted)
		if err != nil || !bytes.Equal(got.Content(), f.input) {
			r.Status = agentv1.JobStatus_JOB_STATUS_FAILED
		}
		r.ResultRef, _ = f.registry.ExternalizeBytes(context.Background(), resourcekv.ResolverID, f.input, "application/json", "job-result", time.Now().Add(time.Minute), trusted)
	}
	f.publish(referencescheduler.SubjectResult, "worker-1", "w1", f.workerSign, token, d.Identity, r)
}

// publish signs a JobResult packet as sender and publishes it on subject.
func (f *e2e) publish(subject, sender, keyID string, key *ecdsa.PrivateKey, token string, id *agentv1.IdentityBinding, r *agentv1.JobResult) {
	msg := make([]byte, 16)
	_, _ = rand.Read(msg)
	p := &agentv1.BusPacket{TraceId: "e2e", SenderId: sender, ProtocolVersion: capsdk.DefaultProtocolVersion, AuthToken: token, Identity: id,
		SignatureMetadata: &agentv1.SignatureMetadata{ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm,
			MessageId: msg, Audience: subject, KeyId: keyID, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute))},
		Payload: &agentv1.BusPacket_JobResult{JobResult: r}}
	if raw, err := capsdk.SignProductionPacket(p, key); err == nil {
		_ = f.nc.Publish(subject, raw)
	}
}
