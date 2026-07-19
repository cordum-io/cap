package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewManagedWorker_ValidConfig(t *testing.T) {
	_, natsURL := startTestNATS(t)

	cfg := ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "echo",
		Pool:            "test-pool",
		NatsURL:         natsURL,
		MaxParallelJobs: 5,
		Capabilities:    []string{"echo", "ping"},
		Labels:          map[string]string{"env": "test"},
		WorkerID:        "test-worker-1",
	}

	w, err := NewManagedWorker(cfg)
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	if w.workerID != "test-worker-1" {
		t.Errorf("expected workerID 'test-worker-1', got %q", w.workerID)
	}
	if w.pool != "test-pool" {
		t.Errorf("expected pool 'test-pool', got %q", w.pool)
	}
	if cap(w.sem) != 5 {
		t.Errorf("expected semaphore capacity 5, got %d", cap(w.sem))
	}
	if w.conn == nil {
		t.Error("expected NATS connection to be established")
	}
}

func TestNewManagedWorker_DefaultsApplied(t *testing.T) {
	_, natsURL := startTestNATS(t)
	t.Setenv("WORKER_ID", "")

	cfg := ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "processor",
		NatsURL:         natsURL,
	}

	w, err := NewManagedWorker(cfg)
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	if cap(w.sem) != 1 {
		t.Errorf("expected default semaphore capacity 1, got %d", cap(w.sem))
	}
	if w.pool != "processor" {
		t.Errorf("expected pool to default to type 'processor', got %q", w.pool)
	}

	expectedSubject := "job.processor.*"
	found := false
	for _, s := range w.subjects {
		if s == expectedSubject {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected subject %q in %v", expectedSubject, w.subjects)
	}
}

func TestNewManagedWorker_SubjectsProvided(t *testing.T) {
	_, natsURL := startTestNATS(t)

	cfg := ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Subjects:        []string{"custom.subject.1", "custom.subject.2"},
		NatsURL:         natsURL,
		WorkerID:        "subject-worker",
	}

	w, err := NewManagedWorker(cfg)
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	if len(w.subjects) != 2 {
		t.Errorf("expected 2 subjects, got %d", len(w.subjects))
	}
}

func TestNewManagedWorker_SubjectDeduplication(t *testing.T) {
	_, natsURL := startTestNATS(t)

	cfg := ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Subjects:        []string{"job.echo.*", "job.echo.*", "  job.echo.*  "},
		NatsURL:         natsURL,
		WorkerID:        "dedup-worker",
	}

	w, err := NewManagedWorker(cfg)
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	if len(w.subjects) != 1 {
		t.Errorf("expected 1 deduplicated subject, got %d: %v", len(w.subjects), w.subjects)
	}
}

func TestNewManagedWorker_InvalidSubjects(t *testing.T) {
	_, natsURL := startTestNATS(t)

	cfg := ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		NatsURL:         natsURL,
		WorkerID:        "no-subjects-worker",
	}

	_, err := NewManagedWorker(cfg)
	if err == nil {
		t.Fatal("expected error for missing subjects")
	}
}

func TestNewManagedWorker_ConnectionFailure(t *testing.T) {
	cfg := ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "echo",
		NatsURL:         "nats://invalid-host-that-does-not-exist:4222",
	}

	_, err := NewManagedWorker(cfg)
	if err == nil {
		t.Fatal("expected error for NATS connection failure")
	}
}

func TestNewManagedWorker_NatsURLFromEnv(t *testing.T) {
	_, natsURL := startTestNATS(t)
	t.Setenv("NATS_URL", natsURL)

	cfg := ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "echo",
		WorkerID:        "env-nats-worker",
	}

	w, err := NewManagedWorker(cfg)
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()
}

