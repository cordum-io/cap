# Security and Observability

## Security
- All jobs traverse a control plane that can enforce RBAC, budgets, and allow/deny policies via the Safety Kernel.
- Pointer-based memory keeps large or sensitive payloads off the bus, reducing the risk of data exfiltration through transit logs.
- Tenants and topics should be validated at submission; Safety decisions and reasons should be recorded in the JobStore for audit.
- Transport-level security (TLS/mTLS) is recommended for bus, Safety Kernel, and memory access.

## Observability
- Correlate events using `trace_id` and `job_id` across JobRequest, JobResult, and heartbeats.
- Suggested metrics (labels: topic, pool, tenant, decision):
  - jobs_received, jobs_dispatched, jobs_completed, jobs_failed, jobs_denied, jobs_timeout
  - worker_active_jobs, worker_max_parallel_jobs
  - safety_decisions_total
- Suggested logs:
  - Safety decisions with reason and tenant.
  - State transitions (old_state, new_state, job_id, topic, worker_id).
  - Heartbeat ingestion and expiry events.
- CAP does not mandate a metrics format; Prometheus, OpenTelemetry, or vendor systems are all acceptable.
