#include "cap/heartbeat.h"
#include "cap/codec.h"
#include "cap/subjects.h"

#include <google/protobuf/util/time_util.h>

#include "cordum/agent/v1/buspacket.pb.h"
#include "cordum/agent/v1/heartbeat.pb.h"

namespace cap {

std::vector<uint8_t> HeartbeatPayload(
    const std::string& worker_id,
    const std::string& pool,
    int32_t active_jobs,
    int32_t max_parallel,
    float cpu_load) {
  return HeartbeatPayloadWithProgress(
      worker_id, pool, active_jobs, max_parallel, cpu_load, 0.0f, 0, "");
}

std::vector<uint8_t> HeartbeatPayloadWithMemory(
    const std::string& worker_id,
    const std::string& pool,
    int32_t active_jobs,
    int32_t max_parallel,
    float cpu_load,
    float memory_load) {
  return HeartbeatPayloadWithProgress(
      worker_id, pool, active_jobs, max_parallel, cpu_load, memory_load, 0, "");
}

std::vector<uint8_t> HeartbeatPayloadWithProgress(
    const std::string& worker_id,
    const std::string& pool,
    int32_t active_jobs,
    int32_t max_parallel,
    float cpu_load,
    float memory_load,
    int32_t progress_pct,
    const std::string& last_memo) {
  cordum::agent::v1::BusPacket packet;
  packet.set_sender_id(worker_id);
  packet.set_protocol_version(1);
  packet.mutable_created_at()->CopyFrom(
      google::protobuf::util::TimeUtil::GetCurrentTime());

  auto* hb = packet.mutable_heartbeat();
  hb->set_worker_id(worker_id);
  hb->set_pool(pool);
  hb->set_active_jobs(active_jobs);
  hb->set_max_parallel_jobs(max_parallel);
  hb->set_cpu_load(cpu_load);
  hb->set_memory_load(memory_load);
  hb->set_progress_pct(progress_pct);
  hb->set_last_memo(last_memo);

  return MarshalDeterministic(packet);
}

bool EmitHeartbeat(BusClient* bus, const std::vector<uint8_t>& payload) {
  if (!bus) return false;
  return bus->Publish(kSubjectHeartbeat, payload);
}

HeartbeatLoop::HeartbeatLoop(BusClient* bus,
                             PayloadFn payload_fn,
                             std::string worker_id,
                             std::chrono::milliseconds interval,
                             MetricsHook* metrics)
    : bus_(bus),
      payload_fn_(std::move(payload_fn)),
      worker_id_(std::move(worker_id)),
      interval_(interval),
      metrics_(metrics) {
  thread_ = std::thread(&HeartbeatLoop::Run, this);
}

HeartbeatLoop::~HeartbeatLoop() {
  Stop();
}

void HeartbeatLoop::Stop() {
  stop_.store(true, std::memory_order_release);
  if (thread_.joinable()) {
    thread_.join();
  }
}

void HeartbeatLoop::Run() {
  while (!stop_.load(std::memory_order_acquire)) {
    auto payload = payload_fn_();
    if (EmitHeartbeat(bus_, payload) && metrics_) {
      metrics_->OnHeartbeatSent(worker_id_);
    }

    // Sleep in small increments to respond quickly to Stop().
    auto deadline = std::chrono::steady_clock::now() + interval_;
    while (!stop_.load(std::memory_order_acquire) &&
           std::chrono::steady_clock::now() < deadline) {
      std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
  }
}

}  // namespace cap