func TestManagedWorker_Run_HandlerRequired(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{WorkerTrustMode: capsdk.WorkerTrustModeOff, AllowUnsigned: true, Type: "echo", NatsURL: natsURL})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = w.Run(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestManagedWorker_Run_ProcessesJobs(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "echo",
		NatsURL:         natsURL,
		WorkerID:        "process-jobs-worker",
		HeartbeatEvery:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	var (
		received    atomic.Bool
		receivedJob *agentv1.JobRequest
		mu          sync.Mutex
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			mu.Lock()
			receivedJob = req
			mu.Unlock()
			received.Store(true)
			return &agentv1.JobResult{
				JobId:     req.GetJobId(),
				Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
				ResultPtr: "result:test-job-123",
			}, nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	nc := testNATSConn(t, natsURL)

	jobReq := &agentv1.JobRequest{
		JobId:      "test-job-123",
		Topic:      "job.echo.submit",
		ContextPtr: "ctx:test-job-123",
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-123",
		SenderId:        "test-client",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload: &agentv1.BusPacket_JobRequest{
			JobRequest: jobReq,
		},
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}

	if err := nc.Publish("job.echo.submit", data); err != nil {
		t.Fatalf("publish job: %v", err)
	}
	nc.Flush()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if received.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !received.Load() {
		t.Fatal("job was not received by handler")
	}

	mu.Lock()
	if receivedJob.GetJobId() != "test-job-123" {
		t.Errorf("expected job id 'test-job-123', got %q", receivedJob.GetJobId())
	}
	mu.Unlock()
}

func TestManagedWorker_Run_ConcurrencyLimit(t *testing.T) {
	_, natsURL := startTestNATS(t)

	maxParallel := int32(2)
	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "slow",
		NatsURL:         natsURL,
		WorkerID:        "concurrency-worker",
		MaxParallelJobs: maxParallel,
		HeartbeatEvery:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	var (
		concurrent atomic.Int32
		maxSeen    atomic.Int32
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			cur := concurrent.Add(1)
			if cur > maxSeen.Load() {
				maxSeen.Store(cur)
			}
			time.Sleep(100 * time.Millisecond)
			concurrent.Add(-1)
			return &agentv1.JobResult{
				JobId:  req.GetJobId(),
				Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
			}, nil
		})
	}()

	time.Sleep(50 * time.Millisecond)

	nc := testNATSConn(t, natsURL)

	for i := 0; i < 5; i++ {
		jobReq := &agentv1.JobRequest{
			JobId: fmt.Sprintf("concurrent-job-%d", i),
			Topic: "job.slow.submit",
		}
		packet := &agentv1.BusPacket{
			TraceId:         jobReq.JobId,
			SenderId:        "test-client",
			ProtocolVersion: capsdk.DefaultProtocolVersion,
			CreatedAt:       timestamppb.Now(),
			Payload:         &agentv1.BusPacket_JobRequest{JobRequest: jobReq},
		}
		data, _ := capsdk.MarshalDeterministic(packet)
		nc.Publish("job.slow.submit", data)
	}
	nc.Flush()

	time.Sleep(500 * time.Millisecond)

	if maxSeen.Load() > maxParallel {
		t.Errorf("max concurrent %d exceeded MaxParallelJobs %d", maxSeen.Load(), maxParallel)
	}
}

func TestManagedWorker_Run_DirectSubject(t *testing.T) {
	_, natsURL := startTestNATS(t)

	workerID := "direct-subject-worker"
	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "echo",
		NatsURL:         natsURL,
		WorkerID:        workerID,
		HeartbeatEvery:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	var received atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			received.Store(true)
			return &agentv1.JobResult{
				JobId:  req.GetJobId(),
				Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
			}, nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	nc := testNATSConn(t, natsURL)

	directSubject := DirectSubject(workerID)
	jobReq := &agentv1.JobRequest{
		JobId: "direct-job",
		Topic: "job.echo.submit",
	}
	packet := &agentv1.BusPacket{
		TraceId:         "direct-trace",
		SenderId:        "test-client",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: jobReq},
	}
	data, _ := capsdk.MarshalDeterministic(packet)

	if err := nc.Publish(directSubject, data); err != nil {
		t.Fatalf("publish to direct subject: %v", err)
	}
	nc.Flush()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if received.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !received.Load() {
		t.Fatal("job not received on direct subject")
	}
}

