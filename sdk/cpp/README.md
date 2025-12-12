# CAP C++ SDK

Minimal C++17 SDK for CAP with helpers to encode/decode BusPackets and wire worker/client logic. Transport is pluggable via a small bus interface; you can bring NATS, Kafka, or any pub/sub.

## Prerequisites
- CMake ≥ 3.20
- A C++17 compiler
- Protobuf compiler and C++ runtime (`protoc`, `libprotobuf-dev`)

## Build & Generate Protos
Pre-generated protobuf C++ stubs live in `/cpp`. To regenerate (or build from source) run from repo root:
```bash
cmake -S sdk/cpp -B sdk/cpp/build -DPROTO_ROOT=../../proto
cmake --build sdk/cpp/build
```
`PROTO_ROOT` points to the repo `proto/` folder; CMake will generate C++ stubs and build `cap_cpp`.

## Using the Library
- `include/cap/subjects.h` — CAP subject constants.
- `include/cap/bus_interface.h` — abstract bus interface you can implement for your transport.
- `include/cap/client.h` — publish `BusPacket{JobRequest}`.
- `include/cap/worker.h` — subscribe to a pool subject and emit `JobResult`.

### Example (pseudo)
```cpp
auto bus = std::make_unique<MyNatsBus>(); // implements cap::BusClient
cap::Worker worker(bus.get(), "job.echo",
  [](const coretex::agent::v1::JobRequest& req) {
    auto res = std::make_unique<coretex::agent::v1::JobResult>();
    res->set_job_id(req.job_id());
    res->set_status(coretex::agent::v1::JOB_STATUS_SUCCEEDED);
    res->set_result_ptr("redis://res/" + req.job_id());
    return res;
  });
worker.Start();

cap::Client client(bus.get(), "gateway-1");
coretex::agent::v1::JobRequest req;
req.set_job_id("job-1");
req.set_topic("job.echo");
req.set_context_ptr("redis://ctx/job-1");
client.Submit("trace-1", req);
```

## Notes
- Transport is abstract: implement `BusClient` with `Publish` and `Subscribe` to your broker.
- Message bodies use protobuf binary encoding for efficiency.
- Default subjects match the CAP transport profile (`sys.job.submit`, `job.<pool>`, `sys.job.result`, `sys.heartbeat`).
