# Simple Echo Example

Scenario: A client submits a small prompt to an echo worker pool (`job.echo`) via a gateway and scheduler. Context and result live in external memory.

## Flow
1) Client writes input context to memory (e.g., Redis) under `ctx:<job_id>`.
2) Client publishes `BusPacket{JobRequest}` to `sys.job.submit`.
3) Scheduler receives the request, calls Safety Kernel, and dispatches to `job.echo` (queue group for the pool).
4) Worker consumes the job, reads `context_ptr`, writes a result to `result_ptr`, and publishes `JobResult` to `sys.job.result`.
5) Scheduler updates state; client reads the result via the pointer.

## Sequence Diagram
```mermaid
sequenceDiagram
    participant Client
    participant Mem as Memory
    participant Bus
    participant Sched as Scheduler
    participant Worker

    Client->>Mem: write ctx:<job_id>
    Client->>Bus: BusPacket{JobRequest} -> sys.job.submit
    Bus->>Sched: JobRequest
    Sched->>Sched: Safety check
    Sched->>Bus: dispatch -> job.echo
    Bus->>Worker: JobRequest (queue group)
    Worker->>Mem: read context_ptr
    Worker->>Mem: write result_ptr
    Worker->>Bus: BusPacket{JobResult} -> sys.job.result
    Bus->>Sched: JobResult (state update)
    Client->>Mem: read result_ptr
```
