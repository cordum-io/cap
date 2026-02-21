#include "cap/agent.h"

#include <atomic>
#include <chrono>
#include <memory>
#include <string>
#include <thread>
#include <vector>

#include <gtest/gtest.h>

#include "cap/bus_interface.h"
#include "cap/codec.h"
#include "cap/metrics.h"
#include "cap/middleware.h"
#include "cordum/agent/v1/buspacket.pb.h"
#include "cordum/agent/v1/job.pb.h"

namespace {

class MockBus : public cap::BusClient {
 public:
  bool Publish(const std::string& subject, const std::vector<uint8_t>& data) override {
    last_publish_subject = subject;
    last_publish_data = data;
    publish_count.fetch_add(1, std::memory_order_relaxed);
    return true;
  }

  cap::SubscriptionHandle Subscribe(const std::string& subject, const std::string&,
                                     MsgHandler handler) override {
    handlers[subject] = std::move(handler);
    return reinterpret_cast<cap::SubscriptionHandle>(1);  // Non-null sentinel.
  }

  // Simulate delivering a message to a subscribed handler.
  void Deliver(const std::string& subject, const std::vector<uint8_t>& data) {
    auto it = handlers.find(subject);
    if (it != handlers.end()) {
      cap::BusMessage msg;
      msg.subject = subject;
      msg.data = data;
      it->second(msg);
    }
  }

  std::string last_publish_subject;
  std::vector<uint8_t> last_publish_data;
  std::atomic<int> publish_count{0};
  std::map<std::string, MsgHandler> handlers;
};

class RecordingMetrics : public cap::MetricsHook {
 public:
  void OnJobReceived(const std::string& job_id, const std::string& topic) override {
    received_jobs.push_back(job_id);
  }
  void OnJobCompleted(const std::string& job_id, int duration_ms, const std::string& status) override {
    completed_jobs.push_back(job_id);
  }
  void OnJobFailed(const std::string& job_id, const std::string& error_msg) override {
    failed_jobs.push_back(job_id);
  }
  void OnHeartbeatSent(const std::string& worker_id) override {
    heartbeat_count.fetch_add(1, std::memory_order_relaxed);
  }
  void OnProgressEmitted(const std::string& job_id, int percent) override {
    progress_count.fetch_add(1, std::memory_order_relaxed);
  }
  void OnError(const std::string& context, const std::string& error_msg) override {
    errors.push_back(context + ": " + error_msg);
  }

  std::vector<std::string> received_jobs;
  std::vector<std::string> completed_jobs;
  std::vector<std::string> failed_jobs;
  std::vector<std::string> errors;
  std::atomic<int> heartbeat_count{0};
  std::atomic<int> progress_count{0};
};

std::vector<uint8_t> MakeJobPacket(const std::string& job_id, const std::string& topic) {
  cordum::agent::v1::BusPacket packet;
  packet.set_trace_id("trace-1");
  packet.set_sender_id("client-1");
  packet.set_protocol_version(1);
  auto* req = packet.mutable_job_request();
  req->set_job_id(job_id);
  req->set_topic(topic);
  return cap::MarshalDeterministic(packet);
}

}  // namespace

TEST(AgentTest, StartAndStopWithoutCrash) {
  MockBus bus;
  cap::AgentConfig config;
  config.bus = &bus;
  config.sender_id = "agent-1";
  config.pool = "test-pool";
  config.heartbeat_interval_sec = 60.0f;  // Long interval to avoid timing noise.

  cap::Agent agent(config);
  agent.RegisterHandler("job.echo", [](const cap::Context&, const cordum::agent::v1::JobRequest& req)
                                         -> std::unique_ptr<cordum::agent::v1::JobResult> {
    auto r = std::make_unique<cordum::agent::v1::JobResult>();
    r->set_job_id(req.job_id());
    r->set_status(cordum::agent::v1::JOB_STATUS_SUCCEEDED);
    return r;
  });

  ASSERT_TRUE(agent.Start());
  agent.Close();
}

