#pragma once

#include <functional>
#include <cstdint>
#include <string>
#include <vector>

namespace cap {

// Message wrapper for bus payloads.
struct BusMessage {
  std::string subject;
  std::vector<uint8_t> data;
};

// Subscription handle placeholder.
using SubscriptionHandle = void*;

// Generic bus interface; implement for your transport (NATS, Kafka, etc.).
class BusClient {
 public:
  using MsgHandler = std::function<void(const BusMessage&)>;

  virtual ~BusClient() = default;

  // Publish binary payload to a subject.
  virtual bool Publish(const std::string& subject, const std::vector<uint8_t>& data) = 0;

  // Subscribe to a subject with optional queue group; returns a handle or nullptr on failure.
  virtual SubscriptionHandle Subscribe(const std::string& subject,
                                       const std::string& queue_group,
                                       MsgHandler handler) = 0;
};

}  // namespace cap
