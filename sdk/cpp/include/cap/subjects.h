#pragma once

namespace cap {

// Default CAP subjects (transport profile)
constexpr const char* kSubjectSubmit = "sys.job.submit";
constexpr const char* kSubjectResult = "sys.job.result";
constexpr const char* kSubjectHeartbeat = "sys.heartbeat";
constexpr const char* kSubjectProgress = "sys.job.progress";
constexpr const char* kSubjectCancel = "sys.job.cancel";

// Pool subjects follow job.<pool> (e.g., job.echo, job.code.llm)

}  // namespace cap