func TestManagedWorker_Run_PublishesResult(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "result-test",
		NatsURL:         natsURL,
		WorkerID:        "result-worker",
		HeartbeatEvery:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{
				JobId:     req.GetJobId(),
				Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
				ResultPtr: "result:success",
			}, nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	nc := testNATSConn(t, natsURL)

	var resultReceived atomic.Bool
	var receivedResult *agentv1.JobResult
	var mu sync.Mutex

	sub, err := nc.Subscribe(capsdk.SubjectResult, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &pkt); err != nil {
			return
		}
		if res := pkt.GetJobResult(); res != nil {
			mu.Lock()
			receivedResult = res
			mu.Unlock()
			resultReceived.Store(true)
		}
	})
	if err != nil {
		t.Fatalf("subscribe to results: %v", err)
	}
	defer sub.Unsubscribe()

	jobReq := &agentv1.JobRequest{
		JobId: "result-job-1",
		Topic: "job.result-test.submit",
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-result",
		SenderId:        "test-client",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: jobReq},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	nc.Publish("job.result-test.submit", data)
	nc.Flush()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if resultReceived.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !resultReceived.Load() {
		t.Fatal("result not published")
	}

	mu.Lock()
	defer mu.Unlock()
	if receivedResult.GetJobId() != "result-job-1" {
		t.Errorf("expected job id 'result-job-1', got %q", receivedResult.GetJobId())
	}
	if receivedResult.GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Errorf("expected succeeded status, got %v", receivedResult.GetStatus())
	}
}

func TestManagedWorker_Run_HandlerError(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "error-test",
		NatsURL:         natsURL,
		WorkerID:        "error-worker",
		HeartbeatEvery:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{
				JobId:  req.GetJobId(),
				Status: agentv1.JobStatus_JOB_STATUS_FAILED,
			}, nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	nc := testNATSConn(t, natsURL)

	var resultReceived atomic.Bool
	var receivedResult *agentv1.JobResult
	var mu sync.Mutex

	sub, err := nc.Subscribe(capsdk.SubjectResult, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &pkt); err != nil {
			return
		}
		if res := pkt.GetJobResult(); res != nil {
			mu.Lock()
			receivedResult = res
			mu.Unlock()
			resultReceived.Store(true)
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	jobReq := &agentv1.JobRequest{JobId: "error-job", Topic: "job.error-test.submit"}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-error",
		SenderId:        "test",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: jobReq},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	nc.Publish("job.error-test.submit", data)
	nc.Flush()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if resultReceived.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !resultReceived.Load() {
		t.Fatal("result not published")
	}

	mu.Lock()
	defer mu.Unlock()
	if receivedResult.GetStatus() != agentv1.JobStatus_JOB_STATUS_FAILED {
		t.Errorf("expected failed status, got %v", receivedResult.GetStatus())
	}
}

func TestManagedWorker_Run_HandlerPanic(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "panic-test",
		NatsURL:         natsURL,
		WorkerID:        "panic-worker",
		HeartbeatEvery:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			panic("boom")
		})
	}()

	time.Sleep(100 * time.Millisecond)

	nc := testNATSConn(t, natsURL)

	results := make(map[string]*agentv1.JobResult)
	var mu sync.Mutex

	sub, err := nc.Subscribe(capsdk.SubjectResult, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &pkt); err != nil {
			return
		}
		if res := pkt.GetJobResult(); res != nil {
			mu.Lock()
			results[res.GetJobId()] = res
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	publishJob := func(id string) {
		jobReq := &agentv1.JobRequest{JobId: id, Topic: "job.panic-test.submit"}
		packet := &agentv1.BusPacket{
			TraceId:         id,
			SenderId:        "test",
			ProtocolVersion: capsdk.DefaultProtocolVersion,
			CreatedAt:       timestamppb.Now(),
			Payload:         &agentv1.BusPacket_JobRequest{JobRequest: jobReq},
		}
		data, _ := capsdk.MarshalDeterministic(packet)
		nc.Publish("job.panic-test.submit", data)
	}

	publishJob("panic-job-1")
	publishJob("panic-job-2")
	nc.Flush()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(results)
		mu.Unlock()
		if count >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(results) != 2 {
		t.Fatalf("expected 2 results after panic, got %d", len(results))
	}
	for id, res := range results {
		if res.GetStatus() != agentv1.JobStatus_JOB_STATUS_FAILED {
			t.Fatalf("expected failed status for %s, got %v", id, res.GetStatus())
		}
		if !strings.Contains(res.GetErrorMessage(), "handler panic") {
			t.Fatalf("expected panic error message for %s, got %q", id, res.GetErrorMessage())
		}
	}
}

