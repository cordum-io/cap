package capsdk

import (
	"errors"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

func TestProtocolErrorMessage(t *testing.T) {
	err := NewMalformedPacketError("bad wire bytes")
	expected := "capsdk: [ERROR_CODE_PROTOCOL_MALFORMED_PACKET]: bad wire bytes"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestIsProtocolError(t *testing.T) {
	err := NewSignatureInvalidError("bad sig")
	if !IsProtocolError(err) {
		t.Fatal("expected IsProtocolError to return true")
	}
	if IsProtocolError(errors.New("plain error")) {
		t.Fatal("expected IsProtocolError to return false for plain error")
	}
}

func TestProtocolErrorCode(t *testing.T) {
	err := NewJobTimeoutError("deadline exceeded")
	code := ProtocolErrorCode(err)
	if code != agentv1.ErrorCode_ERROR_CODE_JOB_TIMEOUT {
		t.Fatalf("expected ERROR_CODE_JOB_TIMEOUT, got %v", code)
	}
	if ProtocolErrorCode(errors.New("plain")) != agentv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatal("expected ERROR_CODE_UNSPECIFIED for plain error")
	}
}

func TestProtocolErrorIs(t *testing.T) {
	err := NewMalformedPacketError("test")
	target := NewMalformedPacketError("different message")
	if !errors.Is(err, target) {
		t.Fatal("expected errors.Is to match by code")
	}

	other := NewSignatureInvalidError("test")
	if errors.Is(err, other) {
		t.Fatal("expected errors.Is to NOT match different codes")
	}
}

func TestAllErrorCodes(t *testing.T) {
	cases := []struct {
		err  *ProtocolError
		code agentv1.ErrorCode
	}{
		{NewVersionMismatchError(""), agentv1.ErrorCode_ERROR_CODE_PROTOCOL_VERSION_MISMATCH},
		{NewMalformedPacketError(""), agentv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED_PACKET},
		{NewUnknownPayloadError(""), agentv1.ErrorCode_ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD},
		{NewSignatureInvalidError(""), agentv1.ErrorCode_ERROR_CODE_PROTOCOL_SIGNATURE_INVALID},
		{NewSignatureMissingError(""), agentv1.ErrorCode_ERROR_CODE_PROTOCOL_SIGNATURE_MISSING},
		{NewJobTimeoutError(""), agentv1.ErrorCode_ERROR_CODE_JOB_TIMEOUT},
		{NewResourceExhaustedError(""), agentv1.ErrorCode_ERROR_CODE_JOB_RESOURCE_EXHAUSTED},
		{NewPermissionDeniedError(""), agentv1.ErrorCode_ERROR_CODE_JOB_PERMISSION_DENIED},
		{NewInvalidInputError(""), agentv1.ErrorCode_ERROR_CODE_JOB_INVALID_INPUT},
		{NewJobNotFoundError(""), agentv1.ErrorCode_ERROR_CODE_JOB_NOT_FOUND},
		{NewDuplicateJobError(""), agentv1.ErrorCode_ERROR_CODE_JOB_DUPLICATE},
		{NewWorkerUnavailableError(""), agentv1.ErrorCode_ERROR_CODE_JOB_WORKER_UNAVAILABLE},
		{NewSafetyDeniedError(""), agentv1.ErrorCode_ERROR_CODE_SAFETY_DENIED},
		{NewPolicyViolationError(""), agentv1.ErrorCode_ERROR_CODE_SAFETY_POLICY_VIOLATION},
		{NewRiskTagBlockedError(""), agentv1.ErrorCode_ERROR_CODE_SAFETY_RISK_TAG_BLOCKED},
		{NewPublishFailedError(""), agentv1.ErrorCode_ERROR_CODE_TRANSPORT_PUBLISH_FAILED},
		{NewSubscribeFailedError(""), agentv1.ErrorCode_ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED},
		{NewConnectionLostError(""), agentv1.ErrorCode_ERROR_CODE_TRANSPORT_CONNECTION_LOST},
	}
	for _, tc := range cases {
		if tc.err.Code != tc.code {
			t.Errorf("expected code %v, got %v", tc.code, tc.err.Code)
		}
	}
}
