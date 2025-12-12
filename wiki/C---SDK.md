# C++ SDK

The C++ SDK for CAP is designed for high-performance applications where low-level control is required.

## Building

The C++ SDK uses CMake for building.

```bash
cmake -S sdk/cpp -B sdk/cpp/build
cmake --build sdk/cpp/build
```

## Client

The `cap::Client` class provides an interface for submitting jobs.

*(Note: The C++ SDK is under development, and the client implementation is not yet complete.)*

### `cap::Client`

```cpp
// sdk/cpp/include/cap/client.h

#include "bus_interface.h"
#include "coretex/agent/v1/job.pb.h"

namespace cap {

class Client {
 public:
  explicit Client(std::unique_ptr<BusInterface> bus);

  void Submit(const coretex::agent::v1::JobRequest& job_request);

 private:
  std::unique_ptr<BusInterface> bus_;
};

}  // namespace cap
```

## Worker

The `cap::Worker` class provides an interface for building workers.

*(Note: The C++ SDK is under development, and the worker implementation is not yet complete.)*

### `cap::Worker`

```cpp
// sdk/cpp/include/cap/worker.h

#include "bus_interface.h"
#include "coretex/agent/v1/job.pb.h"

namespace cap {

class Worker {
 public:
  using Handler = std::function<coretex::agent::v1::JobResult(
      const coretex::agent::v1::JobRequest&)>;

  Worker(std::unique_ptr<BusInterface> bus, std::string subject,
         Handler handler);

  void Start();

 private:
  std::unique_ptr<BusInterface> bus_;
  std::string subject_;
  Handler handler_;
};

}  // namespace cap
```

## Bus Interface

The `cap::BusInterface` is an abstract class that defines the interface for interacting with the NATS bus. You will need to provide a concrete implementation of this interface.

```cpp
// sdk/cpp/include/cap/bus_interface.h

#include <functional>
#include <string>
#include <vector>

namespace cap {

class BusInterface {
 public:
  using MessageHandler = std::function<void(const std::vector<char>&)>;

  virtual ~BusInterface() = default;

  virtual void Publish(const std::string& subject,
                     const std::vector<char>& data) = 0;

  virtual void Subscribe(const std::string& subject,
                       MessageHandler handler) = 0;

  virtual void QueueSubscribe(const std::string& subject,
                            const std::string& queue,
                            MessageHandler handler) = 0;
};

}  // namespace cap
```
