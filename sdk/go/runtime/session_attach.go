package runtime

// Outbound session-token attachment. After a successful Phase-2
// handshake the agent holds a session token; every packet it
// publishes must carry that token so the scheduler can verify trust
// without re-doing the handshake per request.
//
// The token rides on the declared BusPacket.auth_token field. Older
// unknown-field encodings are still accepted by ExtractSessionToken for
// compatibility, but outbound packets should use the typed field.

import (
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const busPacketAuthTokenName protoreflect.Name = "auth_token"

// attachSessionToken writes token into packet's declared auth_token
// field. A nil packet or empty token is a no-op so callers don't need
// to nil-guard each use.
func attachSessionToken(packet *agentv1.BusPacket, token string) {
	if packet == nil || token == "" {
		return
	}
	packet.AuthToken = token
}

// withSessionToken is the convenience the publish paths use: it
// snapshots the agent's current session token and attaches it when
// non-empty. Keeping the snapshot-then-attach logic here prevents a
// race with the renew loop (step-7) that may rotate the token between
// read and attach.
func (a *Agent) withSessionToken(packet *agentv1.BusPacket) {
	if a == nil || packet == nil {
		return
	}
	token, _ := a.SessionToken()
	attachSessionToken(packet, token)
}

// ExtractSessionToken is the inverse helper used by tests and by
// adapters that need to inspect the attached token before publishing.
// Production scheduler code consumes the token via its own
// authTokenFromPacket helper — kept separate so the scheduler side
// does not pull a dependency on the SDK.
func ExtractSessionToken(packet *agentv1.BusPacket) string {
	if packet == nil {
		return ""
	}
	if token := packet.GetAuthToken(); token != "" {
		return token
	}
	authTokenField := packet.ProtoReflect().Descriptor().Fields().ByName(busPacketAuthTokenName)
	if authTokenField == nil {
		return ""
	}
	raw := packet.ProtoReflect().GetUnknown()
	for len(raw) > 0 {
		fieldNum, wireType, tagLen := protowire.ConsumeTag(raw)
		if tagLen < 0 {
			return ""
		}
		raw = raw[tagLen:]
		if fieldNum == authTokenField.Number() && wireType == protowire.BytesType {
			value, valueLen := protowire.ConsumeBytes(raw)
			if valueLen < 0 {
				return ""
			}
			return string(value)
		}
		valueLen := protowire.ConsumeFieldValue(fieldNum, wireType, raw)
		if valueLen < 0 {
			return ""
		}
		raw = raw[valueLen:]
	}
	return ""
}
