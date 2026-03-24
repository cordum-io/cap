package capsdk

import (
	"errors"
	"fmt"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func validJobRequest() *agentv1.JobRequest {
	return &agentv1.JobRequest{
		JobId: "job-1",
		Topic: "job.tools",
	}
}

func validJobResult() *agentv1.JobResult {
	return &agentv1.JobResult{
		JobId:       "job-1",
		Status:      agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
		WorkerId:    "worker-1",
		ExecutionMs: 100,
	}
}

func validBusPacket(payload *agentv1.JobRequest) *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId:         "trace-1",
		SenderId:        "sender-1",
		ProtocolVersion: 1,
		CreatedAt:       timestamppb.Now(),
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: payload},
	}
}

func TestValidateJobRequest(t *testing.T) {
	t.Run("valid minimal", func(t *testing.T) {
		if err := ValidateJobRequest(validJobRequest()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid with budget", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{
			MaxInputTokens:  1000,
			MaxOutputTokens: 500,
			MaxTotalTokens:  1500,
			DeadlineMs:      30000,
		}
		if err := ValidateJobRequest(req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid with all fields", func(t *testing.T) {
		req := validJobRequest()
		req.Priority = agentv1.JobPriority_JOB_PRIORITY_CRITICAL
		req.ContextPtr = "redis://ctx:1"
		req.AdapterId = "adapter-1"
		req.ParentJobId = "parent-1"
		req.WorkflowId = "wf-1"
		req.StepIndex = 2
		req.MemoryId = "mem-1"
		req.TenantId = "tenant-1"
		req.PrincipalId = "user-1"
		if err := ValidateJobRequest(req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil", func(t *testing.T) {
		err := ValidateJobRequest(nil)
		assertValidationError(t, err, "JobRequest")
	})

	t.Run("empty job_id", func(t *testing.T) {
		req := validJobRequest()
		req.JobId = ""
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "job_id")
	})

	t.Run("empty topic", func(t *testing.T) {
		req := validJobRequest()
		req.Topic = ""
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "topic")
	})

	t.Run("negative step_index", func(t *testing.T) {
		req := validJobRequest()
		req.StepIndex = -1
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "step_index")
	})

	t.Run("negative budget.max_input_tokens", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{MaxInputTokens: -1}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "budget.max_input_tokens")
	})

	t.Run("negative budget.max_output_tokens", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{MaxOutputTokens: -1}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "budget.max_output_tokens")
	})

	t.Run("negative budget.max_total_tokens", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{MaxTotalTokens: -1}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "budget.max_total_tokens")
	})

	t.Run("negative budget.deadline_ms", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{DeadlineMs: -1}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "budget.deadline_ms")
	})

	t.Run("zero-value struct", func(t *testing.T) {
		err := ValidateJobRequest(&agentv1.JobRequest{})
		assertValidationError(t, err, "job_id")
	})

	t.Run("oversized risk_tags rejected", func(t *testing.T) {
		req := validJobRequest()
		tags := make([]string, maxRepeatedFieldSize+1)
		for i := range tags {
			tags[i] = "tag"
		}
		req.Meta = &agentv1.JobMetadata{RiskTags: tags}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "meta.risk_tags")
	})

	t.Run("max size risk_tags accepted", func(t *testing.T) {
		req := validJobRequest()
		tags := make([]string, maxRepeatedFieldSize)
		for i := range tags {
			tags[i] = "tag"
		}
		req.Meta = &agentv1.JobMetadata{RiskTags: tags}
		if err := ValidateJobRequest(req); err != nil {
			t.Fatalf("expected max-size risk_tags to pass, got: %v", err)
		}
	})

	t.Run("oversized labels rejected", func(t *testing.T) {
		req := validJobRequest()
		req.Labels = make(map[string]string, maxRepeatedFieldSize+1)
		for i := 0; i <= maxRepeatedFieldSize; i++ {
			req.Labels[fmt.Sprintf("key-%d", i)] = "val"
		}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "labels")
	})

	t.Run("budget overflow max_input_tokens rejected", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{MaxInputTokens: maxBudgetTokens + 1}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "budget.max_input_tokens")
	})

	t.Run("budget overflow max_output_tokens rejected", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{MaxOutputTokens: maxBudgetTokens + 1}
		err := ValidateJobRequest(req)
		assertValidationError(t, err, "budget.max_output_tokens")
	})

	t.Run("budget at max accepted", func(t *testing.T) {
		req := validJobRequest()
		req.Budget = &agentv1.Budget{MaxInputTokens: maxBudgetTokens}
		if err := ValidateJobRequest(req); err != nil {
			t.Fatalf("expected max budget to pass, got: %v", err)
		}
	})
}

