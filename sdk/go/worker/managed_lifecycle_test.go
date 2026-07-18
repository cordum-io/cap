package worker

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestManagedWorkerRenewRechecksSessionAfterWait(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-renew-recheck")
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-renew-recheck", Subjects: []string{"job.renew.recheck"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker.trust.setSession(&capsdk.WorkerHandshakeSession{
		Token: "session-cleared", ExpiresAt: time.Now().Add(200 * time.Millisecond),
	})
	worker.startTrustRenewal(ctx)
	time.Sleep(20 * time.Millisecond)
	worker.trust.clearSession()
	time.Sleep(150 * time.Millisecond)
	if scheduler.challengeCount() != 0 {
		t.Fatalf("renew used a session cleared during wait: %d request(s)", scheduler.challengeCount())
	}
	if token := worker.sessionToken(); token != "" {
		t.Fatalf("cleared session was resurrected as %q", token)
	}
	cancel()
	if err := worker.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestManagedWorkerCleansPartialSubscriptionFailure(t *testing.T) {
	_, natsURL := startTestNATS(t)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-partial", Subjects: []string{"job.partial", "invalid subject"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	var calls atomic.Int32
	err = worker.Run(context.Background(), func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error) {
		calls.Add(1)
		return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "invalid subject") {
		t.Fatalf("Run() error=%v, want invalid second subscription", err)
	}
	publisher := testNATSConn(t, natsURL)
	publishManagedJob(t, publisher, "job.partial", validLowLevelJobPacket())
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("partial subscription failure left %d admitted handler(s)", calls.Load())
	}
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}

func TestManagedWorkerRejectsConcurrentAndRepeatedRun(t *testing.T) {
	_, natsURL := startTestNATS(t)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-run-once", Subjects: []string{"job.run.once"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeOff,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runManagedWorker(t, worker, ctx, successfulManagedHandler)
	awaitManagedSubscriptions(t, worker, 2)
	assertManagedRunRejected(t, worker)
	cancel()
	<-done
	assertManagedRunRejected(t, worker)
	if err := worker.Close(); err != nil {
		t.Fatal(err)
	}
}

func awaitManagedSubscriptions(t *testing.T, worker *ManagedWorker, minimum int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		worker.subsMu.Lock()
		count := len(worker.subs)
		worker.subsMu.Unlock()
		if count >= minimum {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("worker did not install %d subscriptions", minimum)
}

func assertManagedRunRejected(t *testing.T, worker *ManagedWorker) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := worker.Run(ctx, successfulManagedHandler)
	if err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("repeated Run() error=%v, want already-run rejection", err)
	}
}
