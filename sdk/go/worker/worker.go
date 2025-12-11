package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coretexos/cap/sdk/go"
	agentv1 "github.com/coretexos/cap/cortex/agent/v1"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

// Handler processes a JobRequest and returns a JobResult.
type Handler func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error)

// Worker subscribes to a pool subject and handles jobs.
type Worker struct {
	NATS    *nats.Conn
	Subject string
	Handler Handler
}

// Start begins consuming and handling JobRequests. It blocks until the subscription is closed.
func (w *Worker) Start() error {
	if w.NATS == nil {
		return errors.New("worker: NATS connection is nil")
	}
	if w.Subject == "" {
		return errors.New("worker: subject is required")
	}
	if w.Handler == nil {
		return errors.New("worker: handler is required")
	}
	_, err := w.NATS.QueueSubscribe(w.Subject, w.Subject, func(msg *nats.Msg) {
		ctx := context.Background()
		var packet agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &packet); err != nil {
			// cannot decode; log and drop
			return
		}
		req := packet.GetJobRequest()
		if req == nil {
			return
		}
		res, err := w.Handler(ctx, req)
		if err != nil {
			res = &agentv1.JobResult{
				JobId:        req.GetJobId(),
				Status:       agentv1.JobStatus_JOB_STATUS_FAILED,
				ErrorMessage: err.Error(),
			}
		}
		// Fill required metadata if missing
		if res.JobId == "" {
			res.JobId = req.GetJobId()
		}
		if res.WorkerId == "" {
			res.WorkerId = w.NATS.Opts.Name
		}
		if res.ExecutionMs == 0 {
			res.ExecutionMs = 0
		}
		out := &agentv1.BusPacket{
			TraceId:         packet.GetTraceId(),
			SenderId:        w.NATS.Opts.Name,
			ProtocolVersion: capsdk.DefaultProtocolVersion,
			CreatedAt:       packet.CreatedAt,
			Payload: &agentv1.BusPacket_JobResult{
				JobResult: res,
			},
		}
		data, mErr := proto.Marshal(out)
		if mErr != nil {
			return
		}
		_ = w.NATS.Publish(capsdk.SubjectResult, data)
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	return nil
}

// HeartbeatPayload returns a protobuf-encoded heartbeat envelope.
func HeartbeatPayload(workerID, pool string, activeJobs, maxParallel int, cpuLoad float32) ([]byte, error) {
	hb := &agentv1.BusPacket{
		TraceId:         "",
		SenderId:        workerID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload: &agentv1.BusPacket_Heartbeat{
			Heartbeat: &agentv1.Heartbeat{
				WorkerId:        workerID,
				Pool:            pool,
				ActiveJobs:      int32(activeJobs),
				MaxParallelJobs: int32(maxParallel),
				CpuLoad:         cpuLoad,
			},
		},
	}
	return proto.Marshal(hb)
}

// EmitHeartbeat publishes a heartbeat once. Call repeatedly on a ticker.
func EmitHeartbeat(nc *nats.Conn, payload []byte) error {
	return nc.Publish(capsdk.SubjectHeartbeat, payload)
}

// HeartbeatLoop emits heartbeats until ctx is done.
func HeartbeatLoop(ctx context.Context, nc *nats.Conn, payloadFn func() ([]byte, error)) {
	ticker := time.NewTicker(capsdk.DefaultHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload, err := payloadFn()
			if err == nil {
				_ = EmitHeartbeat(nc, payload)
			}
		}
	}
}
