package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type managedEventContextKey struct{}

type managedEventAuthority struct {
	jobID    string
	identity *agentv1.IdentityBinding
	dispatch *agentv1.DispatchIdentity
}

type managedEventPublisher struct {
	worker    *ManagedWorker
	traceID   string
	authority managedEventAuthority
	err       error
}

func withManagedEventPublisher(
	ctx context.Context, worker *ManagedWorker, traceID string, request *agentv1.JobRequest,
) context.Context {
	authority, err := worker.snapshotManagedEventAuthority(request)
	publisher := &managedEventPublisher{worker: worker, traceID: traceID, authority: authority, err: err}
	return context.WithValue(ctx, managedEventContextKey{}, publisher)
}

// PublishManagedProgress publishes progress for the currently executing
// managed job. In production mode it uses the immutable admitted authority.
func PublishManagedProgress(ctx context.Context, progress *agentv1.JobProgress) error {
	publisher, err := managedPublisherFromContext(ctx)
	if err != nil {
		return err
	}
	if progress == nil {
		return errors.New("managed progress required")
	}
	event := proto.Clone(progress).(*agentv1.JobProgress)
	if err := bindManagedProgress(event, publisher.authority); err != nil {
		return err
	}
	if publisher.worker.cfg.Production.Enabled {
		if err := validateManagedProductionProgressResources(
			event, publisher.worker.cfg.Production.ResourceResolvers, time.Now(),
		); err != nil {
			return err
		}
	}
	packet := publisher.worker.managedProgressPacket(publisher.traceID, event, publisher.authority.identity)
	return publisher.worker.publishManagedPacket(capsdk.SubjectProgress, packet)
}

func managedPublisherFromContext(ctx context.Context) (*managedEventPublisher, error) {
	if ctx == nil {
		return nil, errors.New("managed event context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	publisher, ok := ctx.Value(managedEventContextKey{}).(*managedEventPublisher)
	if !ok || publisher == nil || publisher.worker == nil {
		return nil, errors.New("managed event publisher unavailable")
	}
	if publisher.err != nil {
		return nil, publisher.err
	}
	return publisher, nil
}

func (w *ManagedWorker) snapshotManagedEventAuthority(request *agentv1.JobRequest) (managedEventAuthority, error) {
	if request == nil || request.GetJobId() == "" {
		return managedEventAuthority{}, errors.New("managed event request required")
	}
	authority := managedEventAuthority{jobID: request.GetJobId()}
	if !w.cfg.Production.Enabled {
		return authority, nil
	}
	if request.GetIdentity() == nil || request.GetDispatch() == nil {
		return managedEventAuthority{}, errors.New("managed production: request authority required")
	}
	authority.identity = proto.Clone(request.GetIdentity()).(*agentv1.IdentityBinding)
	authority.dispatch = proto.Clone(request.GetDispatch()).(*agentv1.DispatchIdentity)
	return authority, nil
}

func bindManagedResult(
	result *agentv1.JobResult, authority managedEventAuthority,
) (*agentv1.IdentityBinding, error) {
	if result == nil {
		return nil, errors.New("managed result required")
	}
	if authority.identity == nil {
		return nil, nil
	}
	if err := bindManagedJobID(&result.JobId, authority.jobID); err != nil {
		return nil, err
	}
	if err := validateManagedEcho(result.GetIdentity(), result.GetDispatch(), authority); err != nil {
		return nil, err
	}
	result.Identity = cloneManagedIdentity(authority.identity)
	result.Dispatch = cloneManagedDispatch(authority.dispatch)
	return cloneManagedIdentity(authority.identity), nil
}

func bindManagedProgress(progress *agentv1.JobProgress, authority managedEventAuthority) error {
	if err := bindManagedJobID(&progress.JobId, authority.jobID); err != nil {
		return err
	}
	if authority.identity == nil {
		return nil
	}
	if err := validateManagedEcho(progress.GetIdentity(), progress.GetDispatch(), authority); err != nil {
		return err
	}
	progress.Identity = cloneManagedIdentity(authority.identity)
	progress.Dispatch = cloneManagedDispatch(authority.dispatch)
	return nil
}

func bindManagedJobID(target *string, authoritative string) error {
	if *target != "" && *target != authoritative {
		return errors.New("managed event job id conflicts with admitted request")
	}
	*target = authoritative
	return nil
}

func validateManagedEcho(
	identity *agentv1.IdentityBinding, dispatch *agentv1.DispatchIdentity, authority managedEventAuthority,
) error {
	if identity != nil && !proto.Equal(identity, authority.identity) {
		return capsdk.ErrIdentityMismatch
	}
	if dispatch != nil && !proto.Equal(dispatch, authority.dispatch) {
		return capsdk.ErrStaleDispatchEvent
	}
	return nil
}

func cloneManagedIdentity(identity *agentv1.IdentityBinding) *agentv1.IdentityBinding {
	if identity == nil {
		return nil
	}
	return proto.Clone(identity).(*agentv1.IdentityBinding)
}

func cloneManagedDispatch(dispatch *agentv1.DispatchIdentity) *agentv1.DispatchIdentity {
	if dispatch == nil {
		return nil
	}
	return proto.Clone(dispatch).(*agentv1.DispatchIdentity)
}

func (w *ManagedWorker) managedProgressPacket(
	traceID string, progress *agentv1.JobProgress, identity *agentv1.IdentityBinding,
) *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId: traceID, SenderId: w.workerID, ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt: timestamppb.Now(), Identity: cloneManagedIdentity(identity),
		Payload: &agentv1.BusPacket_JobProgress{JobProgress: progress},
	}
}

func (w *ManagedWorker) publishManagedPacket(subject string, packet *agentv1.BusPacket) error {
	token, err := w.privilegedSessionToken()
	if err != nil {
		return err
	}
	packet.AuthToken = token
	data, err := w.marshalManagedEnvelope(subject, packet)
	if err != nil {
		return err
	}
	if err := w.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("publish managed event: %w", err)
	}
	return nil
}

func (w *ManagedWorker) marshalManagedEnvelope(subject string, packet *agentv1.BusPacket) ([]byte, error) {
	if w.cfg.Production.Enabled {
		return w.marshalManagedProductionPacket(subject, packet)
	}
	return marshalValidatedEnvelope(packet, w.cfg.PrivateKey)
}
