package runtime

// Phase-2 worker handshake on Agent.Start. Before the agent subscribes
// to its job topics, it publishes a HandshakeRequest on
// WorkerHandshakeSubject and waits for a HandshakeResponse carrying a
// short-lived session token. The token is stored on the Agent and
// attached to every subsequent outbound packet (step-5 wires the
// middleware).
//
// The handshake is gated by a three-mode flag so the rollout doesn't
// break existing fleets:
//
//   HandshakeModeOff     — legacy. Handshake is skipped entirely.
//   HandshakeModeWarn    — handshake attempted; failure logs WARN but
//                          Start() still succeeds so pre-handshake
//                          workers keep running.
//   HandshakeModeEnforce — handshake attempted; failure returns an
//                          error from Start() so no job handlers get
//                          subscribed without a trusted session.
//
// The flag defaults to off inside cap/ so existing agents keep their
// exact current behaviour; cordum's scheduler-side CORDUM_SDK_HANDSHAKE
// env var picks the enforcement side independently.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
)

// Env var names operators use to drive the handshake flow without
// touching Agent construction code. Adapters (LangChain, CrewAI,
// AutoGen, OpenAI Agents) that re-vend Agent.Start automatically
// pick up the operator's rollout stance this way.
const (
	EnvHandshakeMode   = "CORDUM_SDK_HANDSHAKE"
	EnvAgentTenant     = "CORDUM_AGENT_TENANT"
	EnvAgentSDKVersion = "CORDUM_AGENT_SDK_VERSION"
)

// HandshakeMode enumerates the agent-side handshake enforcement modes.
// Strings match the operator-facing env var values so a single value
// round-trips between log lines, docs, and the scheduler flag.
type HandshakeMode string

const (
	HandshakeModeOff     HandshakeMode = "off"
	HandshakeModeWarn    HandshakeMode = "warn"
	HandshakeModeEnforce HandshakeMode = "enforce"
)

// Default handshake tuning. Exported so adapters (LangChain, CrewAI,
// AutoGen) can override per deployment without reaching into
// implementation details.
const (
	DefaultHandshakeTimeout  = 10 * time.Second
	DefaultHandshakeRetries  = 3
	DefaultHandshakeSDKVersion = "cap-go/v2"
)

// NATSRequester is the optional request/reply interface the Agent uses
// to perform the handshake round-trip. *nats.Conn satisfies this
// natively; test doubles can implement it per-scenario. When the
// agent's NATSConn does NOT implement NATSRequester the handshake is
// either silently skipped (off), logged as a warning (warn), or
// escalated to a Start() error (enforce) — the mode flag decides.
type NATSRequester interface {
	Request(subj string, data []byte, timeout time.Duration) (*nats.Msg, error)
}

// HandshakeRejectedError is returned from a handshake attempt whose
// scheduler response carried Rejected=true. Callers can switch on the
// Reason string (HandshakeReject* constants in cap/sdk/go) to drive
// operator-facing remediation.
type HandshakeRejectedError struct {
	Reason    string
	RequestID string
	AgentID   string
}

func (e *HandshakeRejectedError) Error() string {
	return fmt.Sprintf("cap-runtime: handshake rejected: reason=%s agent_id=%s request_id=%s", e.Reason, e.AgentID, e.RequestID)
}

// sessionState keeps the active session token + exp together under a
// single lock so concurrent readers (middleware attaching the token to
// outbound packets) always see a consistent snapshot.
type sessionState struct {
	mu    sync.RWMutex
	token string
	exp   time.Time
}

func (s *sessionState) set(token string, exp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
	s.exp = exp
}

func (s *sessionState) snapshot() (string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token, s.exp
}

// SessionToken returns the current session token + expiry. An empty
// token means either (a) no handshake has been performed yet, (b) the
// agent is running in HandshakeModeOff, or (c) the handshake was
// skipped because NATSConn doesn't implement NATSRequester. Callers
// should treat the three cases the same at this API boundary — they
// always mean "don't attach a session token to outbound packets".
func (a *Agent) SessionToken() (string, time.Time) {
	if a == nil || a.session == nil {
		return "", time.Time{}
	}
	return a.session.snapshot()
}

// setSession is exposed so the renew loop (step-7) can install a
// rotated token without poking Agent fields directly.
func (a *Agent) setSession(token string, exp time.Time) {
	if a == nil {
		return
	}
	if a.session == nil {
		a.session = &sessionState{}
	}
	a.session.set(token, exp)
}

// activeHandshakeMode returns the effective mode for the agent.
// Resolution order: (1) Agent.HandshakeMode (explicit code-level
// opt-in), (2) CORDUM_SDK_HANDSHAKE env var (operator-level rollout),
// (3) off (legacy default). Unknown values fall back to off so a
// misconfigured env var never silently escalates enforcement.
func (a *Agent) activeHandshakeMode() HandshakeMode {
	raw := strings.ToLower(strings.TrimSpace(string(a.HandshakeMode)))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(os.Getenv(EnvHandshakeMode)))
	}
	switch HandshakeMode(raw) {
	case HandshakeModeWarn:
		return HandshakeModeWarn
	case HandshakeModeEnforce:
		return HandshakeModeEnforce
	}
	return HandshakeModeOff
}

