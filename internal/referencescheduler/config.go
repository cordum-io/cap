// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"time"
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
	// ErrConfigTopicWorker rejects an unknown topic worker or an allowed topic without workers.
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
	return nil, ErrConfigNATSURL
}
