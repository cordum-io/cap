// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/internal/tck"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

// TestNATSE2E runs the runtime against a real nats-server, over plaintext
// loopback and over TLS with verified client certificates. It never skips.
func TestNATSE2E(t *testing.T) {
	flows := []struct {
		name string
		run  func(t *testing.T, tlsOn bool)
	}{
		{"fast-result", e2eFastResult}, {"redelivered-submit", e2eRedeliveredSubmit},
		{"redelivered-result", e2eRedeliveredResult}, {"cancel-after-result", e2eCancelAfterResult},
		{"result-after-cancel", e2eResultAfterCancel}, {"cancel-race", e2eCancelRace},
	}
	for _, transport := range []struct {
		name  string
		tlsOn bool
	}{{"plaintext", false}, {"tls", true}} {
		t.Run(transport.name, func(t *testing.T) {
			for _, flow := range flows {
				t.Run(flow.name, func(t *testing.T) { flow.run(t, transport.tlsOn) })
			}
			if transport.tlsOn {
				t.Run("untrusted-client-refused", e2eUntrustedClient)
			}
		})
	}
}

// e2ePKI names PEM files in a per-test directory: a CA with server and client
// certificates, and an unrelated CA's rogue client certificate.
type e2ePKI struct{ ca, serverCert, serverKey, cert, key, rogueCert, rogueKey string }

func e2eWritePKI(t *testing.T) *e2ePKI {
	t.Helper()
	dir := t.TempDir()
	ca, caKey := e2eIssue(t, dir, "ca", &x509.Certificate{IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}, nil, nil)
	leaf := func(usage x509.ExtKeyUsage) *x509.Certificate {
		return &x509.Certificate{KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	}
	serverLeaf := leaf(x509.ExtKeyUsageServerAuth)
	serverLeaf.IPAddresses, serverLeaf.DNSNames = []net.IP{net.ParseIP("127.0.0.1")}, []string{"localhost"}
	e2eIssue(t, dir, "server", serverLeaf, ca, caKey)
	e2eIssue(t, dir, "client", leaf(x509.ExtKeyUsageClientAuth), ca, caKey)
	rogueCA, rogueKey := e2eIssue(t, dir, "rogue-ca", &x509.Certificate{IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}, nil, nil)
	e2eIssue(t, dir, "rogue", leaf(x509.ExtKeyUsageClientAuth), rogueCA, rogueKey)
	file := func(name string) string { return filepath.Join(dir, name) }
	return &e2ePKI{ca: file("ca.pem"), serverCert: file("server.pem"), serverKey: file("server-key.pem"),
		cert: file("client.pem"), key: file("client-key.pem"), rogueCert: file("rogue.pem"), rogueKey: file("rogue-key.pem")}
}

// e2eIssue writes name.pem and name-key.pem; a nil parent self-signs.
func e2eIssue(t *testing.T, dir, name string, tmpl, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.SerialNumber, err = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127)); err != nil {
		t.Fatal(err)
	}
	tmpl.Subject = pkix.Name{CommonName: "refsched-e2e-" + name}
	tmpl.NotBefore, tmpl.NotAfter = time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	if parent == nil {
		parent, parentKey = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	for file, block := range map[string]*pem.Block{name + ".pem": {Type: "CERTIFICATE", Bytes: der}, name + "-key.pem": {Type: "PRIVATE KEY", Bytes: keyDER}} {
		if err := os.WriteFile(filepath.Join(dir, file), pem.EncodeToMemory(block), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return cert, key
}

// clientTLS resolves a client TLS config with the code RuntimeConfig.Resolve uses.
func (p *e2ePKI) clientTLS(t *testing.T, cert, key string) *tls.Config {
	t.Helper()
	tc, err := RuntimeConfig{TLSCAFile: p.ca, TLSCertFile: cert, TLSKeyFile: key}.resolveTLS("127.0.0.1")
	if err != nil || tc == nil {
		t.Fatalf("resolveTLS: %v", err)
	}
	return tc
}

// e2eTLSServer starts a nats-server that requires and verifies client
// certificates issued by the test CA, and returns its tls:// URL.
func e2eTLSServer(t *testing.T, p *e2ePKI) string {
	t.Helper()
	tc, err := server.GenTLSConfig(&server.TLSConfigOpts{CertFile: p.serverCert, KeyFile: p.serverKey, CaFile: p.ca, Verify: true})
	if err != nil {
		t.Fatal(err)
	}
	if tc.ClientAuth != tls.RequireAndVerifyClientCert || tc.InsecureSkipVerify {
		t.Fatal("TLS server does not verify client certificates")
	}
	s, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true,
		TLS: true, TLSVerify: true, TLSConfig: tc, TLSTimeout: 2})
	if err != nil {
		t.Fatal(err)
	}
	go s.Start()
	t.Cleanup(func() { s.Shutdown(); s.WaitForShutdown() })
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("TLS nats-server not ready")
	}
	return s.ClientURL()
}

