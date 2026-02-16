package captesting

import (
	"context"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

func echoHandler(_ context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
	return &agentv1.JobResult{
		JobId:     req.JobId,
		Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
		ResultPtr: "redis://res/" + req.JobId,
		WorkerId:  "test-worker",
	}, nil
}

func TestSubmitAndWait(t *testing.T) {
	req := &agentv1.JobRequest{
		JobId: "test-1",
		Topic: "job.echo",
	}
	result, err := SubmitAndWait(echoHandler, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a result, got nil")
	}
	if result.Status != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v", result.Status)
	}
	if result.JobId != "test-1" {
		t.Fatalf("expected job_id test-1, got %s", result.JobId)
	}
}

func TestInMemoryBus(t *testing.T) {
	bus := NewInMemoryBus()

	if _, ok := bus.LastPublished(); ok {
		t.Fatal("expected no published messages")
	}

	if err := bus.Publish("test.subject", []byte("hello")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	msg, ok := bus.LastPublished()
	if !ok {
		t.Fatal("expected a published message")
	}
	if msg.Subject != "test.subject" {
		t.Fatalf("expected test.subject, got %s", msg.Subject)
	}
	if string(msg.Data) != "hello" {
		t.Fatalf("expected hello, got %s", msg.Data)
	}
}
