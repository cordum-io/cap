# CAP Adoption Guide for Engineers

## 1) Understand the Wire
- Read `spec/00-index.md` and `proto/coretex/agent/v1/*.proto`.
- Note the required subjects: `sys.job.submit`, `job.<pool>`, `sys.job.result`, `sys.heartbeat`.

## 2) Stand Up the Basics
- Pick a bus (NATS recommended) and a memory store (Redis/S3/etc.).
- Implement SafetyKernel.Check (or stub) to return ALLOW/DENY/THROTTLE/HUMAN.
- Add a simple worker pool subject: `job.echo`.

## 3) Build a Worker
- Subscribe to `job.echo` (queue group).
- Read `context_ptr`, process, write `result_ptr`.
- Emit `BusPacket{JobResult}` with status and timing.

## 4) Add Scheduling + State
- Gateway publishes to `sys.job.submit`, writes context to memory.
- Scheduler processes submission, calls Safety, dispatches to pool, updates job state.
- Reconciler marks TIMEOUT/CANCELLED based on SLAs.

## 5) Add Observability
- Propagate `trace_id`.
- Emit metrics for submissions, dispatches, successes, failures, denials, timeouts.
- Monitor heartbeats and alert on missing workers per pool.

## 6) Move to Workflows
- Add orchestrator that emits child jobs with `workflow_id`, `parent_job_id`, `step_index`.
- Aggregate child results, emit parent `JobResult`.

## 7) Hardening
- Enforce TLS/mTLS on bus and Safety.
- Apply TTLs to pointers; avoid secrets in payloads/pointers.
- Load test: duplicate delivery, out-of-order results, Safety latency, heartbeat loss.

## 8) Ship
- Tag your deployment against a CAP release (v1.0).
- Document any vendor-specific extensions as optional fields.
- Contribute back tests/examples for your language or framework.
