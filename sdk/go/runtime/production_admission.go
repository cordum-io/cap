package runtime

// Fail-closed raw-packet admission for CAP-PRODUCTION (task-a13f83fa, DoD #1
// and #4). Runs BEFORE any protobuf handler sees the packet: raw-wire
// signature verification (never a re-serialized object), metadata/audience/
// expiry, authoritative identity, then atomic replay admission. Kept
// intentionally small and free of job/handler semantics — a rejection here
// means the packet never reaches proto.Unmarshal-based dispatch at all.

import (
	"crypto/sha256"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
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
	return a.Production.Enabled && a.Production.Replay != nil
}

// admitProductionPacket verifies raw wire bytes (never the re-serialized
// object), then performs atomic replay admission. On success it returns the
// verified packet so callers skip a redundant proto.Unmarshal. Any error
// means the packet MUST NOT reach a handler; callers must not log err's
// content if it could embed signature/token bytes (none of the sentinel
// errors below do — they are static/wrapped-static only).
func (a *Agent) admitProductionPacket(raw []byte) (*agentv1.BusPacket, error) {
	trust := a.Production.Trust
	packet, err := capsdk.VerifyProductionPacket(raw, trust)
	if err != nil {
		return nil, err
	}

	meta := packet.GetSignatureMetadata()
	tenant := packet.GetIdentity().GetTenantId()
	audience := meta.GetAudience()
	sender := packet.GetSenderId()
	digest := sha256.Sum256(raw)

	outcome, err := a.Production.Replay.Admit(tenant, audience, sender, meta.GetMessageId(), digest[:], meta.GetExpiresAt().AsTime())
	if err != nil {
		return nil, err
	}
	_ = outcome // ReplayOutcomeFirst and ReplayOutcomeDuplicate are both admissible; only ErrReplayConflict/unavailable reject.

	if req := packet.GetJobRequest(); req != nil && packet.GetIdentity() != nil {
		if err := capsdk.ValidateIdentityBinding(req, packet.GetIdentity()); err != nil {
			return nil, err
		}
	}
	return packet, nil
}
