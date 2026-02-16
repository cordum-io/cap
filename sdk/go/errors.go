package capsdk

import (
	"errors"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

// ProtocolError represents a typed CAP protocol error with a structured ErrorCode.
type ProtocolError struct {
	Code    agentv1.ErrorCode
	Message string
}

// Error implements the error interface.
func (e *ProtocolError) Error() string {
	return fmt.Sprintf("capsdk: [%s]: %s", e.Code.String(), e.Message)
}

// Is supports errors.Is matching by ErrorCode.
func (e *ProtocolError) Is(target error) bool {
	var t *ProtocolError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// IsProtocolError returns true if the error is a *ProtocolError.
func IsProtocolError(err error) bool {
	var pe *ProtocolError
	return errors.As(err, &pe)
}

// ProtocolErrorCode extracts the ErrorCode from a ProtocolError, or returns
// ERROR_CODE_UNSPECIFIED if the error is not a ProtocolError.
func ProtocolErrorCode(err error) agentv1.ErrorCode {
	var pe *ProtocolError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return agentv1.ErrorCode_ERROR_CODE_UNSPECIFIED
}

// Protocol errors (100-199)

func NewVersionMismatchError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_PROTOCOL_VERSION_MISMATCH, Message: msg}
}

func NewMalformedPacketError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED_PACKET, Message: msg}
}

func NewUnknownPayloadError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD, Message: msg}
}

func NewSignatureInvalidError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_PROTOCOL_SIGNATURE_INVALID, Message: msg}
}

func NewSignatureMissingError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_PROTOCOL_SIGNATURE_MISSING, Message: msg}
}

// Job errors (200-299)

func NewJobTimeoutError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_JOB_TIMEOUT, Message: msg}
}

func NewResourceExhaustedError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_JOB_RESOURCE_EXHAUSTED, Message: msg}
}

func NewPermissionDeniedError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_JOB_PERMISSION_DENIED, Message: msg}
}

func NewInvalidInputError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_JOB_INVALID_INPUT, Message: msg}
}

func NewJobNotFoundError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_JOB_NOT_FOUND, Message: msg}
}

func NewDuplicateJobError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_JOB_DUPLICATE, Message: msg}
}

func NewWorkerUnavailableError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_JOB_WORKER_UNAVAILABLE, Message: msg}
}

// Safety errors (300-399)

func NewSafetyDeniedError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_SAFETY_DENIED, Message: msg}
}

func NewPolicyViolationError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_SAFETY_POLICY_VIOLATION, Message: msg}
}

func NewRiskTagBlockedError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_SAFETY_RISK_TAG_BLOCKED, Message: msg}
}

// Transport errors (400-499)

func NewPublishFailedError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_TRANSPORT_PUBLISH_FAILED, Message: msg}
}

func NewSubscribeFailedError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED, Message: msg}
}

func NewConnectionLostError(msg string) *ProtocolError {
	return &ProtocolError{Code: agentv1.ErrorCode_ERROR_CODE_TRANSPORT_CONNECTION_LOST, Message: msg}
}