func TestManagedWorker_Close_GracefulShutdown(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "shutdown-test",
		NatsURL:         natsURL,
		WorkerID:        "shutdown-worker",
		HeartbeatEvery:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		})
		close(runDone)
	}()

	time.Sleep(100 * time.Millisecond)

	if err := w.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	cancel()

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Error("Run did not exit after Close and cancel")
	}
}

func TestManagedWorker_Close_DoubleClose(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "double-close",
		NatsURL:         natsURL,
		WorkerID:        "double-close-worker",
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Errorf("first Close failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Logf("second Close returned error (may be expected): %v", err)
	}
}

func TestManagedWorker_HeartbeatEmission(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "heartbeat-test",
		NatsURL:         natsURL,
		WorkerID:        "heartbeat-worker",
		Pool:            "test-pool",
		MaxParallelJobs: 3,
		HeartbeatEvery:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	nc := testNATSConn(t, natsURL)

	var heartbeatCount atomic.Int32
	var lastHeartbeat *agentv1.Heartbeat
	var mu sync.Mutex

	sub, err := nc.Subscribe(capsdk.SubjectHeartbeat, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &pkt); err != nil {
			return
		}
		if hb := pkt.GetHeartbeat(); hb != nil {
			mu.Lock()
			lastHeartbeat = hb
			mu.Unlock()
			heartbeatCount.Add(1)
		}
	})
	if err != nil {
		t.Fatalf("subscribe to heartbeat: %v", err)
	}
	defer sub.Unsubscribe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		})
	}()

	time.Sleep(200 * time.Millisecond)

	count := heartbeatCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 heartbeats, got %d", count)
	}

	mu.Lock()
	defer mu.Unlock()
	if lastHeartbeat == nil {
		t.Fatal("no heartbeat received")
	}
	if lastHeartbeat.GetWorkerId() != "heartbeat-worker" {
		t.Errorf("expected worker id 'heartbeat-worker', got %q", lastHeartbeat.GetWorkerId())
	}
	if lastHeartbeat.GetPool() != "test-pool" {
		t.Errorf("expected pool 'test-pool', got %q", lastHeartbeat.GetPool())
	}
	if lastHeartbeat.GetMaxParallelJobs() != 3 {
		t.Errorf("expected max parallel 3, got %d", lastHeartbeat.GetMaxParallelJobs())
	}
}

func TestManagedWorker_AgentNameInHeartbeatAndHandshake(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "agentname-test",
		NatsURL:         natsURL,
		WorkerID:        "agentname-worker",
		Pool:            "test-pool",
		MaxParallelJobs: 1,
		HeartbeatEvery:  50 * time.Millisecond,
		// Deliberately padded/dirty to prove the SDK sanitizes display labels.
		AgentName: "  Claude Code — Billing  ",
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	nc := testNATSConn(t, natsURL)

	var (
		mu           sync.Mutex
		hbName       string
		hsName       string
		gotHB, gotHS bool
	)
	hbSub, err := nc.Subscribe(capsdk.SubjectHeartbeat, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if proto.Unmarshal(msg.Data, &pkt) == nil {
			if hb := pkt.GetHeartbeat(); hb != nil {
				mu.Lock()
				hbName, gotHB = hb.GetAgentName(), true
				mu.Unlock()
			}
		}
	})
	if err != nil {
		t.Fatalf("subscribe heartbeat: %v", err)
	}
	defer hbSub.Unsubscribe()

	hsSub, err := nc.Subscribe(capsdk.SubjectHandshake, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if proto.Unmarshal(msg.Data, &pkt) == nil {
			if hs := pkt.GetHandshake(); hs != nil {
				mu.Lock()
				hsName, gotHS = hs.GetAgentName(), true
				mu.Unlock()
			}
		}
	})
	if err != nil {
		t.Fatalf("subscribe handshake: %v", err)
	}
	defer hsSub.Unsubscribe()

	// Ensure both subscriptions are registered server-side before the worker
	// starts and publishes its one-shot startup handshake — NATS core has no
	// replay, so a not-yet-flushed subscription would silently miss it.
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush subscriptions: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		})
	}()

	// Poll until the one-shot handshake and at least one heartbeat have arrived,
	// rather than racing a fixed sleep — the previous 200ms gamble flaked under
	// CI load ("no handshake received").
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := gotHB && gotHS
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if !gotHB {
		t.Fatal("no heartbeat received")
	}
	if !gotHS {
		t.Fatal("no handshake received")
	}
	if hbName != "Claude Code — Billing" {
		t.Errorf("heartbeat AgentName = %q, want sanitized %q", hbName, "Claude Code — Billing")
	}
	if hsName != "Claude Code — Billing" {
		t.Errorf("handshake AgentName = %q, want sanitized %q", hsName, "Claude Code — Billing")
	}
}

