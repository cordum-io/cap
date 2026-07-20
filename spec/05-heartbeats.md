# Heartbeats

Heartbeats advertise worker liveness, capacity, and pool membership so schedulers can route jobs intelligently.

## Semantics
- `worker_id`: stable identity for the worker process.
- `region`: location hint (region/zone/cluster) for locality-aware scheduling.
- `type`: capability class (cpu, gpu, cpu-tools, gpu-code, etc.).
- `cpu_load` / `memory_load` / `gpu_utilization`: utilization percentages (0-100).
- `active_jobs`: number of in-flight jobs on the worker.
- `capabilities`: freeform skills/tools supported (e.g., `python`, `browser`, `embedding`).
- `pool`: pool/subject this worker consumes (e.g., `job.code.llm`).
- `max_parallel_jobs`: advertised concurrency limit.
- `labels`: optional placement/routing metadata (e.g., `region`, `compliance`, `runtime`).
- `progress_pct`: optional task-level progress checkpoint (0-100).
- `last_memo`: optional short string/hash identifying the last successful internal step.
- `auth_token`: optional worker attestation credential presented by the worker to prove it matches a registered `WorkerCredential`. This is a dedicated protocol field and MUST NOT be copied into `labels`.
- `agent_name` (field 19): optional human-facing **display label** for the agent/worker (e.g., `Claude Code — Billing Bot`), surfaced in dashboards and audit summaries. SDKs sanitize and bound it (trim, collapse internal whitespace, drop control characters, cap at 128 characters). It is **not an authentication authority**: schedulers and audit consumers MUST prefer authenticated identity records (worker credential / Agent Identity) over this self-reported value and MUST NOT treat it as proof of identity. Additive/append-only; older workers omit it. It MUST NOT carry secrets, tokens, or PII.
- Tags: `checkpoint-heartbeat`, `progress`.

## Emission Rules
- Default interval SHOULD be 2-5 seconds; set lower for latency-sensitive pools.
- Heartbeats SHOULD be sent even when idle so schedulers can detect zero-load pools.
- Workers SHOULD stop heartbeats immediately before planned shutdown to allow drain.
- Heartbeats SHOULD be published to `sys.heartbeat` without queue groups so all schedulers/controllers see them.
- When emitting progress, workers SHOULD keep `last_memo` short and stable (e.g., step IDs or hashes).
- Workers SHOULD include `auth_token` only when provisioned with a worker credential. Implementations MUST treat it as sensitive, avoid logging it, and rely on transport security (for example TLS on the bus connection).

## Scheduler Behavior
- Treat absent heartbeats as worker loss after a grace window (e.g., 3x interval).
- Prefer workers with lower `active_jobs` and utilization when dispatching.
- Respect `max_parallel_jobs` to avoid overload; pause dispatch when active count meets or exceeds the limit.
- Use `capabilities`/`type` to honor pool-specific requirements (GPU-only pools, tool availability, etc.).
- When worker attestation is enabled, schedulers MUST validate `auth_token` against the credential store before using the heartbeat. A WARN migration may observe and count a heartbeat that omits the credential as explicitly unauthenticated telemetry, but it MUST NOT refresh authenticated liveness, readiness, pool membership, capacity, or dispatch authority. A heartbeat that presents an invalid, expired, revoked, or mismatched credential MUST be rejected even in WARN; presenting bad proof is not legacy omission.
- `agent_name` is informational only: schedulers and audit pipelines MUST NOT use it for authorization or identity resolution, and MUST prefer authenticated records (worker credential / Agent Identity) when both are present.
