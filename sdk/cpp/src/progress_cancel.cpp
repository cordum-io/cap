#include "cap/progress_cancel.h"
#include "cap/codec.h"
#include "cap/subjects.h"

#include <google/protobuf/util/time_util.h>

#include "cordum/agent/v1/buspacket.pb.h"
#include "cordum/agent/v1/job.pb.h"

namespace cap {

std::vector<uint8_t> ProgressPayload(
    const std::string& sender_id,
    const std::string& job_id,
    const std::string& step_id,
    int32_t percent,
    const std::string& message) {
  cordum::agent::v1::BusPacket packet;
  packet.set_sender_id(sender_id);
  packet.set_protocol_version(1);
  packet.mutable_created_at()->CopyFrom(
      google::protobuf::util::TimeUtil::GetCurrentTime());

  auto* progress = packet.mutable_job_progress();
  progress->set_job_id(job_id);
  progress->set_step_id(step_id);
  progress->set_percent(percent);
  progress->set_message(message);

  return MarshalDeterministic(packet);
}

std::vector<uint8_t> CancelPayload(
    const std::string& sender_id,
    const std::string& job_id,
    const std::string& reason,
    const std::string& requested_by) {
  cordum::agent::v1::BusPacket packet;
  packet.set_sender_id(sender_id);
  packet.set_protocol_version(1);
  packet.mutable_created_at()->CopyFrom(
      google::protobuf::util::TimeUtil::GetCurrentTime());

  auto* cancel = packet.mutable_job_cancel();
  cancel->set_job_id(job_id);
  cancel->set_reason(reason);
  cancel->set_requested_by(requested_by);

  return MarshalDeterministic(packet);
}

bool EmitProgress(BusClient* bus, const std::vector<uint8_t>& payload) {
  if (!bus) return false;
  return bus->Publish(kSubjectProgress, payload);
}

bool EmitCancel(BusClient* bus, const std::vector<uint8_t>& payload) {
  if (!bus) return false;
  return bus->Publish(kSubjectCancel, payload);
}

}  // namespace cap
