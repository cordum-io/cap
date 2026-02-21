#include "cap/agent.h"
#include "cap/codec.h"
#include "cap/signing.h"
#include "cap/subjects.h"

#include <chrono>

#include <google/protobuf/util/time_util.h>

#include "cordum/agent/v1/buspacket.pb.h"

namespace cap {

Agent::Agent(AgentConfig config) : config_(std::move(config)) {
  for (auto& [id, key] : config_.public_keys) {
    public_key_map_[id] = key;
  }
}

Agent::~Agent() {
  Close();
}

void Agent::RegisterHandler(const std::string& subject, Handler handler) {
  handlers_[subject] = std::move(handler);
}

bool Agent::Start() {
  if (started_ || !config_.bus) return false;

  // Apply middleware to all handlers before subscribing.
  for (auto& [subject, handler] : handlers_) {
    handler = ApplyMiddleware(handler, config_.middlewares);
  }

  for (const auto& [subject, handler] : handlers_) {
    auto captured_subject = subject;
    auto sub = config_.bus->Subscribe(
        subject, subject,
        [this, captured_subject](const BusMessage& msg) {
          this->OnMessage(captured_subject, msg);
        });

    if (!sub) return false;
    subscriptions_.push_back(sub);
  }

  // Start heartbeat loop.
  auto interval_ms = static_cast<int>(config_.heartbeat_interval_sec * 1000);
  heartbeat_loop_ = std::make_unique<HeartbeatLoop>(
      config_.bus,
      [this]() -> std::vector<uint8_t> {
        return HeartbeatPayload(
            config_.sender_id, config_.pool,
            active_jobs_.load(std::memory_order_relaxed),
            config_.max_parallel, 0.0f);
      },
      config_.sender_id,
      std::chrono::milliseconds(interval_ms),
      config_.metrics);

  started_ = true;
  return true;
}

void Agent::Close() {
  if (!started_) return;
  heartbeat_loop_.reset();
  subscriptions_.clear();
  started_ = false;
}

void Agent::OnMessage(const std::string& subject, const BusMessage& msg) {
  cordum::agent::v1::BusPacket packet;
  if (!packet.ParseFromArray(msg.data.data(), static_cast<int>(msg.data.size()))) {
    if (config_.metrics) {
      config_.metrics->OnError("parse", "failed to parse BusPacket");
    }
    return;
  }

  // Verify signature if public keys are configured.
  if (!public_key_map_.empty()) {
    const std::string& sender = packet.sender_id();
    auto it = public_key_map_.find(sender);
    if (it == public_key_map_.end()) return;
    if (packet.signature().empty()) return;
    if (!VerifyPacketSignature(packet, it->second)) return;
  }

  if (!packet.has_job_request()) return;

  const auto& req = packet.job_request();

  if (config_.metrics) {
    config_.metrics->OnJobReceived(req.job_id(), req.topic());
  }

  auto handler_it = handlers_.find(subject);
  if (handler_it == handlers_.end()) return;

  Context ctx;
  ctx.trace_id = packet.trace_id();
  ctx.sender_id = packet.sender_id();
  ctx.packet = &packet;

  active_jobs_.fetch_add(1, std::memory_order_relaxed);
  auto start = std::chrono::steady_clock::now();

  std::unique_ptr<cordum::agent::v1::JobResult> result;
  try {
    result = handler_it->second(ctx, req);
  } catch (...) {
    active_jobs_.fetch_sub(1, std::memory_order_relaxed);
    if (config_.metrics) {
      config_.metrics->OnJobFailed(req.job_id(), "handler threw exception");
    }
    return;
  }

  active_jobs_.fetch_sub(1, std::memory_order_relaxed);

  auto end = std::chrono::steady_clock::now();
  auto duration_ms = std::chrono::duration_cast<std::chrono::milliseconds>(end - start).count();

  if (!result) {
    result = std::make_unique<cordum::agent::v1::JobResult>();
    result->set_job_id(req.job_id());
    result->set_status(cordum::agent::v1::JOB_STATUS_FAILED);
    result->set_error_message("handler returned null");
  }
  if (result->job_id().empty()) result->set_job_id(req.job_id());
  if (result->worker_id().empty()) result->set_worker_id(config_.sender_id);

  std::string status_str = (result->status() == cordum::agent::v1::JOB_STATUS_FAILED)
                               ? "FAILED"
                               : "SUCCEEDED";
  if (config_.metrics) {
    if (result->status() == cordum::agent::v1::JOB_STATUS_FAILED) {
      config_.metrics->OnJobFailed(req.job_id(), result->error_message());
    } else {
      config_.metrics->OnJobCompleted(req.job_id(), static_cast<int>(duration_ms), status_str);
    }
  }

  // Build response packet.
  cordum::agent::v1::BusPacket out;
  out.set_trace_id(packet.trace_id());
  out.set_sender_id(config_.sender_id);
  out.set_protocol_version(1);
  out.mutable_created_at()->CopyFrom(
      google::protobuf::util::TimeUtil::GetCurrentTime());
  *(out.mutable_job_result()) = *result;

  if (config_.private_key) {
    if (!SignPacket(&out, config_.private_key)) {
      if (config_.metrics) {
        config_.metrics->OnError("signing", "failed to sign outgoing packet");
      }
      return;
    }
  }

  auto buffer = MarshalDeterministic(out);
  if (buffer.empty() && out.ByteSizeLong() > 0) return;
  config_.bus->Publish(kSubjectResult, buffer);
}

}  // namespace cap
