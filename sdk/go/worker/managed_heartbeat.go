package worker

import (
	"context"
	"sync/atomic"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (w *ManagedWorker) startHeartbeat(ctx context.Context) {
	interval := w.cfg.HeartbeatEvery
	if interval <= 0 {
		interval = capsdk.DefaultHeartbeatInterval
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	w.cancelMu.Lock()
	w.cancel = cancel
	w.heartbeatDone = done
	w.cancelMu.Unlock()
	w.publishManagedHeartbeat(heartbeatCtx)
	go func() {
		defer close(done)
		w.runManagedHeartbeatLoop(heartbeatCtx, interval)
	}()
}

func (w *ManagedWorker) stopHeartbeat() {
	w.cancelMu.Lock()
	cancel, done := w.cancel, w.heartbeatDone
	w.cancel = nil
	w.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return
	}
	<-done
	w.cancelMu.Lock()
	if w.heartbeatDone == done {
		w.heartbeatDone = nil
	}
	w.cancelMu.Unlock()
}

func (w *ManagedWorker) runManagedHeartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.publishManagedHeartbeat(ctx)
		}
	}
}

func (w *ManagedWorker) publishManagedHeartbeat(ctx context.Context) {
	payload, err := w.managedHeartbeatPayload()
	if err == nil {
		err = w.conn.Publish(capsdk.SubjectHeartbeat, payload)
	}
	if err == nil {
		w.recordManagedHeartbeatSuccess()
		return
	}
	failures := atomic.AddInt32(&w.consecutiveHBFailures, 1)
	w.logger.Printf("worker: heartbeat publish failed (consecutive=%d): %v", failures, err)
	if !waitManagedTrust(ctx, 500*time.Millisecond) {
		return
	}
	payload, buildErr := w.managedHeartbeatPayload()
	if buildErr == nil && w.conn.Publish(capsdk.SubjectHeartbeat, payload) == nil {
		w.recordManagedHeartbeatSuccess()
		return
	}
	if atomic.AddInt32(&w.consecutiveHBFailures, 1) >= 3 {
		w.logger.Printf("worker: ERROR heartbeat publish failing consistently")
	}
}

func (w *ManagedWorker) managedHeartbeatPayload() ([]byte, error) {
	packet := &agentv1.BusPacket{
		TraceId: w.managedTraceID("heartbeat"), SenderId: w.workerID,
		ProtocolVersion: capsdk.DefaultProtocolVersion, CreatedAt: timestamppb.Now(),
		AuthToken: w.sessionToken(),
		Payload: &agentv1.BusPacket_Heartbeat{Heartbeat: &agentv1.Heartbeat{
			WorkerId: w.workerID, Pool: w.pool, Type: w.cfg.Type,
			ActiveJobs: atomic.LoadInt32(&w.active), MaxParallelJobs: w.cfg.MaxParallelJobs,
			Capabilities: w.cfg.Capabilities, Labels: w.cfg.Labels,
			AgentName: capsdk.SanitizeAgentName(w.cfg.AgentName),
		}},
	}
	return marshalValidatedEnvelope(packet, w.cfg.PrivateKey)
}

func (w *ManagedWorker) recordManagedHeartbeatSuccess() {
	atomic.StoreInt32(&w.consecutiveHBFailures, 0)
	if w.metrics != nil {
		w.metrics.OnHeartbeatSent(w.workerID)
	}
}
