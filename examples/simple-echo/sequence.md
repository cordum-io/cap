# Simple Echo Sequence

## Direct Local-Development Lab

1. The client generates `job_id` and `trace_id`, subscribes to `sys.job.result`, and flushes
   that subscription before any request can be published.
2. It creates and validates `BusPacket{JobRequest}` with `topic=job.echo` and the opaque
   illustrative pointer `demo://context/<job_id>`.
3. The client encodes the packet as protobuf and publishes it directly to the NATS subject
   `job.echo`.
4. One echo worker in the `job.echo` queue group consumes the request and creates a result
   whose opaque illustrative pointer is `demo://result/<job_id>`.
5. The worker publishes `BusPacket{JobResult}` to `sys.job.result`.
6. The client ignores malformed and unrelated traffic, validates the result, requires both
   `trace_id` and `job_id` to match, and exits successfully only for
   `JOB_STATUS_SUCCEEDED`.

Subjects used by this lab:

- Direct request: `job.echo`
- Result: `sys.job.result`

No memory service resolves the `demo://` pointers, and no durable state transitions are
recorded. The lab starts no Gateway, Scheduler, Safety Kernel, policy engine,
authentication authority, durable state store, or retry controller. Correlation by
`trace_id` and `job_id` is not authentication.

## Governed Production Contrast

1. An external caller enters through a Gateway or other trusted ingress, which authenticates
   the caller and authorizes access to the governed path.
2. A trusted component can use a low-level SDK submit helper to encode and publish Scheduler
   ingress at `sys.job.submit`; the helper is not itself a Gateway or authentication layer.
3. The Scheduler consumes that subject, records state, obtains a Safety decision, and
   dispatches the request to `job.<pool>`.
4. The worker publishes its result to `sys.job.result`; the governed control plane records
   the terminal state and applies its result-access contract.

`JobRequest.topic` tells the Scheduler which pool is requested. It does not reroute a packet
already published to `sys.job.submit`; without the governed components, the packet is not
forwarded to `job.echo`.
