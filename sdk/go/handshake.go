package capsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Phase-2 boundary-hardening handshake types.
//
// The existing cordum.agent.v1.Handshake protobuf message covers
// generic protocol negotiation (component role, supported versions,
// ready topics, capability flags). The Phase-2 worker-trust handshake
// is a narrower per-worker authentication round-trip: the worker
// asserts its identity, the scheduler mints a short-lived session
// token, and subsequent packets carry that token for verification.
//
// These types are serialised as JSON inside a BusPacket payload so
// the protocol can evolve (add advisory fields like client_region,
// kernel_id, image_digest) without a proto regeneration across every
// SDK on every release. Missing-field validation is enforced at
// marshal/unmarshal boundaries so a malformed request never reaches
// the scheduler's handshake handler.

// WorkerHandshakeSubject is the NATS subject the worker publishes its
// HandshakeRequest on at Agent.Start. The scheduler subscribes via a
// queue group and replies on the per-request reply inbox.
const WorkerHandshakeSubject = "sys.worker.handshake"

// WorkerHandshakeRenewSubject is the NATS subject for renew requests —
// a separate subject so the scheduler can attach different rate-limit
// policy (renews are bounded by the session-token exp; initial
// handshakes happen at boot).
const WorkerHandshakeRenewSubject = "sys.worker.handshake.renew"

// WorkerHandshakeMaxSkew bounds how much clock skew the scheduler
// tolerates between its wall clock and the HandshakeRequest.Timestamp
// before rejecting for replay safety. 60 s matches the JWT verifier
// tolerance and the audit-chain event time.
const WorkerHandshakeMaxSkew = 60 * time.Second

// WorkerHandshakeNonceLength is the minimum nonce byte count workers
// must supply — 16 bytes (128 bits) of entropy is plenty for replay
// uniqueness at any realistic fleet size. Marshal rejects shorter
// nonces rather than silently accepting them.
const WorkerHandshakeNonceLength = 16

// HandshakeRequest is the worker's initial identity + capability
// assertion. The scheduler verifies every field before minting a
// session token; fields left empty cause the request to be rejected.
type HandshakeRequest struct {
	// AgentID is the worker's stable identity as known to the
	// scheduler's AgentIdentityStore. Used to resolve the tenant,
	// risk tier, and allowed tools the token will carry.
	AgentID string `json:"agent_id"`

	// Tenant scopes the handshake to a single tenant. Must match the
	// tenant the AgentIdentityStore records for AgentID; mismatch
	// triggers a rejected response.
	Tenant string `json:"tenant"`

	// SDKVersion is the semver-ish version string of the worker's
	// SDK. Used by the scheduler to decide whether the worker is new
	// enough to enforce token verification, and to surface "please
	// upgrade" hints when enforce-mode rejects an old SDK.
	SDKVersion string `json:"sdk_version"`

	// Capabilities is the worker-advertised feature set (e.g.
	// "tool:files.read", "capability:code-exec"). The scheduler
	// validates this against the AgentIdentity.AllowedCapabilities
	// record and refuses the handshake on privilege escalation.
	Capabilities []string `json:"capabilities,omitempty"`

	// Nonce is a per-request random value preventing replay. The
	// scheduler SETNX's the nonce in Redis with a short TTL; a repeat
	// submission is refused. Hex-encoded; at least
	// WorkerHandshakeNonceLength bytes of entropy.
	Nonce string `json:"nonce"`

	// RequestID is the worker-chosen correlation ID echoed in the
	// response. Lets a worker pair responses to requests without
	// relying on the NATS reply inbox for uniqueness.
	RequestID string `json:"request_id"`

	// Timestamp is the worker's wall-clock time at the moment the
	// request was built. The scheduler rejects requests with a skew
	// larger than WorkerHandshakeMaxSkew.
	Timestamp time.Time `json:"timestamp"`
}

// HandshakeResponse is the scheduler's reply. A successful response
// carries SessionToken + TokenExp. A rejected response sets
// Rejected=true and fills Reason with a machine-readable sentinel
// plus human-readable detail.
type HandshakeResponse struct {
	// SessionToken is an Ed25519-signed JWT the worker attaches to
	// every outbound packet. Empty on rejection.
	SessionToken string `json:"session_token,omitempty"`

	// TokenExp is the absolute expiry of the SessionToken. The worker
	// schedules renew at ~exp - TokenLifetime/2.
	TokenExp time.Time `json:"token_exp,omitempty"`

	// RequestID echoes HandshakeRequest.RequestID.
	RequestID string `json:"request_id"`

	// Rejected reports the scheduler refused the handshake. Callers
	// MUST check this before using SessionToken — an empty token does
	// not by itself mean "accepted with empty token".
	Rejected bool `json:"rejected,omitempty"`

	// Reason is the rejection sentinel. Canonical values live in
	// HandshakeReject* constants below.
	Reason string `json:"reason,omitempty"`
}

