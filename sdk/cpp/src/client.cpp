#include "cap/client.h"
#include "cap/codec.h"
#include "cap/signing.h"

#include <google/protobuf/util/time_util.h>

namespace cap {

bool Client::Submit(const std::string& trace_id,
                    const cordum::agent::v1::JobRequest& req,
                    int32_t protocol_version) {
  if (!bus_) return false;

  cordum::agent::v1::BusPacket packet;
  packet.set_trace_id(trace_id);
  packet.set_sender_id(sender_id_);
  packet.mutable_created_at()->CopyFrom(google::protobuf::util::TimeUtil::GetCurrentTime());
  packet.set_protocol_version(protocol_version);
  *(packet.mutable_job_request()) = req;

  if (private_key_) {
    if (!SignPacket(&packet, private_key_)) return false;
  }

  auto buffer = MarshalDeterministic(packet);
  if (buffer.empty() && packet.ByteSizeLong() > 0) return false;
  return bus_->Publish(kSubjectSubmit, buffer);
}

}  // namespace cap
