package capsdk

import (
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

// MarshalWorkerTrustPacket validates and deterministically encodes one phase.
func MarshalWorkerTrustPacket(packet *agentv1.BusPacket) ([]byte, error) {
	if err := ValidateWorkerTrustPacket(packet); err != nil {
		return nil, err
	}
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(packet)
	if err != nil {
		return nil, fmt.Errorf("capsdk: marshal worker handshake: %w", err)
	}
	if len(data) > WorkerHandshakeMaxPacketSize {
		return nil, fmt.Errorf("%w: exceeds %d bytes", ErrWorkerHandshakePacket, WorkerHandshakeMaxPacketSize)
	}
	return data, nil
}

// UnmarshalWorkerTrustPacket bounds input and rejects unknown fields.
func UnmarshalWorkerTrustPacket(data []byte) (*agentv1.BusPacket, error) {
	if len(data) == 0 || len(data) > WorkerHandshakeMaxPacketSize {
		return nil, fmt.Errorf("%w: size must be 1..%d", ErrWorkerHandshakePacket, WorkerHandshakeMaxPacketSize)
	}
	packet := &agentv1.BusPacket{}
	if err := (proto.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, packet); err != nil {
		return nil, fmt.Errorf("capsdk: unmarshal worker handshake: %w", err)
	}
	if err := ValidateWorkerTrustPacket(packet); err != nil {
		return nil, err
	}
	return packet, nil
}
