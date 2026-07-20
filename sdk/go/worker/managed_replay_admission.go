package worker

import (
	"context"
	"errors"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

type managedReplayWork struct {
	entry ManagedReplayEntry
	claim ManagedReplayClaim
}

func (w *ManagedWorker) beginManagedReplay(
	ctx context.Context, raw []byte, packet *agentv1.BusPacket, audience string,
) (managedReplayWork, error) {
	trust, err := w.managedProductionTrust(audience)
	if err != nil {
		return managedReplayWork{}, err
	}
	entry, err := w.managedReplayEntry(raw, packet, trust, time.Now())
	if err != nil {
		return managedReplayWork{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, defaultManagedReplayTimeout)
	defer cancel()
	claim, err := w.cfg.Production.Replay.Begin(callCtx, cloneManagedReplayEntry(entry))
	if err != nil {
		return managedReplayWork{}, managedReplayError(err)
	}
	if err := validateManagedReplayClaim(claim); err != nil {
		return managedReplayWork{}, err
	}
	claim.Outcome = cloneManagedReplayOutcome(claim.Outcome)
	return managedReplayWork{entry: entry, claim: claim}, nil
}

func (w *ManagedWorker) managedReplayEntry(
	raw []byte, packet *agentv1.BusPacket, trust capsdk.ProductionTrustStore, now time.Time,
) (ManagedReplayEntry, error) {
	digest, err := capsdk.ProductionSignedBodyDigest(raw)
	if err != nil {
		return ManagedReplayEntry{}, err
	}
	expiresAt := packet.GetSignatureMetadata().GetExpiresAt().AsTime().Add(trust.ClockSkew)
	leaseUntil := now.Add(managedReplayLease(w.cfg.Production))
	if leaseUntil.After(expiresAt) {
		leaseUntil = expiresAt
	}
	return ManagedReplayEntry{
		Tenant: trust.Tenant, Audience: trust.Audience, Sender: trust.Sender,
		MessageID: append([]byte(nil), packet.GetSignatureMetadata().GetMessageId()...),
		Digest: digest[:], ExpiresAt: expiresAt, LeaseUntil: leaseUntil,
	}, nil
}

func validateManagedReplayClaim(claim ManagedReplayClaim) error {
	switch claim.State {
	case ManagedReplayProcess:
		if !validManagedProductionID(claim.LeaseID) {
			return capsdk.ErrReplayStoreUnavailable
		}
	case ManagedReplayPending:
		if claim.LeaseID != "" || claim.Outcome.TraceID != "" || len(claim.Outcome.Result) != 0 {
			return capsdk.ErrReplayStoreUnavailable
		}
	case ManagedReplayCompleted:
		if claim.LeaseID != "" || claim.Outcome.TraceID == "" || len(claim.Outcome.Result) == 0 {
			return capsdk.ErrReplayStoreUnavailable
		}
	default:
		return capsdk.ErrReplayStoreUnavailable
	}
	return nil
}

func managedReplayError(err error) error {
	if errors.Is(err, capsdk.ErrReplayConflict) {
		return capsdk.ErrReplayConflict
	}
	return capsdk.ErrReplayStoreUnavailable
}