// effectiveTenant returns the agent's tenant with a fall-back to
// CORDUM_AGENT_TENANT. Operators set the env var once in the pod
// spec and every adapter inherits it without code changes.
func (a *Agent) effectiveTenant() string {
	if strings.TrimSpace(a.Tenant) != "" {
		return a.Tenant
	}
	return strings.TrimSpace(os.Getenv(EnvAgentTenant))
}

func (a *Agent) handshakeTimeout() time.Duration {
	if a.HandshakeTimeout > 0 {
		return a.HandshakeTimeout
	}
	return DefaultHandshakeTimeout
}

func (a *Agent) handshakeRetries() int {
	if a.HandshakeRetries > 0 {
		return a.HandshakeRetries
	}
	return DefaultHandshakeRetries
}

func (a *Agent) effectiveSDKVersion() string {
	if strings.TrimSpace(a.SDKVersion) != "" {
		return a.SDKVersion
	}
	if v := strings.TrimSpace(os.Getenv(EnvAgentSDKVersion)); v != "" {
		return v
	}
	return DefaultHandshakeSDKVersion
}

// performHandshake drives the full Phase-2 handshake: build request,
// request/reply over WorkerHandshakeSubject, retry with exponential
// backoff up to handshakeRetries, install the session token on
// success, propagate rejection errors without retry (a reject is a
// terminal answer — retrying would just flood the scheduler).
//
// Invariants:
//   - Only called when mode != off.
//   - Returns nil in warn mode even on hard failure; caller inspects
//     the bool `obtained` to know whether a token is available.
//   - In enforce mode any failure — including transport errors and
//     rejections — returns a non-nil error.
func (a *Agent) performHandshake(ctx context.Context) (obtained bool, err error) {
	return a.performHandshakeOn(ctx, capsdk.WorkerHandshakeSubject)
}

// performRenew is the renew-subject variant of performHandshake. Used
// by the auto-renew loop (step-7) to rotate the session token before
// expiry. Shares the exact same wire shape so the scheduler-side
// HandleRenew can reuse SessionTokenIssuer.Issue unchanged.
func (a *Agent) performRenew(ctx context.Context) (bool, error) {
	return a.performHandshakeOn(ctx, capsdk.WorkerHandshakeRenewSubject)
}

// performHandshakeOn is the internal implementation. subject decides
// whether this is an initial handshake or a renew — every other
// control-flow bit (retries, backoff, rejection-is-terminal) is
// identical.
func (a *Agent) performHandshakeOn(ctx context.Context, subject string) (obtained bool, err error) {
	mode := a.activeHandshakeMode()
	if mode == HandshakeModeOff {
		return false, nil
	}
	if strings.TrimSpace(a.SenderID) == "" {
		if mode == HandshakeModeEnforce {
			return false, errors.New("cap-runtime: handshake enforce mode requires a non-empty SenderID")
		}
		a.Logger.Warn("cap-runtime: handshake skipped — empty SenderID", "mode", string(mode))
		return false, nil
	}
	tenant := a.effectiveTenant()
	if tenant == "" {
		if mode == HandshakeModeEnforce {
			return false, errors.New("cap-runtime: handshake enforce mode requires a non-empty Tenant (set Agent.Tenant or CORDUM_AGENT_TENANT)")
		}
		a.Logger.Warn("cap-runtime: handshake skipped — empty Tenant", "mode", string(mode))
		return false, nil
	}
	requester, ok := a.NATS.(NATSRequester)
	if !ok {
		if mode == HandshakeModeEnforce {
			return false, errors.New("cap-runtime: handshake enforce mode requires a NATS connection that supports Request")
		}
		a.Logger.Warn("cap-runtime: handshake skipped — NATS connection does not implement Request",
			"mode", string(mode),
			"agent_id", a.SenderID,
		)
		return false, nil
	}

	timeout := a.handshakeTimeout()
	retries := a.handshakeRetries()
	capabilities := a.advertisedCapabilities()

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		if ctx != nil && ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}
		req, buildErr := buildHandshakeRequest(a.SenderID, tenant, a.effectiveSDKVersion(), capabilities)
		if buildErr != nil {
			lastErr = buildErr
			break // building a request never improves with a retry.
		}
		payload, marshalErr := capsdk.MarshalHandshakeRequest(req)
		if marshalErr != nil {
			lastErr = marshalErr
			break
		}
		msg, reqErr := requester.Request(subject, payload, timeout)
		if reqErr != nil {
			lastErr = reqErr
			a.Logger.Warn("cap-runtime: handshake attempt failed",
				"agent_id", a.SenderID,
				"attempt", attempt+1,
				"retries", retries,
				"error", reqErr,
			)
			sleep := handshakeBackoff(attempt)
			if !sleepCtx(ctx, sleep) {
				lastErr = ctx.Err()
				break
			}
			continue
		}
		resp, parseErr := capsdk.UnmarshalHandshakeResponse(msg.Data)
		if parseErr != nil {
			lastErr = parseErr
			a.Logger.Warn("cap-runtime: handshake response malformed",
				"agent_id", a.SenderID,
				"attempt", attempt+1,
				"error", parseErr,
			)
			sleep := handshakeBackoff(attempt)
			if !sleepCtx(ctx, sleep) {
				lastErr = ctx.Err()
				break
			}
			continue
		}
		if resp.RequestID != req.RequestID {
			lastErr = fmt.Errorf("cap-runtime: handshake response request_id mismatch: got %q want %q", resp.RequestID, req.RequestID)
			continue
		}
		if resp.Rejected {
			// Rejection is terminal: retrying just spams the scheduler.
			return false, &HandshakeRejectedError{
				Reason:    resp.Reason,
				RequestID: resp.RequestID,
				AgentID:   a.SenderID,
			}
		}
		if strings.TrimSpace(resp.SessionToken) == "" {
			lastErr = errors.New("cap-runtime: handshake accepted but session_token was empty")
			continue
		}
		a.setSession(resp.SessionToken, resp.TokenExp)
		a.Logger.Info("cap-runtime: handshake accepted",
			"agent_id", a.SenderID,
			"tenant", tenant,
			"session_exp", resp.TokenExp,
		)
		return true, nil
	}

	if mode == HandshakeModeEnforce {
		if lastErr == nil {
			lastErr = errors.New("cap-runtime: handshake failed after retries")
		}
		return false, fmt.Errorf("cap-runtime: handshake failed (enforce mode): %w", lastErr)
	}
	// warn mode: log, swallow, let Start() proceed.
	if lastErr != nil {
		a.Logger.Warn("cap-runtime: handshake failed; continuing without session token (warn mode)",
			"agent_id", a.SenderID,
			"error", lastErr,
		)
	}
	return false, nil
}

