#pragma once

#include <cstdint>
#include <string>
#include <vector>

#include "cap/bus_interface.h"

namespace cap {

// Payload constructors — return deterministically serialized BusPacket bytes.

std::vector<uint8_t> ProgressPayload(
    const std::string& sender_id,
    const std::string& job_id,
    const std::string& step_id,
    int32_t percent,
    const std::string& message);

std::vector<uint8_t> CancelPayload(
    const std::string& sender_id,
    const std::string& job_id,
    const std::string& reason,
    const std::string& requested_by);

// Publish a progress payload to sys.job.progress.
bool EmitProgress(BusClient* bus, const std::vector<uint8_t>& payload);

// Publish a cancel payload to sys.job.cancel.
bool EmitCancel(BusClient* bus, const std::vector<uint8_t>& payload);

}  // namespace cap
