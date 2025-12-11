# CAP Go SDK

Go SDK with NATS helpers for workers and clients. Uses generated protobuf stubs from `proto/`.

## Quick Start
1. Generate protobuf stubs (once per change):
   ```bash
   ./tools/make_protos.sh
   ```
   This writes Go stubs to `generated/go`. The module uses a replace directive to point at that path.

2. Install deps and run tests:
   ```bash
   cd sdk/go
   go test ./...
   ```

## Structure
- `go.mod` — module + replace for generated protos.
- `bus/` — NATS connector.
- `worker/` — worker skeleton with handler signature.
- `client/` — submission/result helpers.
- `types.go` — common constants and helper functions.

## Usage
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

## Notes
- The protobuf `go_package` is `github.com/cortex-agent-protocol/go/gen/cortex/agent/v1`; the replace in `go.mod` points to `../../generated/go`.
- Swap the NATS adapter if you prefer another bus; only `bus/` needs to change.