// HandshakeReject* are the canonical rejection reasons. Machine-
// readable so the SDK can map them to specific error types and the
// dashboard can render appropriate remediation.
const (
	HandshakeRejectUnknownAgent      = "unknown_agent"
	HandshakeRejectTenantMismatch    = "tenant_mismatch"
	HandshakeRejectReplay            = "replay_detected"
	HandshakeRejectClockSkew         = "clock_skew"
	HandshakeRejectCapabilityDenied  = "capability_denied"
	HandshakeRejectSDKTooOld         = "sdk_too_old"
	HandshakeRejectInternalError     = "internal_error"
	HandshakeRejectMalformedRequest  = "malformed_request"
	HandshakeRejectInvalidSignature  = "invalid_signature"
)

// MarshalHandshakeRequest serialises the request as JSON bytes
// suitable for the BusPacket payload. Validates required fields
// before marshalling so a partial request never reaches the wire.
func MarshalHandshakeRequest(req *HandshakeRequest) ([]byte, error) {
	if err := ValidateHandshakeRequest(req); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal handshake request: %w", err)
	}
	return raw, nil
}

// UnmarshalHandshakeRequest parses JSON bytes into a HandshakeRequest
// and validates required fields. A malformed payload returns the
// specific parse error; a parseable-but-invalid payload returns the
// structured validation error so the handler can emit a canonical
// HandshakeRejectMalformedRequest response.
func UnmarshalHandshakeRequest(raw []byte) (*HandshakeRequest, error) {
	if len(raw) == 0 {
		return nil, errors.New("handshake: empty payload")
	}
	var req HandshakeRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("handshake: parse: %w", err)
	}
	if err := ValidateHandshakeRequest(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

// MarshalHandshakeResponse serialises the response as JSON bytes.
// The scheduler builds this after minting (or refusing) a token;
// validation is minimal because trusted server code produces it.
func MarshalHandshakeResponse(resp *HandshakeResponse) ([]byte, error) {
	if resp == nil {
		return nil, errors.New("handshake: nil response")
	}
	if strings.TrimSpace(resp.RequestID) == "" {
		return nil, errors.New("handshake: response missing request_id")
	}
	if !resp.Rejected && strings.TrimSpace(resp.SessionToken) == "" {
		return nil, errors.New("handshake: accepted response missing session_token")
	}
	if resp.Rejected && strings.TrimSpace(resp.Reason) == "" {
		return nil, errors.New("handshake: rejected response missing reason")
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal handshake response: %w", err)
	}
	return raw, nil
}

// UnmarshalHandshakeResponse parses JSON bytes into a HandshakeResponse.
// Empty RequestID is a parse error because the worker needs it to pair
// the response with its pending in-flight request.
func UnmarshalHandshakeResponse(raw []byte) (*HandshakeResponse, error) {
	if len(raw) == 0 {
		return nil, errors.New("handshake: empty payload")
	}
	var resp HandshakeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("handshake: parse response: %w", err)
	}
	if strings.TrimSpace(resp.RequestID) == "" {
		return nil, errors.New("handshake: response missing request_id")
	}
	return &resp, nil
}

// ValidateHandshakeRequest enforces the required-field contract.
// Exported so the scheduler's handler can run the same check before
// hashing the request into the nonce store.
func ValidateHandshakeRequest(req *HandshakeRequest) error {
	if req == nil {
		return errors.New("handshake: nil request")
	}
	var missing []string
	if strings.TrimSpace(req.AgentID) == "" {
		missing = append(missing, "agent_id")
	}
	if strings.TrimSpace(req.Tenant) == "" {
		missing = append(missing, "tenant")
	}
	if strings.TrimSpace(req.SDKVersion) == "" {
		missing = append(missing, "sdk_version")
	}
	if strings.TrimSpace(req.Nonce) == "" {
		missing = append(missing, "nonce")
	} else if len([]byte(req.Nonce)) < WorkerHandshakeNonceLength {
		return fmt.Errorf("handshake: nonce too short (got %d bytes, want >= %d)", len(req.Nonce), WorkerHandshakeNonceLength)
	}
	if strings.TrimSpace(req.RequestID) == "" {
		missing = append(missing, "request_id")
	}
	if req.Timestamp.IsZero() {
		missing = append(missing, "timestamp")
	}
	if len(missing) > 0 {
		return fmt.Errorf("handshake: missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}