// e2eDial opens one more client connection, over TLS when p is set.
func e2eDial(t *testing.T, f *rtFixture, p *e2ePKI) *nats.Conn {
	t.Helper()
	var opts []nats.Option
	if p != nil {
		opts = append(opts, nats.Secure(p.clientTLS(t, p.cert, p.key)))
	}
	nc, err := nats.Connect(f.cfg.NATSURL, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	return nc
}

// e2eStart mirrors rtStart on a fresh server: plaintext loopback through
// tck.StartEmbeddedNATS, or TLS with client certificates from a per-test CA.
func e2eStart(t *testing.T, tlsOn bool) (*rtFixture, *e2ePKI) {
	t.Helper()
	f := rtConfig(t)
	f.cfg.MaxLiveJobs = 64 // a process-lifetime bound; the race admits 20 jobs
	var p *e2ePKI
	if tlsOn {
		p = e2eWritePKI(t)
		f.cfg.NATSURL, f.cfg.TLS = e2eTLSServer(t, p), p.clientTLS(t, p.cert, p.key)
	} else {
		srv, err := tck.StartEmbeddedNATS()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(srv.Close)
		f.srv, f.cfg.NATSURL = srv, srv.URL()
	}
	f.nc = e2eDial(t, f, p)
	var err error
	for _, subject := range []string{WorkerSubject("w1"), WorkerSubject("w2"), WorkerSubject("w3"), SubjectTerminal, SubjectCancel} {
		if f.subs[subject], err = f.nc.SubscribeSync(subject); err != nil {
			t.Fatal(err)
		}
	}
	if err = f.nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	if f.r, err = NewRuntime(RuntimeOptions{Config: f.cfg, Now: f.clock, TickEvery: time.Hour, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel, f.done = cancel, make(chan error, 1)
	go func() { f.done <- f.r.Run(ctx) }()
	t.Cleanup(func() { rtStop(t, f) })
	select {
	case <-f.r.Ready():
	case err = <-f.done:
		f.done = nil
		t.Fatalf("Run before Ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Ready timeout")
	}
	return f, p
}

// e2eSettle waits for the exact cumulative Stats; the observer can see a
// message before the runtime counts its publish.
func e2eSettle(t *testing.T, f *rtFixture, want RuntimeStats) {
	t.Helper()
	var got RuntimeStats
	defer func() {
		if got != want {
			t.Logf("stats %+v, want %+v", got, want)
		}
	}()
	rtEventually(t, func() bool { got = f.r.Stats(); return got == want })
}

// e2eNonePending flushes the observer and requires empty subscriptions.
func e2eNonePending(t *testing.T, f *rtFixture, subjects ...string) {
	t.Helper()
	if err := f.nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	for _, subject := range subjects {
		if n, _, err := f.subs[subject].Pending(); err != nil || n != 0 {
			t.Fatalf("%s: %d pending, %v", subject, n, err)
		}
	}
}

func e2eStatus(t *testing.T, f *rtFixture, job string, want agentv1.JobStatus) Job {
	t.Helper()
	j, err := f.r.core.Get("t1", job)
	if err != nil || j.Status != want {
		t.Fatalf("job %s: status %v, want %v (%v)", job, j.Status, want, err)
	}
	return j
}

// e2eReply plays a worker that answers a dispatch at once, without waiting for
// the runtime's AckDispatch. It reports errors instead of failing the test.
func e2eReply(nc *nats.Conn, raw []byte, trust capsdk.ProductionTrustStore, key *ecdsa.PrivateKey, templates <-chan *agentv1.BusPacket) error {
	v, err := capsdk.VerifyTrustedProductionPacket(raw, trust)
	if err != nil {
		return err
	}
	var p *agentv1.BusPacket
	select {
	case tpl := <-templates:
		p = proto.Clone(tpl).(*agentv1.BusPacket)
	case <-time.After(time.Second):
		return errors.New("no result template")
	}
	d := v.Packet()
	if d.GetJobRequest().GetJobId() != p.GetJobResult().GetJobId() {
		return errors.New("dispatch for an unexpected job")
	}
	p.Identity = clone(d.Identity)
	p.GetJobResult().Dispatch = clone(d.GetJobRequest().Dispatch)
	out, err := capsdk.SignProductionPacket(p, key)
	if err != nil {
		return err
	}
	return nc.Publish(SubjectResult, out)
}

func e2eFastResult(t *testing.T, tlsOn bool) {
	f, p := e2eStart(t, tlsOn)
	token := rtHandshake(t, f, "w1")
	worker := e2eDial(t, f, p)
	templates, errs := make(chan *agentv1.BusPacket, 1), make(chan error, 8)
	trust := capsdk.ProductionTrustStore{Audience: WorkerSubject("w1"), Tenant: "t1", Sender: f.cfg.SchedulerID,
		PublicKeys: map[string]*ecdsa.PublicKey{"k1": &f.cfg.SigningKey.PublicKey}, Now: f.clock}
	key := f.keys["w1"]
	if _, err := worker.Subscribe(WorkerSubject("w1"), func(m *nats.Msg) {
		if err := e2eReply(worker, m.Data, trust, key, templates); err != nil {
			select {
			case errs <- err:
			default:
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	jobs := []string{"fast-0", "fast-1", "fast-2", "fast-3", "fast-4"}
	for _, id := range jobs {
		templates <- rtPacket(t, "w1", token, SubjectResult, id)
		rtSend(t, f, SubjectSubmit, rtPacket(t, "c1", "", SubjectSubmit, id))
		if r := rtReceive(t, f, SubjectTerminal).GetJobResult(); r.GetJobId() != id || r.GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
			t.Fatalf("terminal for %s: %v", id, r)
		}
	}
	e2eSettle(t, f, RuntimeStats{0, 20, 10})
	for _, id := range jobs {
		if j := e2eStatus(t, f, id, agentv1.JobStatus_JOB_STATUS_SUCCEEDED); !j.Acked {
			t.Fatalf("job %s: dispatch not acknowledged before the reply applied", id)
		}
	}
	select {
	case err := <-errs:
		t.Fatalf("worker: %v", err)
	default:
	}
	e2eSettle(t, f, RuntimeStats{0, 20, 10})
}

func e2eRedeliveredSubmit(t *testing.T, tlsOn bool) {
	f, _ := e2eStart(t, tlsOn)
	rtHandshake(t, f, "w1")
	raw := adSign(t, f.keys["c1"], rtPacket(t, "c1", "", SubjectSubmit, "job"))
	rtRaw(t, f, SubjectSubmit, raw)
	rtReceive(t, f, WorkerSubject("w1"))
	rtEventually(t, func() bool { j, _ := f.r.core.Get("t1", "job"); return j.Acked })
	e2eSettle(t, f, RuntimeStats{0, 3, 1})
	before, _ := f.r.core.Get("t1", "job")
	rtRaw(t, f, SubjectSubmit, raw)
	e2eSettle(t, f, RuntimeStats{1, 3, 1})
	if after, _ := f.r.core.Get("t1", "job"); !reflect.DeepEqual(before, after) {
		t.Fatalf("redelivered submit changed the job: %+v", after)
	}
	e2eNonePending(t, f, WorkerSubject("w1"))
	e2eSettle(t, f, RuntimeStats{1, 3, 1})
}

func e2eRedeliveredResult(t *testing.T, tlsOn bool) {
	f, _ := e2eStart(t, tlsOn)
	token := rtHandshake(t, f, "w1")
	raw := adSign(t, f.keys["w1"], rtResult(t, rtDispatch(t, f, "job"), token))
	rtRaw(t, f, SubjectResult, raw)
	if r := rtReceive(t, f, SubjectTerminal).GetJobResult(); r.GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("terminal: %v", r)
	}
	e2eSettle(t, f, RuntimeStats{0, 4, 2})
	before, _ := f.r.core.Get("t1", "job")
	rtRaw(t, f, SubjectResult, raw)
	e2eSettle(t, f, RuntimeStats{1, 4, 2})
	if after, _ := f.r.core.Get("t1", "job"); !reflect.DeepEqual(before, after) {
		t.Fatalf("redelivered result changed the job: %+v", after)
	}
	e2eNonePending(t, f, SubjectTerminal)
	e2eSettle(t, f, RuntimeStats{1, 4, 2})
}

func e2eCancelAfterResult(t *testing.T, tlsOn bool) {
	f, _ := e2eStart(t, tlsOn)
	token := rtHandshake(t, f, "w1")
	rtSend(t, f, SubjectResult, rtResult(t, rtDispatch(t, f, "job"), token))
	winner := rtReceive(t, f, SubjectTerminal).GetJobResult()
	if winner.GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("terminal: %v", winner)
	}
	e2eSettle(t, f, RuntimeStats{0, 4, 2})
	rtSend(t, f, SubjectClientCancel, rtPacket(t, "c1", "", SubjectClientCancel, "job"))
	if again := rtReceive(t, f, SubjectTerminal).GetJobResult(); !proto.Equal(again, winner) {
		t.Fatalf("cancel after result republished %v, want the winner %v", again, winner)
	}
	e2eSettle(t, f, RuntimeStats{0, 5, 3})
	e2eStatus(t, f, "job", agentv1.JobStatus_JOB_STATUS_SUCCEEDED)
	e2eNonePending(t, f, SubjectCancel, SubjectTerminal)
	e2eSettle(t, f, RuntimeStats{0, 5, 3})
}

func e2eResultAfterCancel(t *testing.T, tlsOn bool) {
	f, _ := e2eStart(t, tlsOn)
	token := rtHandshake(t, f, "w1")
	d := rtDispatch(t, f, "job")
	rtSend(t, f, SubjectClientCancel, rtPacket(t, "c1", "", SubjectClientCancel, "job"))
	if r := rtReceive(t, f, SubjectTerminal).GetJobResult(); r.GetJobId() != "job" || r.GetStatus() != agentv1.JobStatus_JOB_STATUS_CANCELLED {
		t.Fatalf("terminal: %v", r)
	}
	e2eFenced(t, rtReceive(t, f, SubjectCancel), d)
	e2eSettle(t, f, RuntimeStats{0, 4, 3})
	rtSend(t, f, SubjectResult, rtResult(t, d, token))
	e2eSettle(t, f, RuntimeStats{1, 5, 3})
	e2eStatus(t, f, "job", agentv1.JobStatus_JOB_STATUS_CANCELLED)
	e2eNonePending(t, f, SubjectCancel, SubjectTerminal)
	e2eSettle(t, f, RuntimeStats{1, 5, 3})
}

// e2eFenced requires a sys.job.cancel for the dispatched job, fenced to its dispatch.
func e2eFenced(t *testing.T, cancel, dispatch *agentv1.BusPacket) {
	t.Helper()
	c := cancel.GetJobCancel()
	if c.GetJobId() != dispatch.GetJobRequest().GetJobId() || !proto.Equal(c.GetDispatch(), dispatch.GetJobRequest().GetDispatch()) ||
		!proto.Equal(cancel.GetIdentity(), dispatch.GetIdentity()) {
		t.Fatalf("cancel %v is not fenced to dispatch %v", cancel, dispatch.GetJobRequest())
	}
}

// e2ePublishAll publishes every message, then flushes; the error goes to the caller.
func e2ePublishAll(nc *nats.Conn, subject string, raws [][]byte) error {
	for _, raw := range raws {
		if err := nc.Publish(subject, raw); err != nil {
			return err
		}
	}
	return nc.FlushTimeout(2 * time.Second)
}

// e2eCollect requires exactly want verified messages on subject and groups them by job.
func e2eCollect(t *testing.T, f *rtFixture, subject string, want int) map[string][]*agentv1.BusPacket {
	t.Helper()
	if n, _, err := f.subs[subject].Pending(); err != nil || n != want {
		t.Fatalf("%s: %d pending, want %d (%v)", subject, n, want, err)
	}
	byJob := map[string][]*agentv1.BusPacket{}
	for range want {
		p := rtReceive(t, f, subject)
		job := p.GetJobResult().GetJobId() + p.GetJobCancel().GetJobId()
		byJob[job] = append(byJob[job], p)
	}
	return byJob
}

func e2eCancelRace(t *testing.T, tlsOn bool) {
	const n = 20
	f, p := e2eStart(t, tlsOn)
	token := rtHandshake(t, f, "w1")
	ids, dispatches := make([]string, n), map[string]*agentv1.BusPacket{}
	results, cancels := make([][]byte, n), make([][]byte, n)
	for i := range n {
		ids[i] = fmt.Sprint("race-", i)
		dispatches[ids[i]] = rtDispatch(t, f, ids[i])
		results[i] = adSign(t, f.keys["w1"], rtResult(t, dispatches[ids[i]], token))
		cancels[i] = adSign(t, f.keys["c1"], rtPacket(t, "c1", "", SubjectClientCancel, ids[i]))
	}
	e2eSettle(t, f, RuntimeStats{0, 3 * n, n})
	start, errs := make(chan struct{}), make(chan error, 2)
	for subject, raws := range map[string][][]byte{SubjectResult: results, SubjectClientCancel: cancels} {
		nc := e2eDial(t, f, p)
		go func() { <-start; errs <- e2ePublishAll(nc, subject, raws) }()
	}
	close(start)
	for range 2 {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("publisher timeout")
		}
	}
	cancelled := func() (c uint64) {
		for _, id := range ids {
			if j, _ := f.r.core.Get("t1", id); j.Status == agentv1.JobStatus_JOB_STATUS_CANCELLED {
				c++
			}
		}
		return c
	}
	// Per job: 2 core calls and 2 publishes whichever packet wins, plus 1 drop when the cancel wins.
	rtEventually(t, func() bool {
		s := f.r.Stats()
		return s.CoreCalls == 5*n && s.Published == 3*n && s.Dropped == cancelled()
	})
	rtStop(t, f)
	c := cancelled()
	if got, want := f.r.Stats(), (RuntimeStats{c, 5 * n, 3 * n}); got != want {
		t.Fatalf("stats %+v, want %+v", got, want)
	}
	if err := f.nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	e2eRaceWinners(t, f, ids, dispatches, int(c))
	e2eDrained(t, f)
	t.Logf("cancel-race split: %d SUCCEEDED, %d CANCELLED", n-int(c), c)
}

// e2eRaceWinners requires one winner per job: a SUCCEEDED winner announced
// twice and never cancelled, or a CANCELLED winner announced once with one
// fenced sys.job.cancel.
func e2eRaceWinners(t *testing.T, f *rtFixture, ids []string, dispatches map[string]*agentv1.BusPacket, c int) {
	t.Helper()
	terminals := e2eCollect(t, f, SubjectTerminal, 2*(len(ids)-c)+c)
	cancels := e2eCollect(t, f, SubjectCancel, c)
	for _, id := range ids {
		j, _ := f.r.core.Get("t1", id)
		switch j.Status {
		case agentv1.JobStatus_JOB_STATUS_SUCCEEDED:
			if len(terminals[id]) != 2 || len(cancels[id]) != 0 {
				t.Fatalf("%s SUCCEEDED: %d terminals, %d cancels", id, len(terminals[id]), len(cancels[id]))
			}
			for _, p := range terminals[id] {
				if !proto.Equal(p.GetJobResult(), j.Result) {
					t.Fatalf("%s: terminal %v, winner %v", id, p.GetJobResult(), j.Result)
				}
			}
		case agentv1.JobStatus_JOB_STATUS_CANCELLED:
			if len(terminals[id]) != 1 || terminals[id][0].GetJobResult().GetStatus() != j.Status || len(cancels[id]) != 1 {
				t.Fatalf("%s CANCELLED: %d terminals, %d cancels", id, len(terminals[id]), len(cancels[id]))
			}
			e2eFenced(t, cancels[id][0], dispatches[id])
		default:
			t.Fatalf("%s: no single winner, status %v", id, j.Status)
		}
	}
}

// e2eDrained requires closed subscriptions, a closed connection and no runtime goroutine.
func e2eDrained(t *testing.T, f *rtFixture) {
	t.Helper()
	for _, sub := range f.r.subs {
		if sub.IsValid() {
			t.Fatal("subscription remains live after drain")
		}
	}
	if !f.r.nc.IsClosed() || f.r.goroutines.Load() != 0 {
		t.Fatalf("after drain: closed=%v goroutines=%d", f.r.nc.IsClosed(), f.r.goroutines.Load())
	}
}

func e2eUntrustedClient(t *testing.T) {
	f, p := e2eStart(t, true)
	rogue := rtConfig(t)
	rogue.cfg.NATSURL, rogue.cfg.TLS = f.cfg.NATSURL, p.clientTLS(t, p.rogueCert, p.rogueKey)
	r, err := NewRuntime(RuntimeOptions{Config: rogue.cfg, Now: rogue.clock, TickEvery: time.Hour, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	select {
	case err := <-done:
		if err != ErrRuntimeTransport {
			t.Fatalf("untrusted client certificate: Run returned %v, want ErrRuntimeTransport", err)
		}
	case <-r.Ready():
		t.Fatal("untrusted client certificate reached Ready")
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not refuse the untrusted client certificate")
	}
	select {
	case <-r.Ready():
		t.Fatal("Ready closed for an untrusted client certificate")
	default:
	}
}
