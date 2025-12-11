# Security and Observability

Security and visibility are mandatory for production deployments of CAP.

## Authentication and Authorization
- Bus connections MUST be authenticated (mTLS, tokens) and authorized per subject.
- `sender_id` SHOULD map to an authenticated principal for auditability.
- Gateways SHOULD validate client identity before accepting submissions.

## Data Protection
- Keep payloads out of the bus; pointers SHOULD reference data protected by access control (scoped tokens, signed URLs, or per-tenant credentials).
- Encrypt data at rest in the memory fabric when supported by the backend.
- Avoid embedding secrets in pointers or `env`; use short-lived credentials instead.

## Safety and Privacy
- Use the Safety Kernel for policy enforcement (deny, throttle, human-review).
- Redact or hash sensitive fields in logs; prefer `redacted_context_ptr` for sanitized copies.

## Observability
- Metrics: emit counters for submissions, dispatches, successes, failures, denials, timeouts, and safety decisions; track latency buckets for submission->dispatch and dispatch->result.
- Tracing: propagate `trace_id` across gateway, scheduler, worker, and orchestrator spans; include pointer URIs as attributes, not payloads.
- Logging: log state transitions with `job_id`, `trace_id`, `status`, `worker_id`, `pool`, `decision`, and `latency_ms`.
- Heartbeat monitoring: alert on missing heartbeats per pool/region; track utilization trends.

## Compliance and Retention
- Configure TTLs for contexts/results per tenant; purge expired data regularly.
- Keep audit logs of safety decisions and job state transitions for a policy-defined retention window.
