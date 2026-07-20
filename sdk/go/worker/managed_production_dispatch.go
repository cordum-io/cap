package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func (w *ManagedWorker) handleManagedProductionMessage(
	ctx context.Context, message *nats.Msg, packet *agentv1.BusPacket,
	request *agentv1.JobRequest, handler Handler,
) {
	work, err := w.beginManagedReplay(ctx, message.Data, packet, message.Subject)
	if err != nil {
		w.logManagedProductionError("replay admission", err)
		if errors.Is(err, capsdk.ErrReplayConflict) {
			w.terminateManagedMessage(message)
		} else {
			w.retryManagedMessage(message)
		}
		return
	}
	switch work.claim.State {
	case ManagedReplayPending:
		w.retryManagedMessage(message)
	case ManagedReplayCompleted:
		w.replayManagedCompleted(message, request, work.claim.Outcome)
	case ManagedReplayProcess:
		w.processManagedProduction(ctx, message, packet, request, handler, work)
	}
}

func (w *ManagedWorker) processManagedProduction(
	ctx context.Context, message *nats.Msg, packet *agentv1.BusPacket,
	request *agentv1.JobRequest, handler Handler, work managedReplayWork,
) {
	handlerCtx, lease := w.startManagedReplayLease(ctx, work, message)
	defer lease.finish()
	if w.metrics != nil {
		w.metrics.OnJobReceived(request.GetJobId(), request.GetTopic())
	}
	eventCtx, result, execution := w.executeManagedHandler(handlerCtx, packet, request, handler)
	outcome, prepareErr := w.prepareManagedReplayOutcome(eventCtx, result)
	leaseErr := lease.stop()
	if prepareErr != nil || leaseErr != nil || ctx.Err() != nil {
		w.abortManagedReplay(work)
		w.logManagedProductionError("prepare outcome", errors.Join(prepareErr, leaseErr, ctx.Err()))
		w.retryManagedMessage(message)
		return
	}
	if err := w.completeManagedReplay(work, outcome); err != nil {
		w.logManagedProductionError("complete outcome", err)
		w.retryManagedMessage(message)
		return
	}
	w.recordManagedMetric(result, execution)
	if err := w.publishManagedReplayOutcome(eventCtx, outcome); err != nil {
		w.logManagedProductionError("publish outcome", err)
		w.retryManagedMessage(message)
		return
	}
	w.ackManagedMessage(message)
}

func (w *ManagedWorker) prepareManagedReplayOutcome(
	ctx context.Context, result *agentv1.JobResult,
) (ManagedReplayOutcome, error) {
	publisher, err := managedPublisherFromContext(ctx)
	if err != nil {
		return ManagedReplayOutcome{}, err
	}
	if _, err := bindManagedResult(result, publisher.authority); err != nil {
		return ManagedReplayOutcome{}, err
	}
	if result.GetWorkerId() != w.workerID {
		return ManagedReplayOutcome{}, errors.New("managed production: result worker conflicts with session")
	}
	if err := validateManagedProductionResultResources(
		result, w.cfg.Production.ResourceResolvers, time.Now(),
	); err != nil {
		return ManagedReplayOutcome{}, err
	}
	if err := capsdk.ValidateJobResult(result); err != nil {
		return ManagedReplayOutcome{}, err
	}
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(result)
	if err != nil {
		return ManagedReplayOutcome{}, fmt.Errorf("marshal managed result: %w", err)
	}
	return ManagedReplayOutcome{TraceID: publisher.traceID, Result: data}, nil
}

func (w *ManagedWorker) replayManagedCompleted(
	message *nats.Msg, request *agentv1.JobRequest, outcome ManagedReplayOutcome,
) {
	ctx := withManagedEventPublisher(context.Background(), w, outcome.TraceID, request)
	if err := w.publishManagedReplayOutcome(ctx, outcome); err != nil {
		w.logManagedProductionError("replay outcome", err)
		w.retryManagedMessage(message)
		return
	}
	w.ackManagedMessage(message)
}

func (w *ManagedWorker) publishManagedReplayOutcome(
	ctx context.Context, outcome ManagedReplayOutcome,
) error {
	result := &agentv1.JobResult{}
	if err := proto.Unmarshal(outcome.Result, result); err != nil {
		return errors.New("managed production: corrupt durable outcome")
	}
	if err := w.publishManagedResultForContext(ctx, result); err != nil {
		return err
	}
	if err := w.conn.FlushTimeout(defaultManagedReplayTimeout); err != nil {
		return fmt.Errorf("flush managed outcome: %w", err)
	}
	return nil
}

func (w *ManagedWorker) ackManagedMessage(message *nats.Msg) {
	if !isManagedJetStreamMessage(message) {
		return
	}
	if err := message.Ack(); err != nil {
		w.logManagedProductionError("ack input", err)
	}
}

func (w *ManagedWorker) retryManagedMessage(message *nats.Msg) {
	if !isManagedJetStreamMessage(message) {
		return
	}
	if err := message.NakWithDelay(defaultManagedReplayRetryDelay); err != nil {
		w.logManagedProductionError("retry input", err)
	}
}

func (w *ManagedWorker) terminateManagedMessage(message *nats.Msg) {
	if !isManagedJetStreamMessage(message) {
		return
	}
	if err := message.Term(); err != nil {
		w.logManagedProductionError("terminate input", err)
	}
}

func (w *ManagedWorker) settleManagedProductionRejection(message *nats.Msg, err error) {
	w.logManagedProductionError("packet admission", err)
	if errors.Is(err, errManagedProductionSessionUnavailable) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		w.retryManagedMessage(message)
		return
	}
	w.terminateManagedMessage(message)
}

func isManagedJetStreamMessage(message *nats.Msg) bool {
	if message == nil {
		return false
	}
	_, err := message.Metadata()
	return err == nil
}

func (w *ManagedWorker) logManagedProductionError(operation string, err error) {
	if err != nil && w.logger != nil {
		w.logger.Printf("worker: managed production %s failed", operation)
	}
}
