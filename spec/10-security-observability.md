# Security and Observability

Security and visibility are mandatory for production deployments of CAP. For a more detailed guide to security best practices, see [11 Security Best Practices](11-security-best-practices.md).

## Authentication and Authorization
- Bus connections MUST be authenticated (mTLS, tokens) and authorized per subject.
- `sender_id` SHOULD map to an authenticated principal for auditability.
- Gateways SHOULD validate client identity before accepting submissions.
- Worker attestation SHOULD use a control-plane-issued worker credential. The worker presents the credential as `Heartbeat.auth_token`, and the scheduler validates it against the credential store before treating the worker identity as authoritative.
- Runtime session authentication MAY use `BusPacket.auth_token` after a successful handshake or scheduler-issued session establishment. SDK/runtime code attaches this trusted context to outbound packets; gateways and schedulers MUST NOT copy untrusted caller-supplied values into it.
- `auth_token` is sensitive whether it appears on `Heartbeat` or the enclosing `BusPacket`. Implementations MUST NOT place it in labels, logs, traces, or metrics dimensions.

## Data Protection
- Keep payloads out of the bus; pointers SHOULD reference data protected by access control (scoped tokens, signed URLs, or per-tenant credentials).
- Encrypt data at rest in the memory fabric when supported by the backend.
- Avoid embedding secrets in pointers or `env`; use short-lived credentials instead.

## Safety and Privacy
- Use the Safety Kernel for policy enforcement (deny, throttle, human-review).
- Redact or hash sensitive fields in logs; prefer `redacted_context_ptr` for sanitized copies.

## Observability
- Metrics: emit counters for submissions, dispatches, successes, failures, denials, timeouts, and safety decisions; track latency buckets for submission->dispatch and dispatch->result.
- Tracing: propagate a stable `trace_id` across gateway, scheduler, worker, and orchestrator spans; child jobs SHOULD reuse the parent `trace_id`.
- Workflow topology: include `workflow_id`, `parent_job_id`, and `step_index` as attributes to reconstruct DAGs without inspecting payloads.
- Logging: log state transitions with `job_id`, `trace_id`, `status`, `worker_id`, `pool`, `decision`, and `latency_ms`.
- Heartbeat monitoring: alert on missing heartbeats per pool/region; track utilization trends.
- Attestation observability: record whether a heartbeat was attested, unattested, or invalid, but never record the raw `auth_token`. Handshake registries SHOULD also persist `ready_topics` so operators can distinguish “worker connected” from “worker ready for topic X”.

## Worker Attestation Flow

1. Control plane (for example pack install or external worker registration) creates a worker credential and stores only its hashed form in the credential store.
2. The plaintext token is delivered out-of-band to the worker operator/process configuration.
3. The worker includes that token in `Heartbeat.auth_token` on periodic heartbeats.
4. The scheduler checks the presented token against the credential store before trusting the worker identity for routing and accounting.
5. During migration, deployments MAY run in warn/off modes so older workers without `auth_token` continue to operate while operators roll out credentials.

Runtime sessions can additionally issue a short-lived session token after handshake. That token rides on `BusPacket.auth_token` so schedulers can validate job results, progress, cancellation acknowledgements, and other bus packets without repeating the full attestation exchange for each packet.

## Compliance and Retention
- Configure TTLs for contexts/results per tenant; purge expired data regularly.
- Keep audit logs of safety decisions and job state transitions for a policy-defined retention window.
