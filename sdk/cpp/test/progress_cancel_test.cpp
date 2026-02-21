#include "cap/progress_cancel.h"

#include <atomic>
#include <string>
#include <vector>

#include <gtest/gtest.h>

#include "cap/bus_interface.h"
#include "cordum/agent/v1/buspacket.pb.h"
#include "cordum/agent/v1/job.pb.h"

namespace {

class MockBus : public cap::BusClient {
 public:
  bool Publish(const std::string& subject, const std::vector<uint8_t>& data) override {
    last_subject = subject;
    last_data = data;
    return true;
  }

  cap::SubscriptionHandle Subscribe(const std::string&, const std::string&,
                                     MsgHandler) override {
    return nullptr;
  }

  std::string last_subject;
  std::vector<uint8_t> last_data;
};

}  // namespace

TEST(ProgressCancelTest, ProgressPayloadProducesValidPacket) {
  auto bytes = cap::ProgressPayload("sender-1", "job-42", "step-3", 75, "processing");
  ASSERT_FALSE(bytes.empty());

  cordum::agent::v1::BusPacket packet;
  ASSERT_TRUE(packet.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  EXPECT_EQ(packet.sender_id(), "sender-1");
  EXPECT_EQ(packet.protocol_version(), 1);
  ASSERT_TRUE(packet.has_job_progress());

  const auto& p = packet.job_progress();
  EXPECT_EQ(p.job_id(), "job-42");
  EXPECT_EQ(p.step_id(), "step-3");
  EXPECT_EQ(p.percent(), 75);
  EXPECT_EQ(p.message(), "processing");
}

TEST(ProgressCancelTest, ProgressPayloadZeroPercent) {
  auto bytes = cap::ProgressPayload("s", "j", "st", 0, "");
  ASSERT_FALSE(bytes.empty());

  cordum::agent::v1::BusPacket packet;
  ASSERT_TRUE(packet.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  ASSERT_TRUE(packet.has_job_progress());
  EXPECT_EQ(packet.job_progress().percent(), 0);
}

TEST(ProgressCancelTest, CancelPayloadProducesValidPacket) {
  auto bytes = cap::CancelPayload("sender-1", "job-99", "timeout", "admin");
  ASSERT_FALSE(bytes.empty());

  cordum::agent::v1::BusPacket packet;
  ASSERT_TRUE(packet.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
  EXPECT_EQ(packet.sender_id(), "sender-1");
  ASSERT_TRUE(packet.has_job_cancel());

  const auto& c = packet.job_cancel();
  EXPECT_EQ(c.job_id(), "job-99");
  EXPECT_EQ(c.reason(), "timeout");
  EXPECT_EQ(c.requested_by(), "admin");
}

TEST(ProgressCancelTest, EmitProgressPublishesToCorrectSubject) {
  MockBus bus;
  auto payload = cap::ProgressPayload("s", "j", "st", 50, "half");
  ASSERT_TRUE(cap::EmitProgress(&bus, payload));
  EXPECT_EQ(bus.last_subject, "sys.job.progress");
}

TEST(ProgressCancelTest, EmitCancelPublishesToCorrectSubject) {
  MockBus bus;
  auto payload = cap::CancelPayload("s", "j", "reason", "user");
  ASSERT_TRUE(cap::EmitCancel(&bus, payload));
  EXPECT_EQ(bus.last_subject, "sys.job.cancel");
}

TEST(ProgressCancelTest, EmitProgressRejectsNullBus) {
  std::vector<uint8_t> payload = {1};
  EXPECT_FALSE(cap::EmitProgress(nullptr, payload));
}

TEST(ProgressCancelTest, EmitCancelRejectsNullBus) {
  std::vector<uint8_t> payload = {1};
  EXPECT_FALSE(cap::EmitCancel(nullptr, payload));
}
