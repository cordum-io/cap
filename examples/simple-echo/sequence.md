# Sequence

1. Client generates `job_id` and writes context to memory at `ctx:<job_id>` (e.g., `redis://ctx/<job_id>`).
2. Client publishes `BusPacket{JobRequest}` to `sys.job.submit`.
3. Scheduler receives request, records `PENDING -> SCHEDULED`, calls Safety Kernel.
4. Scheduler dispatches to pool subject `job.echo` (queue group load balances workers) and marks `DISPATCHED`.
5. Worker consumes, marks `RUNNING`, reads `context_ptr`, processes, writes result to `result_ptr` (e.g., `redis://res/<job_id>`).
6. Worker publishes `BusPacket{JobResult}` to `sys.job.result` with status `SUCCEEDED` (or `FAILED`).
7. Scheduler records terminal state and pointer; client fetches result via `result_ptr`.

Subjects used:
- Submission: `sys.job.submit`
- Worker pool: `job.echo`
- Results: `sys.job.result`

State transitions:
- PENDING → SCHEDULED → DISPATCHED → RUNNING → SUCCEEDED/FAILED
