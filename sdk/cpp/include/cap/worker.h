#pragma once

#include <functional>
#include <memory>
#include <string>

#include "cap/bus_interface.h"
#include "cap/subjects.h"
#include "coretex/agent/v1/buspacket.pb.h"

namespace cap {

using JobHandler = std::function<std::unique_ptr<coretex::agent::v1::JobResult>(
    const coretex::agent::v1::JobRequest&)>;

class Worker {
 public:
  Worker(BusClient* bus,
         std::string subject,
         std::string sender_id,
         JobHandler handler,
         int32_t protocol_version = 1)
      : bus_(bus),
        subject_(std::move(subject)),
        sender_id_(std::move(sender_id)),
        handler_(std::move(handler)),
        protocol_version_(protocol_version) {}

  bool Start();

 private:
  void OnMessage(const BusMessage& msg);

  BusClient* bus_;
  std::string subject_;
  std::string sender_id_;
  JobHandler handler_;
  int32_t protocol_version_;
  SubscriptionHandle sub_{nullptr};
};

}  // namespace cap
