# Transport and Deployment Profiles

CAP is transport-agnostic. It assumes a pub/sub system with named subjects/topics and competing consumers.

## Abstract Bus Requirements
- Subjects/topics with wildcard support (e.g., `sys.job.*`, `job.>`).
- Queue groups / competing consumers to balance work across a pool.
- At-least-once delivery recommended; idempotent consumers mitigate duplicates.
- Reasonable message size limits (payloads are envelopes only; contexts/results live off-bus).

## Deployment Profiles

### Single-node Profile
- Minimal setup: one gateway, one scheduler, a few workers.
- Pools may be implicit (one subject per worker type).
- State can be in-memory; timeouts and safety can be stubbed.

### Cluster Profile
- Multiple scheduler replicas and worker pools.
- Heartbeats required for load-aware routing.
- Safety Kernel invoked for every job.
- JobStore should be durable (Redis/SQL/etc.).
- Backpressure and timeouts enforced per topic/tenant.

## Transport Notes
- NATS is a natural fit (subjects, wildcards, queue groups), but CAP is not limited to NATS.
- Kafka or other brokers can be used if they support similar routing semantics and competing consumers.
- If the bus lacks wildcards, subjects can be emulated via naming conventions and subscription lists.
