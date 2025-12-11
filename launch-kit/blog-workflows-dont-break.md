# Building LLM Workflows That Don’t Break Using CAP

Chaining tools is easy; keeping workflows reliable under load is hard. CAP standardizes the control plane so orchestrated jobs stay consistent.

## Common Failure Modes
- Ad-hoc job IDs and missing correlation.
- Orchestrators bypass safety checks for child jobs.
- Workers lose state; results overwrite each other.
- No visibility into per-step timing or failures.

## CAP’s Workflow Guardrails
- `workflow_id`, `parent_job_id`, `step_index` on every JobRequest.
- Shared `trace_id` across parent/child for observability.
- Safety Kernel invoked before dispatch of every child.
- State machine: `PENDING -> SCHEDULED -> DISPATCHED -> RUNNING -> {SUCCEEDED|FAILED|TIMEOUT|DENIED|CANCELLED}`.

## Pattern
1) Parent job submitted to `sys.job.submit`.  
2) Orchestrator consumes, emits child jobs with parent metadata.  
3) Scheduler routes children to `job.<pool>` subjects.  
4) Orchestrator aggregates `JobResult`s and emits the parent result.  
5) Clients watch state transitions via API or JobStore.

## Ops Tips
- Cascade cancellations from parent to children.
- Timeout policy per pool; reconcile stale jobs.
- Persist intermediate manifests via pointers for resumption.

## Takeaway
Workflows break when control planes are bespoke. CAP makes the control plane a standard so orchestrators can be swapped, audited, and scaled.
