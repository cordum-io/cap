package capsdk

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

// Worker trust handshake constants are shared by adapters and runtimes.
const (
	WorkerHandshakeChallengeSubject    = "sys.worker.handshake.challenge"
	WorkerHandshakeAuthenticateSubject = "sys.worker.handshake.authenticate"
	WorkerHandshakeAudience            = "cordum-scheduler"
	WorkerHandshakeProtocolVersion     = int32(1)
	WorkerHandshakeNonceSize           = 32
	WorkerHandshakeMaxPacketSize       = 64 * 1024
	WorkerHandshakeMaxIdentityLength   = 256
	WorkerHandshakeMaxSessionTokenSize = 16 * 1024
	WorkerHandshakeMaxSkew             = time.Minute
	WorkerHandshakeMaxLifetime         = time.Minute
)

// WorkerTrustMode controls admission after an authenticated handshake failure.
// It never permits an unsigned token to be minted or installed.
type WorkerTrustMode uint8

const (
	WorkerTrustModeOff WorkerTrustMode = iota + 1
	WorkerTrustModeWarn
	WorkerTrustModeEnforce
)

var (
	ErrWorkerTrustMode        = errors.New("capsdk: invalid worker trust mode")
	ErrWorkerTrustConfig      = errors.New("capsdk: invalid worker trust configuration")
	ErrWorkerHandshakePacket  = errors.New("capsdk: invalid worker handshake packet")
	ErrWorkerHandshakeBinding = errors.New("capsdk: worker handshake correlation failed")
	ErrWorkerHandshakeExpired = errors.New("capsdk: worker handshake expired")
)

// WorkerTrustConfig pins both worker proof identity and scheduler trust.
type WorkerTrustConfig struct {
	WorkerID            string
	ExpectedAgentID     string
	TenantID            string
	Audience            string
	ProofKeyID          string
	ProofPrivateKey     *ecdsa.PrivateKey
	ExpectedSchedulerID string
	SchedulerPublicKeys map[string]*ecdsa.PublicKey
	SDKVersion          string
}

// WorkerHandshakeRequestOptions supplies unique correlation and time inputs.
type WorkerHandshakeRequestOptions struct {
	RequestID   string
	TraceID     string
	Purpose     agentv1.WorkerHandshakePurpose
	ClientNonce []byte
	CreatedAt   time.Time
}

// WorkerHandshakeSession is returned only after a signed accepted result.
type WorkerHandshakeSession struct {
	Token     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// WorkerHandshakeRejectionError exposes only the protocol rejection enum.
type WorkerHandshakeRejectionError struct {
	Reason    agentv1.WorkerHandshakeRejectionReason
	RequestID string
	AgentID   string
	WorkerID  string
}

func (e *WorkerHandshakeRejectionError) Error() string {
	return "capsdk: worker handshake rejected"
}

// ParseWorkerTrustMode requires an explicit off, warn, or enforce value.
func ParseWorkerTrustMode(raw string) (WorkerTrustMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "off":
		return WorkerTrustModeOff, nil
	case "warn":
		return WorkerTrustModeWarn, nil
	case "enforce":
		return WorkerTrustModeEnforce, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrWorkerTrustMode, raw)
	}
}

func (m WorkerTrustMode) String() string {
	switch m {
	case WorkerTrustModeOff:
		return "off"
	case WorkerTrustModeWarn:
		return "warn"
	case WorkerTrustModeEnforce:
		return "enforce"
	default:
		return "invalid"
	}
}

// ValidateWorkerTrustConfig rejects partial or unpinned trust configuration.
func ValidateWorkerTrustConfig(config *WorkerTrustConfig) error {
	if config == nil {
		return fmt.Errorf("%w: nil", ErrWorkerTrustConfig)
	}
	if missingTrustConfigString(config) || invalidTrustConfigString(config) {
		return fmt.Errorf("%w: required identity field is empty", ErrWorkerTrustConfig)
	}
	if config.Audience != WorkerHandshakeAudience {
		return fmt.Errorf("%w: audience must be %q", ErrWorkerTrustConfig, WorkerHandshakeAudience)
	}
	if !validP256PrivateKey(config.ProofPrivateKey) {
		return fmt.Errorf("%w: proof key is not P-256", ErrWorkerTrustConfig)
	}
	if len(config.SchedulerPublicKeys) == 0 {
		return fmt.Errorf("%w: scheduler keys are empty", ErrWorkerTrustConfig)
	}
	for keyID, key := range config.SchedulerPublicKeys {
		if !canonicalTrustString(keyID) || !validP256PublicKey(key) {
			return fmt.Errorf("%w: scheduler key %q is invalid", ErrWorkerTrustConfig, keyID)
		}
	}
	return nil
}

func invalidTrustConfigString(config *WorkerTrustConfig) bool {
	values := []string{
		config.WorkerID, config.ExpectedAgentID, config.TenantID, config.Audience,
		config.ProofKeyID, config.ExpectedSchedulerID, config.SDKVersion,
	}
	for _, value := range values {
		if !canonicalTrustString(value) {
			return true
		}
	}
	return false
}

func canonicalTrustString(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > WorkerHandshakeMaxIdentityLength {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func missingTrustConfigString(config *WorkerTrustConfig) bool {
	values := []string{
		config.WorkerID, config.ExpectedAgentID, config.TenantID, config.Audience,
		config.ProofKeyID, config.ExpectedSchedulerID, config.SDKVersion,
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}
