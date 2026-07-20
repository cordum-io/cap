package capsdk

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestProductionSignerRejectsNestedUnknownField(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.GetSignatureMetadata().ProtoReflect().SetUnknown(
		appendBytesField(nil, 99, []byte("unknown")),
	)
	if _, err := SignProductionPacket(packet, key); !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("SignProductionPacket error=%v, want nested-unknown rejection", err)
	}
}

func TestProductionVerifierRejectsSignedNestedUnknownField(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.GetJobResult().ProtoReflect().SetUnknown(
		appendBytesField(nil, 99, []byte("unknown")),
	)
	unsigned, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	raw := signProductionWireForTest(t, unsigned, key)
	if _, err := VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a")); !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want nested-unknown rejection", err)
	}
}
