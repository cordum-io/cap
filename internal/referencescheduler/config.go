// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"maps"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

// Config sentinels are bare: nothing wraps a path, file content or key bytes.
var (
	// ErrConfigNATSURL rejects an empty, unparseable or non-nats/tls URL.
	ErrConfigNATSURL = errors.New("referencescheduler: invalid NATS URL")
	// ErrConfigTLSCA rejects a missing, unreadable or empty CA bundle.
	ErrConfigTLSCA = errors.New("referencescheduler: invalid TLS CA")
	// ErrConfigTLSCert rejects a missing, unreadable or unparseable client certificate.
	ErrConfigTLSCert = errors.New("referencescheduler: invalid TLS client certificate")
	// ErrConfigTLSKey rejects a missing, unreadable or mismatched client key.
	ErrConfigTLSKey = errors.New("referencescheduler: invalid TLS client key")
	// ErrConfigPlaintextRemote rejects plaintext to a non-loopback host.
	ErrConfigPlaintextRemote = errors.New("referencescheduler: plaintext NATS to a non-loopback host")
	// ErrConfigPlaintextDev rejects loopback plaintext without the explicit dev flag.
	ErrConfigPlaintextDev = errors.New("referencescheduler: loopback plaintext needs the dev flag")
	// ErrConfigScheduler rejects a missing identity or an invalid scheduler key.
	ErrConfigScheduler = errors.New("referencescheduler: invalid scheduler identity")
	// ErrConfigEmptyTrustStore rejects an empty worker trust store.
	ErrConfigEmptyTrustStore = errors.New("referencescheduler: empty worker trust store")
	// ErrConfigWorkerID rejects a worker id that is not a single NATS subject token.
	ErrConfigWorkerID = errors.New("referencescheduler: invalid worker id")
	// ErrConfigWorker rejects an invalid or duplicate worker entry.
	ErrConfigWorker = errors.New("referencescheduler: invalid worker entry")
	// ErrConfigEmptyClients rejects an empty client identity set.
	ErrConfigEmptyClients = errors.New("referencescheduler: empty client set")
	// ErrConfigClient rejects an invalid or duplicate client entry.
	ErrConfigClient = errors.New("referencescheduler: invalid client entry")
	// ErrConfigEmptyAllowlist rejects an empty allowlist.
	ErrConfigEmptyAllowlist = errors.New("referencescheduler: empty allowlist")
	// ErrConfigAllowRule rejects an allow rule with a blank or untrimmed field.
	ErrConfigAllowRule = errors.New("referencescheduler: invalid allow rule")
	// ErrConfigTopicWorker rejects an unknown topic worker or an allow rule with no same-tenant worker.
	ErrConfigTopicWorker = errors.New("referencescheduler: invalid topic worker")
	// ErrConfigBounds rejects a runtime bound outside its allowed range.
	ErrConfigBounds = errors.New("referencescheduler: invalid runtime bounds")
)

// RuntimeConfig is the file-level runtime configuration; paths name PEM files.
type RuntimeConfig struct {
	NATSURL                                                    string
	TLSCAFile, TLSCertFile, TLSKeyFile                         string
	DevPlaintextLoopback                                       bool // plaintext only to a loopback host
	SchedulerID                                                string
	SigningKeyID, SigningKeyFile                               string // production-signing P-256 private key
	TrustKeyID, TrustKeyFile                                   string // worker-trust signing P-256 private key
	Workers                                                    []WorkerEntry
	Clients                                                    []ClientEntry
	Allow                                                      []AllowRule
	TopicWorkers                                               map[string][]string // topic -> worker ids
	Limits                                                     Limits
	MaxPacketBytes, MaxConcurrency, MaxLiveJobs, MaxChallenges int
	ChallengeTTL, SessionTTL                                   time.Duration
}

// WorkerEntry is one trusted worker; the files hold P-256 public-key PEMs.
type WorkerEntry struct {
	WorkerID, TenantID, AgentID string
	ProofKeyID, ProofKeyFile    string
	SigningKeyFiles             map[string]string // production key id -> public-key file
}

// ClientEntry is one client allowed to submit and cancel.
type ClientEntry struct {
	SenderID, TenantID string
	SigningKeyFiles    map[string]string // production key id -> public-key file
}

// ResolvedWorker is a WorkerEntry with parsed keys.
type ResolvedWorker struct {
	WorkerID, TenantID, AgentID, ProofKeyID string
	ProofKey                                *ecdsa.PublicKey
	SigningKeys                             map[string]*ecdsa.PublicKey
}

// ResolvedClient is a ClientEntry with parsed keys.
type ResolvedClient struct {
	SenderID, TenantID string
	SigningKeys        map[string]*ecdsa.PublicKey
}