func TestValidateJobResult(t *testing.T) {
	t.Run("valid minimal", func(t *testing.T) {
		if err := ValidateJobResult(validJobResult()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid with all fields", func(t *testing.T) {
		res := validJobResult()
		res.ResultPtr = "redis://res:1"
		res.ErrorCode = "E_TEMP"
		res.ErrorMessage = "temporary failure"
		res.ArtifactPtrs = []string{"redis://art:1", "redis://art:2"}
		if err := ValidateJobResult(res); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil", func(t *testing.T) {
		err := ValidateJobResult(nil)
		assertValidationError(t, err, "JobResult")
	})

	t.Run("empty job_id", func(t *testing.T) {
		res := validJobResult()
		res.JobId = ""
		err := ValidateJobResult(res)
		assertValidationError(t, err, "job_id")
	})

	t.Run("unspecified status", func(t *testing.T) {
		res := validJobResult()
		res.Status = agentv1.JobStatus_JOB_STATUS_UNSPECIFIED
		err := ValidateJobResult(res)
		assertValidationError(t, err, "status")
	})

	t.Run("empty worker_id", func(t *testing.T) {
		res := validJobResult()
		res.WorkerId = ""
		err := ValidateJobResult(res)
		assertValidationError(t, err, "worker_id")
	})

	t.Run("negative execution_ms", func(t *testing.T) {
		res := validJobResult()
		res.ExecutionMs = -1
		err := ValidateJobResult(res)
		assertValidationError(t, err, "execution_ms")
	})

	t.Run("zero-value struct", func(t *testing.T) {
		err := ValidateJobResult(&agentv1.JobResult{})
		assertValidationError(t, err, "job_id")
	})
}

func TestValidateBusPacket(t *testing.T) {
	t.Run("valid with job_request", func(t *testing.T) {
		pkt := validBusPacket(validJobRequest())
		if err := ValidateBusPacket(pkt); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid with job_result", func(t *testing.T) {
		pkt := &agentv1.BusPacket{
			TraceId:         "trace-1",
			SenderId:        "sender-1",
			ProtocolVersion: 1,
			CreatedAt:       timestamppb.Now(),
			Payload:         &agentv1.BusPacket_JobResult{JobResult: validJobResult()},
		}
		if err := ValidateBusPacket(pkt); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid with heartbeat", func(t *testing.T) {
		pkt := &agentv1.BusPacket{
			TraceId:         "trace-1",
			SenderId:        "sender-1",
			ProtocolVersion: 1,
			CreatedAt:       timestamppb.Now(),
			Payload:         &agentv1.BusPacket_Heartbeat{Heartbeat: &agentv1.Heartbeat{WorkerId: "w-1", Pool: "p-1"}},
		}
		if err := ValidateBusPacket(pkt); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil", func(t *testing.T) {
		err := ValidateBusPacket(nil)
		assertValidationError(t, err, "BusPacket")
	})

	t.Run("empty trace_id", func(t *testing.T) {
		pkt := validBusPacket(validJobRequest())
		pkt.TraceId = ""
		err := ValidateBusPacket(pkt)
		assertValidationError(t, err, "trace_id")
	})

	t.Run("empty sender_id", func(t *testing.T) {
		pkt := validBusPacket(validJobRequest())
		pkt.SenderId = ""
		err := ValidateBusPacket(pkt)
		assertValidationError(t, err, "sender_id")
	})

	t.Run("protocol_version zero", func(t *testing.T) {
		pkt := validBusPacket(validJobRequest())
		pkt.ProtocolVersion = 0
		err := ValidateBusPacket(pkt)
		assertValidationError(t, err, "protocol_version")
	})

	t.Run("nil created_at", func(t *testing.T) {
		pkt := validBusPacket(validJobRequest())
		pkt.CreatedAt = nil
		err := ValidateBusPacket(pkt)
		assertValidationError(t, err, "created_at")
	})

	t.Run("nil payload", func(t *testing.T) {
		pkt := validBusPacket(validJobRequest())
		pkt.Payload = nil
		err := ValidateBusPacket(pkt)
		assertValidationError(t, err, "payload")
	})

	t.Run("delegates to job_request validation", func(t *testing.T) {
		bad := &agentv1.JobRequest{JobId: "", Topic: "t"}
		pkt := validBusPacket(bad)
		err := ValidateBusPacket(pkt)
		assertValidationError(t, err, "job_id")
	})

	t.Run("delegates to job_result validation", func(t *testing.T) {
		bad := &agentv1.JobResult{JobId: "", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED, WorkerId: "w"}
		pkt := &agentv1.BusPacket{
			TraceId:         "trace-1",
			SenderId:        "sender-1",
			ProtocolVersion: 1,
			CreatedAt:       timestamppb.Now(),
			Payload:         &agentv1.BusPacket_JobResult{JobResult: bad},
		}
		err := ValidateBusPacket(pkt)
		assertValidationError(t, err, "job_id")
	})

	t.Run("zero-value struct", func(t *testing.T) {
		err := ValidateBusPacket(&agentv1.BusPacket{})
		assertValidationError(t, err, "trace_id")
	})
}

// assertValidationError checks that the error is a *ValidationError with the expected field.
func assertValidationError(t *testing.T, err error, expectedField string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if ve.Field != expectedField {
		t.Fatalf("expected field %q, got %q (message: %s)", expectedField, ve.Field, ve.Message)
	}
}
