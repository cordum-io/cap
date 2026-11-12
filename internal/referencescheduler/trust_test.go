package referencescheduler

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	trIssue          = agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE
	trRenew          = agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW
	trReplay         = agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_REPLAY_DETECTED
	trExpired        = agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_CHALLENGE_EXPIRED
	trAuthFailed     = agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED
	trSessionInvalid = agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_SESSION_INVALID
	trSkew           = agentv1.WorkerHandshakeRejectionReason_WORKER_HANDSHAKE_REJECTION_REASON_CLOCK_SKEW
)

type trFixture struct {
	now    time.Time
	cfg    IssuerConfig
	client capsdk.WorkerTrustConfig
	i      *Issuer
}

func trKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func trConfig(t *testing.T) *trFixture {
	t.Helper()
	f := &trFixture{now: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)}
	server, proof := trKey(t), trKey(t)
	f.cfg = IssuerConfig{SchedulerID: "scheduler", ServerKeyID: "server-key", ServerKey: server,
		Audience: capsdk.WorkerHandshakeAudience, Now: func() time.Time { return f.now },
		ChallengeTTL: 20 * time.Second, SessionTTL: 5 * time.Minute, MaxChallenges: 2,
		Workers: []TrustedWorker{{"worker", "tenant", "agent", "proof", &proof.PublicKey},
			{"other", "other-tenant", "other-agent", "proof", &trKey(t).PublicKey}}}
	f.client = capsdk.WorkerTrustConfig{WorkerID: "worker", ExpectedAgentID: "agent", TenantID: "tenant",
		Audience: f.cfg.Audience, ProofKeyID: "proof", ProofPrivateKey: proof, SDKVersion: "test-v1",
		ExpectedSchedulerID: f.cfg.SchedulerID, SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"server-key": &server.PublicKey}}
	return f
}