TEST(AgentTest, DispatchesJobToRegisteredHandler) {
  MockBus bus;
  cap::AgentConfig config;
  config.bus = &bus;
  config.sender_id = "agent-1";
  config.pool = "test";
  config.heartbeat_interval_sec = 60.0f;

  bool handler_called = false;

  cap::Agent agent(config);
  agent.RegisterHandler("job.echo", [&handler_called](const cap::Context& ctx,
                                                       const cordum::agent::v1::JobRequest& req)
                                         -> std::unique_ptr<cordum::agent::v1::JobResult> {
    handler_called = true;
    EXPECT_EQ(req.job_id(), "job-42");
    EXPECT_EQ(ctx.trace_id, "trace-1");
    auto r = std::make_unique<cordum::agent::v1::JobResult>();
    r->set_job_id(req.job_id());
    r->set_status(cordum::agent::v1::JOB_STATUS_SUCCEEDED);
    return r;
  });

  ASSERT_TRUE(agent.Start());

  auto data = MakeJobPacket("job-42", "echo");
  bus.Deliver("job.echo", data);

  EXPECT_TRUE(handler_called);
  EXPECT_EQ(bus.last_publish_subject, "sys.job.result");
  agent.Close();
}

TEST(AgentTest, MiddlewareChainIsApplied) {
  MockBus bus;
  std::vector<int> order;

  auto mw = [&order](cap::Handler next) -> cap::Handler {
    return [&order, next = std::move(next)](const cap::Context& ctx,
                                            const cordum::agent::v1::JobRequest& req)
               -> std::unique_ptr<cordum::agent::v1::JobResult> {
      order.push_back(1);
      return next(ctx, req);
    };
  };

  cap::AgentConfig config;
  config.bus = &bus;
  config.sender_id = "agent-1";
  config.pool = "test";
  config.heartbeat_interval_sec = 60.0f;
  config.middlewares = {mw};

  cap::Agent agent(config);
  agent.RegisterHandler("job.echo", [&order](const cap::Context&,
                                              const cordum::agent::v1::JobRequest& req)
                                         -> std::unique_ptr<cordum::agent::v1::JobResult> {
    order.push_back(2);
    auto r = std::make_unique<cordum::agent::v1::JobResult>();
    r->set_job_id(req.job_id());
    r->set_status(cordum::agent::v1::JOB_STATUS_SUCCEEDED);
    return r;
  });

  ASSERT_TRUE(agent.Start());
  bus.Deliver("job.echo", MakeJobPacket("j-1", "echo"));

  ASSERT_EQ(order.size(), 2u);
  EXPECT_EQ(order[0], 1);
  EXPECT_EQ(order[1], 2);
  agent.Close();
}

TEST(AgentTest, MetricsHooksFire) {
  MockBus bus;
  RecordingMetrics metrics;

  cap::AgentConfig config;
  config.bus = &bus;
  config.sender_id = "agent-1";
  config.pool = "test";
  config.heartbeat_interval_sec = 60.0f;
  config.metrics = &metrics;

  cap::Agent agent(config);
  agent.RegisterHandler("job.echo", [](const cap::Context&,
                                        const cordum::agent::v1::JobRequest& req)
                                         -> std::unique_ptr<cordum::agent::v1::JobResult> {
    auto r = std::make_unique<cordum::agent::v1::JobResult>();
    r->set_job_id(req.job_id());
    r->set_status(cordum::agent::v1::JOB_STATUS_SUCCEEDED);
    return r;
  });

  ASSERT_TRUE(agent.Start());
  bus.Deliver("job.echo", MakeJobPacket("j-1", "echo"));

  ASSERT_EQ(metrics.received_jobs.size(), 1u);
  EXPECT_EQ(metrics.received_jobs[0], "j-1");
  ASSERT_EQ(metrics.completed_jobs.size(), 1u);
  EXPECT_EQ(metrics.completed_jobs[0], "j-1");

  agent.Close();
}

TEST(AgentTest, HeartbeatLoopStartsOnStart) {
  MockBus bus;
  RecordingMetrics metrics;

  cap::AgentConfig config;
  config.bus = &bus;
  config.sender_id = "agent-1";
  config.pool = "test";
  config.heartbeat_interval_sec = 0.1f;  // 100ms interval.
  config.metrics = &metrics;

  cap::Agent agent(config);
  agent.RegisterHandler("job.echo", [](const cap::Context&,
                                        const cordum::agent::v1::JobRequest& req)
                                         -> std::unique_ptr<cordum::agent::v1::JobResult> {
    auto r = std::make_unique<cordum::agent::v1::JobResult>();
    r->set_job_id(req.job_id());
    return r;
  });

  ASSERT_TRUE(agent.Start());
  std::this_thread::sleep_for(std::chrono::milliseconds(350));
  agent.Close();

  // Should have sent at least 2 heartbeats in 350ms with 100ms interval.
  EXPECT_GE(metrics.heartbeat_count.load(), 2);
}
