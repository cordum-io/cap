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
- Enable persistence/streaming when available for durability; at-least-once delivery is acceptable with idempotent consumers.
- Prefer small message bodies; large payloads must stay behind pointers.

## Kafka Profile (alternative)
- Map subjects to topics with a prefix (`cap.sys.job.submit`, `cap.job.code.llm`).
- Use consumer groups to represent pools.
- Deduplicate via `job_id` because Kafka provides at-least-once under failures.

## Delivery Expectations
- At-least-once delivery MUST be assumed; duplicate suppression belongs to consumers.
- Ordering is not guaranteed across partitions/subjects; state transitions must be idempotent.
- Producers SHOULD set reasonable timeouts and retries on publish; consumers SHOULD handle transient failures.

## Security at Transport Layer
- Use TLS for all bus connections.
- Require authentication for publishers and subscribers; map identities to `sender_id`.
- Authorize subjects per role: gateways may publish submit; workers subscribe to pools; schedulers subscribe to submit/result.
