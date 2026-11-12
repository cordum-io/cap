package referencescheduler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

// TrustedWorker is one configured worker's handshake trust entry.
type TrustedWorker struct {
	WorkerID, TenantID, AgentID, ProofKeyID string
	ProofKey                                *ecdsa.PublicKey // P-256
}

// IssuerConfig supplies resolved trust values; Now must be concurrency-safe.
type IssuerConfig struct {
	SchedulerID, ServerKeyID, Audience string
	ServerKey                          *ecdsa.PrivateKey // P-256
	Workers                            []TrustedWorker
	Now                                func() time.Time
	ChallengeTTL, SessionTTL           time.Duration
	MaxChallenges                      int
}

// Session is the authenticated identity associated with a live token.
type Session struct {
	WorkerID, TenantID, AgentID string
	ExpiresAt                   time.Time
}

// Issuer is process-local authority, not suitable for multiple replicas.
type Issuer struct {
	cfg        IssuerConfig
	workers    map[string]TrustedWorker
	mu         sync.Mutex
	challenges map[string]*agentv1.WorkerHandshakeChallenge
	sessions   map[string]Session
	tokens     map[string]string
}

var (
	// ErrHandshakeTooLarge rejects oversized input before decoding.
	ErrHandshakeTooLarge = errors.New("referencescheduler: handshake packet too large")
	// ErrHandshakeRefused drops a request without disclosing its contents.
	ErrHandshakeRefused = errors.New("referencescheduler: handshake refused")
)

// NewIssuer validates and snapshots the configured trust entries.
func NewIssuer(cfg IssuerConfig) (*Issuer, error) {
	if !issuerStrings(cfg.SchedulerID, cfg.ServerKeyID, cfg.Audience) || cfg.Now == nil ||
		cfg.ChallengeTTL <= 0 || cfg.ChallengeTTL > capsdk.WorkerHandshakeMaxLifetime ||
		cfg.SessionTTL <= 0 || cfg.MaxChallenges <= 0 || len(cfg.Workers) == 0 || !issuerPrivateKey(cfg.ServerKey) {
		return nil, ErrInvalidConfig
	}
	workers := make(map[string]TrustedWorker, len(cfg.Workers))
	for _, w := range cfg.Workers {
		if _, exists := workers[w.WorkerID]; exists || !issuerStrings(w.WorkerID, w.TenantID, w.AgentID, w.ProofKeyID) || !issuerPublicKey(w.ProofKey) {
			return nil, ErrInvalidConfig
		}
		w.ProofKey = issuerCopyKey(w.ProofKey)
		workers[w.WorkerID] = w
	}
	cfg.ServerKey = &ecdsa.PrivateKey{PublicKey: *issuerCopyKey(&cfg.ServerKey.PublicKey), D: new(big.Int).Set(cfg.ServerKey.D)}
	cfg.Workers = nil
	return &Issuer{cfg: cfg, workers: workers, challenges: make(map[string]*agentv1.WorkerHandshakeChallenge),
		sessions: make(map[string]Session), tokens: make(map[string]string)}, nil
}

func issuerStrings(values ...string) bool {
	for _, v := range values {
		if v == "" || v != strings.TrimSpace(v) {
			return false
		}
	}
	return true
}

func issuerPublicKey(k *ecdsa.PublicKey) bool {
	return k != nil && k.Curve == elliptic.P256() && k.X != nil && k.Y != nil && k.Curve.IsOnCurve(k.X, k.Y)
}

func issuerPrivateKey(k *ecdsa.PrivateKey) bool {
	if k == nil || k.D == nil || !issuerPublicKey(&k.PublicKey) || k.D.Sign() <= 0 || k.D.Cmp(k.Curve.Params().N) >= 0 {
		return false
	}
	x, y := k.Curve.ScalarBaseMult(k.D.Bytes())
	return x.Cmp(k.X) == 0 && y.Cmp(k.Y) == 0
}

func issuerCopyKey(k *ecdsa.PublicKey) *ecdsa.PublicKey {
	return &ecdsa.PublicKey{Curve: k.Curve, X: new(big.Int).Set(k.X), Y: new(big.Int).Set(k.Y)}
}

// HandleChallenge returns a signed challenge or a bare sentinel (drop).
func (i *Issuer) HandleChallenge(raw []byte) ([]byte, error) { return nil, ErrHandshakeRefused }

// HandleAuthenticate returns a signed result or a bare sentinel (drop).
func (i *Issuer) HandleAuthenticate(raw []byte) ([]byte, error) { return nil, ErrHandshakeRefused }

// Session resolves only a live token belonging to the requested worker.
func (i *Issuer) Session(workerID, token string) (Session, bool) { return Session{}, false }
