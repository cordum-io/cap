package referencescheduler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"strings"
	"sync"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
func (i *Issuer) HandleChallenge(raw []byte) ([]byte, error) {
	p, err := issuerDecode(raw)
	if err != nil {
		return nil, err
	}
	r := p.GetWorkerHandshakeChallengeRequest()
	if r == nil {
		return nil, ErrHandshakeRefused
	}
	w, ok := i.workers[r.WorkerId]
	if !ok || capsdk.VerifyTrustHandshake(p, map[string]*ecdsa.PublicKey{w.ProofKeyID: w.ProofKey}) != nil {
		return nil, ErrHandshakeRefused
	}
	now := i.cfg.Now()
	if r.Audience != i.cfg.Audience || !issuerWithinSkew(p.CreatedAt, now) {
		return nil, ErrHandshakeRefused
	}
	c, err := i.newChallenge(r, w, now)
	if err != nil || !i.storeChallenge(c, now) {
		return nil, ErrHandshakeRefused
	}
	reply := i.packet(c.TraceId, now)
	reply.Payload = &agentv1.BusPacket_WorkerHandshakeChallenge{WorkerHandshakeChallenge: c}
	encoded, err := i.signReply(reply)
	if err != nil {
		i.mu.Lock()
		delete(i.challenges, c.ChallengeId)
		i.mu.Unlock()
	}
	return encoded, err
}

func issuerDecode(raw []byte) (*agentv1.BusPacket, error) {
	if len(raw) > capsdk.WorkerHandshakeMaxPacketSize {
		return nil, ErrHandshakeTooLarge
	}
	p, err := capsdk.UnmarshalWorkerTrustPacket(raw)
	if err != nil || capsdk.ValidateWorkerTrustPacket(p) != nil {
		return nil, ErrHandshakeRefused
	}
	return p, nil
}

func issuerWithinSkew(created *timestamppb.Timestamp, now time.Time) bool {
	delta := created.AsTime().Sub(now)
	return delta >= -capsdk.WorkerHandshakeMaxSkew && delta <= capsdk.WorkerHandshakeMaxSkew
}

func (i *Issuer) newChallenge(r *agentv1.WorkerHandshakeChallengeRequest, w TrustedWorker, now time.Time) (*agentv1.WorkerHandshakeChallenge, error) {
	random := make([]byte, 16+capsdk.WorkerHandshakeNonceSize)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return nil, ErrHandshakeRefused
	}
	return &agentv1.WorkerHandshakeChallenge{
		ChallengeId: hex.EncodeToString(random[:16]), ServerNonce: random[16:],
		RequestId: r.RequestId, TraceId: r.TraceId, WorkerId: r.WorkerId,
		ProofKeyId: r.ProofKeyId, ProofAlgorithm: r.ProofAlgorithm, Audience: r.Audience,
		Purpose: r.Purpose, ClientNonce: append([]byte(nil), r.ClientNonce...),
		ProtocolVersion: r.ProtocolVersion, SdkVersion: r.SdkVersion,
		AgentId: w.AgentID, TenantId: w.TenantID, ServerKeyId: i.cfg.ServerKeyID,
		IssuedAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(i.cfg.ChallengeTTL)),
	}, nil
}

func (i *Issuer) storeChallenge(c *agentv1.WorkerHandshakeChallenge, now time.Time) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	for id, live := range i.challenges {
		if !now.Before(live.ExpiresAt.AsTime()) {
			delete(i.challenges, id)
		}
	}
	if len(i.challenges) >= i.cfg.MaxChallenges || i.challenges[c.ChallengeId] != nil {
		return false
	}
	i.challenges[c.ChallengeId] = proto.Clone(c).(*agentv1.WorkerHandshakeChallenge)
	return true
}

func (i *Issuer) packet(trace string, now time.Time) *agentv1.BusPacket {
	return &agentv1.BusPacket{TraceId: trace, SenderId: i.cfg.SchedulerID,
		CreatedAt: timestamppb.New(now), ProtocolVersion: capsdk.WorkerHandshakeProtocolVersion}
}

func (i *Issuer) signReply(p *agentv1.BusPacket) ([]byte, error) {
	if capsdk.SignTrustHandshake(p, i.cfg.ServerKey) != nil || capsdk.ValidateWorkerTrustPacket(p) != nil {
		return nil, ErrHandshakeRefused
	}
	raw, err := capsdk.MarshalWorkerTrustPacket(p)
	if err != nil {
		return nil, ErrHandshakeRefused
	}
	return raw, nil
}

