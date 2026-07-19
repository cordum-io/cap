package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
)

const (
	EnvHandshakeMode   = "CORDUM_SDK_HANDSHAKE"
	EnvAgentTenant     = "CORDUM_AGENT_TENANT"
	EnvAgentSDKVersion = "CORDUM_AGENT_SDK_VERSION"
)

// HandshakeMode controls admission after an authenticated trust failure.
type HandshakeMode string

const (
	HandshakeModeOff     HandshakeMode = "off"
	HandshakeModeWarn    HandshakeMode = "warn"
	HandshakeModeEnforce HandshakeMode = "enforce"
)

const (
	DefaultHandshakeTimeout    = 10 * time.Second
	DefaultHandshakeRetries    = 3
	DefaultHandshakeSDKVersion = "cap-go/v2"
	MaxHandshakeRetries        = 10
)

// NATSRequester is the request/reply surface used by the trust exchange.
type NATSRequester interface {
	Request(subj string, data []byte, timeout time.Duration) (*nats.Msg, error)
}

type natsContextRequester interface {
	RequestWithContext(ctx context.Context, subj string, data []byte) (*nats.Msg, error)
}

// HandshakeRejectedError preserves the legacy string fields while exposing
// the normative protobuf reason separately.
type HandshakeRejectedError struct {
	Reason          string
	RequestID       string
	AgentID         string
	WorkerID        string
	RejectionReason agentv1.WorkerHandshakeRejectionReason
}

func (e *HandshakeRejectedError) Error() string {
	return fmt.Sprintf("cap-runtime: handshake rejected: reason=%s agent_id=%s request_id=%s", e.Reason, e.AgentID, e.RequestID)
}

func legacyRejectionReason(reason agentv1.WorkerHandshakeRejectionReason) string {
	const prefix = "WORKER_HANDSHAKE_REJECTION_REASON_"
	return strings.ToLower(strings.TrimPrefix(reason.String(), prefix))
}

type sessionState struct {
	mu    sync.Mutex
	token string
	exp   time.Time
}

func (s *sessionState) set(token string, exp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token, s.exp = token, exp.UTC()
}

func (s *sessionState) active(now time.Time) (string, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && !s.exp.After(now) {
		s.token, s.exp = "", time.Time{}
	}
	return s.token, s.exp
}

func (s *sessionState) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token, s.exp = "", time.Time{}
}

// SessionToken returns only a still-unexpired, atomically installed token.
func (a *Agent) SessionToken() (string, time.Time) {
	if a == nil || a.session == nil {
		return "", time.Time{}
	}
	return a.session.active(time.Now())
}

func (a *Agent) setSession(token string, exp time.Time) {
	if a == nil {
		return
	}
	if a.session == nil {
		a.session = &sessionState{}
	}
	a.session.set(token, exp)
}

func (a *Agent) clearSession() {
	if a != nil && a.session != nil {
		a.session.clear()
	}
}

func (a *Agent) resolveHandshakeMode() (HandshakeMode, error) {
	raw := strings.TrimSpace(string(a.HandshakeMode))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv(EnvHandshakeMode))
	}
	if raw == "" {
		return HandshakeModeOff, nil
	}
	mode, err := capsdk.ParseWorkerTrustMode(raw)
	if err != nil {
		return "", fmt.Errorf("cap-runtime: invalid handshake mode %q: %w", raw, err)
	}
	return HandshakeMode(mode.String()), nil
}

func (a *Agent) activeHandshakeMode() HandshakeMode {
	if a != nil && a.trustMode != "" {
		return a.trustMode
	}
	mode, _ := a.resolveHandshakeMode()
	return mode
}

func (a *Agent) validateTrustStartup() error {
	mode := a.trustMode
	if mode == "" {
		var err error
		mode, err = a.resolveHandshakeMode()
		if err != nil {
			return err
		}
	}
	trust := a.workerTrustConfig()
	if mode == HandshakeModeOff {
		if workerTrustConfigured(*trust) {
			return errors.New("cap-runtime: handshake mode off conflicts with worker trust configuration")
		}
		if !a.AllowUnsigned && (len(a.PublicKeys) == 0 || a.PrivateKey == nil) {
			return errors.New("cap-runtime: signing keys required; set AllowUnsigned for explicit unsigned legacy mode")
		}
		return nil
	}
	if err := capsdk.ValidateWorkerTrustConfig(trust); err != nil {
		return fmt.Errorf("cap-runtime: worker trust configuration: %w", err)
	}
	if a.HandshakeTimeout < 0 || a.HandshakeRetries < 0 {
		return errors.New("cap-runtime: worker trust configuration requires non-negative timeout and retries")
	}
	if a.HandshakeTimeout > capsdk.WorkerHandshakeMaxLifetime || a.HandshakeRetries > MaxHandshakeRetries {
		return errors.New("cap-runtime: worker trust configuration exceeds timeout or retry bounds")
	}
	if a.SenderID == "" {
		a.SenderID = trust.WorkerID
	}
	if a.SenderID != trust.WorkerID {
		return fmt.Errorf("cap-runtime: SenderID %q does not match worker trust identity", a.SenderID)
	}
	return nil
}

func (a *Agent) workerTrustConfig() *capsdk.WorkerTrustConfig {
	if a.trustConfig != nil {
		return a.trustConfig
	}
	return &a.WorkerTrust
}

func workerTrustConfigured(config capsdk.WorkerTrustConfig) bool {
	return config.WorkerID != "" || config.ExpectedAgentID != "" || config.TenantID != "" ||
		config.Audience != "" || config.ProofKeyID != "" || config.ProofPrivateKey != nil ||
		config.ExpectedSchedulerID != "" || len(config.SchedulerPublicKeys) != 0 || config.SDKVersion != ""
}

func (a *Agent) effectiveTenant() string {
	if a.activeHandshakeMode() != HandshakeModeOff {
		return a.workerTrustConfig().TenantID
	}
	if strings.TrimSpace(a.Tenant) != "" {
		return strings.TrimSpace(a.Tenant)
	}
	return strings.TrimSpace(os.Getenv(EnvAgentTenant))
}

func (a *Agent) effectiveSDKVersion() string {
	if a.activeHandshakeMode() != HandshakeModeOff {
		return a.workerTrustConfig().SDKVersion
	}
	if strings.TrimSpace(a.SDKVersion) != "" {
		return strings.TrimSpace(a.SDKVersion)
	}
	if value := strings.TrimSpace(os.Getenv(EnvAgentSDKVersion)); value != "" {
		return value
	}
	return DefaultHandshakeSDKVersion
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
