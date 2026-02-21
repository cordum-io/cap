#include "cap/middleware.h"

#include <memory>
#include <string>
#include <vector>

#include <gtest/gtest.h>

#include "cordum/agent/v1/buspacket.pb.h"

namespace {

cap::Handler MakeEchoHandler() {
  return [](const cap::Context& ctx, const cordum::agent::v1::JobRequest& req)
             -> std::unique_ptr<cordum::agent::v1::JobResult> {
    auto result = std::make_unique<cordum::agent::v1::JobResult>();
    result->set_job_id(req.job_id());
    result->set_status(cordum::agent::v1::JOB_STATUS_SUCCEEDED);
    return result;
  };
}

}  // namespace

TEST(MiddlewareTest, EmptyMiddlewareListReturnsBaseHandler) {
  auto handler = MakeEchoHandler();
  auto wrapped = cap::ApplyMiddleware(handler, {});

  cap::Context ctx;
  cordum::agent::v1::JobRequest req;
  req.set_job_id("job-1");

  auto result = wrapped(ctx, req);
  ASSERT_NE(result, nullptr);
  EXPECT_EQ(result->job_id(), "job-1");
  EXPECT_EQ(result->status(), cordum::agent::v1::JOB_STATUS_SUCCEEDED);
}

TEST(MiddlewareTest, SingleMiddlewareWrapsCorrectly) {
  std::vector<std::string> order;

  cap::Middleware mw = [&order](cap::Handler next) -> cap::Handler {
    return [&order, next = std::move(next)](const cap::Context& ctx,
                                            const cordum::agent::v1::JobRequest& req)
               -> std::unique_ptr<cordum::agent::v1::JobResult> {
      order.push_back("before");
      auto result = next(ctx, req);
      order.push_back("after");
      return result;
    };
  };

  auto handler = [&order](const cap::Context&, const cordum::agent::v1::JobRequest& req)
                     -> std::unique_ptr<cordum::agent::v1::JobResult> {
    order.push_back("handler");
    auto result = std::make_unique<cordum::agent::v1::JobResult>();
    result->set_job_id(req.job_id());
    return result;
  };

  auto wrapped = cap::ApplyMiddleware(handler, {mw});

  cap::Context ctx;
  cordum::agent::v1::JobRequest req;
  req.set_job_id("job-1");
  wrapped(ctx, req);

  ASSERT_EQ(order.size(), 3u);
  EXPECT_EQ(order[0], "before");
  EXPECT_EQ(order[1], "handler");
  EXPECT_EQ(order[2], "after");
}

TEST(MiddlewareTest, ThreeMiddlewaresExecuteInFIFOOrder) {
  std::vector<int> order;

  auto make_mw = [&order](int id) -> cap::Middleware {
    return [&order, id](cap::Handler next) -> cap::Handler {
      return [&order, id, next = std::move(next)](const cap::Context& ctx,
                                                   const cordum::agent::v1::JobRequest& req)
                 -> std::unique_ptr<cordum::agent::v1::JobResult> {
        order.push_back(id);
        return next(ctx, req);
      };
    };
  };

  auto wrapped = cap::ApplyMiddleware(MakeEchoHandler(),
                                       {make_mw(1), make_mw(2), make_mw(3)});

  cap::Context ctx;
  cordum::agent::v1::JobRequest req;
  req.set_job_id("job-1");
  wrapped(ctx, req);

  ASSERT_EQ(order.size(), 3u);
  EXPECT_EQ(order[0], 1);
  EXPECT_EQ(order[1], 2);
  EXPECT_EQ(order[2], 3);
}

TEST(MiddlewareTest, LoggingMiddlewareDoesNotInterfereWithResult) {
  auto wrapped = cap::ApplyMiddleware(MakeEchoHandler(), {cap::LoggingMiddleware()});

  cap::Context ctx;
  cordum::agent::v1::JobRequest req;
  req.set_job_id("logged-job");

  auto result = wrapped(ctx, req);
  ASSERT_NE(result, nullptr);
  EXPECT_EQ(result->job_id(), "logged-job");
  EXPECT_EQ(result->status(), cordum::agent::v1::JOB_STATUS_SUCCEEDED);
}