// buildHandshakeRequest constructs a ready-to-marshal request. The
// nonce is 16 bytes of crypto/rand (hex-encoded), satisfying the
// capsdk.WorkerHandshakeNonceLength minimum. The request_id is a
// separate 16-byte random so a response collision on nonce value
// cannot be used to spoof the request_id pairing.
func buildHandshakeRequest(agentID, tenant, sdkVersion string, capabilities []string) (*capsdk.HandshakeRequest, error) {
	nonce, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("cap-runtime: generate nonce: %w", err)
	}
	reqID, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("cap-runtime: generate request_id: %w", err)
	}
	return &capsdk.HandshakeRequest{
		AgentID:      agentID,
		Tenant:       tenant,
		SDKVersion:   sdkVersion,
		Capabilities: capabilities,
		Nonce:        nonce,
		RequestID:    reqID,
		Timestamp:    time.Now().UTC(),
	}, nil
}

// advertisedCapabilities returns a deterministic view of the topics
// the agent has registered handlers for — the scheduler validates this
// against the AgentIdentity.AllowedCapabilities record and refuses any
// topic that exceeds the allowlist. Returning a nil slice when no
// handlers are registered is intentional: "no capabilities" is a
// legitimate state for an agent that uses only background tasks.
func (a *Agent) advertisedCapabilities() []string {
	if len(a.handlers) == 0 {
		return nil
	}
	out := make([]string, 0, len(a.handlers))
	for topic := range a.handlers {
		out = append(out, topic)
	}
	// Stable order so the scheduler-side audit event has a canonical
	// representation across retries.
	sortStrings(out)
	return out
}

// randomHex returns n bytes of crypto/rand hex-encoded. Panics would
// be wrong here — crypto/rand failures happen on the boot path and we
// want the caller to surface the error cleanly so the operator sees
// "handshake failed: generate nonce" rather than a stack trace.
func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// handshakeBackoff returns the sleep interval between retry attempts.
// 100ms, 300ms, 900ms, ... — exponential with a fixed 3x multiplier.
// No jitter because handshake retries are already well-spaced at 10s
// timeouts and the scheduler-side nonce store guarantees replay
// protection even when a stampede of retries arrives simultaneously.
func handshakeBackoff(attempt int) time.Duration {
	base := 100 * time.Millisecond
	mult := time.Duration(1)
	for range attempt {
		mult *= 3
	}
	return base * mult
}

// sleepCtx sleeps for d, returning true on completion and false on
// context cancellation. Callers use the bool to bail out of the retry
// loop when the surrounding context expires.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	if ctx == nil {
		time.Sleep(d)
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// sortStrings is a tiny inline sort — pulling in sort on this hot path
// works fine, but a hand-rolled bubble for ≤8 capabilities is cheaper
// at boot and keeps the runtime package dep-surface stable.
func sortStrings(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
