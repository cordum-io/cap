package capsdk

import (
	"strings"
	"testing"
	"time"
)

// validRequest returns a fully-populated HandshakeRequest — individual
// tests mutate one field at a time to assert the validator rejects
// each required field independently.
func validRequest() *HandshakeRequest {
	return &HandshakeRequest{
		AgentID:      "agent-alpha",
		Tenant:       "default",
		SDKVersion:   "v1.2.3",
		Capabilities: []string{"tool:files.read"},
		Nonce:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RequestID:    "req-123",
		Timestamp:    time.Now().UTC(),
	}
}

// TestHandshakeRequest_RoundTripPreservesEveryField pins the wire
// contract: marshal followed by unmarshal yields a struct with every
// field identical to the source. A silent JSON-tag rename would break
// cross-SDK compatibility (Python/Node parse the same bytes).
func TestHandshakeRequest_RoundTripPreservesEveryField(t *testing.T) {
	t.Parallel()
	src := validRequest()
	raw, err := MarshalHandshakeRequest(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalHandshakeRequest(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.AgentID != src.AgentID {
		t.Errorf("AgentID: got %q, want %q", got.AgentID, src.AgentID)
	}
	if got.Tenant != src.Tenant {
		t.Errorf("Tenant mismatch")
	}
	if got.SDKVersion != src.SDKVersion {
		t.Errorf("SDKVersion mismatch")
	}
	if len(got.Capabilities) != len(src.Capabilities) {
		t.Errorf("Capabilities len mismatch: %d vs %d", len(got.Capabilities), len(src.Capabilities))
	}
	if got.Nonce != src.Nonce {
		t.Errorf("Nonce mismatch")
	}
	if got.RequestID != src.RequestID {
		t.Errorf("RequestID mismatch")
	}
	if !got.Timestamp.Equal(src.Timestamp) {
		t.Errorf("Timestamp mismatch: %v vs %v", got.Timestamp, src.Timestamp)
	}
}

// TestHandshakeRequest_MissingFieldsRejected is a table test against
// ValidateHandshakeRequest. Each case zeros a different required
// field and asserts the returned error names that field so the
// scheduler's error response carries the specific remediation.
func TestHandshakeRequest_MissingFieldsRejected(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*HandshakeRequest)
		needle  string
	}{
		{"nil", nil, "nil request"},
		{"missing agent_id", func(r *HandshakeRequest) { r.AgentID = "" }, "agent_id"},
		{"missing tenant", func(r *HandshakeRequest) { r.Tenant = "" }, "tenant"},
		{"missing sdk_version", func(r *HandshakeRequest) { r.SDKVersion = "" }, "sdk_version"},
		{"missing nonce", func(r *HandshakeRequest) { r.Nonce = "" }, "nonce"},
		{"short nonce", func(r *HandshakeRequest) { r.Nonce = "short" }, "nonce too short"},
		{"missing request_id", func(r *HandshakeRequest) { r.RequestID = "" }, "request_id"},
		{"missing timestamp", func(r *HandshakeRequest) { r.Timestamp = time.Time{} }, "timestamp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.mutate == nil {
				err = ValidateHandshakeRequest(nil)
			} else {
				req := validRequest()
				tc.mutate(req)
				err = ValidateHandshakeRequest(req)
			}
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Errorf("error %q did not mention %q", err, tc.needle)
			}
		})
	}
}

// TestHandshakeRequest_MarshalRefusesInvalid confirms Marshal is the
// last line of defence — a caller that bypasses ValidateHandshakeRequest
// and hands a partial struct to Marshal still gets rejected rather
// than having a half-valid payload land on the wire.
func TestHandshakeRequest_MarshalRefusesInvalid(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Tenant = ""
	if _, err := MarshalHandshakeRequest(req); err == nil {
		t.Fatal("expected marshal to refuse invalid request")
	}
}