func TestManagedWorker_PublishesHandshakeReadyTopics(t *testing.T) {
	_, natsURL := startTestNATS(t)

	configuredTopics := []string{"job.echo.submit", "job.audit.submit"}
	readyTopics := managedAdmittedSubjects("handshake-ready-worker", configuredTopics)
	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Subjects:        configuredTopics,
		NatsURL:         natsURL,
		WorkerID:        "handshake-ready-worker",
		HeartbeatEvery:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	nc := testNATSConn(t, natsURL)

	var (
		received  atomic.Bool
		handshake *agentv1.Handshake
		mu        sync.Mutex
	)

	sub, err := nc.Subscribe(capsdk.SubjectHandshake, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &pkt); err != nil {
			return
		}
		if hs := pkt.GetHandshake(); hs != nil {
			mu.Lock()
			handshake = hs
			mu.Unlock()
			received.Store(true)
		}
	})
	if err != nil {
		t.Fatalf("subscribe to handshake: %v", err)
	}
	defer sub.Unsubscribe()
	// The startup handshake is a one-shot Core NATS publish. Wait until the
	// server has processed this subscription so a slow client write loop cannot
	// make the assertion race the worker's publish.
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush handshake subscription: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if received.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !received.Load() {
		t.Fatal("handshake not published")
	}

	mu.Lock()
	defer mu.Unlock()
	if handshake == nil {
		t.Fatal("no handshake received")
	}
	if got := handshake.GetReadyTopics(); len(got) != len(readyTopics) {
		t.Fatalf("ready_topics len = %d, want %d (%v)", len(got), len(readyTopics), got)
	}
	for i, topic := range readyTopics {
		if handshake.GetReadyTopics()[i] != topic {
			t.Fatalf("ready_topics[%d] = %q, want %q", i, handshake.GetReadyTopics()[i], topic)
		}
	}
}

func TestManagedWorker_HeartbeatFailureTracking(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "hb-fail-test",
		NatsURL:         natsURL,
		WorkerID:        "hb-fail-worker",
		HeartbeatEvery:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	if atomic.LoadInt32(&w.consecutiveHBFailures) != 0 {
		t.Fatalf("expected 0 initial heartbeat failures, got %d", w.consecutiveHBFailures)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		})
	}()

	time.Sleep(200 * time.Millisecond)

	if failures := atomic.LoadInt32(&w.consecutiveHBFailures); failures != 0 {
		t.Errorf("expected 0 heartbeat failures after successful beats, got %d", failures)
	}
}

func TestManagedWorker_HeartbeatFailureCounter_ResetOnSuccess(t *testing.T) {
	_, natsURL := startTestNATS(t)

	w, err := NewManagedWorker(ManagedConfig{
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
		Type:            "hb-reset-test",
		NatsURL:         natsURL,
		WorkerID:        "hb-reset-worker",
		HeartbeatEvery:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker failed: %v", err)
	}
	defer w.Close()

	atomic.StoreInt32(&w.consecutiveHBFailures, 5)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		})
	}()

	time.Sleep(200 * time.Millisecond)

	if failures := atomic.LoadInt32(&w.consecutiveHBFailures); failures != 0 {
		t.Errorf("expected heartbeat failures to reset to 0 after successful publish, got %d", failures)
	}
}
