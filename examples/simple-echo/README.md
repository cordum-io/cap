# Simple Echo Example

These six canonical programs implement a **direct local-development transport lab** in Go,
Python, and Node. They demonstrate CAP protobuf envelopes, a NATS worker pool, result
validation, and correlation. They do not start or emulate a governed Cordum deployment.

## Direct Local-Development Topology

```text
client -- BusPacket{JobRequest} --> NATS job.echo --> echo worker
client <-- BusPacket{JobResult} --- NATS sys.job.result <-- echo worker
```

1. Before publishing, the client subscribes to `sys.job.result` and flushes the subscription.
2. It builds and validates a `BusPacket{JobRequest}` whose `topic` is `job.echo` and whose
   `context_ptr` is `demo://context/<job_id>`.
3. A prominent development-only path publishes the encoded packet directly to `job.echo`.
4. The worker consumes from `job.echo` and publishes a `BusPacket{JobResult}` to
   `sys.job.result` with `result_ptr` set to `demo://result/<job_id>`.
5. The client ignores malformed or unrelated global results, validates the matching packet
   and result, correlates both `trace_id` and `job_id`, and succeeds only for
   `JOB_STATUS_SUCCEEDED`.

The `demo://context/...` and `demo://result/...` values are illustrative opaque pointers.
This lab does not start a memory service, store content at those locations, or hydrate the
pointers. A real deployment defines the pointer scheme, authorization, retention, and data
lifecycle outside the wire protocol.

## Security and Reliability Boundary

Direct publish to `job.echo` deliberately bypasses the Gateway, Scheduler, Safety Kernel,
policy evaluation, authenticated identity, durable job state, and retry orchestration. The
example packets are unsigned. Matching `trace_id` and `job_id` prevents accepting an
unrelated result, but **correlation is not authentication**. Use this topology only for an
isolated local development lab.

## Governed Production Topology

Production clients use the stable SDK submit helpers and a running control plane:

```text
external client --> Gateway / trusted ingress --> sys.job.submit --> Scheduler
Scheduler --> Safety decision --> Scheduler --> job.<pool> --> worker
worker --> sys.job.result --> Scheduler / governed result handling
```

The low-level SDK submit helpers publish Scheduler ingress; they neither implement the
external Gateway nor authenticate a caller. The governed path authenticates at trusted
ingress, applies policy, records state, selects the pool, and owns retry behavior. Setting
`JobRequest.topic` does not make NATS forward a packet from `sys.job.submit` to the worker
subject; the Scheduler performs that dispatch.

## Canonical Programs

- Go: [`go-client/main.go`](go-client/main.go) and [`go-worker/main.go`](go-worker/main.go)
- Python: [`python-client/main.py`](python-client/main.py) and [`python-worker/main.py`](python-worker/main.py)
- Node: [`node-client/main.js`](node-client/main.js) and [`node-worker/main.js`](node-worker/main.js)

The [Getting Started guide](../../docs/getting-started.md) contains synchronized copies of
these files and the exact install/run commands. See also the concise
[sequence](sequence.md), illustrative [message objects](messages.json), and
[transport troubleshooting](../../docs/troubleshooting.md#5-job-stuck-in-pending).
