package worker

import (
	"context"
	"crypto/ecdsa"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

func TestManagedWorkerRejectsInvalidInboundBeforeHandler(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-boundary")
	observer := testNATSConn(t, natsURL)
	legacy := make(chan *agentv1.BusPacket, 1)
	subscribeManagedCapture(t, observer, capsdk.SubjectHandshake, legacy)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-boundary", Subjects: []string{"job.boundary"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := runManagedWorker(t, worker, ctx, func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error) {
		calls.Add(1)
		return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	})
	_ = awaitManagedPacket(t, legacy)

	for _, packet := range invalidManagedInboundPackets(t, scheduler) {
		publishManagedJob(t, observer, "job.boundary", packet)
	}
	time.Sleep(100 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("invalid packets reached handler %d time(s)", calls.Load())
	}
	cancel()
	<-done
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}

func TestManagedWorkerRejectsInvalidOutboundBeforePublish(t *testing.T) {
	_, natsURL := startTestNATS(t)
	observer := testNATSConn(t, natsURL)
	results := make(chan *agentv1.BusPacket, 1)
	legacy := make(chan *agentv1.BusPacket, 1)
	subscribeManagedCapture(t, observer, capsdk.SubjectResult, results)
	subscribeManagedCapture(t, observer, capsdk.SubjectHandshake, legacy)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-outbound", Subjects: []string{"job.outbound"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
		AllowUnsigned:   true,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runManagedWorker(t, worker, ctx, func(_ context.Context, request *agentv1.JobRequest) (*agentv1.JobResult, error) {
		return &agentv1.JobResult{JobId: request.JobId}, nil
	})
	_ = awaitManagedPacket(t, legacy)
	publishManagedJob(t, observer, "job.outbound", validLowLevelJobPacket())
	select {
	case packet := <-results:
		t.Fatalf("published validator-rejected result: %v", packet)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	<-done
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}

func TestManagedWorkerEnforceRejectsDispatchWithoutLiveSession(t *testing.T) {
	schedulerKey := managedP256Key(t)
	worker := &ManagedWorker{trust: &managedTrustState{
		mode: capsdk.WorkerTrustModeEnforce,
		config: &capsdk.WorkerTrustConfig{
			ExpectedSchedulerID: "scheduler-live",
			SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"scheduler-key": &schedulerKey.PublicKey},
		},
	}}
	packet := validLowLevelJobPacket()
	packet.SenderId = "scheduler-live"
	signManagedJobWithKey(t, packet, schedulerKey)
	if err := worker.validateInboundPacket(packet); err == nil {
		t.Fatal("enforce accepted a signed dispatch without a live session")
	}
	worker.trust.setSession(&capsdk.WorkerHandshakeSession{Token: "live", ExpiresAt: time.Now().Add(time.Hour)})
	if err := worker.validateInboundPacket(packet); err != nil {
		t.Fatalf("live trusted dispatch rejected: %v", err)
	}
}

func invalidManagedInboundPackets(t *testing.T, scheduler *managedTrustScheduler) []*agentv1.BusPacket {
	t.Helper()
	unsupported := validLowLevelJobPacket()
	unsupported.ProtocolVersion = capsdk.DefaultProtocolVersion + 1
	signManagedJob(t, unsupported, scheduler)
	missingTrace := validLowLevelJobPacket()
	missingTrace.TraceId = ""
	signManagedJob(t, missingTrace, scheduler)
	wrongSender := validLowLevelJobPacket()
	signManagedJob(t, wrongSender, scheduler)
	wrongSender.SenderId = "scheduler-attacker"
	tampered := validLowLevelJobPacket()
	signManagedJob(t, tampered, scheduler)
	tampered.GetJobRequest().JobId = "tampered"
	return []*agentv1.BusPacket{unsupported, missingTrace, wrongSender, tampered}
}

func signManagedJob(t *testing.T, packet *agentv1.BusPacket, scheduler *managedTrustScheduler) {
	t.Helper()
	packet.SenderId = scheduler.config.ExpectedSchedulerID
	if err := capsdk.SignPacket(packet, scheduler.schedulerKey); err != nil {
		t.Fatalf("sign managed job: %v", err)
	}
	if _, err := proto.Marshal(packet); err != nil {
		t.Fatalf("marshal managed job: %v", err)
	}
}

func signManagedJobWithKey(t *testing.T, packet *agentv1.BusPacket, key *ecdsa.PrivateKey) {
	t.Helper()
	if err := capsdk.SignPacket(packet, key); err != nil {
		t.Fatalf("sign managed job: %v", err)
	}
}
