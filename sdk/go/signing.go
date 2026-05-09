package capsdk

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrMissingSignature indicates a packet did not include a signature.
	ErrMissingSignature = errors.New("capsdk: missing signature")
	// ErrInvalidSignature indicates a signature could not be verified.
	ErrInvalidSignature = errors.New("capsdk: invalid signature")
)

// MarshalDeterministic serializes a protobuf message with deterministic ordering.
func MarshalDeterministic(msg proto.Message) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("capsdk: message is nil")
	}
	return proto.MarshalOptions{Deterministic: true}.Marshal(msg)
}

// MarshalUnsignedForSignature serializes a BusPacket with signature cleared
// using CAP's cross-SDK signing order.
func MarshalUnsignedForSignature(packet *agentv1.BusPacket) ([]byte, error) {
	if packet == nil {
		return nil, errors.New("capsdk: packet is nil")
	}
	clone := proto.Clone(packet).(*agentv1.BusPacket)
	clone.Signature = nil
	return marshalPacketForSignature(clone)
}

// SignPacket signs a BusPacket and stores the signature on the packet.
func SignPacket(packet *agentv1.BusPacket, key *ecdsa.PrivateKey) error {
	if packet == nil {
		return errors.New("capsdk: packet is nil")
	}
	if key == nil {
		return errors.New("capsdk: private key is nil")
	}

	unsignedData, err := MarshalUnsignedForSignature(packet)
	if err != nil {
		return fmt.Errorf("marshal unsigned packet: %w", err)
	}

	hash := sha256.Sum256(unsignedData)
	signature, err := ecdsa.SignASN1(rand.Reader, key, hash[:])
	if err != nil {
		return fmt.Errorf("sign packet: %w", err)
	}
	packet.Signature = signature
	return nil
}

// VerifyPacketSignature verifies a BusPacket signature using the provided public key.
func VerifyPacketSignature(packet *agentv1.BusPacket, key *ecdsa.PublicKey) error {
	if packet == nil {
		return errors.New("capsdk: packet is nil")
	}
	if key == nil {
		return errors.New("capsdk: public key is nil")
	}
	signature := packet.GetSignature()
	if len(signature) == 0 {
		return ErrMissingSignature
	}

	unsignedData, err := MarshalUnsignedForSignature(packet)
	if err != nil {
		return fmt.Errorf("marshal unsigned packet: %w", err)
	}
	hash := sha256.Sum256(unsignedData)
	if !ecdsa.VerifyASN1(key, hash[:], signature) {
		return ErrInvalidSignature
	}
	return nil
}

func marshalPacketForSignature(packet *agentv1.BusPacket) ([]byte, error) {
	if packet.GetAuthToken() == "" || packet.GetPayload() == nil {
		return MarshalDeterministic(packet)
	}
	// Python protobuf and protobufjs encode BusPacket oneof payload fields
	// before auth_token when signature is cleared. Go's generated fast path
	// emits scalar fields before oneofs, so auth_token would otherwise be
	// signed before the payload and fail cross-SDK verification.
	clone := proto.Clone(packet).(*agentv1.BusPacket)
	authToken := clone.AuthToken
	clone.AuthToken = ""

	data, err := MarshalDeterministic(clone)
	if err != nil {
		return nil, err
	}
	authBytes, err := MarshalDeterministic(&agentv1.BusPacket{AuthToken: authToken})
	if err != nil {
		return nil, err
	}
	return append(data, authBytes...), nil
}
