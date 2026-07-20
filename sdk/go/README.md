# CAP Go SDK

Go SDK with NATS helpers for workers and clients. Uses generated protobuf stubs from `proto/`.

## Quick Start
1. Generate protobuf stubs (once per change):
   ```bash
   ./tools/make_protos.sh
   ```
   This writes Go stubs to `/cordum/agent/v1`.

2. Install deps and run tests:
   ```bash
   go test ./sdk/go/...
   ```

## Structure
- Go module: `github.com/cordum-io/cap/v2` (see root `go.mod`).
- `bus/` — NATS connector.
- `worker/` — worker skeleton with handler signature.
- `client/` — submission/result helpers.
- `types.go` — common constants and helper functions.

## Usage

### Runtime (High-Level SDK)
The runtime hides NATS/Redis plumbing and gives you typed handlers.

```go
type Input struct {
    Prompt string `json:"prompt"`
}

type Output struct {
    Summary string `json:"summary"`
}

agent := &runtime.Agent{
    Retries:       2,
    HandshakeMode: runtime.HandshakeModeOff, // local legacy compatibility only
}

runtime.Register(agent, "job.summarize", func(ctx runtime.Context, input Input) (Output, error) {
    return Output{Summary: input.Prompt[:140]}, nil
})

if err := agent.Start(); err != nil {
    log.Fatal(err)
}
select {}
```

Environment:
- `NATS_URL` (default `nats://127.0.0.1:4222`)
- `REDIS_URL` (default `redis://127.0.0.1:6379/0`)

### Authenticated worker trust

In this unreleased source tree, production workers should use `HandshakeModeWarn` only for a measured migration
and `HandshakeModeEnforce` for fail-closed admission. Both require a complete
`capsdk.WorkerTrustConfig`: enrolled worker/agent/tenant identities, exact
`capsdk.WorkerHandshakeAudience`, an active P-256 proof key ID/private key,
expected scheduler identity, pinned scheduler public keys, and SDK version.
`CORDUM_SDK_HANDSHAKE` can supply the mode. An unset mode preserves legacy
`off`, while an unknown non-empty value fails startup. Off mode rejects dormant
trust material and is compatibility-only.

The runtime performs bounded protobuf request/reply on
`sys.worker.handshake.challenge` and `sys.worker.handshake.authenticate` before subscriptions, verifies the pinned
scheduler challenge/result, installs the short-lived opaque session, attaches
it before signing outbound packets, and renews with the current token. Renewal
never falls back to ISSUE; expiry, revocation, supersession, or a binding or
audience mismatch invalidates the session.

Proof-key enrollment/rotation/revocation is a control-plane operation. Register
only the worker public key, overlap scheduler pins during rotation, and never
put private keys or tokens in logs. The public low-level trust builders/codecs/
signers/verifiers support custom adapters and compatibility testing; they do
not enroll keys, issue or revoke sessions, authorize topics, or turn a legacy
`sys.handshake` capability advertisement into authenticated identity. See
[`runtime/README_HANDSHAKE.md`](runtime/README_HANDSHAKE.md).

### Heartbeats
- `HeartbeatPayload` builds a heartbeat with CPU load only.
- `HeartbeatPayloadWithMemory` includes both CPU and memory utilization.

```go
nc, _ := bus.Connect(bus.Config{URL: "nats://127.0.0.1:4222"})
defer nc.Close()

w := worker.Worker{
    NATS: nc,
    Subject: "job.echo",
    Handler: func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
        // read from req.ContextPtr, produce result_ptr elsewhere
        return &agentv1.JobResult{
            JobId: req.JobId,
            Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
            ResultPtr: "redis://res/" + req.JobId,
            WorkerId: "worker-echo-1",
        }, nil
    },
}
go w.Start()
// publish jobs with client.Submit(...)
```

Client submit example:
```go
req := &agentv1.JobRequest{
    JobId:      "job-echo-1",
    Topic:      "job.echo",
    ContextPtr: "redis://ctx/job-echo-1",
}
if err := client.Submit(context.Background(), nc, req, "trace-1", "client-go", privateKey); err != nil {
    log.Fatal(err)
}
```

If the context is already canceled or past its deadline, `Submit` returns `ctx.Err()` without publishing.

### ManagedWorker (Batteries-Included)
The `worker.ManagedWorker` is a higher-level abstraction than `worker.Worker`. It handles the full lifecycle:
- **Auto-connect**: Connects to NATS with optional TLS from environment.
- **Handshake**: Publishes capabilities to `sys.handshake` on startup.
- **Heartbeats**: Maintains a background heartbeat loop on `sys.heartbeat`.
- **Panic Recovery**: Recovers from handler panics and reports them as `JOB_STATUS_FAILED`.
- **Concurrency**: Controls parallel job execution via `MaxParallelJobs`.
- **Graceful Shutdown**: Drains NATS and waits for in-flight jobs to finish on `Close()`.

```go
mgr, _ := worker.NewManagedWorker(worker.ManagedConfig{
    Type:            "summarizer",
    MaxParallelJobs: 4,
    Capabilities:    []string{"text-processing", "nlp"},
    WorkerTrustMode: capsdk.WorkerTrustModeOff, // local legacy compatibility only
})

err := mgr.Run(ctx, func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
    // handler logic
    return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
})
```

ManagedWorker automatically calls `NATSTLSConfigFromEnv()` if `NATSTLSConfig` is not provided in the config.
For authenticated deployments, replace `WorkerTrustModeOff` with
`WorkerTrustModeWarn` or `WorkerTrustModeEnforce` and supply `WorkerTrust` plus a
bounded `WorkerTrustTimeout`. Its capability topics can narrow, but never expand,
the topics authorized by the control plane. Warn mode fails open only for
operational request unavailability; malformed, unpinned, tampered, mismatched,
or rejected trust responses stop admission. Privileged results require a live
session in both trust modes, while tokenless warn telemetry remains observable.

