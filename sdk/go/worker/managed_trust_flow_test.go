package worker

import (
	"context"
	"crypto/ecdsa"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

func TestManagedWorkerAuthenticatesBeforeAdmissionAndMirrorsCapability(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-secure")
	observer := testNATSConn(t, natsURL)
	legacy := make(chan *agentv1.BusPacket, 1)
	heartbeats := make(chan *agentv1.BusPacket, 4)
	results := make(chan *agentv1.BusPacket, 1)
	subscribeManagedCapture(t, observer, capsdk.SubjectHandshake, legacy)
	subscribeManagedCapture(t, observer, capsdk.SubjectHeartbeat, heartbeats)
	subscribeManagedCapture(t, observer, capsdk.SubjectResult, results)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-secure", Subjects: []string{"job.secure"}, NatsURL: natsURL,
		Capabilities: []string{"progress", "cancel"}, AgentName: "Secure Worker",
		HeartbeatEvery: 20 * time.Millisecond, PrivateKey: managedP256Key(t),
		PublicKeys:      map[string]*ecdsa.PublicKey{scheduler.config.ExpectedSchedulerID: &scheduler.schedulerKey.PublicKey},
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runDone := runManagedWorker(t, worker, ctx, func(_ context.Context, request *agentv1.JobRequest) (*agentv1.JobResult, error) {
		return &agentv1.JobResult{JobId: request.JobId, Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	})
	t.Cleanup(func() { cancel(); <-runDone; _ = worker.Close() })

	authenticate := awaitManagedAuthenticate(t, scheduler, 0)
	embedded := authenticate.GetWorkerHandshakeAuthenticate().GetCapabilityHandshake()
	legacyPacket := awaitManagedPacket(t, legacy)
	if !proto.Equal(embedded, legacyPacket.GetHandshake()) {
		t.Fatalf("legacy handshake differs from authenticated capability\nembedded=%v\nlegacy=%v", embedded, legacyPacket.GetHandshake())
	}
	assertManagedSessionEnvelope(t, legacyPacket, "session-1")
	assertManagedSessionEnvelope(t, awaitManagedPacket(t, heartbeats), "session-1")

	job := validLowLevelJobPacket()
	job.SenderId = scheduler.config.ExpectedSchedulerID
	if err := capsdk.SignPacket(job, scheduler.schedulerKey); err != nil {
		t.Fatalf("sign job: %v", err)
	}
	data, err := capsdk.MarshalDeterministic(job)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	if err := observer.Publish("job.secure", data); err != nil {
		t.Fatalf("publish job: %v", err)
	}
	assertManagedSessionEnvelope(t, awaitManagedPacket(t, results), "session-1")
}

func TestManagedWorkerEnforceTrustFailureInstallsNoJobSubscription(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-enforce")
	scheduler.invalidChallenge = true
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-enforce", Subjects: []string{"job.enforce"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	defer worker.Close()
	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = worker.Run(ctx, func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error) {
		calls.Add(1)
		return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "worker trust") {
		t.Fatalf("Run() error=%v, want trust failure", err)
	}
	publisher := testNATSConn(t, natsURL)
	publishManagedJob(t, publisher, "job.enforce", validLowLevelJobPacket())
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("trust failure admitted %d job(s)", calls.Load())
	}
}

func TestManagedWorkerWarnTrustFailureContinuesWithoutToken(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-warn")
	scheduler.invalidChallenge = true
	observer := testNATSConn(t, natsURL)
	legacy := make(chan *agentv1.BusPacket, 1)
	subscribeManagedCapture(t, observer, capsdk.SubjectHandshake, legacy)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-warn", Subjects: []string{"job.warn"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeWarn, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runManagedWorker(t, worker, ctx, successfulManagedHandler)
	packet := awaitManagedPacket(t, legacy)
	if scheduler.challengeCount() == 0 {
		t.Fatal("warn mode continued without attempting authenticated trust")
	}
	if packet.GetAuthToken() != "" {
		t.Fatalf("warn failure installed unverified token %q", packet.GetAuthToken())
	}
	cancel()
	<-done
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}

func TestManagedWorkerRenewUsesCurrentTokenAndStopsOnCancel(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-renew")
	scheduler.tokenTTL = 300 * time.Millisecond
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-renew", Subjects: []string{"job.renew"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runManagedWorker(t, worker, ctx, successfulManagedHandler)
	renew := awaitManagedAuthenticate(t, scheduler, 1)
	challenge := renew.GetWorkerHandshakeAuthenticate().GetChallenge()
	if challenge.GetPurpose() != agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW {
		t.Fatalf("renew purpose=%v", challenge.GetPurpose())
	}
	if renew.GetAuthToken() != "session-1" {
		t.Fatalf("renew auth_token=%q, want current session", renew.GetAuthToken())
	}
	cancel()
	<-done
	before := scheduler.authenticateAt(2)
	time.Sleep(200 * time.Millisecond)
	if before == nil && scheduler.authenticateAt(2) != nil {
		t.Fatal("renew loop continued after Run cancellation")
	}
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}

func TestManagedWorkerEnforceRenewFailureClearsLiveSession(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-renew-fail")
	scheduler.tokenTTL = 400 * time.Millisecond
	scheduler.failAuthFrom = 2
	observer := testNATSConn(t, natsURL)
	heartbeats := make(chan *agentv1.BusPacket, 32)
	subscribeManagedCapture(t, observer, capsdk.SubjectHeartbeat, heartbeats)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-renew-fail", Subjects: []string{"job.renew.fail"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeEnforce, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 100 * time.Millisecond, HeartbeatEvery: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManagedWorker(t, worker, ctx, successfulManagedHandler)
	_ = awaitManagedAuthenticate(t, scheduler, 1)
	select {
	case runErr := <-done:
		if runErr == nil || !strings.Contains(runErr.Error(), "renewal failed") {
			t.Fatalf("Run() error=%v, want enforce renewal failure", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("enforce worker retained admission after renewal failure")
	}
	if token := worker.sessionToken(); token != "" {
		t.Fatalf("enforce renewal failure retained live token %q", token)
	}
	for len(heartbeats) > 0 {
		<-heartbeats
	}
	select {
	case packet := <-heartbeats:
		t.Fatalf("heartbeat continued after fatal renewal: %v", packet)
	case <-time.After(50 * time.Millisecond):
	}
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}

func TestManagedWorkerWarnRenewFailureRetainsOnlyUnexpiredSession(t *testing.T) {
	_, natsURL := startTestNATS(t)
	scheduler := newManagedTrustScheduler(t, natsURL, "worker-warn-renew")
	scheduler.tokenTTL = 300 * time.Millisecond
	scheduler.failAuthFrom = 2
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-warn-renew", Subjects: []string{"job.warn.renew"}, NatsURL: natsURL,
		WorkerTrustMode: capsdk.WorkerTrustModeWarn, WorkerTrust: scheduler.config,
		WorkerTrustTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runManagedWorker(t, worker, ctx, successfulManagedHandler)
	_ = awaitManagedAuthenticate(t, scheduler, 1)
	if token := worker.sessionToken(); token != "session-1" {
		t.Fatalf("warn renewal failure token=%q, want still-live session", token)
	}
	time.Sleep(200 * time.Millisecond)
	if scheduler.authenticateAt(2) != nil {
		t.Fatal("warn renewal failure retried without bounded backoff")
	}
	if token := worker.sessionToken(); token != "" {
		t.Fatalf("warn renewal failure retained expired token %q", token)
	}
	cancel()
	<-done
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}
}
