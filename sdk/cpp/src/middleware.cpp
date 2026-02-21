#include "cap/middleware.h"

#include <chrono>
#include <iostream>

namespace cap {

Handler ApplyMiddleware(Handler base, const std::vector<Middleware>& middlewares) {
  Handler handler = std::move(base);
  for (auto it = middlewares.rbegin(); it != middlewares.rend(); ++it) {
    handler = (*it)(std::move(handler));
  }
  return handler;
}

Middleware LoggingMiddleware() {
  return [](Handler next) -> Handler {
    return [next = std::move(next)](const Context& ctx,
                                    const cordum::agent::v1::JobRequest& req)
               -> std::unique_ptr<cordum::agent::v1::JobResult> {
      auto start = std::chrono::steady_clock::now();
      auto result = next(ctx, req);
      auto end = std::chrono::steady_clock::now();
      auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(end - start).count();
      std::cout << "[cap] job=" << req.job_id() << " duration=" << ms << "ms" << std::endl;
      return result;
    };
  };
}

}  // namespace cap
