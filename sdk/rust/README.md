# CAP Rust SDK

Rust SDK for the Cordum Agent Protocol (CAP). Provides async NATS-based worker/client abstractions, ECDSA packet signing, deterministic serialization, heartbeat/progress helpers, middleware, and metrics hooks.

## Quick Start

### Prerequisites
- Rust 1.70+ (edition 2021)
- Protobuf compiler (`protoc`) for build-time code generation

### Build
```bash
cd sdk/rust
cargo build
```

### Run Tests
```bash
cargo test
```

## Usage

### Agent (High-Level API)

```rust
use cap_sdk::agent::AgentBuilder;
use cap_sdk::middleware::Context;
use cap_sdk::pb::{JobRequest, JobResult, JobStatus};

#[tokio::main]
async fn main() {
    let mut agent = AgentBuilder::new("worker-rust-1")
        .nats_url("nats://localhost:4222")
        .pool("job.tools")
        .max_parallel(4)
        .build()
        .await
        .unwrap();

    agent.register("job.summarize", Arc::new(|ctx: &Context, req: &JobRequest| {
        Ok(JobResult {
            job_id: req.job_id.clone(),
            worker_id: "worker-rust-1".into(),
            status: JobStatus::Succeeded as i32,
            ..Default::default()
        })
    }));

    agent.start().await.unwrap();
}
```

### Worker (Low-Level API)

```rust
use cap_sdk::worker::Worker;
use cap_sdk::middleware::Context;
use cap_sdk::pb::{JobRequest, JobResult, JobStatus};

let nc = cap_sdk::bus::connect_nats(&Default::default()).await?;

let worker = Worker::new(
    nc, "job.echo".into(), "worker-echo-1".into(),
    Arc::new(|_ctx: &Context, req: &JobRequest| {
        Ok(JobResult {
            job_id: req.job_id.clone(),
            worker_id: "worker-echo-1".into(),
            status: JobStatus::Succeeded as i32,
            ..Default::default()
        })
    }),
);

worker.start().await?;
```

### Client (Job Submission)

```rust
use cap_sdk::client::Client;
use cap_sdk::pb::JobRequest;

let nc = cap_sdk::bus::connect_nats(&Default::default()).await?;
let client = Client::new(nc);

let req = JobRequest {
    job_id: "job-1".into(),
    topic: "job.echo".into(),
    context_ptr: "redis://ctx/job-1".into(),
    ..Default::default()
};

client.submit(&req, "trace-1", "client-rust", None).await?;
```

### Heartbeats

```rust
use cap_sdk::heartbeat::{self, HeartbeatLoop};

// One-shot heartbeat
let payload = heartbeat::heartbeat_payload("worker-1", "job.tools", 2, 8, 45.0);
heartbeat::emit_heartbeat(&nc, &payload).await?;

// Heartbeat loop (background tokio task)
let loop_ = HeartbeatLoop::start(
    nc.clone(),
    || heartbeat::heartbeat_payload("worker-1", "job.tools", 0, 8, 0.0),
    "worker-1".into(),
    Duration::from_secs(5),
    metrics.clone(),
);
// ...
loop_.stop().await;
```

### Progress and Cancel

```rust
use cap_sdk::progress;

let payload = progress::progress_payload("worker-1", "job-1", "step-3", 75, "processing");
progress::emit_progress(&nc, &payload).await?;

let cancel = progress::cancel_payload("client-1", "job-1", "timeout", "admin");
progress::emit_cancel(&nc, &cancel).await?;
```

## Middleware

Add cross-cutting concerns without modifying handlers:

```rust
use cap_sdk::middleware::{self, MiddlewareFn};

// Logging middleware
let mw = middleware::logging_middleware();
agent_builder = agent_builder.middleware(mw);

// Custom middleware
let custom: MiddlewareFn = Arc::new(|handler| {
    Arc::new(move |ctx, req| {
        println!("before job {}", req.job_id);
        let result = handler(ctx, req);
        println!("after job");
        result
    })
});
```

Middleware executes in registration order (FIFO).

## Signing

ECDSA P-256 with SHA-256 (ASN.1/DER format), interoperable with Go/Python/Node/Java/C# SDKs:

```rust
use cap_sdk::signing;

// Load keys from PEM
let private_key = signing::load_private_key(pem_str)?;
let public_key = signing::load_public_key(pem_str)?;

// Sign a packet
signing::sign_packet(&mut packet, &private_key)?;

// Verify a packet
signing::verify_packet_signature(&packet, &public_key)?;
```

## Metrics

Implement `MetricsHook` to integrate with Prometheus, OpenTelemetry, or any observability system:

```rust
use cap_sdk::metrics::MetricsHook;

pub trait MetricsHook: Send + Sync {
    fn on_job_received(&self, job_id: &str, topic: &str) {}
    fn on_job_completed(&self, job_id: &str, duration_ms: u64, status: &str) {}
    fn on_job_failed(&self, job_id: &str, error_msg: &str) {}
    fn on_heartbeat_sent(&self, worker_id: &str) {}
    fn on_progress_emitted(&self, job_id: &str, percent: i32) {}
    fn on_error(&self, category: &str, message: &str) {}
}
```

Default is `NoopMetrics` (zero overhead).

## Testing

Test handlers without a real NATS server:

```rust
use cap_sdk::testing::{self, MockBus};
use cap_sdk::pb::{JobRequest, JobStatus};

let req = JobRequest {
    job_id: "test-1".into(),
    topic: "job.echo".into(),
    ..Default::default()
};

let result = testing::submit_and_wait(my_handler, &req);
assert_eq!(result.status, JobStatus::Succeeded as i32);
```

## Structure

- `cap_sdk::pb` — Generated protobuf types (BusPacket, JobRequest, JobResult, etc.)
- `cap_sdk::codec` — Deterministic protobuf serialization
- `cap_sdk::signing` — ECDSA P-256 packet signing/verification
- `cap_sdk::validate` — Input validation
- `cap_sdk::worker` — Low-level NATS subscriber with job dispatch
- `cap_sdk::client` — Job submission helper
- `cap_sdk::agent` — High-level runtime with handler registration
- `cap_sdk::heartbeat` — Heartbeat payload builders and loop
- `cap_sdk::progress` — Progress/cancel payload builders
- `cap_sdk::middleware` — Composable interceptor chain
- `cap_sdk::metrics` — Pluggable observability callbacks
- `cap_sdk::errors` — Typed error codes and types
- `cap_sdk::testing` — MockBus, submit_and_wait, RecordingMetrics
