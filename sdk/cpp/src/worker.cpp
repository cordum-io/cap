#include "cap/worker.h"

namespace cap {

bool Worker::Start() {
  if (!bus_) return false;
  sub_ = bus_->Subscribe(subject_, subject_, [this](const BusMessage& msg) {
    this->OnMessage(msg);
  });
  return sub_ != nullptr;
}

void Worker::OnMessage(const BusMessage& msg) {
  coretex::agent::v1::BusPacket packet;
  if (!packet.ParseFromArray(msg.data.data(), static_cast<int>(msg.data.size()))) {
    return;
  }
  if (!packet.has_job_request()) {
    return;
  }

  const auto& req = packet.job_request();
  std::unique_ptr<coretex::agent::v1::JobResult> res;
  if (handler_) {
    res = handler_(req);
  }
  if (!res) {
    res = std::make_unique<coretex::agent::v1::JobResult>();
    res->set_job_id(req.job_id());
    res->set_status(coretex::agent::v1::JOB_STATUS_FAILED);
    res->set_error_message("handler returned null");
  }
  if (res->job_id().empty()) res->set_job_id(req.job_id());

  coretex::agent::v1::BusPacket out;
  out.set_trace_id(packet.trace_id());
  out.set_sender_id(sender_id_);
  out.set_protocol_version(protocol_version_);
  *(out.mutable_job_result()) = *res;

  const auto size = out.ByteSizeLong();
  std::vector<uint8_t> buffer(size);
  if (!out.SerializeToArray(buffer.data(), static_cast<int>(buffer.size()))) {
    return;
  }
  bus_->Publish(kSubjectResult, buffer);
}

}  // namespace cap
