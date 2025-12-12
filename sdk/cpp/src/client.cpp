#include "cap/client.h"

#include <google/protobuf/util/time_util.h>

namespace cap {

bool Client::Submit(const std::string& trace_id,
                    const coretex::agent::v1::JobRequest& req,
                    int32_t protocol_version) {
  if (!bus_) return false;

  coretex::agent::v1::BusPacket packet;
  packet.set_trace_id(trace_id);
  packet.set_sender_id(sender_id_);
  packet.mutable_created_at()->CopyFrom(google::protobuf::util::TimeUtil::GetCurrentTime());
  packet.set_protocol_version(protocol_version);
  *(packet.mutable_job_request()) = req;

  const auto size = packet.ByteSizeLong();
  std::vector<uint8_t> buffer(size);
  if (!packet.SerializeToArray(buffer.data(), static_cast<int>(buffer.size()))) {
    return false;
  }
  return bus_->Publish(kSubjectSubmit, buffer);
}

}  // namespace cap