// ResolvedConfig is the validated configuration; it never aliases the input.
type ResolvedConfig struct {
	NATSURL                                                    string
	TLS                                                        *tls.Config // nil only in the validated loopback dev case
	SchedulerID                                                string
	SigningKeyID                                               string
	SigningKey                                                 *ecdsa.PrivateKey
	TrustKeyID                                                 string
	TrustKey                                                   *ecdsa.PrivateKey
	Workers                                                    map[string]ResolvedWorker // by worker id
	Clients                                                    map[string]ResolvedClient // by sender id
	Allow                                                      []AllowRule
	TopicWorkers                                               map[string][]string
	Limits                                                     Limits
	MaxPacketBytes, MaxConcurrency, MaxLiveJobs, MaxChallenges int
	ChallengeTTL, SessionTTL                                   time.Duration
}

// Resolve validates the configuration and returns it with parsed keys, or a
// bare ErrConfig* sentinel. It reads files but never logs.
func (c RuntimeConfig) Resolve() (*ResolvedConfig, error) {
	u, err := url.Parse(c.NATSURL)
	if err != nil || (u.Scheme != "nats" && u.Scheme != "tls") || u.Hostname() == "" {
		return nil, ErrConfigNATSURL
	}
	r := &ResolvedConfig{
		NATSURL: c.NATSURL, SchedulerID: c.SchedulerID, SigningKeyID: c.SigningKeyID, TrustKeyID: c.TrustKeyID,
		Limits: c.Limits, MaxPacketBytes: c.MaxPacketBytes, MaxConcurrency: c.MaxConcurrency, MaxLiveJobs: c.MaxLiveJobs,
		MaxChallenges: c.MaxChallenges, ChallengeTTL: c.ChallengeTTL, SessionTTL: c.SessionTTL,
	}
	steps := []func() error{
		func() (err error) { r.TLS, err = c.resolveTLS(u.Hostname()); return },
		func() error { return c.resolveScheduler(r) },
		func() (err error) { r.Workers, err = resolveWorkers(c.Workers); return },
		func() (err error) { r.Clients, err = resolveClients(c.Clients); return },
		func() (err error) { r.Allow, err = resolveAllow(c.Allow); return },
		func() (err error) { r.TopicWorkers, err = resolveTopics(c.TopicWorkers, r.Workers, r.Allow); return },
		c.checkBounds,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// resolveTLS builds the client TLS config. Plaintext passes only to a parsed
// loopback host with the dev flag; any TLS path makes all three required.
func (c RuntimeConfig) resolveTLS(host string) (*tls.Config, error) {
	if c.TLSCAFile == "" && c.TLSCertFile == "" && c.TLSKeyFile == "" {
		switch {
		case !isLoopback(host):
			return nil, ErrConfigPlaintextRemote
		case !c.DevPlaintextLoopback:
			return nil, ErrConfigPlaintextDev
		}
		return nil, nil
	}
	caPEM, err := os.ReadFile(c.TLSCAFile)
	pool := x509.NewCertPool()
	if err != nil || !pool.AppendCertsFromPEM(caPEM) {
		return nil, ErrConfigTLSCA
	}
	certPEM, err := os.ReadFile(c.TLSCertFile)
	if err != nil || !firstCertParses(certPEM) {
		return nil, ErrConfigTLSCert
	}
	keyPEM, err := os.ReadFile(c.TLSKeyFile)
	if err != nil {
		return nil, ErrConfigTLSKey
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, ErrConfigTLSKey
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, Certificates: []tls.Certificate{pair}}, nil
}

// isLoopback tests the parsed URL host: exactly "localhost" or a loopback IP literal.
func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

func firstCertParses(data []byte) bool {
	for {
		var b *pem.Block
		if b, data = pem.Decode(data); b == nil {
			return false
		}
		if b.Type == "CERTIFICATE" {
			_, err := x509.ParseCertificate(b.Bytes)
			return err == nil
		}
	}
}

func (c RuntimeConfig) resolveScheduler(r *ResolvedConfig) error {
	r.SigningKey, r.TrustKey = readPrivateKey(c.SigningKeyFile), readPrivateKey(c.TrustKeyFile)
	if !trimmed(c.SchedulerID) || !trimmed(c.SigningKeyID) || !trimmed(c.TrustKeyID) || r.SigningKey == nil || r.TrustKey == nil {
		return ErrConfigScheduler
	}
	return nil
}

func resolveWorkers(entries []WorkerEntry) (map[string]ResolvedWorker, error) {
	if len(entries) == 0 {
		return nil, ErrConfigEmptyTrustStore
	}
	out := make(map[string]ResolvedWorker, len(entries))
	for _, w := range entries {
		if !validWorkerID(w.WorkerID) {
			return nil, ErrConfigWorkerID
		}
		_, dup := out[w.WorkerID]
		proof, sign := readPublicKey(w.ProofKeyFile), publicKeys(w.SigningKeyFiles)
		if dup || !trimmed(w.TenantID) || !trimmed(w.AgentID) || !trimmed(w.ProofKeyID) || proof == nil || sign == nil {
			return nil, ErrConfigWorker
		}
		out[w.WorkerID] = ResolvedWorker{
			WorkerID: w.WorkerID, TenantID: w.TenantID, AgentID: w.AgentID,
			ProofKeyID: w.ProofKeyID, ProofKey: proof, SigningKeys: sign,
		}
	}
	return out, nil
}

func resolveClients(entries []ClientEntry) (map[string]ResolvedClient, error) {
	if len(entries) == 0 {
		return nil, ErrConfigEmptyClients
	}
	out := make(map[string]ResolvedClient, len(entries))
	for _, e := range entries {
		_, dup := out[e.SenderID]
		keys := publicKeys(e.SigningKeyFiles)
		if dup || !trimmed(e.SenderID) || !trimmed(e.TenantID) || keys == nil {
			return nil, ErrConfigClient
		}
		out[e.SenderID] = ResolvedClient{SenderID: e.SenderID, TenantID: e.TenantID, SigningKeys: keys}
	}
	return out, nil
}

func resolveAllow(rules []AllowRule) ([]AllowRule, error) {
	if len(rules) == 0 {
		return nil, ErrConfigEmptyAllowlist
	}
	for _, r := range rules {
		if !trimmed(r.TenantID) || !trimmed(r.Topic) || !trimmed(r.Capability) {
			return nil, ErrConfigAllowRule
		}
	}
	return slices.Clone(rules), nil
}

// resolveTopics checks every listed worker against the trust store and every
// allow rule for a same-tenant worker. Topics are visited in sorted order so
// that the reported class does not depend on map iteration.
func resolveTopics(topics map[string][]string, workers map[string]ResolvedWorker, allow []AllowRule) (map[string][]string, error) {
	out := make(map[string][]string, len(topics))
	for _, topic := range slices.Sorted(maps.Keys(topics)) {
		for _, id := range topics[topic] {
			if !validWorkerID(id) {
				return nil, ErrConfigWorkerID
			}
			if _, ok := workers[id]; !ok {
				return nil, ErrConfigTopicWorker
			}
		}
		out[topic] = slices.Clone(topics[topic])
	}
	// Worker sessions are tenant-bound: an allowed (tenant, topic) needs a worker of
	// that tenant, or its jobs would stay SCHEDULED forever (the core has no timeout).
	for _, rule := range allow {
		if !slices.ContainsFunc(out[rule.Topic], func(id string) bool { return workers[id].TenantID == rule.TenantID }) {
			return nil, ErrConfigTopicWorker
		}
	}
	return out, nil
}

func (c RuntimeConfig) checkBounds() error {
	if c.MaxPacketBytes <= 0 || c.MaxPacketBytes > capsdk.DefaultProductionMaxRawBytes || c.MaxConcurrency <= 0 || c.MaxLiveJobs <= 0 {
		return ErrConfigBounds
	}
	return nil
}

func trimmed(s string) bool { return s != "" && s == strings.TrimSpace(s) }

// validWorkerID reports whether id is one NATS subject token, because
// WorkerSubject concatenates it verbatim.
func validWorkerID(id string) bool {
	if id == "" || len(id) > maxIDBytes || !utf8.ValidString(id) || strings.ContainsAny(id, ".*>") {
		return false
	}
	return !strings.ContainsFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

// publicKeys parses a key id -> file map; nil means empty, a blank id or a bad key.
func publicKeys(files map[string]string) map[string]*ecdsa.PublicKey {
	if len(files) == 0 {
		return nil
	}
	out := make(map[string]*ecdsa.PublicKey, len(files))
	for id, path := range files {
		if k := readPublicKey(path); trimmed(id) && k != nil {
			out[id] = k
		} else {
			return nil
		}
	}
	return out
}

func readBlock(path string) *pem.Block {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	b, _ := pem.Decode(data)
	return b
}

// readPrivateKey returns the P-256 key of an EC PRIVATE KEY or PKCS8 PEM file, or nil.
func readPrivateKey(path string) *ecdsa.PrivateKey {
	b := readBlock(path)
	if b == nil {
		return nil
	}
	var k *ecdsa.PrivateKey
	switch b.Type {
	case "EC PRIVATE KEY":
		k, _ = x509.ParseECPrivateKey(b.Bytes)
	case "PRIVATE KEY":
		if a, err := x509.ParsePKCS8PrivateKey(b.Bytes); err == nil {
			k, _ = a.(*ecdsa.PrivateKey)
		}
	}
	if k == nil || k.Curve != elliptic.P256() {
		return nil
	}
	return k
}

// readPublicKey returns the P-256 key of a PUBLIC KEY PEM file, or nil.
func readPublicKey(path string) *ecdsa.PublicKey {
	b := readBlock(path)
	if b == nil || b.Type != "PUBLIC KEY" {
		return nil
	}
	a, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		return nil
	}
	k, _ := a.(*ecdsa.PublicKey)
	if k == nil || k.Curve != elliptic.P256() {
		return nil
	}
	return k
}
