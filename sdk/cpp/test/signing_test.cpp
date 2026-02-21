#include "cap/signing.h"

#include <cstring>
#include <string>

#include <gtest/gtest.h>
#include <openssl/evp.h>
#include <openssl/ec.h>
#include <openssl/pem.h>
#include <openssl/bio.h>

#include "cordum/agent/v1/buspacket.pb.h"
#include "cordum/agent/v1/job.pb.h"

namespace {

// Helper: generate an EC P-256 key pair and export as PEM strings.
struct KeyPair {
  std::string private_pem;
  std::string public_pem;
};

KeyPair GenerateKeyPair() {
  KeyPair kp;
  EVP_PKEY_CTX* ctx = EVP_PKEY_CTX_new_id(EVP_PKEY_EC, nullptr);
  EVP_PKEY_keygen_init(ctx);
  EVP_PKEY_CTX_set_ec_paramgen_curve_nid(ctx, NID_X9_62_prime256v1);
  EVP_PKEY* pkey = nullptr;
  EVP_PKEY_keygen(ctx, &pkey);
  EVP_PKEY_CTX_free(ctx);

  // Private PEM
  BIO* bio = BIO_new(BIO_s_mem());
  PEM_write_bio_PrivateKey(bio, pkey, nullptr, nullptr, 0, nullptr, nullptr);
  char* buf = nullptr;
  long len = BIO_get_mem_data(bio, &buf);
  kp.private_pem.assign(buf, len);
  BIO_free(bio);

  // Public PEM
  bio = BIO_new(BIO_s_mem());
  PEM_write_bio_PUBKEY(bio, pkey);
  len = BIO_get_mem_data(bio, &buf);
  kp.public_pem.assign(buf, len);
  BIO_free(bio);

  EVP_PKEY_free(pkey);
  return kp;
}

cordum::agent::v1::BusPacket MakeTestPacket() {
  cordum::agent::v1::BusPacket packet;
  packet.set_trace_id("test-trace");
  packet.set_sender_id("test-sender");
  packet.set_protocol_version(1);
  auto* req = packet.mutable_job_request();
  req->set_job_id("job-1");
  req->set_topic("test.topic");
  return packet;
}

}  // namespace

TEST(SigningTest, LoadKeys) {
  auto kp = GenerateKeyPair();

  EVP_PKEY* priv = cap::LoadPrivateKey(kp.private_pem);
  ASSERT_NE(priv, nullptr);
  EVP_PKEY_free(priv);

  EVP_PKEY* pub = cap::LoadPublicKey(kp.public_pem);
  ASSERT_NE(pub, nullptr);
  EVP_PKEY_free(pub);
}

TEST(SigningTest, LoadInvalidKey) {
  EVP_PKEY* priv = cap::LoadPrivateKey("not a pem");
  EXPECT_EQ(priv, nullptr);

  EVP_PKEY* pub = cap::LoadPublicKey("not a pem");
  EXPECT_EQ(pub, nullptr);
}

TEST(SigningTest, SignAndVerifyRoundTrip) {
  auto kp = GenerateKeyPair();
  EVP_PKEY* priv = cap::LoadPrivateKey(kp.private_pem);
  EVP_PKEY* pub = cap::LoadPublicKey(kp.public_pem);
  ASSERT_NE(priv, nullptr);
  ASSERT_NE(pub, nullptr);

  auto packet = MakeTestPacket();
  EXPECT_TRUE(packet.signature().empty());

  ASSERT_TRUE(cap::SignPacket(&packet, priv));
  EXPECT_FALSE(packet.signature().empty());

  EXPECT_TRUE(cap::VerifyPacketSignature(packet, pub));

  EVP_PKEY_free(priv);
  EVP_PKEY_free(pub);
}

TEST(SigningTest, TamperDetection) {
  auto kp = GenerateKeyPair();
  EVP_PKEY* priv = cap::LoadPrivateKey(kp.private_pem);
  EVP_PKEY* pub = cap::LoadPublicKey(kp.public_pem);

  auto packet = MakeTestPacket();
  ASSERT_TRUE(cap::SignPacket(&packet, priv));

  // Tamper with the sender_id
  packet.set_sender_id("tampered-sender");
  EXPECT_FALSE(cap::VerifyPacketSignature(packet, pub));

  EVP_PKEY_free(priv);
  EVP_PKEY_free(pub);
}

TEST(SigningTest, InvalidSignatureRejection) {
  auto kp = GenerateKeyPair();
  EVP_PKEY* pub = cap::LoadPublicKey(kp.public_pem);

  auto packet = MakeTestPacket();
  packet.set_signature("garbage-bytes-not-a-real-signature");

  EXPECT_FALSE(cap::VerifyPacketSignature(packet, pub));

  EVP_PKEY_free(pub);
}

TEST(SigningTest, MissingSignature) {
  auto kp = GenerateKeyPair();
  EVP_PKEY* pub = cap::LoadPublicKey(kp.public_pem);

  auto packet = MakeTestPacket();
  // No signature set
  EXPECT_FALSE(cap::VerifyPacketSignature(packet, pub));

  EVP_PKEY_free(pub);
}

TEST(SigningTest, NullChecks) {
  auto kp = GenerateKeyPair();
  EVP_PKEY* priv = cap::LoadPrivateKey(kp.private_pem);
  EVP_PKEY* pub = cap::LoadPublicKey(kp.public_pem);

  EXPECT_FALSE(cap::SignPacket(nullptr, priv));

  auto packet = MakeTestPacket();
  EXPECT_FALSE(cap::SignPacket(&packet, nullptr));

  EXPECT_FALSE(cap::VerifyPacketSignature(packet, nullptr));

  EVP_PKEY_free(priv);
  EVP_PKEY_free(pub);
}

TEST(SigningTest, DifferentKeyRejection) {
  auto kp1 = GenerateKeyPair();
  auto kp2 = GenerateKeyPair();
  EVP_PKEY* priv1 = cap::LoadPrivateKey(kp1.private_pem);
  EVP_PKEY* pub2 = cap::LoadPublicKey(kp2.public_pem);

  auto packet = MakeTestPacket();
  ASSERT_TRUE(cap::SignPacket(&packet, priv1));

  // Verify with a different key should fail
  EXPECT_FALSE(cap::VerifyPacketSignature(packet, pub2));

  EVP_PKEY_free(priv1);
  EVP_PKEY_free(pub2);
}
