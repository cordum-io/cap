#pragma once

#include <atomic>
#include <chrono>
#include <cstdint>
#include <functional>
#include <memory>
#include <string>
#include <thread>
#include <vector>

#include "cap/bus_interface.h"
#include "cap/metrics.h"

namespace cap {

// Payload constructors — return deterministically serialized BusPacket bytes.

std::vector<uint8_t> HeartbeatPayload(
    const std::string& worker_id,
    const std::string& pool,
    int32_t active_jobs,
    int32_t max_parallel,
    float cpu_load);

std::vector<uint8_t> HeartbeatPayloadWithMemory(
    const std::string& worker_id,
    const std::string& pool,
    int32_t active_jobs,
    int32_t max_parallel,
    float cpu_load,
    float memory_load);

std::vector<uint8_t> HeartbeatPayloadWithProgress(
    const std::string& worker_id,
    const std::string& pool,
    int32_t active_jobs,
    int32_t max_parallel,
    float cpu_load,
    float memory_load,
    int32_t progress_pct,
    const std::string& last_memo);

// Publish a heartbeat payload to sys.heartbeat.
bool EmitHeartbeat(BusClient* bus, const std::vector<uint8_t>& payload);

// HeartbeatLoop runs a background thread that emits heartbeats at a fixed interval.
// RAII: destructor calls Stop().
class HeartbeatLoop {
 public:
  using PayloadFn = std::function<std::vector<uint8_t>()>;

  HeartbeatLoop(BusClient* bus,
                PayloadFn payload_fn,
                std::string worker_id,
                std::chrono::milliseconds interval = std::chrono::seconds(5),
                MetricsHook* metrics = nullptr);

  ~HeartbeatLoop();

  HeartbeatLoop(const HeartbeatLoop&) = delete;
  HeartbeatLoop& operator=(const HeartbeatLoop&) = delete;

  void Stop();

 private:
  void Run();

  BusClient* bus_;
  PayloadFn payload_fn_;
  std::string worker_id_;
  std::chrono::milliseconds interval_;
  MetricsHook* metrics_;
  std::atomic<bool> stop_{false};
  std::thread thread_;
};

}  // namespace cap
