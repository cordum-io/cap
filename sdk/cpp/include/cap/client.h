#pragma once

#include <string>
#include <vector>

#include <openssl/evp.h>

#include "cap/bus_interface.h"
#include "cap/subjects.h"
#include "cordum/agent/v1/buspacket.pb.h"

namespace cap {

class Client {
 public:
  Client(BusClient* bus, std::string sender_id,
         EVP_PKEY* private_key = nullptr)
      : bus_(bus),
        sender_id_(std::move(sender_id)),
        private_key_(private_key) {}

  bool Submit(const std::string& trace_id,
              const cordum::agent::v1::JobRequest& req,
              int32_t protocol_version = 1);

 private:
  BusClient* bus_;
  std::string sender_id_;
  EVP_PKEY* private_key_;
};

}  // namespace cap
