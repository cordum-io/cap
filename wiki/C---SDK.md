# C++ SDK

The C++ SDK for CAP is designed for high-performance applications where low-level control is required.

## Building

The C++ SDK uses CMake for building.

```bash
cmake -S sdk/cpp -B sdk/cpp/build
cmake --build sdk/cpp/build
```

## Client

The `cap::Client` class publishes `JobRequest` envelopes onto the bus.

### `cap::Client`

```cpp
// sdk/cpp/include/cap/client.h

#include "cap/bus_interface.h"
#include "cap/subjects.h"
#include "coretex/agent/v1/buspacket.pb.h"

namespace cap {

class Client {
 public:
  Client(BusClient* bus, std::string sender_id);

  bool Submit(const std::string& trace_id,
              const coretex::agent::v1::JobRequest& req,
              int32_t protocol_version = 1);

 private:
  BusClient* bus_;
  std::string sender_id_;
};

}  // namespace cap
```

## Worker

The `cap::Worker` class subscribes to a pool subject and emits `JobResult` packets.

### `cap::Worker`

```cpp
// sdk/cpp/include/cap/worker.h

#include "cap/bus_interface.h"
#include "cap/subjects.h"
#include "coretex/agent/v1/buspacket.pb.h"

namespace cap {

using JobHandler = std::function<std::unique_ptr<coretex::agent::v1::JobResult>(
    const coretex::agent::v1::JobRequest&)>;

class Worker {
 public:
  Worker(BusClient* bus,
         std::string subject,
         std::string sender_id,
         JobHandler handler,
         int32_t protocol_version = 1);

  bool Start();

 private:
  void OnMessage(const BusMessage& msg);

  BusClient* bus_;
  std::string subject_;
  std::string sender_id_;
  JobHandler handler_;
  int32_t protocol_version_;
  SubscriptionHandle sub_{nullptr};
};

}  // namespace cap
```

## Bus Interface

The `cap::BusClient` interface defines the transport adapter you must implement (NATS, Kafka, etc.).

```cpp
// sdk/cpp/include/cap/bus_interface.h

#include <functional>
#include <string>
#include <vector>

namespace cap {

struct BusMessage {
  std::string subject;
  std::vector<uint8_t> data;
};

using SubscriptionHandle = void*;

class BusClient {
 public:
  using MsgHandler = std::function<void(const BusMessage&)>;

  virtual ~BusClient() = default;
  virtual bool Publish(const std::string& subject, const std::vector<uint8_t>& data) = 0;
  virtual SubscriptionHandle Subscribe(const std::string& subject,
                                       const std::string& queue_group,
                                       MsgHandler handler) = 0;
};

}  // namespace cap
```

## Notes
- No signing helpers are included in the C++ SDK yet; add them in your bus or worker layer if required.
- Default subjects are defined in `sdk/cpp/include/cap/subjects.h`.
