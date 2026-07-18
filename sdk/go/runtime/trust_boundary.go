package runtime

import (
	"errors"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func (a *Agent) validateInboundPacket(packet *agentv1.BusPacket) error {
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return err
	}
	mode := a.activeHandshakeMode()
	if mode == HandshakeModeWarn || mode == HandshakeModeEnforce {
		if mode == HandshakeModeEnforce {
			if token, _ := a.SessionToken(); token == "" {
				return errors.New("cap-runtime: authenticated session is not active")
			}
		}
		return a.verifyPinnedSchedulerPacket(packet)
	}
	return a.verifyLegacyInboundPacket(packet)
}

func (a *Agent) verifyPinnedSchedulerPacket(packet *agentv1.BusPacket) error {
	trust := a.workerTrustConfig()
	if packet.GetSenderId() != trust.ExpectedSchedulerID {
		return errors.New("cap-runtime: scheduler sender mismatch")
	}
	if len(packet.GetSignature()) == 0 {
		return errors.New("cap-runtime: scheduler signature missing")
	}
	for _, key := range trust.SchedulerPublicKeys {
		if capsdk.VerifyPacketSignature(packet, key) == nil {
			return nil
		}
	}
	return errors.New("cap-runtime: scheduler signature invalid")
}

func (a *Agent) verifyLegacyInboundPacket(packet *agentv1.BusPacket) error {
	if a.PublicKeys == nil {
		return nil
	}
	publicKey, ok := a.PublicKeys[packet.GetSenderId()]
	if !ok {
		return fmt.Errorf("cap-runtime: no public key for sender %q", packet.GetSenderId())
	}
	if len(packet.GetSignature()) == 0 {
		return capsdk.NewSignatureMissingError("sender " + packet.GetSenderId())
	}
	return capsdk.VerifyPacketSignature(packet, publicKey)
}
