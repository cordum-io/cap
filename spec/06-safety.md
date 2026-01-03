# Safety

CAP makes safety a first-class control-plane hook via the Safety Kernel.

## Decision Model
- Outcomes: `ALLOW`, `DENY`, `REQUIRE_HUMAN`, `THROTTLE`.
- Inputs: `job_id`, `topic`, `tenant`, `principal_id`, `priority`, `budget`, optional `estimated_cost`, `labels`, `memory_id`, and optional `effective_config` (marshaled EffectiveConfig for the request context).
- Outputs: decision, human-readable `reason`, optional `redacted_context_ptr`, and optional policy metadata (`policy_snapshot`, `rule_id`, `constraints`, `approval_required`, `approval_ref`).

## Canonical Service (see `proto/coretex/agent/v1/safety.proto`)
```proto
service SafetyKernel {
  rpc Check(PolicyCheckRequest) returns (PolicyCheckResponse);
  rpc Evaluate(PolicyCheckRequest) returns (PolicyCheckResponse);
  rpc Explain(PolicyCheckRequest) returns (PolicyCheckResponse);
  rpc Simulate(PolicyCheckRequest) returns (PolicyCheckResponse);
  rpc ListSnapshots(ListSnapshotsRequest) returns (ListSnapshotsResponse);
}
```

## Scheduler/Gateway Requirements
- Call SafetyKernel before dispatching a `JobRequest` into a pool subject.
- If `DENY`, set job state to `DENIED` and emit a `JobResult` with `status=JOB_STATUS_DENIED` and an error message.
- If `REQUIRE_HUMAN`, pause dispatch and surface state to the client; allow an out-of-band approval path to resume.
- If `THROTTLE`, delay or queue based on policy; retry the check after the backoff window.
- If `redacted_context_ptr` is provided, schedulers SHOULD dispatch using the redacted pointer in place of the original.

## Performance and Reliability
- Safety checks SHOULD complete quickly (<250ms) to avoid head-of-line blocking.
- Cache positive decisions when appropriate, keyed by `tenant+topic+adapter_id+priority`.
- On SafetyKernel outage, schedulers MAY fail-closed (preferred) or fail-open by explicit operator policy; the chosen behavior MUST be documented.

## Auditing
- Log every decision with `trace_id`, `job_id`, `decision`, and `reason` for downstream observability and compliance.
- Include the safety decision in job metadata for later reconciliation.
