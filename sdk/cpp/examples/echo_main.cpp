#include <iostream>
#include <memory>

#include "cap/bus_interface.h"
#include "cap/client.h"
#include "cap/worker.h"
#include "coretex/agent/v1/job.pb.h"

// Dummy in-process bus for demonstration (not production).
class InMemoryBus : public cap::BusClient {
 public:
  bool Publish(const std::string& subject, const std::vector<uint8_t>& data) override {
    if (handler_) {
      handler_(cap::BusMessage{subject, data});
      return true;
    }
    return false;
  }

  SubscriptionHandle Subscribe(const std::string& subject,
                               const std::string& /*queue_group*/,
                               MsgHandler handler) override {
    subject_ = subject;
    handler_ = std::move(handler);
    return reinterpret_cast<SubscriptionHandle>(this);
  }

 private:
  std::string subject_;
  MsgHandler handler_;
};

int main() {
  InMemoryBus bus;

  cap::Worker worker(&bus, "job.echo", "worker-echo-1",
                     [](const coretex::agent::v1::JobRequest& req) {
                       auto res = std::make_unique<coretex::agent::v1::JobResult>();
                       res->set_job_id(req.job_id());
                       res->set_status(coretex::agent::v1::JOB_STATUS_SUCCEEDED);
                       res->set_result_ptr("redis://res/" + req.job_id());
                       return res;
                     });
  worker.Start();

  cap::Client client(&bus, "gateway-1");
  coretex::agent::v1::JobRequest req;
  req.set_job_id("job-1");
  req.set_topic("job.echo");
  req.set_context_ptr("redis://ctx/job-1");
  client.Submit("trace-1", req);

  std::cout << "Echo job submitted via in-memory bus\n";
  return 0;
}
