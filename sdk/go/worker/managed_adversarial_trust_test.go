package worker

import (
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestManagedWorkerZeroTrustModePreservesLegacyOff(t *testing.T) {
	_, natsURL := startTestNATS(t)
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: "worker-zero", Subjects: []string{"job.zero"}, NatsURL: natsURL,
	})
	if err != nil {
		t.Fatalf("zero trust mode must retain legacy off behavior: %v", err)
	}
	if worker.trust.mode != capsdk.WorkerTrustModeOff {
		t.Fatalf("normalized trust mode=%s, want off", worker.trust.mode)
	}
	if err := worker.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
}

func TestManagedWorkerTrustModesDoNotPublishTokenlessResults(t *testing.T) {
	for _, mode := range []capsdk.WorkerTrustMode{capsdk.WorkerTrustModeWarn, capsdk.WorkerTrustModeEnforce} {
		t.Run(mode.String(), func(t *testing.T) {
			worker := newManagedWorkerWithoutSession(t, mode)
			result := &agentv1.JobResult{JobId: "job-secure", WorkerId: worker.workerID, Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}
			if err := worker.publishManagedResult("trace-secure", result); err == nil {
				t.Fatalf("%s mode published a result without a live session", mode)
			}
		})
	}
}

func TestManagedWorkerEnforceDoesNotBuildTokenlessHeartbeat(t *testing.T) {
	worker := newManagedWorkerWithoutSession(t, capsdk.WorkerTrustModeEnforce)
	if _, err := worker.managedHeartbeatPayload(); err == nil {
		t.Fatal("enforce mode built a heartbeat without a live session")
	}
}

func TestManagedWorkerWarnAllowsTokenlessTelemetry(t *testing.T) {
	worker := newManagedWorkerWithoutSession(t, capsdk.WorkerTrustModeWarn)
	if _, err := worker.managedHeartbeatPayload(); err != nil {
		t.Fatalf("warn mode must retain tokenless heartbeat telemetry: %v", err)
	}
}

func TestManagedWorkerCloseClearsSessionAfterDrain(t *testing.T) {
	worker := newManagedWorkerWithoutSession(t, capsdk.WorkerTrustModeEnforce)
	worker.trust.setSession(&capsdk.WorkerHandshakeSession{Token: "session-live", ExpiresAt: time.Now().Add(time.Hour)})
	if err := worker.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if token := worker.sessionToken(); token != "" {
		t.Fatalf("session after Close()=%q, want empty", token)
	}
}

func newManagedWorkerWithoutSession(t *testing.T, mode capsdk.WorkerTrustMode) *ManagedWorker {
	t.Helper()
	_, natsURL := startTestNATS(t)
	trust := managedTrustConfig(t, "worker-adversarial")
	worker, err := NewManagedWorker(ManagedConfig{
		WorkerID: trust.WorkerID, Subjects: []string{"job.adversarial"}, NatsURL: natsURL,
		WorkerTrustMode: mode, WorkerTrust: trust,
	})
	if err != nil {
		t.Fatalf("NewManagedWorker(): %v", err)
	}
	t.Cleanup(func() { _ = worker.Close() })
	return worker
}