func trNew(t *testing.T) *trFixture {
	t.Helper()
	f := trConfig(t)
	var err error
	f.i, err = NewIssuer(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func trRequest(t *testing.T, f *trFixture, purpose agentv1.WorkerHandshakePurpose) *agentv1.BusPacket {
	t.Helper()
	p, err := capsdk.BuildWorkerHandshakeChallengeRequest(&f.client, capsdk.WorkerHandshakeRequestOptions{
		RequestID: "request", TraceID: "trace", Purpose: purpose, ClientNonce: bytes.Repeat([]byte{7}, 32), CreatedAt: f.now})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func trMarshal(t *testing.T, p *agentv1.BusPacket) []byte {
	t.Helper()
	raw, err := capsdk.MarshalWorkerTrustPacket(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func trSign(t *testing.T, f *trFixture, p *agentv1.BusPacket) {
	t.Helper()
	if err := capsdk.SignTrustHandshake(p, f.client.ProofPrivateKey); err != nil {
		t.Fatal(err)
	}
}

func trDecode(t *testing.T, f *trFixture, raw []byte, err error) *agentv1.BusPacket {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	p, err := capsdk.UnmarshalWorkerTrustPacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = capsdk.VerifyTrustHandshake(p, f.client.SchedulerPublicKeys); err != nil {
		t.Fatal(err)
	}
	if p.SenderId != f.cfg.SchedulerID {
		t.Fatal("scheduler identity mismatch")
	}
	return p
}

func trAuth(t *testing.T, f *trFixture, purpose agentv1.WorkerHandshakePurpose, token string) (*agentv1.BusPacket, *capsdk.VerifiedWorkerHandshakeChallenge) {
	t.Helper()
	request := trRequest(t, f, purpose)
	raw, err := f.i.HandleChallenge(trMarshal(t, request))
	response := trDecode(t, f, raw, err)
	verified, err := capsdk.VerifyWorkerHandshakeChallenge(&f.client, request, response, f.now)
	if err != nil {
		t.Fatal(err)
	}
	capability := &agentv1.Handshake{ComponentId: f.client.WorkerID, Role: agentv1.ComponentRole_COMPONENT_ROLE_WORKER,
		SdkVersion: f.client.SDKVersion, SupportedVersions: []int32{1}}
	p, err := capsdk.BuildWorkerHandshakeAuthenticate(&f.client, verified, capability, token, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return p, verified
}

func trFinish(t *testing.T, f *trFixture, auth *agentv1.BusPacket, v *capsdk.VerifiedWorkerHandshakeChallenge) *capsdk.WorkerHandshakeSession {
	t.Helper()
	raw, err := f.i.HandleAuthenticate(trMarshal(t, auth))
	p := trDecode(t, f, raw, err)
	s, err := capsdk.VerifyWorkerHandshakeResult(&f.client, v, auth, p, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func trIssueSession(t *testing.T, f *trFixture) *capsdk.WorkerHandshakeSession {
	t.Helper()
	auth, v := trAuth(t, f, trIssue, "")
	return trFinish(t, f, auth, v)
}

func TestIssuer_RoundTripWithSDKWorker(t *testing.T) {
	f := trNew(t)
	first := trIssueSession(t, f)
	trActive(t, f, first.ExpiresAt)
	s, ok := f.i.Session(f.client.WorkerID, first.Token)
	if !ok || s.WorkerID != "worker" || s.TenantID != "tenant" || s.AgentID != "agent" || !s.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatal("session identity or expiry mismatch")
	}
	if _, ok = f.i.Session("other", first.Token); ok {
		t.Fatal("cross-worker session accepted")
	}
	f.now = f.now.Add(time.Second)
	auth, v := trAuth(t, f, trRenew, first.Token)
	second := trFinish(t, f, auth, v)
	trActive(t, f, second.ExpiresAt)
	if second.Token == first.Token {
		t.Fatal("renewal did not rotate token")
	}
	if _, ok = f.i.Session("worker", first.Token); ok {
		t.Fatal("superseded token accepted")
	}
	if _, ok = f.i.Session("worker", second.Token); !ok {
		t.Fatal("renewed token missing")
	}
	if len(first.Token) != 43 || len(second.Token) != 43 {
		t.Fatal("unexpected token lengths")
	}
	t.Logf("issue token length=%d; renew token length=%d", len(first.Token), len(second.Token))
}

func trActive(t *testing.T, f *trFixture, expires time.Time) {
	t.Helper()
	s, ok := f.i.Active("worker")
	if !ok || s.WorkerID != "worker" || s.TenantID != "tenant" || s.AgentID != "agent" || !s.ExpiresAt.Equal(expires) {
		t.Fatal("active session identity or expiry mismatch")
	}
	if _, ok := f.i.Active("unknown"); ok {
		t.Fatal("unknown worker active")
	}
}

func TestIssuer_AuthenticateIsSingleUse(t *testing.T) {
	f := trNew(t)
	auth, v := trAuth(t, f, trIssue, "")
	trFinish(t, f, auth, v)
	raw, err := f.i.HandleAuthenticate(trMarshal(t, auth))
	p := trDecode(t, f, raw, err)
	_, err = capsdk.VerifyWorkerHandshakeResult(&f.client, v, auth, p, f.now)
	var rejected *capsdk.WorkerHandshakeRejectionError
	if !errors.As(err, &rejected) || rejected.Reason != trReplay {
		t.Fatal("redelivery must be REPLAY_DETECTED")
	}
	if len(f.i.sessions) != 1 || len(f.i.challenges) != 0 {
		t.Fatal("redelivery mutated state")
	}
}

func TestIssuer_ConcurrentAuthenticateOneWinner(t *testing.T) {
	f := trNew(t)
	auth, v := trAuth(t, f, trIssue, "")
	raw := trMarshal(t, auth)
	type reply struct {
		data []byte
		err  error
	}
	results := make(chan reply, 16)
	start := make(chan struct{})
	for n := 0; n < 16; n++ {
		go func() { <-start; data, err := f.i.HandleAuthenticate(raw); results <- reply{data, err} }()
	}
	close(start)
	accepted, rejected := 0, 0
	for n := 0; n < 16; n++ {
		r := <-results
		p := trDecode(t, f, r.data, r.err)
		_, err := capsdk.VerifyWorkerHandshakeResult(&f.client, v, auth, p, f.now)
		var rejection *capsdk.WorkerHandshakeRejectionError
		switch {
		case err == nil:
			accepted++
		case errors.As(err, &rejection) && rejection.Reason == trReplay:
			rejected++
		default:
			t.Fatal("unexpected concurrent result", err)
		}
	}
	if accepted != 1 || rejected != 15 || len(f.i.sessions) != 1 {
		t.Fatalf("accepted=%d rejected=%d sessions=%d", accepted, rejected, len(f.i.sessions))
	}
	t.Logf("accepted=%d REPLAY_DETECTED=%d", accepted, rejected)
}

type trRefusal struct {
	name      string
	challenge bool
	renew     string
	change    func(*testing.T, *trFixture, *agentv1.BusPacket)
	reason    agentv1.WorkerHandshakeRejectionReason
}

func trChallengeRefusals() []trRefusal {
	return []trRefusal{
		{name: "b_unknown_worker", challenge: true, change: func(t *testing.T, f *trFixture, p *agentv1.BusPacket) {
			p.SenderId = "unknown"
			p.GetWorkerHandshakeChallengeRequest().WorkerId = "unknown"
			trSign(t, f, p)
		}},
		{name: "c_wrong_proof_challenge", challenge: true, change: trWrongKey},
		{name: "d_bad_signature_challenge", challenge: true, change: trBadSignature},
		{name: "g_audience_challenge", challenge: true, change: func(t *testing.T, f *trFixture, p *agentv1.BusPacket) {
			p.GetWorkerHandshakeChallengeRequest().Audience = "other"
			trSign(t, f, p)
		}},
		{name: "i_clock_challenge", challenge: true, change: trBadClock},
	}
}

func trAuthenticateRefusals() []trRefusal {
	return []trRefusal{
		{name: "a_expired", reason: trExpired, change: func(_ *testing.T, f *trFixture, _ *agentv1.BusPacket) { f.now = f.now.Add(f.cfg.ChallengeTTL) }},
		{name: "c_wrong_proof_authenticate", change: trWrongKey},
		{name: "d_bad_signature_authenticate", change: trBadSignature},
		{name: "e_tenant", reason: trAuthFailed, change: func(t *testing.T, f *trFixture, p *agentv1.BusPacket) {
			p.GetWorkerHandshakeAuthenticate().Challenge.TenantId = "other"
			trSign(t, f, p)
		}},
		{name: "f_agent", reason: trAuthFailed, change: func(t *testing.T, f *trFixture, p *agentv1.BusPacket) {
			p.GetWorkerHandshakeAuthenticate().Challenge.AgentId = "other"
			trSign(t, f, p)
		}},
		{name: "g_audience_authenticate", reason: trAuthFailed, change: func(t *testing.T, f *trFixture, p *agentv1.BusPacket) {
			p.GetWorkerHandshakeAuthenticate().Challenge.Audience = "other"
			trSign(t, f, p)
		}},
		{name: "h_unknown_session", renew: "unknown", reason: trSessionInvalid},
		{name: "h_expired_session", renew: "expired", reason: trSessionInvalid},
		{name: "i_clock_authenticate", reason: trSkew, change: trBadClock},
	}
}

func trWrongKey(t *testing.T, _ *trFixture, p *agentv1.BusPacket) {
	t.Helper()
	if err := capsdk.SignTrustHandshake(p, trKey(t)); err != nil {
		t.Fatal(err)
	}
}

func trBadSignature(_ *testing.T, _ *trFixture, p *agentv1.BusPacket) {
	p.Signature[len(p.Signature)-1] ^= 1
}

func trBadClock(t *testing.T, f *trFixture, p *agentv1.BusPacket) {
	p.CreatedAt = timestamppb.New(f.now.Add(capsdk.WorkerHandshakeMaxSkew + time.Nanosecond))
	trSign(t, f, p)
}

func TestIssuer_RefusalsMintNothing(t *testing.T) {
	for _, tc := range append(trChallengeRefusals(), trAuthenticateRefusals()...) {
		t.Run(tc.name, func(t *testing.T) { trRefusalRun(t, tc) })
	}
}

func trRefusalRun(t *testing.T, tc trRefusal) {
	t.Helper()
	f := trNew(t)
	p := trRequest(t, f, trIssue)
	handle := f.i.HandleChallenge
	if !tc.challenge {
		purpose, token := trIssue, ""
		if tc.renew != "" {
			purpose, token = trRenew, "unknown-token"
		}
		if tc.renew == "expired" {
			token = trIssueSession(t, f).Token
			f.now = f.now.Add(f.cfg.SessionTTL)
			f.i.Session("worker", token)
		}
		p, _ = trAuth(t, f, purpose, token)
		handle = f.i.HandleAuthenticate
	}
	before := len(f.i.sessions)
	if tc.change != nil {
		tc.change(t, f, p)
	}
	raw, err := handle(trMarshal(t, p))
	if tc.reason == 0 {
		trDrop(t, raw, err, ErrHandshakeRefused)
	} else {
		result := trDecode(t, f, raw, err)
		if result.GetWorkerHandshakeResult().Accepted || result.GetWorkerHandshakeResult().RejectionReason != tc.reason {
			t.Fatal("wrong rejection reason")
		}
		if result.AuthToken != "" || result.GetWorkerHandshakeResult().TokenExpiresAt != nil {
			t.Fatal("rejection leaked token metadata")
		}
		if !proto.Equal(result.GetWorkerHandshakeResult().Challenge, p.GetWorkerHandshakeAuthenticate().Challenge) {
			t.Fatal("rejection correlation changed")
		}
	}
	if len(f.i.sessions) != before {
		t.Fatal("refusal minted session")
	}
}

func trDrop(t *testing.T, raw []byte, err, want error) {
	t.Helper()
	if raw != nil || !errors.Is(err, want) || err.Error() != want.Error() {
		t.Fatal("drop must return nil and bare sentinel", err)
	}
}

func TestIssuer_Bounds(t *testing.T) {
	t.Run("size_before_decode", func(t *testing.T) {
		f := trNew(t)
		for _, handle := range []func([]byte) ([]byte, error){f.i.HandleChallenge, f.i.HandleAuthenticate} {
			raw, err := handle(bytes.Repeat([]byte{255}, capsdk.WorkerHandshakeMaxPacketSize+1))
			trDrop(t, raw, err, ErrHandshakeTooLarge)
		}
	})
	t.Run("challenge_cap_and_eviction", trChallengeBounds)
	t.Run("active_expiry", func(t *testing.T) {
		f := trNew(t)
		trIssueSession(t, f)
		f.now = f.now.Add(f.cfg.SessionTTL)
		if _, ok := f.i.Active("worker"); ok {
			t.Fatal("expired worker active")
		}
		if len(f.i.sessions) != 0 || len(f.i.tokens) != 0 {
			t.Fatal("Active did not evict expired session")
		}
	})
	t.Run("session_supersession_and_expiry", func(t *testing.T) {
		f := trNew(t)
		first, second := trIssueSession(t, f), trIssueSession(t, f)
		if _, ok := f.i.Session("worker", first.Token); ok {
			t.Fatal("second issue did not supersede first")
		}
		if len(f.i.sessions) != 1 || len(f.i.tokens) != 1 {
			t.Fatal("sessions not bounded by worker count")
		}
		f.now = f.now.Add(f.cfg.SessionTTL)
		if _, ok := f.i.Session("worker", second.Token); ok {
			t.Fatal("expired token accepted")
		}
		if len(f.i.sessions) != 0 || len(f.i.tokens) != 0 {
			t.Fatal("expired session not evicted")
		}
	})
}

func trChallengeBounds(t *testing.T) {
	f := trNew(t)
	first, _ := trAuth(t, f, trIssue, "")
	trAuth(t, f, trIssue, "")
	raw, err := f.i.HandleChallenge(trMarshal(t, trRequest(t, f, trIssue)))
	trDrop(t, raw, err, ErrHandshakeRefused)
	if len(f.i.challenges) != 2 {
		t.Fatal("live challenge evicted")
	}
	if _, ok := f.i.challenges[first.GetWorkerHandshakeAuthenticate().Challenge.ChallengeId]; !ok {
		t.Fatal("oldest live challenge evicted")
	}
	f.now = f.now.Add(f.cfg.ChallengeTTL)
	trAuth(t, f, trIssue, "")
	if len(f.i.challenges) != 1 {
		t.Fatal("expired challenges not evicted")
	}
}

func TestIssuer_ErrorsCarryNoSecrets(t *testing.T) {
	f := trNew(t)
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	p := trRequest(t, f, trIssue)
	r := p.GetWorkerHandshakeChallengeRequest()
	r.RequestId, r.TraceId, r.SdkVersion = "CANARY-request", "CANARY-trace", "CANARY-sdk"
	r.ClientNonce = bytes.Repeat([]byte("CANARY!!"), 4)
	p.TraceId = r.TraceId
	trSign(t, f, p)
	raw, err := f.i.HandleChallenge(trMarshal(t, p))
	trDecode(t, f, raw, err) // prove a valid canary-bearing request reaches the issuer.
	trBadSignature(t, f, p)
	raw, err = f.i.HandleChallenge(trMarshal(t, p))
	trDrop(t, raw, err, ErrHandshakeRefused)
	auth, _ := trAuth(t, f, trRenew, "CANARY-token")
	trBadSignature(t, f, auth)
	raw, err = f.i.HandleAuthenticate(trMarshal(t, auth))
	trDrop(t, raw, err, ErrHandshakeRefused)
	if strings.Contains(err.Error(), "CANARY") || logs.Len() != 0 {
		t.Fatal("secret or unexpected log output")
	}
}

func TestNewIssuer_RejectsInvalidConfig(t *testing.T) {
	for name, change := range trInvalidConfigs() {
		t.Run(name, func(t *testing.T) {
			f := trConfig(t)
			change(&f.cfg)
			i, err := NewIssuer(f.cfg)
			if i != nil || err != ErrInvalidConfig {
				t.Fatal("expected bare ErrInvalidConfig")
			}
		})
	}
	t.Run("valid", func(t *testing.T) { trNew(t) })
}

func trInvalidConfigs() map[string]func(*IssuerConfig) {
	return map[string]func(*IssuerConfig){
		"scheduler_blank":     func(c *IssuerConfig) { c.SchedulerID = " " },
		"scheduler_untrimmed": func(c *IssuerConfig) { c.SchedulerID = " scheduler" },
		"server_id":           func(c *IssuerConfig) { c.ServerKeyID = "" },
		"audience":            func(c *IssuerConfig) { c.Audience = " audience " },
		"server_nil":          func(c *IssuerConfig) { c.ServerKey = nil },
		"server_scalar_nil":   func(c *IssuerConfig) { c.ServerKey.D = nil },
		"server_scalar_zero":  func(c *IssuerConfig) { c.ServerKey.D = new(big.Int) },
		"server_scalar_range": func(c *IssuerConfig) { c.ServerKey.D = new(big.Int).Set(elliptic.P256().Params().N) },
		"server_mismatch":     func(c *IssuerConfig) { c.ServerKey.D = big.NewInt(1) },
		"workers_empty":       func(c *IssuerConfig) { c.Workers = nil },
		"workers_duplicate":   func(c *IssuerConfig) { c.Workers = append(c.Workers, c.Workers[0]) },
		"worker_id":           func(c *IssuerConfig) { c.Workers[0].WorkerID = "" },
		"tenant":              func(c *IssuerConfig) { c.Workers[0].TenantID = " " },
		"agent":               func(c *IssuerConfig) { c.Workers[0].AgentID = " agent " },
		"proof_id":            func(c *IssuerConfig) { c.Workers[0].ProofKeyID = "" },
		"proof_nil":           func(c *IssuerConfig) { c.Workers[0].ProofKey = nil },
		"proof_curve":         func(c *IssuerConfig) { c.Workers[0].ProofKey.Curve = elliptic.P384() },
		"proof_x_nil":         func(c *IssuerConfig) { c.Workers[0].ProofKey.X = nil },
		"proof_off_curve":     func(c *IssuerConfig) { c.Workers[0].ProofKey.X = big.NewInt(0) },
		"clock":               func(c *IssuerConfig) { c.Now = nil },
		"challenge_zero":      func(c *IssuerConfig) { c.ChallengeTTL = 0 },
		"challenge_long":      func(c *IssuerConfig) { c.ChallengeTTL = capsdk.WorkerHandshakeMaxLifetime + 1 },
		"session_zero":        func(c *IssuerConfig) { c.SessionTTL = 0 },
		"challenge_cap_zero":  func(c *IssuerConfig) { c.MaxChallenges = 0 },
	}
}
