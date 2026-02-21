#include "cap/worker.h"
#include "cap/codec.h"
#include "cap/signing.h"

#include <google/protobuf/util/time_util.h>

namespace cap {

bool Worker::Start() {
  if (!bus_) return false;
  sub_ = bus_->Subscribe(subject_, subject_, [this](const BusMessage& msg) {
    this->OnMessage(msg);
  });
  return sub_ != nullptr;
}

void Worker::OnMessage(const BusMessage& msg) {
  cordum::agent::v1::BusPacket packet;
  if (!packet.ParseFromArray(msg.data.data(), static_cast<int>(msg.data.size()))) {
    return;
  }

  // Verify signature if public keys are configured.
  if (!public_keys_.empty()) {
    const std::string& sender = packet.sender_id();
    auto it = public_keys_.find(sender);
    if (it == public_keys_.end()) return;
    if (packet.signature().empty()) return;
    if (!VerifyPacketSignature(packet, it->second)) return;
  }

  if (!packet.has_job_request()) {
    return;
  }

  const auto& req = packet.job_request();
  std::unique_ptr<cordum::agent::v1::JobResult> res;
  if (handler_) {
    res = handler_(req);
  }
  if (!res) {
    res = std::make_unique<cordum::agent::v1::JobResult>();
    res->set_job_id(req.job_id());
    res->set_status(cordum::agent::v1::JOB_STATUS_FAILED);
    res->set_error_message("handler returned null");
  }
  if (res->job_id().empty()) res->set_job_id(req.job_id());
  if (res->worker_id().empty()) res->set_worker_id(sender_id_);

  cordum::agent::v1::BusPacket out;
  out.set_trace_id(packet.trace_id());
  out.set_sender_id(sender_id_);
  out.set_protocol_version(protocol_version_);
  out.mutable_created_at()->CopyFrom(google::protobuf::util::TimeUtil::GetCurrentTime());
  *(out.mutable_job_result()) = *res;

  if (private_key_) {
    if (!SignPacket(&out, private_key_)) return;
  }

  auto buffer = MarshalDeterministic(out);
  if (buffer.empty() && out.ByteSizeLong() > 0) return;
  bus_->Publish(kSubjectResult, buffer);
}

}  // namespace cap
