# Try CAP in 5 Minutes

See the Cordum Agent Protocol in action — no Cordum stack required. This playground runs a minimal CAP job round-trip using just NATS and the Python SDK.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (with Compose)

## Run It

```bash
cd playground
docker compose up --build
```

You'll see:

1. **NATS** starts as the message bus
2. **echo-worker** connects and subscribes to `job.echo`
3. **submit** sends a `BusPacket{JobRequest}` to the worker
4. **echo-worker** processes the job and publishes a `BusPacket{JobResult}`
5. **submit** receives the result and prints it

```
submit  | Submitting job playground-a1b2c3d4 (trace=trace-e5f6g7h8)
submit  |   topic:       job.echo
submit  |   context_ptr: inline://hello-from-cap-playground
worker  | Received job playground-a1b2c3d4 (topic=job.echo)
worker  | Completed job playground-a1b2c3d4 -> SUCCEEDED
submit  | Result received!
submit  |   status:     JOB_STATUS_SUCCEEDED
submit  |   result_ptr: echo://inline://hello-from-cap-playground
============================================================
  CAP Playground Demo Complete!
============================================================
```

## Clean Up

```bash
docker compose down
```

## What Just Happened?

```
Submitter                    NATS                    Echo Worker
    |                          |                          |
    |-- BusPacket{JobRequest} -|-> job.echo ------------->|
    |                          |                          |
    |                          |<- BusPacket{JobResult} --|
    |<-- sys.job.result -------|                          |
```

1. The **submitter** created a `BusPacket` containing a `JobRequest` and published it to the `job.echo` subject
2. The **echo worker** received the packet, processed the job, and published a `BusPacket{JobResult}` to `sys.job.result`
3. The **submitter** received the result and printed the outcome

This is the core CAP lifecycle — jobs flow through a bus as protobuf `BusPacket` envelopes. In a full Cordum deployment, a **Scheduler** and **Safety Kernel** sit between submit and dispatch, adding policy enforcement, pool routing, and heartbeat monitoring.

## Next Steps

- [Getting Started Guide](../docs/getting-started.md) — build your own worker
- [Full Spec](../spec/00-index.md) — protocol details
- [Examples](../examples/) — more patterns (workflows, heartbeats)
- [SDK Comparison](../docs/sdk-comparison.md) — pick your language
