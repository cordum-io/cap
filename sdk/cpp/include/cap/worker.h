#pragma once

#include <functional>
#include <map>
#include <memory>
#include <string>

#include <openssl/evp.h>

#include "cap/bus_interface.h"
#include "cap/subjects.h"
#include "cordum/agent/v1/buspacket.pb.h"

namespace cap {

using JobHandler = std::function<std::unique_ptr<cordum::agent::v1::JobResult>(
    const cordum::agent::v1::JobRequest&)>;

class Worker {
 public:
  Worker(BusClient* bus,
         std::string subject,
         std::string sender_id,
         JobHandler handler,
         int32_t protocol_version = 1,
         EVP_PKEY* private_key = nullptr,
         std::map<std::string, EVP_PKEY*> public_keys = {})
      : bus_(bus),
        subject_(std::move(subject)),
        sender_id_(std::move(sender_id)),
        handler_(std::move(handler)),
        protocol_version_(protocol_version),
        private_key_(private_key),
        public_keys_(std::move(public_keys)) {}

  bool Start();

 private:
  void OnMessage(const BusMessage& msg);

  BusClient* bus_;
  std::string subject_;
  std::string sender_id_;
  JobHandler handler_;
  int32_t protocol_version_;
  EVP_PKEY* private_key_;
  std::map<std::string, EVP_PKEY*> public_keys_;
  SubscriptionHandle sub_{nullptr};
};

}  // namespace cap
