#pragma once

namespace cap {

// Default CAP subjects (transport profile)
constexpr const char* kSubjectSubmit = "sys.job.submit";
constexpr const char* kSubjectResult = "sys.job.result";
constexpr const char* kSubjectHeartbeat = "sys.heartbeat";

// Pool subjects follow job.<pool> (e.g., job.echo, job.code.llm)

}  // namespace cap
