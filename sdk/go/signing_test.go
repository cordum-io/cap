package capsdk

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"math/big"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestSignPacketWithAuthTokenPayloadUsesCrossSDKOrder(t *testing.T) {
	key := testPrivateSigningKey()
	packet := authTokenPayloadPacket()

	if err := SignPacket(packet, key); err != nil {
		t.Fatalf("SignPacket: %v", err)
	}
	if err := VerifyPacketSignature(packet, &key.PublicKey); err != nil {
		t.Fatalf("VerifyPacketSignature: %v", err)
	}

	unsigned, err := MarshalUnsignedForSignature(packet)
	if err != nil {
		t.Fatalf("MarshalUnsignedForSignature: %v", err)
	}
	assertTagOrder(t, unsigned, 11, 18)
	assertDiffersFromGoGeneratedOrder(t, packet, unsigned)

	packet.AuthToken = "tampered-token"
	if err := VerifyPacketSignature(packet, &key.PublicKey); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered VerifyPacketSignature=%v want %v", err, ErrInvalidSignature)
	}
}

func authTokenPayloadPacket() *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId:         "trace-auth-token-signing",
		SenderId:        "worker-1",
		ProtocolVersion: DefaultProtocolVersion,
		AuthToken:       "session-token-signing",
		Payload: &agentv1.BusPacket_JobResult{JobResult: &agentv1.JobResult{
			JobId:    "job-1",
			Status:   agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
			WorkerId: "worker-1",
		}},
	}
}

func assertTagOrder(t *testing.T, data []byte, first, second protowire.Number) {
	t.Helper()
	firstIndex := bytes.Index(data, protowire.AppendTag(nil, first, protowire.BytesType))
	secondIndex := bytes.Index(data, protowire.AppendTag(nil, second, protowire.BytesType))
	if firstIndex < 0 || secondIndex < 0 {
		t.Fatalf("missing tags %d/%d in %x", first, second, data)
	}
	if firstIndex > secondIndex {
		t.Fatalf("field %d appears after field %d in %x", first, second, data)
	}
}

func assertDiffersFromGoGeneratedOrder(t *testing.T, packet *agentv1.BusPacket, canonical []byte) {
	t.Helper()
	clone := proto.Clone(packet).(*agentv1.BusPacket)
	clone.Signature = nil
	generated, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	if err != nil {
		t.Fatalf("marshal generated order: %v", err)
	}
	if bytes.Equal(generated, canonical) {
		t.Fatal("canonical signing bytes unexpectedly match Go generated order")
	}
}

func testPrivateSigningKey() *ecdsa.PrivateKey {
	curve := elliptic.P256()
	d := big.NewInt(1)
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}
}
