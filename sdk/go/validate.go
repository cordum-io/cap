package capsdk

import (
	"errors"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

// ValidationError describes a single validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("capsdk: validation: %s: %s", e.Field, e.Message)
}

// Sentinel validation errors.
var (
	ErrEmptyJobID   = errors.New("capsdk: job_id must not be empty")
	ErrEmptyTopic   = errors.New("capsdk: topic must not be empty")
	ErrEmptyTraceID = errors.New("capsdk: trace_id must not be empty")
	ErrEmptySender  = errors.New("capsdk: sender_id must not be empty")
)

// ValidateJobRequest checks semantic constraints on a JobRequest.
// It returns nil if the message is valid.
func ValidateJobRequest(req *agentv1.JobRequest) error {
	if req == nil {
		return &ValidationError{Field: "JobRequest", Message: "must not be nil"}
	}
	if req.GetJobId() == "" {
		return &ValidationError{Field: "job_id", Message: "must not be empty"}
	}
	if req.GetTopic() == "" {
		return &ValidationError{Field: "topic", Message: "must not be empty"}
	}
	if p := req.GetPriority(); p < 0 || p > agentv1.JobPriority_JOB_PRIORITY_CRITICAL {
		return &ValidationError{Field: "priority", Message: fmt.Sprintf("invalid value %d", p)}
	}
	if req.GetStepIndex() < 0 {
		return &ValidationError{Field: "step_index", Message: "must not be negative"}
	}
	if b := req.GetBudget(); b != nil {
		if b.GetMaxInputTokens() < 0 {
			return &ValidationError{Field: "budget.max_input_tokens", Message: "must not be negative"}
		}
		if b.GetMaxOutputTokens() < 0 {
			return &ValidationError{Field: "budget.max_output_tokens", Message: "must not be negative"}
		}
		if b.GetMaxTotalTokens() < 0 {
			return &ValidationError{Field: "budget.max_total_tokens", Message: "must not be negative"}
		}
		if b.GetDeadlineMs() < 0 {
			return &ValidationError{Field: "budget.deadline_ms", Message: "must not be negative"}
		}
	}
	return nil
}

// ValidateJobResult checks semantic constraints on a JobResult.
// It returns nil if the message is valid.
func ValidateJobResult(res *agentv1.JobResult) error {
	if res == nil {
		return &ValidationError{Field: "JobResult", Message: "must not be nil"}
	}
	if res.GetJobId() == "" {
		return &ValidationError{Field: "job_id", Message: "must not be empty"}
	}
	if res.GetStatus() == agentv1.JobStatus_JOB_STATUS_UNSPECIFIED {
		return &ValidationError{Field: "status", Message: "must not be UNSPECIFIED"}
	}
	if res.GetWorkerId() == "" {
		return &ValidationError{Field: "worker_id", Message: "must not be empty"}
	}
	if res.GetExecutionMs() < 0 {
		return &ValidationError{Field: "execution_ms", Message: "must not be negative"}
	}
	return nil
}

// ValidateBusPacket checks semantic constraints on a BusPacket.
// If the packet carries a JobRequest or JobResult payload, it is also validated.
// It returns nil if the message is valid.
func ValidateBusPacket(pkt *agentv1.BusPacket) error {
	if pkt == nil {
		return &ValidationError{Field: "BusPacket", Message: "must not be nil"}
	}
	if pkt.GetTraceId() == "" {
		return &ValidationError{Field: "trace_id", Message: "must not be empty"}
	}
	if pkt.GetSenderId() == "" {
		return &ValidationError{Field: "sender_id", Message: "must not be empty"}
	}
	if pkt.GetProtocolVersion() <= 0 {
		return &ValidationError{Field: "protocol_version", Message: "must be > 0"}
	}
	if pkt.GetCreatedAt() == nil {
		return &ValidationError{Field: "created_at", Message: "must not be nil"}
	}
	if pkt.GetPayload() == nil {
		return &ValidationError{Field: "payload", Message: "must not be nil"}
	}
	// Delegate to payload-specific validation.
	switch p := pkt.GetPayload().(type) {
	case *agentv1.BusPacket_JobRequest:
		if err := ValidateJobRequest(p.JobRequest); err != nil {
			return err
		}
	case *agentv1.BusPacket_JobResult:
		if err := ValidateJobResult(p.JobResult); err != nil {
			return err
		}
	}
	return nil
}
