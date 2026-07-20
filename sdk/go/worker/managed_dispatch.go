package worker

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (w *ManagedWorker) dispatch(ctx context.Context, message *nats.Msg, handler Handler) {
	if ctx.Err() != nil {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if !w.acquireManagedSlot(ctx) {
			return
		}
		defer w.releaseManagedSlot()
		w.handleManagedMessage(ctx, message, handler)
	}()
}

func (w *ManagedWorker) handleManagedMessage(ctx context.Context, message *nats.Msg, handler Handler) {
	packet, request, err := w.decodeManagedRequest(message.Data, message.Subject)
	if err != nil {
		if errors.Is(err, errManagedProductionDuplicate) {
			return
		}
		w.logger.Printf("worker: packet rejected: %v", err)
		return
	}
	if w.metrics != nil {
		w.metrics.OnJobReceived(request.GetJobId(), request.GetTopic())
	}
	started := time.Now()
	handlerCtx := withManagedEventPublisher(ctx, w, packet.GetTraceId(), request)
	result, handlerErr, panicked := w.callManagedHandler(handlerCtx, handler, request)
	execution := time.Since(started).Milliseconds()
	result = managedHandlerResult(request, result, handlerErr, panicked)
	normalizeResult(result, request.GetJobId(), w.workerID, execution)
	w.recordManagedMetric(result, execution)
	if err := w.publishManagedResultForContext(handlerCtx, result); err != nil {
		w.logger.Printf("worker: publish result failed: %v", err)
	}
}

func (w *ManagedWorker) decodeManagedRequest(data []byte, audience string) (*agentv1.BusPacket, *agentv1.JobRequest, error) {
	packet, err := w.decodeManagedPacket(data, audience)
	if err != nil {
		return nil, nil, err
	}
	request := packet.GetJobRequest()
	if request == nil {
		return nil, nil, errors.New("packet is not a job request")
	}
	return packet, request, nil
}

func (w *ManagedWorker) decodeManagedPacket(data []byte, audience string) (*agentv1.BusPacket, error) {
	if w.cfg.Production.Enabled {
		return w.admitManagedProductionPacket(data, audience)
	}
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(data, packet); err != nil {
		return nil, fmt.Errorf("decode packet: %w", err)
	}
	if err := w.validateInboundPacket(packet); err != nil {
		return nil, err
	}
	return packet, nil
}

func (w *ManagedWorker) callManagedHandler(ctx context.Context, handler Handler, request *agentv1.JobRequest) (result *agentv1.JobResult, err error, panicked bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicked = true
			err = fmt.Errorf("handler panic: %v", recovered)
			w.logger.Printf("worker: handler panic: %v", recovered)
			w.logger.Printf("worker: handler panic stack: %s", debug.Stack())
		}
	}()
	result, err = handler(ctx, request)
	return result, err, false
}

func managedHandlerResult(request *agentv1.JobRequest, result *agentv1.JobResult, err error, panicked bool) *agentv1.JobResult {
	if result == nil {
		result = &agentv1.JobResult{JobId: request.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_FAILED}
		if err == nil {
			result.ErrorMessage = "handler returned nil"
		}
	}
	if err != nil {
		if result.Status == agentv1.JobStatus_JOB_STATUS_UNSPECIFIED {
			result.Status = agentv1.JobStatus_JOB_STATUS_FAILED
		}
		if panicked || strings.TrimSpace(result.ErrorMessage) == "" {
			result.ErrorMessage = err.Error()
		}
	}
	return result
}

func (w *ManagedWorker) publishManagedResult(traceID string, result *agentv1.JobResult) error {
	return w.publishManagedResultForRequest(traceID, nil, result)
}

func (w *ManagedWorker) publishManagedResultForRequest(
	traceID string, request *agentv1.JobRequest, result *agentv1.JobResult,
) error {
	if request == nil {
		if w.cfg.Production.Enabled {
			return errors.New("managed production: admitted request required")
		}
		return w.publishManagedResultWithAuthority(traceID, managedEventAuthority{jobID: result.GetJobId()}, result)
	}
	ctx := withManagedEventPublisher(context.Background(), w, traceID, request)
	return w.publishManagedResultForContext(ctx, result)
}

func (w *ManagedWorker) publishManagedResultForContext(ctx context.Context, result *agentv1.JobResult) error {
	publisher, err := managedPublisherFromContext(ctx)
	if err != nil {
		return err
	}
	return w.publishManagedResultWithAuthority(publisher.traceID, publisher.authority, result)
}

func (w *ManagedWorker) publishManagedResultWithAuthority(
	traceID string, authority managedEventAuthority, result *agentv1.JobResult,
) error {
	identity, err := bindManagedResult(result, authority)
	if err != nil {
		return err
	}
	packet := &agentv1.BusPacket{
		TraceId: traceID, SenderId: w.workerID,
		ProtocolVersion: capsdk.DefaultProtocolVersion, CreatedAt: timestamppb.Now(), Identity: identity,
		Payload: &agentv1.BusPacket_JobResult{JobResult: result},
	}
	return w.publishManagedPacket(capsdk.SubjectResult, packet)
}

func (w *ManagedWorker) recordManagedMetric(result *agentv1.JobResult, execution int64) {
	if w.metrics == nil {
		return
	}
	if result.Status == agentv1.JobStatus_JOB_STATUS_FAILED {
		w.metrics.OnJobFailed(result.JobId, result.ErrorMessage)
		return
	}
	w.metrics.OnJobCompleted(result.JobId, execution, result.Status.String())
}

func (w *ManagedWorker) acquireManagedSlot(ctx context.Context) bool {
	select {
	case w.sem <- struct{}{}:
		atomic.AddInt32(&w.active, 1)
		return true
	case <-ctx.Done():
		return false
	}
}

func (w *ManagedWorker) releaseManagedSlot() {
	<-w.sem
	atomic.AddInt32(&w.active, -1)
}
