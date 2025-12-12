#pragma once

#include <string>
#include <vector>

#include "cap/bus_interface.h"
#include "cap/subjects.h"
#include "coretex/agent/v1/buspacket.pb.h"

namespace cap {

class Client {
 public:
  Client(BusClient* bus, std::string sender_id)
      : bus_(bus), sender_id_(std::move(sender_id)) {}

  bool Submit(const std::string& trace_id,
              const coretex::agent::v1::JobRequest& req,
              int32_t protocol_version = 1);

 private:
  BusClient* bus_;
  std::string sender_id_;
};

}  // namespace cap
