#pragma once

#include <string>

namespace cap {

class MetricsHook {
 public:
  virtual ~MetricsHook() = default;
  virtual void OnJobReceived(const std::string& job_id, const std::string& topic) {}
  virtual void OnJobCompleted(const std::string& job_id, int duration_ms, const std::string& status) {}
  virtual void OnJobFailed(const std::string& job_id, const std::string& error_msg) {}
  virtual void OnHeartbeatSent(const std::string& worker_id) {}
  virtual void OnProgressEmitted(const std::string& job_id, int percent) {}
  virtual void OnError(const std::string& context, const std::string& error_msg) {}
};

}  // namespace cap
