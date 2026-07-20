package worker

import (
	"context"
	"io"
	"log"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestManagedProductionCompletedOutcomeSurvivesPublishFailure(t *testing.T) {
	_, natsURL := startTestNATS(t)
	observer := testNATSConn(t, natsURL)
	results := make(chan *nats.Msg, 1)
	sub, err := observer.ChanSubscribe(capsdk.SubjectResult, results)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	if err := observer.Flush(); err != nil {
		t.Fatal(err)
	}

	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	closed := testNATSConn(t, natsURL)
	closed.Close()
	worker.conn = closed
	worker.logger = log.New(io.Discard, "", 0)
	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "settle-msg-id001")
	var handlerCalls atomic.Int32
	handler := func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error) {
		handlerCalls.Add(1)
		return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	}
	worker.handleManagedMessage(context.Background(), managedReplayMessage(raw), handler)

	worker.conn = testNATSConn(t, natsURL)
	worker.handleManagedMessage(context.Background(), managedReplayMessage(raw), handler)
	if err := worker.conn.Flush(); err != nil {
		t.Fatal(err)
	}
	awaitProductionEvent(t, results)
	if got := handlerCalls.Load(); got != 1 {
		t.Fatalf("handler calls=%d, want durable outcome replay without re-execution", got)
	}
}

func TestManagedProductionPreparationFailureReleasesLease(t *testing.T) {
	_, natsURL := startTestNATS(t)
	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	worker.conn = testNATSConn(t, natsURL)
	worker.logger = log.New(io.Discard, "", 0)
	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "settle-msg-id002")
	var handlerCalls atomic.Int32
	handler := func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error) {
		handlerCalls.Add(1)
		return &agentv1.JobResult{}, nil
	}
	worker.handleManagedMessage(context.Background(), managedReplayMessage(raw), handler)
	worker.handleManagedMessage(context.Background(), managedReplayMessage(raw), handler)
	if got := handlerCalls.Load(); got != 2 {
		t.Fatalf("handler calls=%d, want redelivery after outcome preparation failure", got)
	}
}

func TestManagedProductionRenewsLeaseAcrossLongHandler(t *testing.T) {
	_, natsURL := startTestNATS(t)
	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	worker.cfg.Production.ReplayLease = 40 * time.Millisecond
	worker.conn = testNATSConn(t, natsURL)
	worker.logger = log.New(io.Discard, "", 0)
	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "settle-msg-id003")
	started, release := make(chan struct{}), make(chan struct{})
	var handlerCalls atomic.Int32
	handler := func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error) {
		handlerCalls.Add(1)
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.handleManagedMessage(context.Background(), managedReplayMessage(raw), handler)
	}()
	<-started
	time.Sleep(70 * time.Millisecond)
	worker.handleManagedMessage(context.Background(), managedReplayMessage(raw), handler)
	if got := handlerCalls.Load(); got != 1 {
		t.Fatalf("concurrent handler calls=%d, want renewed pending lease", got)
	}
	close(release)
	<-done
}

func TestManagedProductionCompletedJetStreamDeliveryIsAcked(t *testing.T) {
	ns, natsURL := startManagedJetStream(t)
	defer ns.Shutdown()
	nc := testNATSConn(t, natsURL)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{
		Name: "MANAGED_PRODUCTION_JOBS", Subjects: []string{"job.production.real"}, Storage: nats.MemoryStorage,
	}); err != nil {
		t.Fatal(err)
	}

	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	worker.conn, worker.logger = nc, log.New(io.Discard, "", 0)
	worker.admitted, worker.queue = []string{"job.production.real"}, "production-workers"
	worker.cfg.Production.Stream = "MANAGED_PRODUCTION_JOBS"
	worker.cfg.Production.ReplayLease = 100 * time.Millisecond
	worker.sem = make(chan struct{}, 1)
	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "settle-msg-id004")
	var handlers atomic.Int32
	err = worker.subscribeManagedSubjects(context.Background(), func(
		context.Context, *agentv1.JobRequest,
	) (*agentv1.JobResult, error) {
		handlers.Add(1)
		time.Sleep(350 * time.Millisecond)
		return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
	})
	if err != nil {
		t.Fatalf("subscribe managed production: %v", err)
	}
	t.Cleanup(worker.unsubscribeAll)
	if _, err := js.Publish("job.production.real", raw); err != nil {
		t.Fatal(err)
	}
	waitForManagedCount(t, &handlers, 1)
	waitForManagedConsumerAck(t, js, worker)
	info, err := js.ConsumerInfo(
		worker.cfg.Production.Stream, managedProductionDurableName(worker.queue, "job.production.real"),
	)
	if err != nil || info.NumRedelivered != 0 {
		t.Fatalf("JetStream renewal evidence info=%v error=%v", info, err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := handlers.Load(); got != 1 {
		t.Fatalf("JetStream handler calls=%d, want one ACKed delivery", got)
	}
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)/2] ^= 0x01
	if _, err := js.Publish("job.production.real", tampered); err != nil {
		t.Fatal(err)
	}
	waitForManagedConsumerAck(t, js, worker)
	if got := handlers.Load(); got != 1 {
		t.Fatalf("poison input reached handler: calls=%d", got)
	}
}

