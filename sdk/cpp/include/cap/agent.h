#pragma once

#include <atomic>
#include <map>
#include <memory>
#include <string>
#include <vector>

#include <openssl/evp.h>

#include "cap/bus_interface.h"
#include "cap/heartbeat.h"
#include "cap/metrics.h"
#include "cap/middleware.h"

namespace cap {

struct AgentConfig {
  BusClient* bus = nullptr;
  std::string sender_id;
  std::string pool;
  int max_parallel = 1;
  EVP_PKEY* private_key = nullptr;
  std::vector<std::pair<std::string, EVP_PKEY*>> public_keys;
  MetricsHook* metrics = nullptr;
  std::vector<Middleware> middlewares;
  float heartbeat_interval_sec = 5.0f;
};

class Agent {
 public:
  explicit Agent(AgentConfig config);
  ~Agent();

  Agent(const Agent&) = delete;
  Agent& operator=(const Agent&) = delete;

  // Register a job handler for a subject.
  void RegisterHandler(const std::string& subject, Handler handler);

  // Start subscribes to all registered subjects and starts heartbeat loop.
  bool Start();

  // Close stops heartbeat, unsubscribes, graceful shutdown.
  void Close();

 private:
  void OnMessage(const std::string& subject, const BusMessage& msg);

  AgentConfig config_;
  std::map<std::string, EVP_PKEY*> public_key_map_;
  std::map<std::string, Handler> handlers_;
  std::vector<SubscriptionHandle> subscriptions_;
  std::unique_ptr<HeartbeatLoop> heartbeat_loop_;
  std::atomic<int> active_jobs_{0};
  bool started_{false};
};

}  // namespace cap
