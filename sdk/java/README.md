# CAP Java SDK

Java SDK for the Cordum Agent Protocol (CAP). Provides NATS-based worker/client abstractions, ECDSA packet signing, deterministic serialization, heartbeat/progress helpers, middleware, and metrics hooks.

## Quick Start

### Prerequisites
- Java 11+
- Maven or Gradle

### Build

**Maven:**
```bash
cd sdk/java
mvn clean install
```

**Gradle:**
```bash
cd sdk/java
gradle build
```

### Generate Protobuf Types
Proto compilation is handled automatically by the Maven/Gradle protobuf plugins during build. For standalone generation:
```bash
CAP_RUN_JAVA=1 ./tools/make_protos.sh
```

### Run Tests
```bash
mvn test
# or
gradle test
```

## Usage

### Agent (High-Level API)

```java
import io.cordum.cap.*;
import io.cordum.cap.agent.v1.*;

Agent agent = Agent.builder()
    .natsUrl("nats://localhost:4222")
    .senderId("worker-java-1")
    .pool("job.tools")
    .maxParallel(4)
    .build();

agent.registerHandler("job.summarize", (ctx, req) -> {
    // Process the job request
    return JobResult.newBuilder()
        .setJobId(req.getJobId())
        .setWorkerId("worker-java-1")
        .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
        .setResultPtr("redis://result/" + req.getJobId())
        .build();
});

agent.start();
```

### Worker (Low-Level API)

```java
Connection nc = Bus.connect(new Bus.Config("nats://localhost:4222"));

Worker worker = new Worker(nc, "job.echo", "worker-echo-1", (ctx, req) -> {
    return JobResult.newBuilder()
        .setJobId(req.getJobId())
        .setWorkerId("worker-echo-1")
        .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
        .build();
});

worker.start();
```

### Client (Job Submission)

```java
Connection nc = Bus.connect(new Bus.Config("nats://localhost:4222"));
Client client = new Client(nc);

JobRequest req = JobRequest.newBuilder()
    .setJobId("job-1")
    .setTopic("job.echo")
    .setContextPtr("redis://ctx/job-1")
    .build();

client.submit(req, "trace-1", "client-java", privateKey);
```

### Heartbeats

```java
// One-shot heartbeat
byte[] payload = Heartbeat.heartbeatPayload("worker-1", "job.tools", 2, 8, 45.0);
Heartbeat.emitHeartbeat(nc, payload);

// Heartbeat loop (periodic, background thread)
Heartbeat.HeartbeatLoop loop = new Heartbeat.HeartbeatLoop(
    nc,
    () -> Heartbeat.heartbeatPayload("worker-1", "job.tools", activeJobs.get(), 8, getCpuLoad()),
    5000, // interval ms
    metrics,
    "worker-1"
);
loop.start();
// ...
loop.stop();
```

### Progress and Cancel

```java
byte[] progress = Progress.progressPayload("worker-1", "job-1", "step-3", 75, "processing");
Progress.emitProgress(nc, progress);

byte[] cancel = Progress.cancelPayload("client-1", "job-1", "timeout", "admin");
Progress.emitCancel(nc, cancel);
```

## Middleware

Add cross-cutting concerns without modifying handlers:

```java
// Built-in logging middleware
agent.use(Middleware.loggingMiddleware(logger));

// Custom middleware
agent.use(next -> (ctx, req) -> {
    long start = System.currentTimeMillis();
    JobResult result = next.handle(ctx, req);
    long elapsed = System.currentTimeMillis() - start;
    logger.info("Job " + req.getJobId() + " took " + elapsed + "ms");
    return result;
});
```

Middleware executes in registration order (FIFO).

## Signing

ECDSA P-256 with SHA-256 (ASN.1/DER format), interoperable with Go/Python/Node/Rust SDKs:

```java
// Load keys from PEM
PrivateKey privateKey = Signing.loadPrivateKey(pemString);
PublicKey publicKey = Signing.loadPublicKey(pemString);

// Sign a packet
BusPacket signed = Signing.signPacket(packet, privateKey);

// Verify a packet
boolean valid = Signing.verifyPacketSignature(packet, publicKey);
```

## Metrics

Implement `Metrics` to integrate with Prometheus, OpenTelemetry, or any observability system:

```java
public interface Metrics {
    void onJobReceived(String jobId, String topic);
    void onJobCompleted(String jobId, long durationMs, String status);
    void onJobFailed(String jobId, String errorMsg);
    void onHeartbeatSent(String workerId);
    void onProgressEmitted(String jobId, int percent);
    void onError(String category, String message);
}
```

Default is `Metrics.NOOP` (zero overhead).

## Testing

Test handlers without a real NATS server:

```java
import io.cordum.cap.Testing;

Testing.MockBus bus = new Testing.MockBus();

JobRequest req = JobRequest.newBuilder()
    .setJobId("test-1")
    .setTopic("job.echo")
    .build();

JobResult result = Testing.submitAndWait(myHandler, req);
assertEquals(JobStatus.JOB_STATUS_SUCCEEDED, result.getStatus());
```

## Structure

- `io.cordum.cap` — SDK classes (Codec, Signing, Validate, Worker, Client, Agent, etc.)
- `io.cordum.cap.agent.v1` — Generated protobuf types (BusPacket, JobRequest, JobResult, etc.)
- Proto source: `proto/cordum/agent/v1/*.proto`

## API Reference

| Class | Description |
|-------|-------------|
| `Agent` | High-level runtime with handler registration, middleware, heartbeats |
| `Worker` | Low-level NATS subscriber with job dispatch |
| `Client` | Job submission helper |
| `Bus` | NATS connection factory |
| `Codec` | Deterministic protobuf serialization |
| `Signing` | ECDSA P-256 packet signing/verification |
| `Validate` | Input validation (JobRequest, JobResult, BusPacket) |
| `Heartbeat` | Heartbeat payload builders and emission loop |
| `Progress` | Progress/cancel payload builders and emission |
| `Middleware` | Composable interceptor chain |
| `Metrics` | Pluggable observability callbacks |
| `Errors` | Typed error codes and exception classes |
| `Testing` | MockBus, submitAndWait, RecordingMetrics |