// HandleAuthenticate returns a signed result or a bare sentinel (drop).
func (i *Issuer) HandleAuthenticate(raw []byte) ([]byte, error) {
	p, err := issuerDecode(raw)
	if err != nil {
		return nil, err
	}
	a := p.GetWorkerHandshakeAuthenticate()
	if a == nil {
		return nil, ErrHandshakeRefused
	}
	w, ok := i.workers[a.Challenge.WorkerId]
	if !ok || capsdk.VerifyTrustHandshake(p, map[string]*ecdsa.PublicKey{w.ProofKeyID: w.ProofKey}) != nil {
		return nil, ErrHandshakeRefused
	}
	reply, err := i.authenticate(p)
	if err != nil {
		return nil, err
	}
	encoded, err := i.signReply(reply)
	if err != nil && reply.AuthToken != "" {
		i.mu.Lock()
		i.removeSession(reply.AuthToken)
		i.mu.Unlock()
	}
	return encoded, err
}

func (i *Issuer) authenticate(p *agentv1.BusPacket) (*agentv1.BusPacket, error) {
	// ponytail: one process-wide critical section; HA needs shared atomic authority.
	i.mu.Lock()
	defer i.mu.Unlock()
	now := i.cfg.Now()
	c := p.GetWorkerHandshakeAuthenticate().Challenge
	r := &agentv1.WorkerHandshakeResult{Challenge: c, IssuedAt: timestamppb.New(now), RejectionReason: i.checkAuthenticate(p, now)}
	reply := i.packet(c.TraceId, now)
	reply.Payload = &agentv1.BusPacket_WorkerHandshakeResult{WorkerHandshakeResult: r}
	if r.RejectionReason != 0 {
		return reply, nil
	}
	delete(i.challenges, c.ChallengeId)
	token, err := i.mint(c.WorkerId, now)
	if err != nil {
		return nil, ErrHandshakeRefused
	}
	reply.AuthToken, r.Accepted = token, true
	r.TokenExpiresAt = timestamppb.New(i.sessions[token].ExpiresAt)
	return reply, nil
}

// checkAuthenticate runs only with mu held; failures do not consume a live challenge.
func (i *Issuer) checkAuthenticate(p *agentv1.BusPacket, now time.Time) agentv1.WorkerHandshakeRejectionReason {
	if !issuerWithinSkew(p.CreatedAt, now) {
		return agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_CLOCK_SKEW
	}
	c := p.GetWorkerHandshakeAuthenticate().Challenge
	live, ok := i.challenges[c.ChallengeId]
	if !ok {
		return agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_REPLAY_DETECTED
	}
	if !proto.Equal(live, c) {
		return agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED
	}
	if !now.Before(live.ExpiresAt.AsTime()) {
		delete(i.challenges, c.ChallengeId)
		return agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_CHALLENGE_EXPIRED
	}
	if c.Purpose == agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW {
		if _, ok := i.liveSession(c.WorkerId, p.AuthToken, now); !ok {
			return agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_SESSION_INVALID
		}
	}
	return agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED
}

// mint runs only with mu held, after consuming the challenge.
func (i *Issuer) mint(worker string, now time.Time) (string, error) {
	random := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", ErrHandshakeRefused
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	if _, exists := i.sessions[token]; exists {
		return "", ErrHandshakeRefused
	}
	for old, s := range i.sessions {
		if !now.Before(s.ExpiresAt) {
			i.removeSession(old)
		}
	}
	i.removeSession(i.tokens[worker])
	w := i.workers[worker]
	i.sessions[token] = Session{worker, w.TenantID, w.AgentID, now.Add(i.cfg.SessionTTL)}
	i.tokens[worker] = token
	return token, nil
}

func (i *Issuer) removeSession(token string) {
	if s, ok := i.sessions[token]; ok {
		delete(i.sessions, token)
		if i.tokens[s.WorkerID] == token {
			delete(i.tokens, s.WorkerID)
		}
	}
}

func (i *Issuer) liveSession(worker, token string, now time.Time) (Session, bool) {
	s, ok := i.sessions[token]
	if ok && !now.Before(s.ExpiresAt) {
		i.removeSession(token)
		return Session{}, false
	}
	if !ok || s.WorkerID != worker {
		return Session{}, false
	}
	return s, true
}

// Session resolves only a live token belonging to the requested worker.
func (i *Issuer) Session(workerID, token string) (Session, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.liveSession(workerID, token, i.cfg.Now())
}

// Active returns a worker's current live session without revealing its token.
func (i *Issuer) Active(workerID string) (Session, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.liveSession(workerID, i.tokens[workerID], i.cfg.Now())
}
