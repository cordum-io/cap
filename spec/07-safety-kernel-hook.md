# Safety Kernel Hook

The Safety Kernel is a logical policy decision point invoked before dispatching a job. It can allow, deny, require human review, or throttle based on tenant, topic, priority, budget, or content signals.

## Inputs
- `job_id`
- `topic`
- `tenant`
- `priority`
- Optional: estimated cost, sandbox flags, content hash pointers.

## Outputs
- `decision`: ALLOW, DENY, REQUIRE_HUMAN, THROTTLE.
- `reason`: human-readable explanation.
- `redacted_context_ptr`: optional pointer to a sanitized context payload.

## Transport
- CAP does not mandate a transport. gRPC is recommended for low-latency calls; see `proto/cortex/agent/v1/safety.proto`.
- Implementations may also use REST or embedded policy engines as long as the semantics match.

## Flow
1. Scheduler (or gateway) receives `JobRequest` on `sys.job.submit`.
2. Scheduler calls Safety Kernel with job metadata.
3. Safety Kernel returns a decision:
   - **ALLOW**: scheduler continues to pool selection and dispatch.
   - **DENY**: scheduler marks job `DENIED` and emits a `JobResult`/alert.
   - **REQUIRE_HUMAN**: scheduler pauses or routes to a human queue (implementation-defined).
   - **THROTTLE**: scheduler defers or re-enqueues based on policy.
4. The decision and reason should be recorded in the JobStore for audit and surfaced to clients.

## Recommendations
- Fail-closed: timeouts or errors should default to `DENY` or `THROTTLE` based on risk appetite.
- Policies should be tenant-aware and topic-aware.
- Consider redaction: Safety Kernel can return a pointer to a redacted context to reduce data exposure downstream.
