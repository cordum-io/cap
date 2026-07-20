package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/nats-io/nats.go"
)

func (w *ManagedWorker) subscribeManagedSubjects(ctx context.Context, handler Handler) error {
	if w.cfg.Production.Enabled {
		return w.subscribeManagedProductionSubjects(ctx, handler)
	}
	return w.subscribeManagedCompatibilitySubjects(ctx, handler)
}

func (w *ManagedWorker) subscribeManagedCompatibilitySubjects(
	ctx context.Context, handler Handler,
) error {
	for _, subject := range w.admitted {
		queue := managedQueue(w.queue, subject)
		sub, err := w.conn.QueueSubscribe(subject, queue, func(message *nats.Msg) {
			w.dispatch(ctx, message, handler)
		})
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", subject, err)
		}
		w.subsAppend(sub)
	}
	return nil
}

func (w *ManagedWorker) subscribeManagedProductionSubjects(
	ctx context.Context, handler Handler,
) error {
	stream := w.cfg.Production.Stream
	if !validManagedNATSName(stream) {
		return errorsManagedProductionStream()
	}
	js, err := w.conn.JetStream()
	if err != nil {
		return fmt.Errorf("managed production JetStream: %w", err)
	}
	for _, subject := range w.admitted {
		if err := w.subscribeManagedProductionSubject(ctx, js, stream, subject, handler); err != nil {
			return err
		}
	}
	return nil
}

func (w *ManagedWorker) subscribeManagedProductionSubject(
	ctx context.Context, js nats.JetStreamContext, stream, subject string, handler Handler,
) error {
	queue := managedQueue(w.queue, subject)
	durable := managedProductionDurableName(queue, subject)
	sub, err := js.QueueSubscribe(subject, queue, func(message *nats.Msg) {
		w.dispatch(ctx, message, handler)
	}, nats.BindStream(stream), nats.Durable(durable), nats.ManualAck(), nats.AckExplicit(),
		nats.AckWait(managedReplayLease(w.cfg.Production)), nats.MaxAckPending(int(w.cfg.MaxParallelJobs)))
	if err != nil {
		return fmt.Errorf("subscribe durable production %s: %w", subject, err)
	}
	w.subsAppend(sub)
	return nil
}

func managedQueue(configured, subject string) string {
	if configured != "" {
		return configured
	}
	return subject
}

func managedProductionDurableName(queue, subject string) string {
	digest := sha256.Sum256([]byte(queue + "\x00" + subject))
	return "cap-worker-" + hex.EncodeToString(digest[:16])
}

func errorsManagedProductionStream() error {
	return fmt.Errorf("managed production: canonical JetStream name required")
}
