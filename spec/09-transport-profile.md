# Transport Profile

CAP is transport-agnostic but assumes a pub/sub fabric with subjects/topics and queue groups (competing consumers). This profile provides a recommended mapping for common transports.

## Subject/Topic Conventions
- Submission: `sys.job.submit`
- Results: `sys.job.result`
- Heartbeats: `sys.heartbeat`
- Alerts: `sys.alert`
- Work pools: `job.<pool>` (e.g., `job.code.llm`, `job.tools`, `job.image`)

## NATS Profile (recommended default)
- Use queue groups for pool subjects so multiple workers share load.
- Disable queue groups for `sys.heartbeat` so all schedulers see all heartbeats.
- Enable JetStream (or equivalent) for durability on `sys.job.submit` and `sys.job.result` when jobs are long-running or high-value.
- Assume at-least-once delivery; workers and schedulers MUST be idempotent on (`job_id`, pointers).
- Ack on successful handling; prefer pull/flow-control when the pool is large to avoid overload.
- Prefer small message bodies; large payloads must stay behind pointers.

## Kafka Profile (alternative)
- Map subjects to topics with a prefix (`cap.sys.job.submit`, `cap.job.code.llm`).
- Use consumer groups to represent pools.
- Deduplicate via `job_id` because Kafka provides at-least-once under failures.

## Delivery Expectations
- At-least-once delivery MUST be assumed; duplicate suppression belongs to consumers.
- Ordering is not guaranteed across partitions/subjects; state transitions must be idempotent.
- Producers SHOULD set reasonable timeouts and retries on publish; consumers SHOULD handle transient failures.
- Retry strategy: re-publish with the same `job_id` only if the workload is idempotent; otherwise create a new job and link via `parent_job_id`.
- Idempotency guidance: treat `job_id + result_ptr` as the idempotency key and avoid side effects if a result already exists.

## Security at Transport Layer
- Use TLS for all bus connections.
- Require authentication for publishers and subscribers; map identities to `sender_id`.
- Authorize subjects per role: gateways may publish submit; workers subscribe to pools; schedulers subscribe to submit/result.
