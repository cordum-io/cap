package worker

import (
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func (w *ManagedWorker) validateInboundPacket(packet *agentv1.BusPacket) error {
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return err
	}
	if w.trust != nil && w.trust.mode != capsdk.WorkerTrustModeOff {
		if w.trust.mode == capsdk.WorkerTrustModeEnforce && w.trust.token(time.Now()) == "" {
			return errors.New("worker trust: authenticated session is not active")
		}
		return w.verifyTrustedSchedulerPacket(packet)
	}
	if w.cfg.PublicKeys == nil {
		return nil
	}
	publicKey, ok := w.cfg.PublicKeys[packet.GetSenderId()]
	if !ok {
		return fmt.Errorf("no public key for sender %q", packet.GetSenderId())
	}
	if len(packet.GetSignature()) == 0 {
		return errors.New("missing packet signature")
	}
	return capsdk.VerifyPacketSignature(packet, publicKey)
}

func (w *ManagedWorker) verifyTrustedSchedulerPacket(packet *agentv1.BusPacket) error {
	if packet.GetSenderId() != w.trust.config.ExpectedSchedulerID {
		return errors.New("worker trust: scheduler sender mismatch")
	}
	if len(packet.GetSignature()) == 0 {
		return errors.New("worker trust: scheduler signature missing")
	}
	for _, publicKey := range w.trust.config.SchedulerPublicKeys {
		if capsdk.VerifyPacketSignature(packet, publicKey) == nil {
			return nil
		}
	}
	return errors.New("worker trust: scheduler signature invalid")
}
