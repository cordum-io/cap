package runtime

// Fail-closed raw-packet admission for CAP-PRODUCTION (task-a13f83fa, DoD #1
// and #4). Runs BEFORE any protobuf handler sees the packet: raw-wire
// signature verification (never a re-serialized object), metadata/audience/
// expiry, authoritative identity, then atomic replay admission. Kept
// intentionally small and free of job/handler semantics — a rejection here
// means the packet never reaches proto.Unmarshal-based dispatch at all.

import (
	"crypto/ecdsa"
	"errors"
	"math/big"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

var (
	ErrProductionSessionRequired = errors.New("cap-runtime: CAP-PRODUCTION requires worker trust ENFORCE")
	ErrProductionTrustConflict   = errors.New("cap-runtime: CAP-PRODUCTION trust conflicts with authenticated session")
)

// ProductionAdmission configures the CAP-PRODUCTION raw-packet admission
// layer. A nil/zero-value ProductionAdmission on Agent means production mode
// is OFF; the legacy validateInboundPacket path (handshake modes / object
// signatures) applies unchanged. This is the explicit migration/compat
// boundary — production is never inferred, only opted into.
type ProductionAdmission struct {
	Trust   capsdk.ProductionTrustStore
	Replay  capsdk.ReplayStore
	Enabled bool
}

func (a *Agent) productionAdmissionEnabled() bool {
	return a.activeProductionAdmission().Enabled
}

func (a *Agent) freezeProductionAdmission() {
	frozen := a.Production
	frozen.Trust.PublicKeys = cloneProductionKeys(a.Production.Trust.PublicKeys)
	a.productionConfig = &frozen
}

func (a *Agent) activeProductionAdmission() ProductionAdmission {
	if a.productionConfig != nil {
		return *a.productionConfig
	}
	return a.Production
}

func cloneProductionKeys(input map[string]*ecdsa.PublicKey) map[string]*ecdsa.PublicKey {
	if input == nil {
		return nil
	}
	output := make(map[string]*ecdsa.PublicKey, len(input))
	for keyID, key := range input {
		if key == nil {
			output[keyID] = nil
			continue
		}
		clone := *key
		clone.X = cloneBigInt(key.X)
		clone.Y = cloneBigInt(key.Y)
		output[keyID] = &clone
	}
	return output
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}

func (a *Agent) productionSessionActive() bool {
	if a.activeHandshakeMode() != HandshakeModeEnforce {
		return false
	}
	token, _ := a.SessionToken()
	return token != ""
}

func (a *Agent) validateProductionStartup() error {
	production := a.activeProductionAdmission()
	if !production.Enabled {
		return nil
	}
	if production.Replay == nil {
		return capsdk.ErrReplayStoreUnavailable
	}
	if a.activeHandshakeMode() != HandshakeModeEnforce {
		return ErrProductionSessionRequired
	}
	configured := production.Trust
	authority := a.workerTrustConfig()
	if configured.Tenant != "" && configured.Tenant != authority.TenantID {
		return ErrProductionTrustConflict
	}
	if configured.Sender != "" && configured.Sender != authority.ExpectedSchedulerID {
		return ErrProductionTrustConflict
	}
	if configured.ResolveKey == nil && len(configured.PublicKeys) == 0 {
		return ErrProductionTrustConflict
	}
	return nil
}

func (a *Agent) productionTrustForAudience(audience string) (capsdk.ProductionTrustStore, error) {
	trust := a.activeProductionAdmission().Trust
	trust.Audience = audience
	if a.activeHandshakeMode() != HandshakeModeEnforce {
		return trust, nil
	}
	authority := a.workerTrustConfig()
	if authority.TenantID == "" || authority.ExpectedSchedulerID == "" {
		return trust, ErrProductionSessionRequired
	}
	trust.Tenant = authority.TenantID
	trust.Sender = authority.ExpectedSchedulerID
	return trust, nil
}

// admitProductionPacket verifies raw wire bytes (never the re-serialized
// object), then performs atomic replay admission. On success it returns the
// verified packet so callers skip a redundant proto.Unmarshal. Any error
// means the packet MUST NOT reach a handler; callers must not log err's
// content if it could embed signature/token bytes (none of the sentinel
// errors below do — they are static/wrapped-static only).
func (a *Agent) admitProductionPacket(raw []byte) (*agentv1.BusPacket, error) {
	return a.admitProductionPacketForAudience(raw, a.activeProductionAdmission().Trust.Audience)
}

func (a *Agent) admitProductionPacketForAudience(raw []byte, audience string) (*agentv1.BusPacket, error) {
	trust, err := a.productionTrustForAudience(audience)
	if err != nil {
		return nil, err
	}
	packet, err := capsdk.VerifyProductionPacket(raw, trust)
	if err != nil {
		return nil, err
	}
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return nil, err
	}

	meta := packet.GetSignatureMetadata()
	tenant := packet.GetIdentity().GetTenantId()
	verifiedAudience := meta.GetAudience()
	sender := packet.GetSenderId()
	if req := packet.GetJobRequest(); req != nil && packet.GetIdentity() != nil {
		if err := capsdk.ValidateIdentityBinding(req, packet.GetIdentity()); err != nil {
			return nil, err
		}
	}
	production := a.activeProductionAdmission()
	if production.Replay == nil {
		return nil, capsdk.ErrReplayStoreUnavailable
	}
	digest, err := capsdk.ProductionSignedBodyDigest(raw)
	if err != nil {
		return nil, err
	}

	replayExpiry := meta.GetExpiresAt().AsTime().Add(trust.ClockSkew)
	outcome, err := production.Replay.Admit(
		tenant, verifiedAudience, sender, meta.GetMessageId(), digest[:], replayExpiry,
	)
	if err != nil {
		if errors.Is(err, capsdk.ErrReplayConflict) {
			return nil, capsdk.ErrReplayConflict
		}
		return nil, capsdk.ErrReplayStoreUnavailable
	}
	if outcome == capsdk.ReplayOutcomeDuplicate {
		return nil, nil // ACK/drop identical broker redelivery; never run side effects twice.
	}
	if outcome != capsdk.ReplayOutcomeFirst {
		return nil, capsdk.ErrReplayStoreUnavailable
	}

	return packet, nil
}