func TestManagedProductionWrongWorkerDeliveryIsRetried(t *testing.T) {
	ns, natsURL := startManagedJetStream(t)
	defer ns.Shutdown()
	nc := testNATSConn(t, natsURL)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{
		Name: "MANAGED_PRODUCTION_JOBS", Subjects: []string{"job.production.real"}, Storage: nats.MemoryStorage,
	}); err != nil {
		t.Fatal(err)
	}

	schedulerKey := managedP256Key(t)
	worker := managedProductionWorkerForTest(t, schedulerKey)
	worker.conn, worker.logger = nc, log.New(io.Discard, "", 0)
	worker.admitted, worker.queue = []string{"job.production.real"}, "production-workers"
	worker.cfg.Production.Stream = "MANAGED_PRODUCTION_JOBS"
	worker.sem = make(chan struct{}, 1)
	var handlers atomic.Int32
	if err := worker.subscribeManagedSubjects(context.Background(), func(
		context.Context, *agentv1.JobRequest,
	) (*agentv1.JobResult, error) {
		handlers.Add(1)
		return &agentv1.JobResult{}, nil
	}); err != nil {
		t.Fatalf("subscribe managed production: %v", err)
	}
	t.Cleanup(worker.unsubscribeAll)

	raw := managedProductionRequestWire(t, schedulerKey, "job.production.real", "wrong-worker-id1")
	raw = mutateManagedProductionRequestWire(t, raw, schedulerKey, func(packet *agentv1.BusPacket) {
		packet.GetJobRequest().Dispatch.AssignedWorkerId = "worker-other"
	})
	if _, err := js.Publish("job.production.real", raw); err != nil {
		t.Fatal(err)
	}
	waitForManagedRedelivery(t, js, worker)
	if got := handlers.Load(); got != 0 {
		t.Fatalf("wrong-worker delivery reached handler: calls=%d", got)
	}
}

func waitForManagedConsumerAck(t *testing.T, js nats.JetStreamContext, worker *ManagedWorker) {
	t.Helper()
	durable := managedProductionDurableName(worker.queue, "job.production.real")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		info, err := js.ConsumerInfo(worker.cfg.Production.Stream, durable)
		if err == nil && info.NumAckPending == 0 && info.NumPending == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("JetStream consumer %s did not settle", durable)
}

func waitForManagedRedelivery(t *testing.T, js nats.JetStreamContext, worker *ManagedWorker) {
	t.Helper()
	durable := managedProductionDurableName(worker.queue, "job.production.real")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		info, err := js.ConsumerInfo(worker.cfg.Production.Stream, durable)
		if err == nil && info.NumRedelivered > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("wrong-worker delivery was terminated instead of retried")
}

func managedReplayMessage(raw []byte) *nats.Msg {
	return &nats.Msg{Subject: "job.production.real", Data: append([]byte(nil), raw...)}
}

func startManagedJetStream(t *testing.T) (*server.Server, string) {
	t.Helper()
	ns, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true,
		JetStream: true, StoreDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("embedded JetStream did not start")
	}
	return ns, ns.ClientURL()
}

func waitForManagedCount(t *testing.T, count *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for count.Load() < want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := count.Load(); got < want {
		t.Fatalf("count=%d, want at least %d", got, want)
	}
}
