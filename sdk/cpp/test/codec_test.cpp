#include "cap/codec.h"

#include <string>
#include <vector>

#include <gtest/gtest.h>

#include "cordum/agent/v1/buspacket.pb.h"
#include "cordum/agent/v1/job.pb.h"
#include "cordum/agent/v1/heartbeat.pb.h"

namespace {

cordum::agent::v1::BusPacket MakePacket() {
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

TEST(CodecTest, DeterministicSerializationIsStable) {
  auto packet = MakePacket();

  auto first = cap::MarshalDeterministic(packet);
  ASSERT_FALSE(first.empty());

  for (int i = 0; i < 100; ++i) {
    auto next = cap::MarshalDeterministic(packet);
    ASSERT_EQ(first.size(), next.size()) << "iteration " << i;
    EXPECT_EQ(first, next) << "iteration " << i;
  }
}

TEST(CodecTest, DeterministicHeartbeatIsStable) {
  cordum::agent::v1::BusPacket packet;
  packet.set_sender_id("worker-1");
  packet.set_protocol_version(1);
  auto* hb = packet.mutable_heartbeat();
  hb->set_worker_id("worker-1");
  hb->set_pool("pool-a");
  hb->set_active_jobs(3);
  hb->set_max_parallel_jobs(8);
  hb->set_cpu_load(42.5);
  hb->set_memory_load(55.0);
  hb->set_progress_pct(67);
  hb->set_last_memo("checkpoint");

  auto first = cap::MarshalDeterministic(packet);
  ASSERT_FALSE(first.empty());

  for (int i = 0; i < 50; ++i) {
    EXPECT_EQ(first, cap::MarshalDeterministic(packet));
  }
}

TEST(CodecTest, MarshalUnsignedForSignatureStripsSignature) {
  auto packet = MakePacket();
  packet.set_signature("fake-sig-data");

  auto bytes = cap::MarshalUnsignedForSignature(packet);
  ASSERT_FALSE(bytes.empty());

  // Deserialize and verify signature is gone
  cordum::agent::v1::BusPacket decoded;
  ASSERT_TRUE(decoded.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  EXPECT_TRUE(decoded.signature().empty());
  EXPECT_EQ(decoded.sender_id(), "test-sender");
  EXPECT_EQ(decoded.trace_id(), "test-trace");
  EXPECT_EQ(decoded.protocol_version(), 1);
  EXPECT_EQ(decoded.job_request().job_id(), "job-1");
}

TEST(CodecTest, MarshalUnsignedPreservesAllOtherFields) {
  auto packet = MakePacket();
  packet.set_signature("to-be-stripped");

  auto without_sig = cap::MarshalUnsignedForSignature(packet);

  // Also serialize a copy with no signature
  cordum::agent::v1::BusPacket clean;
  clean.CopyFrom(packet);
  clean.clear_signature();
  auto clean_bytes = cap::MarshalDeterministic(clean);

  EXPECT_EQ(without_sig, clean_bytes);
}

TEST(CodecTest, EmptyMessage) {
  cordum::agent::v1::BusPacket empty;
  auto bytes = cap::MarshalDeterministic(empty);
  // Empty protobuf message serializes to zero bytes
  EXPECT_TRUE(bytes.empty());
}

TEST(CodecTest, RoundTrip) {
  auto packet = MakePacket();
  auto bytes = cap::MarshalDeterministic(packet);
  ASSERT_FALSE(bytes.empty());

  cordum::agent::v1::BusPacket decoded;
  ASSERT_TRUE(decoded.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  EXPECT_EQ(decoded.trace_id(), packet.trace_id());
  EXPECT_EQ(decoded.sender_id(), packet.sender_id());
  EXPECT_EQ(decoded.protocol_version(), packet.protocol_version());
  EXPECT_EQ(decoded.job_request().job_id(), packet.job_request().job_id());
  EXPECT_EQ(decoded.job_request().topic(), packet.job_request().topic());
}
