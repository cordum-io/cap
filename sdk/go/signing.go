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

// SignPacket signs a BusPacket and stores the signature on the packet.
func SignPacket(packet *agentv1.BusPacket, key *ecdsa.PrivateKey) error {
	if packet == nil {
		return errors.New("capsdk: packet is nil")
	}
	if key == nil {
		return errors.New("capsdk: private key is nil")
	}

	clone := proto.Clone(packet).(*agentv1.BusPacket)
	clone.Signature = nil
	unsignedData, err := MarshalDeterministic(clone)
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

	clone := proto.Clone(packet).(*agentv1.BusPacket)
	clone.Signature = nil
	unsignedData, err := MarshalDeterministic(clone)
	if err != nil {
		return fmt.Errorf("marshal unsigned packet: %w", err)
	}
	hash := sha256.Sum256(unsignedData)
	if !ecdsa.VerifyASN1(key, hash[:], signature) {
		return ErrInvalidSignature
	}
	return nil
}
