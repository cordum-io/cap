# CAP Ruby SDK

Ruby SDK for the Cordum Agent Protocol (CAP). Provides NATS-based worker/client abstractions, ECDSA packet signing, deterministic serialization, heartbeat/progress helpers, middleware, and metrics hooks.

## Quick Start

### Prerequisites
- Ruby 3.1+
- Bundler

### Install
```bash
cd sdk/ruby
bundle install
```

### Run Tests
```bash
bundle exec rspec
```

### Generate Protobuf Types
Proto compilation outputs to `proto/`. For standalone generation:
```bash
CAP_RUN_RUBY=1 ./tools/make_protos.sh
```

## Usage

### Agent (High-Level API)

```ruby
require 'cordum/cap'

agent = Cordum::Cap::Agent.new(
  nats_url: 'nats://localhost:4222',
  sender_id: 'worker-ruby-1',
  pool: 'job.tools',
  max_parallel: 4
)

agent.register('job.summarize') do |ctx, req|
  Cordum::Agent::V1::JobResult.new(
    job_id: req.job_id,
    worker_id: 'worker-ruby-1',
    status: :JOB_STATUS_SUCCEEDED,
    result_ptr: "redis://result/#{req.job_id}"
  )
end

agent.start
```

### Worker (Low-Level API)

```ruby
nc = Cordum::Cap::Bus.connect(url: 'nats://localhost:4222')

worker = Cordum::Cap::Worker.new(
  nc: nc,
  subject: 'job.echo',
  sender_id: 'worker-echo-1',
  handler: ->(ctx, req) {
    Cordum::Agent::V1::JobResult.new(
      job_id: req.job_id,
      worker_id: 'worker-echo-1',
      status: :JOB_STATUS_SUCCEEDED
    )
  }
)

worker.start
```

### Client (Job Submission)

```ruby
nc = Cordum::Cap::Bus.connect(url: 'nats://localhost:4222')
client = Cordum::Cap::Client.new(nc: nc)

req = Cordum::Agent::V1::JobRequest.new(
  job_id: 'job-1',
  topic: 'job.echo',
  context_ptr: 'redis://ctx/job-1'
)

client.submit(req, trace_id: 'trace-1', sender_id: 'client-ruby', private_key: key)
```

### Heartbeats

```ruby
# One-shot heartbeat
payload = Cordum::Cap::HeartbeatHelper.heartbeat_payload('worker-1', 'job.tools', 2, 8, 45.0)
Cordum::Cap::HeartbeatHelper.emit_heartbeat(nc, payload)

# Heartbeat loop (background thread)
loop = Cordum::Cap::HeartbeatLoop.new(
  nc: nc,
  payload_fn: -> { Cordum::Cap::HeartbeatHelper.heartbeat_payload('worker-1', 'job.tools', 0, 8, 0.0) },
  worker_id: 'worker-1',
  interval: 5,
  metrics: metrics
)
loop.start
# ...
loop.stop
```

### Progress and Cancel

```ruby
payload = Cordum::Cap::ProgressHelper.progress_payload('worker-1', 'job-1', 'step-3', 75, 'processing')
Cordum::Cap::ProgressHelper.emit_progress(nc, payload)

cancel = Cordum::Cap::ProgressHelper.cancel_payload('client-1', 'job-1', 'timeout', 'admin')
Cordum::Cap::ProgressHelper.emit_cancel(nc, cancel)
```

## Middleware

Add cross-cutting concerns without modifying handlers:

```ruby
# Built-in logging middleware
agent.use(Cordum::Cap::MiddlewareChain.logging)

# Custom middleware
agent.use(->(handler) {
  ->(ctx, req) {
    start = Process.clock_gettime(Process::CLOCK_MONOTONIC, :millisecond)
    result = handler.call(ctx, req)
    elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC, :millisecond) - start
    puts "Job #{req.job_id} took #{elapsed}ms"
    result
  }
})
```

Middleware executes in registration order (FIFO).

## Signing

ECDSA P-256 with SHA-256 (ASN.1/DER format), interoperable with Go/Python/Node/Java/Rust/C# SDKs:

```ruby
# Load keys from PEM
private_key = Cordum::Cap::Signing.load_private_key(pem_string)
public_key = Cordum::Cap::Signing.load_public_key(pem_string)

# Sign a packet
Cordum::Cap::Signing.sign_packet(packet, private_key)

# Verify a packet
valid = Cordum::Cap::Signing.verify_packet_signature(packet, public_key)
```

## Metrics

Include `MetricsHook` to integrate with Prometheus, OpenTelemetry, or any observability system:

```ruby
module Cordum::Cap::MetricsHook
  def on_job_received(job_id, topic); end
  def on_job_completed(job_id, duration_ms, status); end
  def on_job_failed(job_id, error_msg); end
  def on_heartbeat_sent(worker_id); end
  def on_progress_emitted(job_id, percent); end
  def on_error(category, message); end
end
```

Default is `NoopMetrics` (zero overhead).

## Testing

Test handlers without a real NATS server:

```ruby
require 'cordum/cap'

bus = Cordum::Cap::MockNats.new

req = Cordum::Agent::V1::JobRequest.new(
  job_id: 'test-1',
  topic: 'job.echo'
)

result = Cordum::Cap::TestHelper.submit_and_wait(my_handler, req)
expect(result.status).to eq(:JOB_STATUS_SUCCEEDED)
```

## Structure

- `Cordum::Cap` — SDK classes (Codec, Signing, Validate, Worker, Client, Agent, etc.)
- `Cordum::Agent::V1` — Generated protobuf types (BusPacket, JobRequest, JobResult, etc.)
- Proto source: `proto/cordum/agent/v1/*.proto`

## API Reference

| Class/Module | Description |
|-------|-------------|
| `Agent` | High-level runtime with handler registration, middleware, heartbeats |
| `Worker` | Low-level NATS subscriber with job dispatch |
| `Client` | Job submission helper |
| `Bus` | NATS connection factory |
| `Codec` | Deterministic protobuf serialization |
| `Signing` | ECDSA P-256 packet signing/verification |
| `Validate` | Input validation (job_request!, job_result!, bus_packet!) |
| `HeartbeatHelper` | Heartbeat payload builders |
| `HeartbeatLoop` | Background heartbeat emission thread |
| `ProgressHelper` | Progress/cancel payload builders and emission |
| `MiddlewareChain` | Composable interceptor chain |
| `MetricsHook` | Pluggable observability callbacks (mixin module) |
| `CapError` | Typed error codes and exception classes |
| `MockNats` / `TestHelper` | Testing utilities and recording metrics |
