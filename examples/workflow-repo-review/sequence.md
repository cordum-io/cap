# Sequence

1. Client writes repo review context to `ctx:<parent_job_id>` (includes repo URL, branch, limits) and publishes parent `JobRequest` to `sys.job.submit` with topic `job.workflow.repo.review`.
2. Scheduler records `PENDING -> SCHEDULED`, calls Safety Kernel, dispatches to orchestrator pool, marks `DISPATCHED`.
3. Orchestrator consumes parent, reads context, and emits child `JobRequest`s to `sys.job.submit`:
   - Child A: `job.repo.scan` (step_index=0) with `parent_job_id` set.
   - Child B: `job.repo.partition` (step_index=1) using scan output pointer.
   - Child C-N: `job.code.llm` per batch (step_index>=2).
   - Optional summary child: `job.chat.simple` for report text.
4. Scheduler and Safety process each child; workers execute and publish `JobResult`s to `sys.job.result`.
5. Orchestrator tracks child completions (via JobStore or bus), aggregates results, and writes a report to `res:<parent_job_id>`.
6. Orchestrator publishes parent `JobResult` (status `SUCCEEDED` or `FAILED`) to `sys.job.result`.
7. Client reads report from `result_ptr`.

Subjects used:
- Submission: `sys.job.submit`
- Parent pool: `job.workflow.repo.review`
- Child pools: `job.repo.scan`, `job.repo.partition`, `job.code.llm`, `job.chat.simple`
- Results: `sys.job.result`

State transitions (parent): PENDING → SCHEDULED → DISPATCHED → RUNNING → SUCCEEDED/FAILED
State transitions (children): PENDING → SCHEDULED → DISPATCHED → RUNNING → SUCCEEDED/FAILED
