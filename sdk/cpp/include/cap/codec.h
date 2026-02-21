#pragma once

#include <cstdint>
#include <vector>

#include <google/protobuf/message.h>

#include "cordum/agent/v1/buspacket.pb.h"

namespace cap {

// MarshalDeterministic serializes a protobuf message with deterministic field ordering.
// Returns serialized bytes, or empty vector on failure.
std::vector<uint8_t> MarshalDeterministic(const google::protobuf::Message& msg);

// MarshalUnsignedForSignature serializes a BusPacket with the signature field cleared.
// Creates a copy, clears the signature field, then calls MarshalDeterministic.
// Used as the input to ECDSA signing/verification.
std::vector<uint8_t> MarshalUnsignedForSignature(
    const cordum::agent::v1::BusPacket& packet);

}  // namespace cap
