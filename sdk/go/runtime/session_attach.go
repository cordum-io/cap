package runtime

// Outbound session-token attachment. After a successful Phase-2
// handshake the agent holds a session token; every packet it
// publishes must carry that token so the scheduler can verify trust
// without re-doing the handshake per request.
//
// The token rides on BusPacket unknown field 18 (BytesType). Putting
// it in unknown bytes preserves forward/backward compatibility:
//
//   - Older schedulers that don't know about session tokens ignore
//     the extra bytes (protobuf unknown-fields pass-through).
//   - Older workers that don't publish the field can still interop;
//     the scheduler simply observes an empty token and applies its
//     HandshakeMode policy (off/warn/enforce).
//
// The existing cordum scheduler already parses field 18 on inbound
// heartbeats (core/controlplane/scheduler/engine.go:authTokenFromPacket).
// We keep the same field number so there is no wire-format churn —
// new tokens just flow through the existing parser.

import (
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// busPacketAuthTokenField is the protobuf field number used for the
// session token in unknown-field bytes. Mirrored from the cordum
// scheduler-side parser — changing this without coordination with the
// scheduler would silently drop the token.
const busPacketAuthTokenField = 18

// attachSessionToken writes token into packet's unknown-field bytes
// (BytesType, field busPacketAuthTokenField). A nil packet or empty
// token is a no-op so callers don't need to nil-guard each use.
//
// The function is idempotent: calling it twice writes the token twice,
// which the scheduler parser accepts (it returns the first occurrence)
// but we avoid that by calling it at exactly one point per publish.
func attachSessionToken(packet *agentv1.BusPacket, token string) {
	if packet == nil || token == "" {
		return
	}
	raw := packet.ProtoReflect().GetUnknown()
	buf := make([]byte, 0, len(raw)+len(token)+8)
	buf = append(buf, raw...)
	buf = protowire.AppendTag(buf, busPacketAuthTokenField, protowire.BytesType)
	buf = protowire.AppendString(buf, token)
	packet.ProtoReflect().SetUnknown(buf)
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
	raw := packet.ProtoReflect().GetUnknown()
	for len(raw) > 0 {
		fieldNum, wireType, tagLen := protowire.ConsumeTag(raw)
		if tagLen < 0 {
			return ""
		}
		raw = raw[tagLen:]
		if fieldNum == busPacketAuthTokenField && wireType == protowire.BytesType {
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

// Suppress unused-import warning on platforms where proto isn't
// otherwise referenced in this file. The symbol is used via the
// packet's ProtoReflect() method only, so keeping the import explicit
// avoids go-mod-tidy removing it on future edits.
var _ = proto.Marshal
