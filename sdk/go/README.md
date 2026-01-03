# CAP Go SDK

Go SDK with NATS helpers for workers and clients. Uses generated protobuf stubs from `proto/`.

## Quick Start
1. Generate protobuf stubs (once per change):
   ```bash
   ./tools/make_protos.sh
   ```
   This writes Go stubs to `/coretex/agent/v1`.

2. Install deps and run tests:
   ```bash
   go test ./sdk/go/...
   ```

## Structure
- Go module: `github.com/coretexos/cap/v2` (see root `go.mod`).
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

## Signing
- `client.Submit` and `worker.Worker` sign envelopes when you pass a non-nil ECDSA private key (P-256); configure `PublicKeys` to verify incoming packets when you want authenticity enforcement.
- Generate a keypair in Go:
  ```go
  priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
  pub := &priv.PublicKey
  ```
 - Pass `nil` as the private key to send unsigned envelopes.

## Notes
- The protobuf `go_package` is `github.com/coretexos/cap/v2/coretex/agent/v1`.
- Swap the NATS adapter if you prefer another bus; only `bus/` needs to change.