### TLS Helpers
CAP Go SDK provides helpers to build `*tls.Config` from standard environment variables. These are used by `ManagedWorker` and the `runtime.Agent` by default.

#### NATS TLS
`NATSTLSConfigFromEnv()` reads:
- `NATS_TLS_CA`: Path to CA certificate PEM.
- `NATS_TLS_CERT` / `NATS_TLS_KEY`: Path to client certificate/key pair (must be set together).
- `NATS_TLS_SERVER_NAME`: SNI server name override.
- `NATS_TLS_INSECURE`: Set to `1` or `true` to skip certificate verification (dev only).

#### Redis TLS
`RedisTLSConfigFromEnv()` reads:
- `REDIS_TLS_CA`: Path to CA certificate PEM.
- `REDIS_TLS_CERT` / `REDIS_TLS_KEY`: Path to client certificate/key pair.
- `REDIS_TLS_SERVER_NAME`: SNI server name override.
- `REDIS_TLS_INSECURE`: Set to `1` or `true` to skip certificate verification.

### Job Options
The high-level `runtime` supports per-job configuration using the Option pattern:

```go
runtime.Register(agent, "job.risky", myHandler, runtime.WithRetries(5))
```

- `WithRetries(n)`: Overrides the agent's default retry count for a specific topic.

## Testing

The `testing` package lets you test handlers without running NATS or Redis.

```go
import (
    "testing"
    agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
    captesting "github.com/cordum-io/cap/v2/sdk/go/testing"
)

func TestEchoHandler(t *testing.T) {
    req := &agentv1.JobRequest{JobId: "test-1", Topic: "job.echo"}
    result, err := captesting.SubmitAndWait(myHandler, req)
    if err != nil {
        t.Fatal(err)
    }
    if result.Status != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
        t.Fatalf("unexpected status: %v", result.Status)
    }
}
```

- `captesting.InMemoryBus` — implements `NATSConn` without NATS.
- `captesting.SubmitAndWait(handler, request)` — runs a low-level worker handler and returns the result.
- `captesting.SubmitToRuntime(bus, request)` — sends a request to a runtime `Agent` wired to an `InMemoryBus`.

## Middleware

Add cross-cutting concerns (logging, auth, metrics) without modifying handlers:

```go
// Built-in logging middleware
agent.Use(runtime.LoggingMiddleware(logger))

// Custom middleware
agent.Use(func(next runtime.HandlerFunc) runtime.HandlerFunc {
    return func(ctx runtime.Context, data any) (any, error) {
        ctx.Logger.Printf("before job %s", ctx.Job.GetJobId())
        out, err := next(ctx, data)
        ctx.Logger.Printf("after job %s", ctx.Job.GetJobId())
        return out, err
    }
})
```

Middleware executes in registration order (FIFO). Each can inspect context,
measure timing, or short-circuit by returning without calling `next`.

## Signing
- `client.Submit` and `worker.Worker` sign envelopes when you pass a non-nil ECDSA private key (P-256); configure `PublicKeys` to verify incoming packets when you want authenticity enforcement.
- Signatures are computed over deterministic protobuf serialization (map entries ordered by key) to ensure cross-SDK verification.
- Generate a keypair in Go:
  ```go
  priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
  pub := &priv.PublicKey
  ```
- Unsigned legacy transport is explicit: set `AllowUnsigned: true` on Go
  clients/workers/runtimes. Package-level callers use `SubmitUnsigned`;
  ordinary `Submit` rejects a nil private key.

## Generating API Docs

Go exports are automatically rendered on [pkg.go.dev](https://pkg.go.dev/github.com/cordum-io/cap/v2/sdk/go).
To browse docs locally:

```bash
go doc ./sdk/go/...
```

## Observability

### Structured Logging
The runtime Agent and Worker use `*slog.Logger` (stdlib) for structured logging. All log calls include contextual fields (`job_id`, `trace_id`, `topic`, `sender_id`). Pass a custom logger or leave nil for the default:

```go
agent := &runtime.Agent{
    Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
}
```

### MetricsHook
Implement `capsdk.MetricsHook` to integrate with Prometheus, OpenTelemetry, or any metrics system:

```go
type MetricsHook interface {
    OnJobReceived(jobID, topic string)
    OnJobCompleted(jobID string, durationMs int64, status string)
    OnJobFailed(jobID string, errorMsg string)
    OnHeartbeatSent(workerID string)
}
```

The default is `NoopMetrics` (zero overhead). Example Prometheus integration:

```go
type promMetrics struct {
    jobsReceived  *prometheus.CounterVec
    jobDuration   *prometheus.HistogramVec
}

func (m *promMetrics) OnJobReceived(jobID, topic string) {
    m.jobsReceived.WithLabelValues(topic).Inc()
}
func (m *promMetrics) OnJobCompleted(jobID string, durationMs int64, status string) {
    m.jobDuration.WithLabelValues(status).Observe(float64(durationMs))
}
func (m *promMetrics) OnJobFailed(jobID, errorMsg string) {
    m.jobsReceived.WithLabelValues("failed").Inc()
}
func (m *promMetrics) OnHeartbeatSent(workerID string) {}

agent := &runtime.Agent{
    Metrics: &promMetrics{...},
}
```

The `trace_id` is propagated through all log and metrics calls for distributed tracing correlation.


## Notes
- The protobuf `go_package` is `github.com/cordum-io/cap/v2/cordum/agent/v1`.
- Swap the NATS adapter if you prefer another bus; only `bus/` needs to change.
