package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type recordingMetrics struct {
	mu        sync.Mutex
	received  []string
	completed []string
	failed    []string
}

func (r *recordingMetrics) OnJobReceived(jobID, topic string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.received = append(r.received, jobID+":"+topic)
}

func (r *recordingMetrics) OnJobCompleted(jobID string, durationMs int64, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completed = append(r.completed, jobID+":"+status)
}

func (r *recordingMetrics) OnJobFailed(jobID string, errorMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = append(r.failed, jobID+":"+errorMsg)
}

func (r *recordingMetrics) OnHeartbeatSent(workerID string) {}

func TestMetricsOnSuccess(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	metrics := &recordingMetrics{}
	jobID := "metrics-job-1"
	ctxKey := "ctx:" + jobID
	payload, _ := json.Marshal(map[string]string{"prompt": "hello"})
	_ = store.Set(context.Background(), ctxKey, payload)

	agent := &Agent{
		HandshakeMode: HandshakeModeOff,
		NATS:          mock,
		Store:         store,
		SenderID:      "worker-m1",
		Metrics:       metrics,
	}

	Register(agent, "job.metrics", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.metrics",
		ContextPtr: "redis://" + ctxKey,
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-m1",
		SenderId:        "client-m1",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.metrics", data)

	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if len(metrics.received) != 1 || metrics.received[0] != jobID+":job.metrics" {
		t.Fatalf("expected OnJobReceived(%s, job.metrics), got %v", jobID, metrics.received)
	}
	if len(metrics.completed) != 1 || metrics.completed[0] != jobID+":SUCCEEDED" {
		t.Fatalf("expected OnJobCompleted(%s, SUCCEEDED), got %v", jobID, metrics.completed)
	}
	if len(metrics.failed) != 0 {
		t.Fatalf("expected no failures, got %v", metrics.failed)
	}
}

func TestMetricsOnFailure(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	metrics := &recordingMetrics{}
	jobID := "metrics-job-2"
	// Empty context_ptr triggers failure
	agent := &Agent{
		HandshakeMode: HandshakeModeOff,
		NATS:          mock,
		Store:         store,
		SenderID:      "worker-m2",
		Metrics:       metrics,
	}

	Register(agent, "job.metrics.fail", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.metrics.fail",
		ContextPtr: "redis://missing-key",
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-m2",
		SenderId:        "client-m2",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.metrics.fail", data)

	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if len(metrics.received) != 1 {
		t.Fatalf("expected 1 OnJobReceived, got %d", len(metrics.received))
	}
	if len(metrics.failed) != 1 {
		t.Fatalf("expected 1 OnJobFailed, got %d", len(metrics.failed))
	}
	if len(metrics.completed) != 0 {
		t.Fatalf("expected 0 OnJobCompleted, got %d", len(metrics.completed))
	}
}
