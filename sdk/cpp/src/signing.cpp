#include "cap/signing.h"
#include "cap/codec.h"

#include <cstring>
#include <string>
#include <vector>

#include <openssl/bio.h>
#include <openssl/evp.h>
#include <openssl/pem.h>

namespace cap {

bool SignPacket(cordum::agent::v1::BusPacket* packet, EVP_PKEY* private_key) {
  if (!packet || !private_key) return false;

  auto unsigned_bytes = MarshalUnsignedForSignature(*packet);
  if (unsigned_bytes.empty() && packet->ByteSizeLong() > 0) return false;

  // SHA-256 hash
  unsigned char hash[EVP_MAX_MD_SIZE];
  unsigned int hash_len = 0;
  if (EVP_Digest(unsigned_bytes.data(), unsigned_bytes.size(), hash, &hash_len,
                 EVP_sha256(), nullptr) != 1) {
    return false;
  }

  // ECDSA sign the hash
  EVP_PKEY_CTX* ctx = EVP_PKEY_CTX_new(private_key, nullptr);
  if (!ctx) return false;

  bool ok = false;
  if (EVP_PKEY_sign_init(ctx) == 1) {
    size_t sig_len = 0;
    if (EVP_PKEY_sign(ctx, nullptr, &sig_len, hash, hash_len) == 1) {
      std::vector<unsigned char> sig(sig_len);
      if (EVP_PKEY_sign(ctx, sig.data(), &sig_len, hash, hash_len) == 1) {
        packet->set_signature(sig.data(), sig_len);
        ok = true;
      }
    }
  }

  EVP_PKEY_CTX_free(ctx);
  return ok;
}

bool VerifyPacketSignature(const cordum::agent::v1::BusPacket& packet,
                           EVP_PKEY* public_key) {
  if (!public_key) return false;

  const std::string& sig = packet.signature();
  if (sig.empty()) return false;

  auto unsigned_bytes = MarshalUnsignedForSignature(packet);

  // SHA-256 hash
  unsigned char hash[EVP_MAX_MD_SIZE];
  unsigned int hash_len = 0;
  if (EVP_Digest(unsigned_bytes.data(), unsigned_bytes.size(), hash, &hash_len,
                 EVP_sha256(), nullptr) != 1) {
    return false;
  }

  // ECDSA verify
  EVP_PKEY_CTX* ctx = EVP_PKEY_CTX_new(public_key, nullptr);
  if (!ctx) return false;

  bool ok = false;
  if (EVP_PKEY_verify_init(ctx) == 1) {
    ok = EVP_PKEY_verify(ctx, reinterpret_cast<const unsigned char*>(sig.data()),
                         sig.size(), hash, hash_len) == 1;
  }

  EVP_PKEY_CTX_free(ctx);
  return ok;
}

EVP_PKEY* LoadPrivateKey(const std::string& pem) {
  BIO* bio = BIO_new_mem_buf(pem.data(), static_cast<int>(pem.size()));
  if (!bio) return nullptr;

  EVP_PKEY* key = PEM_read_bio_PrivateKey(bio, nullptr, nullptr, nullptr);
  BIO_free(bio);
  return key;
}

EVP_PKEY* LoadPublicKey(const std::string& pem) {
  BIO* bio = BIO_new_mem_buf(pem.data(), static_cast<int>(pem.size()));
  if (!bio) return nullptr;

  EVP_PKEY* key = PEM_read_bio_PUBKEY(bio, nullptr, nullptr, nullptr);
  BIO_free(bio);
  return key;
}

}  // namespace cap
