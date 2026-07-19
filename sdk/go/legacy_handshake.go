package capsdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Deprecated: use WorkerHandshakeChallengeSubject and the protobuf trust flow.
const WorkerHandshakeSubject = "sys.worker.handshake"

// Deprecated: use WorkerHandshakeChallengeSubject with RENEW purpose.
const WorkerHandshakeRenewSubject = "sys.worker.handshake.renew"

// WorkerHandshakeNonceLength is retained for source compatibility only.
// New protobuf requests require WorkerHandshakeNonceSize exact bytes.
const WorkerHandshakeNonceLength = 16

// Deprecated: HandshakeRequest is the retired JSON codec shape. Runtime
// transports never publish it; use WorkerHandshakeRequestOptions instead.
type HandshakeRequest struct {
	AgentID      string    `json:"agent_id"`
	Tenant       string    `json:"tenant"`
	SDKVersion   string    `json:"sdk_version"`
	Capabilities []string  `json:"capabilities,omitempty"`
	Nonce        string    `json:"nonce"`
	RequestID    string    `json:"request_id"`
	Timestamp    time.Time `json:"timestamp"`
}

// Deprecated: HandshakeResponse is the retired JSON codec shape. Runtime
// transports accept only signed protobuf WorkerHandshakeResult packets.
type HandshakeResponse struct {
	SessionToken string    `json:"session_token,omitempty"`
	TokenExp     time.Time `json:"token_exp,omitempty"`
	RequestID    string    `json:"request_id"`
	Rejected     bool      `json:"rejected,omitempty"`
	Reason       string    `json:"reason,omitempty"`
}

// Deprecated legacy rejection strings retained for source compatibility.
const (
	HandshakeRejectUnknownAgent     = "unknown_agent"
	HandshakeRejectTenantMismatch   = "tenant_mismatch"
	HandshakeRejectReplay           = "replay_detected"
	HandshakeRejectClockSkew        = "clock_skew"
	HandshakeRejectCapabilityDenied = "capability_denied"
	HandshakeRejectSDKTooOld        = "sdk_too_old"
	HandshakeRejectInternalError    = "internal_error"
	HandshakeRejectMalformedRequest = "malformed_request"
	HandshakeRejectInvalidSignature = "invalid_signature"
)

// MarshalHandshakeRequest encodes the retired JSON shape without enabling its
// transport. New integrations must use MarshalWorkerTrustPacket.
func MarshalHandshakeRequest(request *HandshakeRequest) ([]byte, error) {
	if err := ValidateHandshakeRequest(request); err != nil {
		return nil, err
	}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal handshake request: %w", err)
	}
	return boundLegacyHandshake(data)
}

// UnmarshalHandshakeRequest decodes only the retired, bounded JSON shape.
func UnmarshalHandshakeRequest(data []byte) (*HandshakeRequest, error) {
	if err := validateLegacyHandshakeSize(data); err != nil {
		return nil, err
	}
	request := &HandshakeRequest{}
	if err := decodeLegacyHandshake(data, request); err != nil {
		return nil, fmt.Errorf("handshake: parse: %w", err)
	}
	if err := ValidateHandshakeRequest(request); err != nil {
		return nil, err
	}
	return request, nil
}

// MarshalHandshakeResponse encodes the retired JSON response shape.
func MarshalHandshakeResponse(response *HandshakeResponse) ([]byte, error) {
	if err := validateLegacyHandshakeResponse(response); err != nil {
		return nil, err
	}
	data, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal handshake response: %w", err)
	}
	return boundLegacyHandshake(data)
}

// UnmarshalHandshakeResponse decodes and validates the retired response shape.
func UnmarshalHandshakeResponse(data []byte) (*HandshakeResponse, error) {
	if err := validateLegacyHandshakeSize(data); err != nil {
		return nil, err
	}
	response := &HandshakeResponse{}
	if err := decodeLegacyHandshake(data, response); err != nil {
		return nil, fmt.Errorf("handshake: parse response: %w", err)
	}
	if err := validateLegacyHandshakeResponse(response); err != nil {
		return nil, err
	}
	return response, nil
}

// ValidateHandshakeRequest validates the retired JSON request shape.
func ValidateHandshakeRequest(request *HandshakeRequest) error {
	if request == nil {
		return errors.New("handshake: nil request")
	}
	missing := legacyMissingRequestFields(request)
	if len(missing) != 0 {
		return fmt.Errorf("handshake: missing required field(s): %s", strings.Join(missing, ", "))
	}
	if len([]byte(request.Nonce)) < WorkerHandshakeNonceLength {
		return fmt.Errorf("handshake: nonce too short (got %d bytes, want >= %d)", len([]byte(request.Nonce)), WorkerHandshakeNonceLength)
	}
	return nil
}

func legacyMissingRequestFields(request *HandshakeRequest) []string {
	fields := []struct{ name, value string }{
		{"agent_id", request.AgentID}, {"tenant", request.Tenant}, {"sdk_version", request.SDKVersion},
		{"nonce", request.Nonce}, {"request_id", request.RequestID},
	}
	missing := make([]string, 0, len(fields)+1)
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if request.Timestamp.IsZero() {
		missing = append(missing, "timestamp")
	}
	return missing
}

func validateLegacyHandshakeResponse(response *HandshakeResponse) error {
	if response == nil {
		return errors.New("handshake: nil response")
	}
	if strings.TrimSpace(response.RequestID) == "" {
		return errors.New("handshake: response missing request_id")
	}
	if response.Rejected && strings.TrimSpace(response.Reason) == "" {
		return errors.New("handshake: rejected response missing reason")
	}
	if response.Rejected && (response.SessionToken != "" || !response.TokenExp.IsZero()) {
		return errors.New("handshake: rejected response carries token metadata")
	}
	if !response.Rejected && strings.TrimSpace(response.SessionToken) == "" {
		return errors.New("handshake: accepted response missing session_token")
	}
	return nil
}

type legacyHandshakeMessage interface {
	HandshakeRequest | HandshakeResponse
}

func decodeLegacyHandshake[T legacyHandshakeMessage](data []byte, destination *T) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON value")
	}
	return nil
}

func validateLegacyHandshakeSize(data []byte) error {
	if len(data) == 0 {
		return errors.New("handshake: empty payload")
	}
	if len(data) > WorkerHandshakeMaxPacketSize {
		return fmt.Errorf("handshake: payload exceeds %d bytes", WorkerHandshakeMaxPacketSize)
	}
	return nil
}

func boundLegacyHandshake(data []byte) ([]byte, error) {
	if len(data) > WorkerHandshakeMaxPacketSize {
		return nil, fmt.Errorf("handshake: payload exceeds %d bytes", WorkerHandshakeMaxPacketSize)
	}
	return data, nil
}
