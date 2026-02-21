#pragma once

#include <functional>
#include <memory>
#include <string>
#include <vector>

#include "cordum/agent/v1/buspacket.pb.h"

namespace cap {

struct Context {
  std::string trace_id;
  std::string sender_id;
  const cordum::agent::v1::BusPacket* packet;
};

// Handler processes a job request within a context and returns a result.
using Handler = std::function<std::unique_ptr<cordum::agent::v1::JobResult>(
    const Context&, const cordum::agent::v1::JobRequest&)>;

// Middleware wraps a handler, returning a new handler.
using Middleware = std::function<Handler(Handler next)>;

// ApplyMiddleware composes a chain of middlewares around a base handler.
// Middlewares execute in FIFO order (first added = outermost wrapper).
Handler ApplyMiddleware(Handler base, const std::vector<Middleware>& middlewares);

// LoggingMiddleware logs job_id and duration for each request to stdout.
Middleware LoggingMiddleware();

}  // namespace cap
