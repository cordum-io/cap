package capsdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestProductionSignedBodyDigestRejectsNonMinimalFieldEncoding(t *testing.T) {
	tests := map[string][]byte{
		"varint value": {0x20, 0x81, 0x00, 0x72, 0x01, 0x01},
		"bytes length": {0x0a, 0x81, 0x00, 0x00, 0x72, 0x01, 0x01},
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ProductionSignedBodyDigest(raw); !errors.Is(err, ErrMalformedProductionWire) {
				t.Fatalf("ProductionSignedBodyDigest error=%v, want ErrMalformedProductionWire", err)
			}
		})
	}
	minimal := []byte{0x20, 0x01, 0x0a, 0x01, 0x00, 0x72, 0x01, 0x01}
	if _, err := ProductionSignedBodyDigest(minimal); err != nil {
		t.Fatalf("minimal known-field wire rejected: %v", err)
	}
}

func TestProductionSigningRejectsNonP256Key(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	if _, err := SignProductionPacket(packet, key); err == nil {
		t.Fatal("SignProductionPacket accepted a P-384 key for the P-256 algorithm")
	}
}

func TestProductionVerifyRejectsNonP256Key(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	unsigned, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	digest := productionDigest(unsigned)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatalf("SignASN1: %v", err)
	}
	raw := appendSignatureField(unsigned, signature)
	_, err = VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a"))
	if !errors.Is(err, ErrUnsupportedProductionKey) {
		t.Fatalf("VerifyProductionPacket error=%v, want ErrUnsupportedProductionKey", err)
	}
}

func TestProductionSignedBodyDigestRejectsOversizeWire(t *testing.T) {
	if _, err := ProductionSignedBodyDigest(make([]byte, DefaultProductionMaxRawBytes+1)); !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("ProductionSignedBodyDigest error=%v, want ErrMalformedProductionWire", err)
	}
}

func TestProductionVerifyRejectsAmbiguousTopLevelWire(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	unsigned, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	tests := map[string][]byte{
		"unknown field":        appendBytesField(unsigned, 99, []byte("unknown")),
		"duplicate sender":     appendBytesField(unsigned, 2, []byte("other-sender")),
		"second payload":       appendBytesField(unsigned, 12, nil),
		"wrong known wiretype": protowire.AppendVarint(protowire.AppendTag(append([]byte(nil), unsigned...), 1, protowire.VarintType), 1),
	}
	for name, mutated := range tests {
		t.Run(name, func(t *testing.T) {
			raw := signProductionWireForTest(t, mutated, key)
			_, err := VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a"))
			if !errors.Is(err, ErrMalformedProductionWire) {
				t.Fatalf("VerifyProductionPacket error=%v, want ErrMalformedProductionWire", err)
			}
		})
	}
}

func TestProductionVerifyRejectsAmbiguousProtocolVersionWire(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.ProtocolVersion = 0
	unsigned, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	tests := map[string]uint64{
		"zero": 0, "unsupported": 2, "int32 overflow": 1<<32 + 1, "negative int32": ^uint64(0),
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			mutated := protowire.AppendTag(append([]byte(nil), unsigned...), 4, protowire.VarintType)
			mutated = protowire.AppendVarint(mutated, value)
			raw := signProductionWireForTest(t, mutated, key)
			_, err := VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a"))
			if !errors.Is(err, ErrMalformedProductionWire) {
				t.Fatalf("VerifyProductionPacket error=%v, want protocol-wire rejection", err)
			}
		})
	}
}

func appendBytesField(raw []byte, number protowire.Number, value []byte) []byte {
	result := protowire.AppendTag(append([]byte(nil), raw...), number, protowire.BytesType)
	return protowire.AppendBytes(result, value)
}

func signProductionWireForTest(t *testing.T, unsigned []byte, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	digest := productionDigest(unsigned)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatalf("ecdsa.SignASN1: %v", err)
	}
	return appendSignatureField(unsigned, signature)
}
