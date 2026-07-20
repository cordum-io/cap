package capsdk

import (
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

// VerifiedProductionPacket is an opaque proof that CAP-PRODUCTION verified
// exact raw bytes against locally supplied trust. External packages can carry
// and inspect this value, but cannot bind an arbitrary BusPacket to it.
type VerifiedProductionPacket struct {
	state *verifiedProductionState
}

type verifiedProductionState struct {
	packet       *agentv1.BusPacket
	subject      string
	tenant       string
	sender       string
	sessionToken string
	messageID    []byte
	digest       [32]byte
}

func newVerifiedProductionPacket(
	packet *agentv1.BusPacket, trust ProductionTrustStore, digest [32]byte,
) VerifiedProductionPacket {
	return VerifiedProductionPacket{state: &verifiedProductionState{
		packet:  proto.Clone(packet).(*agentv1.BusPacket),
		subject: trust.Audience, tenant: trust.Tenant, sender: trust.Sender,
		sessionToken: packet.GetAuthToken(),
		messageID:    append([]byte(nil), packet.GetSignatureMetadata().GetMessageId()...),
		digest:       digest,
	}}
}

// IsVerified reports whether this value was returned by successful exact-wire
// verification. The externally constructible zero value is never verified.
func (v VerifiedProductionPacket) IsVerified() bool {
	return v.state != nil && v.state.packet != nil
}

// Packet returns an immutable snapshot of the verified packet.
func (v VerifiedProductionPacket) Packet() *agentv1.BusPacket {
	if !v.IsVerified() {
		return nil
	}
	return proto.Clone(v.state.packet).(*agentv1.BusPacket)
}

// Subject is the actual transport subject used as the verified audience.
func (v VerifiedProductionPacket) Subject() string {
	if !v.IsVerified() {
		return ""
	}
	return v.state.subject
}

// Tenant is the authenticated tenant authority used for key resolution.
func (v VerifiedProductionPacket) Tenant() string {
	if !v.IsVerified() {
		return ""
	}
	return v.state.tenant
}

// Sender is the authenticated transport/session sender used for key resolution.
func (v VerifiedProductionPacket) Sender() string {
	if !v.IsVerified() {
		return ""
	}
	return v.state.sender
}

// SessionToken is the signature-covered session token from the verified wire.
func (v VerifiedProductionPacket) SessionToken() string {
	if !v.IsVerified() {
		return ""
	}
	return v.state.sessionToken
}

// MessageID returns a copy of the signature-covered replay identifier.
func (v VerifiedProductionPacket) MessageID() []byte {
	if !v.IsVerified() {
		return nil
	}
	return append([]byte(nil), v.state.messageID...)
}

// SignedBodyDigest returns SHA-256 over the exact unsigned wire body.
func (v VerifiedProductionPacket) SignedBodyDigest() [32]byte {
	if !v.IsVerified() {
		return [32]byte{}
	}
	return v.state.digest
}
