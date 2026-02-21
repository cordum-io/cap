#include "cap/heartbeat.h"

#include <atomic>
#include <chrono>
#include <string>
#include <thread>
#include <vector>

#include <gtest/gtest.h>

#include "cap/bus_interface.h"
#include "cordum/agent/v1/buspacket.pb.h"
#include "cordum/agent/v1/heartbeat.pb.h"

namespace {

class MockBus : public cap::BusClient {
 public:
  bool Publish(const std::string& subject, const std::vector<uint8_t>& data) override {
    publish_count.fetch_add(1, std::memory_order_relaxed);
    last_subject = subject;
    last_data = data;
    return true;
  }

  cap::SubscriptionHandle Subscribe(const std::string&, const std::string&,
                                     MsgHandler) override {
    return nullptr;
  }

  std::atomic<int> publish_count{0};
  std::string last_subject;
  std::vector<uint8_t> last_data;
};

}  // namespace

TEST(HeartbeatTest, HeartbeatPayloadProducesValidPacket) {
  auto bytes = cap::HeartbeatPayload("worker-1", "pool-a", 3, 8, 42.5f);
  ASSERT_FALSE(bytes.empty());

  cordum::agent::v1::BusPacket packet;
  ASSERT_TRUE(packet.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  EXPECT_EQ(packet.sender_id(), "worker-1");
  EXPECT_EQ(packet.protocol_version(), 1);
  ASSERT_TRUE(packet.has_heartbeat());

  const auto& hb = packet.heartbeat();
  EXPECT_EQ(hb.worker_id(), "worker-1");
  EXPECT_EQ(hb.pool(), "pool-a");
  EXPECT_EQ(hb.active_jobs(), 3);
  EXPECT_EQ(hb.max_parallel_jobs(), 8);
  EXPECT_FLOAT_EQ(hb.cpu_load(), 42.5f);
}

TEST(HeartbeatTest, WithMemorySetsMemoryLoad) {
  auto bytes = cap::HeartbeatPayloadWithMemory("w-1", "pool", 1, 4, 10.0f, 55.0f);
  ASSERT_FALSE(bytes.empty());

  cordum::agent::v1::BusPacket packet;
  ASSERT_TRUE(packet.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  ASSERT_TRUE(packet.has_heartbeat());

  EXPECT_FLOAT_EQ(packet.heartbeat().memory_load(), 55.0f);
  EXPECT_EQ(packet.heartbeat().progress_pct(), 0);
  EXPECT_TRUE(packet.heartbeat().last_memo().empty());
}

TEST(HeartbeatTest, WithProgressSetsAllFields) {
  auto bytes = cap::HeartbeatPayloadWithProgress(
      "w-2", "gpu-pool", 5, 10, 80.0f, 60.0f, 75, "step-3-done");
  ASSERT_FALSE(bytes.empty());

  cordum::agent::v1::BusPacket packet;
  ASSERT_TRUE(packet.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  ASSERT_TRUE(packet.has_heartbeat());

  const auto& hb = packet.heartbeat();
  EXPECT_EQ(hb.worker_id(), "w-2");
  EXPECT_EQ(hb.pool(), "gpu-pool");
  EXPECT_EQ(hb.active_jobs(), 5);
  EXPECT_EQ(hb.max_parallel_jobs(), 10);
  EXPECT_FLOAT_EQ(hb.cpu_load(), 80.0f);
  EXPECT_FLOAT_EQ(hb.memory_load(), 60.0f);
  EXPECT_EQ(hb.progress_pct(), 75);
  EXPECT_EQ(hb.last_memo(), "step-3-done");
}

TEST(HeartbeatTest, EmitHeartbeatPublishesToCorrectSubject) {
  MockBus bus;
  auto payload = cap::HeartbeatPayload("w-1", "pool", 0, 1, 0.0f);
  ASSERT_TRUE(cap::EmitHeartbeat(&bus, payload));
  EXPECT_EQ(bus.last_subject, "sys.heartbeat");
  EXPECT_EQ(bus.publish_count.load(), 1);
}

TEST(HeartbeatTest, EmitHeartbeatRejectNullBus) {
  std::vector<uint8_t> payload = {1, 2, 3};
  EXPECT_FALSE(cap::EmitHeartbeat(nullptr, payload));
}

TEST(HeartbeatTest, HeartbeatLoopTicksAndStops) {
  MockBus bus;
  cap::HeartbeatLoop loop(
      &bus,
      []() { return cap::HeartbeatPayload("w-1", "pool", 0, 1, 0.0f); },
      "w-1",
      std::chrono::milliseconds(100));

  // Wait enough time for a few ticks.
  std::this_thread::sleep_for(std::chrono::milliseconds(350));
  loop.Stop();

  // Should have published at least 2 heartbeats (immediate + after 100ms + after 200ms).
  EXPECT_GE(bus.publish_count.load(), 2);
}

TEST(HeartbeatTest, HeartbeatLoopStopIsIdempotent) {
  MockBus bus;
  cap::HeartbeatLoop loop(
      &bus,
      []() { return cap::HeartbeatPayload("w-1", "pool", 0, 1, 0.0f); },
      "w-1",
      std::chrono::milliseconds(5000));

  loop.Stop();
  loop.Stop();  // Second stop should not crash.
}
