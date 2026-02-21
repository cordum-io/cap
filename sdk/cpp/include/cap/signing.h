#pragma once

#include <string>
#include <vector>

#include <openssl/evp.h>

#include "cordum/agent/v1/buspacket.pb.h"

namespace cap {

// SignPacket signs a BusPacket using ECDSA P-256 with SHA-256.
// Clones the packet, clears the signature field, serializes deterministically,
// SHA-256 hashes, ECDSA signs in ASN.1/DER format, stores on original packet.
// Returns true on success.
bool SignPacket(cordum::agent::v1::BusPacket* packet, EVP_PKEY* private_key);

// VerifyPacketSignature verifies a BusPacket's ECDSA signature.
// Extracts signature, clones packet, clears signature, serializes deterministically,
// SHA-256 hashes, verifies ASN.1/DER signature against public key.
// Returns true if signature is valid.
bool VerifyPacketSignature(const cordum::agent::v1::BusPacket& packet,
                           EVP_PKEY* public_key);

// LoadPrivateKey loads a PEM-encoded EC private key.
// Returns nullptr on failure. Caller owns the returned key.
EVP_PKEY* LoadPrivateKey(const std::string& pem);

// LoadPublicKey loads a PEM-encoded EC public key.
// Returns nullptr on failure. Caller owns the returned key.
EVP_PKEY* LoadPublicKey(const std::string& pem);

}  // namespace cap
