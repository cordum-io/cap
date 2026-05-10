package runtime

import (
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const testSessionToken = "session-token-typed"

func TestAttachSessionTokenWritesTypedField(t *testing.T) {
	packet := &agentv1.BusPacket{}

	attachSessionToken(packet, testSessionToken)

	if got := packet.GetAuthToken(); got != testSessionToken {
		t.Fatalf("auth_token=%q want %q", got, testSessionToken)
	}
	if got := packet.ProtoReflect().GetUnknown(); len(got) != 0 {
		t.Fatalf("attachSessionToken wrote unknown bytes: %x", got)
	}
}

func TestAttachSessionTokenRoundTripsTypedField(t *testing.T) {
	packet := &agentv1.BusPacket{}
	attachSessionToken(packet, testSessionToken)

	data, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	var decoded agentv1.BusPacket
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal packet: %v", err)
	}

	if got := decoded.GetAuthToken(); got != testSessionToken {
		t.Fatalf("decoded auth_token=%q want %q", got, testSessionToken)
	}
	if got := ExtractSessionToken(&decoded); got != testSessionToken {
		t.Fatalf("ExtractSessionToken=%q want %q", got, testSessionToken)
	}
}

func TestExtractSessionTokenReadsLegacyUnknownField(t *testing.T) {
	packet := &agentv1.BusPacket{}
	field := authTokenField(t, packet)
	var raw []byte
	raw = protowire.AppendTag(raw, field.Number(), protowire.BytesType)
	raw = protowire.AppendString(raw, testSessionToken)
	packet.ProtoReflect().SetUnknown(raw)

	if got := packet.GetAuthToken(); got != "" {
		t.Fatalf("typed auth_token=%q want empty before decode", got)
	}
	if got := ExtractSessionToken(packet); got != testSessionToken {
		t.Fatalf("legacy ExtractSessionToken=%q want %q", got, testSessionToken)
	}
}

func TestBusPacketAuthTokenFieldDescriptor(t *testing.T) {
	field := authTokenField(t, &agentv1.BusPacket{})

	if field.Number() != 18 {
		t.Fatalf("auth_token field number=%d want 18", field.Number())
	}
	if field.Kind() != protoreflect.StringKind {
		t.Fatalf("auth_token kind=%v want string", field.Kind())
	}
	if field.ContainingOneof() != nil {
		t.Fatalf("auth_token unexpectedly belongs to oneof %s", field.ContainingOneof().FullName())
	}
}

func TestBusPacketAppendOnlyFieldNumbers(t *testing.T) {
	fields := (&agentv1.BusPacket{}).ProtoReflect().Descriptor().Fields()
	want := map[protoreflect.Name]protoreflect.FieldNumber{
		"trace_id":         1,
		"sender_id":        2,
		"created_at":       3,
		"protocol_version": 4,
		"job_request":      10,
		"job_result":       11,
		"heartbeat":        12,
		"alert":            13,
		"signature":        14,
		"job_progress":     15,
		"job_cancel":       16,
		"handshake":        17,
		"auth_token":       18,
	}

	for name, wantNumber := range want {
		field := fields.ByName(name)
		if field == nil {
			t.Fatalf("missing BusPacket field %s", name)
		}
		if field.Number() != wantNumber {
			t.Fatalf("%s field number=%d want %d", name, field.Number(), wantNumber)
		}
	}
}

func authTokenField(t *testing.T, packet *agentv1.BusPacket) protoreflect.FieldDescriptor {
	t.Helper()
	field := packet.ProtoReflect().Descriptor().Fields().ByName(busPacketAuthTokenName)
	if field == nil {
		t.Fatal("BusPacket.auth_token descriptor missing")
	}
	return field
}