// TestHandshakeResponse_RoundTripAcceptedPath covers the common
// accepted case: a scheduler that mints a session token and replies.
func TestHandshakeResponse_RoundTripAcceptedPath(t *testing.T) {
	t.Parallel()
	exp := time.Now().Add(time.Hour).UTC()
	src := &HandshakeResponse{
		SessionToken: "jwt.header.body.sig",
		TokenExp:     exp,
		RequestID:    "req-123",
	}
	raw, err := MarshalHandshakeResponse(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalHandshakeResponse(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SessionToken != src.SessionToken {
		t.Errorf("SessionToken mismatch")
	}
	if !got.TokenExp.Equal(exp) {
		t.Errorf("TokenExp mismatch: %v vs %v", got.TokenExp, exp)
	}
	if got.Rejected {
		t.Errorf("accepted response marshalled Rejected=true")
	}
}

// TestHandshakeResponse_RoundTripRejectedPath covers the explicit
// rejection path: Rejected=true + a sentinel Reason + no token.
func TestHandshakeResponse_RoundTripRejectedPath(t *testing.T) {
	t.Parallel()
	src := &HandshakeResponse{
		RequestID: "req-123",
		Rejected:  true,
		Reason:    HandshakeRejectUnknownAgent,
	}
	raw, err := MarshalHandshakeResponse(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalHandshakeResponse(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.Rejected {
		t.Errorf("Rejected lost through round-trip")
	}
	if got.Reason != HandshakeRejectUnknownAgent {
		t.Errorf("Reason mismatch: %q", got.Reason)
	}
	if got.SessionToken != "" {
		t.Errorf("rejected response has non-empty SessionToken")
	}
}

// TestHandshakeResponse_MarshalRefusesAcceptedWithEmptyToken catches
// the specific error QA flagged on sibling tasks: a "successful"
// response that accidentally carries an empty SessionToken would
// silently dispatch workers without trust. Marshal refuses so the
// scheduler's handler must either mint a token or set Rejected.
func TestHandshakeResponse_MarshalRefusesAcceptedWithEmptyToken(t *testing.T) {
	t.Parallel()
	resp := &HandshakeResponse{
		RequestID: "req-123",
		// Rejected:false, SessionToken:""
	}
	if _, err := MarshalHandshakeResponse(resp); err == nil {
		t.Error("expected marshal to refuse accepted response with empty token")
	}
}

// TestHandshakeResponse_MarshalRefusesRejectedWithoutReason mirrors
// the above for the rejection path — a rejected response MUST carry
// a canonical Reason so the worker + audit trail have a stable
// remediation signal.
func TestHandshakeResponse_MarshalRefusesRejectedWithoutReason(t *testing.T) {
	t.Parallel()
	resp := &HandshakeResponse{RequestID: "req-123", Rejected: true}
	if _, err := MarshalHandshakeResponse(resp); err == nil {
		t.Error("expected marshal to refuse rejected response without reason")
	}
}

// TestHandshakeResponse_UnmarshalRequiresRequestID pins the one
// field the worker uses to pair a response with its pending in-flight
// request — without it the SDK cannot correlate reliably.
func TestHandshakeResponse_UnmarshalRequiresRequestID(t *testing.T) {
	t.Parallel()
	// Hand-crafted JSON bypassing MarshalHandshakeResponse's validator.
	raw := []byte(`{"session_token":"x","token_exp":"2026-01-01T00:00:00Z"}`)
	if _, err := UnmarshalHandshakeResponse(raw); err == nil {
		t.Error("expected unmarshal to refuse response without request_id")
	}
}

// TestHandshakeSubjects pins the NATS subject names — SDKs in other
// languages publish to these exact strings, so a silent rename would
// partition the fleet.
func TestHandshakeSubjects(t *testing.T) {
	t.Parallel()
	if WorkerHandshakeSubject != "sys.worker.handshake" {
		t.Errorf("WorkerHandshakeSubject = %q", WorkerHandshakeSubject)
	}
	if WorkerHandshakeRenewSubject != "sys.worker.handshake.renew" {
		t.Errorf("WorkerHandshakeRenewSubject = %q", WorkerHandshakeRenewSubject)
	}
}
