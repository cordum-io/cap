# CAP C#/.NET SDK

C#/.NET SDK for the Cordum Agent Protocol (CAP). Provides NATS-based worker/client abstractions, ECDSA packet signing, deterministic serialization, heartbeat/progress helpers, middleware, and metrics hooks.

## Quick Start

### Prerequisites
- .NET 8.0+
- Protobuf compiler (`protoc`) — optional, `Grpc.Tools` handles compilation during build

### Build
```bash
cd sdk/dotnet
dotnet build
```

### Run Tests
```bash
dotnet test
```

### Generate Protobuf Types
Proto compilation is handled automatically by `Grpc.Tools` during `dotnet build`. For standalone generation:
```bash
CAP_RUN_CSHARP=1 ./tools/make_protos.sh
```

## Usage

### Agent (High-Level API)

```csharp
using Cordum.Cap;
using Cordum.Agent.V1;

await using var agent = await new AgentBuilder("worker-dotnet-1")
    .NatsUrl("nats://localhost:4222")
    .Pool("job.tools")
    .MaxParallel(4)
    .BuildAsync();

agent.Register("job.summarize", (ctx, req) =>
{
    return Task.FromResult(new JobResult
    {
        JobId = req.JobId,
        WorkerId = "worker-dotnet-1",
        Status = JobStatus.Succeeded,
        ResultPtr = "redis://result/" + req.JobId,
    });
});

await agent.StartAsync(CancellationToken.None);
```

### Worker (Low-Level API)

```csharp
var nc = await Bus.ConnectNats(new BusConfig { Url = "nats://localhost:4222" });

var worker = new Worker(nc, "job.echo", "worker-echo-1", (ctx, req) =>
{
    return Task.FromResult(new JobResult
    {
        JobId = req.JobId,
        WorkerId = "worker-echo-1",
        Status = JobStatus.Succeeded,
    });
});

await worker.StartAsync(CancellationToken.None);
```

### Client (Job Submission)

```csharp
var nc = await Bus.ConnectNats(new BusConfig { Url = "nats://localhost:4222" });
var client = new Client(nc);

var req = new JobRequest
{
    JobId = "job-1",
    Topic = "job.echo",
    ContextPtr = "redis://ctx/job-1",
};

await client.SubmitAsync(req, "trace-1", "client-dotnet", privateKey);
```

### Heartbeats

```csharp
// One-shot heartbeat
var payload = HeartbeatHelper.HeartbeatPayload("worker-1", "job.tools", 2, 8, 45.0);
await HeartbeatHelper.EmitHeartbeat(nc, payload);

// Heartbeat loop (periodic, background task)
await using var loop = HeartbeatHelper.StartLoop(
    nc,
    () => HeartbeatHelper.HeartbeatPayload("worker-1", "job.tools", activeJobs, 8, cpuLoad),
    "worker-1",
    TimeSpan.FromSeconds(5),
    metrics);
// ...
// loop is stopped and disposed via IAsyncDisposable
```

### Progress and Cancel

```csharp
var progress = ProgressHelper.ProgressPayload("worker-1", "job-1", "step-3", 75, "processing");
await ProgressHelper.EmitProgress(nc, progress);

var cancel = ProgressHelper.CancelPayload("client-1", "job-1", "timeout", "admin");
await ProgressHelper.EmitCancel(nc, cancel);
```

## Middleware

Add cross-cutting concerns without modifying handlers:

```csharp
// Built-in logging middleware
builder.WithMiddleware(MiddlewareExtensions.LoggingMiddleware());

// Custom middleware
builder.WithMiddleware(next => async (ctx, req) =>
{
    var sw = Stopwatch.StartNew();
    var result = await next(ctx, req);
    Console.WriteLine($"Job {req.JobId} took {sw.ElapsedMilliseconds}ms");
    return result;
});
```

Middleware executes in registration order (FIFO).

## Signing

ECDSA P-256 with SHA-256 (ASN.1/DER format), interoperable with Go/Python/Node/Java/Rust SDKs:

```csharp
using System.Security.Cryptography;
using Cordum.Cap;

// Load keys from PEM
ECDsa privateKey = Signing.LoadPrivateKey(pemString);
ECDsa publicKey = Signing.LoadPublicKey(pemString);

// Sign a packet
Signing.SignPacket(packet, privateKey);

// Verify a packet
bool valid = Signing.VerifyPacketSignature(packet, publicKey);
```

## Metrics

Implement `IMetricsHook` to integrate with Prometheus, OpenTelemetry, or any observability system:

```csharp
public interface IMetricsHook
{
    void OnJobReceived(string jobId, string topic);
    void OnJobCompleted(string jobId, long durationMs, string status);
    void OnJobFailed(string jobId, string errorMsg);
    void OnHeartbeatSent(string workerId);
    void OnProgressEmitted(string jobId, int percent);
    void OnError(string category, string message);
}
```

Default is `NoopMetrics` (zero overhead).

## Testing

Test handlers without a real NATS server:

```csharp
using Cordum.Cap;

var bus = new MockBus();

var req = new JobRequest
{
    JobId = "test-1",
    Topic = "job.echo",
};

var result = TestHelper.SubmitAndWait(myHandler, req);
Assert.Equal(JobStatus.Succeeded, result.Status);
```

## Structure

- `Cordum.Cap` — SDK classes (Codec, Signing, Validate, Worker, Client, Agent, etc.)
- `Cordum.Agent.V1` — Generated protobuf types (BusPacket, JobRequest, JobResult, etc.)
- Proto source: `proto/cordum/agent/v1/*.proto`

## API Reference

| Class | Description |
|-------|-------------|
| `AgentBuilder` / `Agent` | High-level runtime with handler registration, middleware, heartbeats |
| `Worker` | Low-level NATS subscriber with job dispatch |
| `Client` | Job submission helper |
| `Bus` | NATS connection factory |
| `Codec` | Deterministic protobuf serialization |
| `Signing` | ECDSA P-256 packet signing/verification |
| `Validate` | Input validation (JobRequest, JobResult, BusPacket) |
| `HeartbeatHelper` | Heartbeat payload builders and emission loop |
| `ProgressHelper` | Progress/cancel payload builders and emission |
| `MiddlewareExtensions` | Composable interceptor chain |
| `IMetricsHook` | Pluggable observability callbacks |
| `CapException` | Typed error codes and exception classes |
| `MockBus` / `TestHelper` | Testing utilities and recording metrics |
