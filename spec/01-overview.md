# Overview

## What CAP Is
- A distributed job protocol for AI agents that communicates over a pub/sub bus.
- Standardizes envelopes, job messages, heartbeats, and workflow metadata so schedulers, workers, orchestrators, and gateways can interoperate.
- Protocol-first: protobuf is the contract; these pages define semantics and required behavior for compatibility.

## Goals and Scope
- Enable multi-agent, multi-node execution with consistent lifecycle tracking.
- Keep payloads off the bus by mandating opaque pointers for inputs/outputs.
- Make safety and policy enforcement first-class via a dedicated hook.
- Remain transport-agnostic while providing a recommended subject profile.
- Support orchestrated workflows without changing the core job abstraction.

## Non-goals
- Not a model API or prompt format; CAP is vendor/model agnostic.
- Not a UI or product; CAP only defines the wire contract and semantics.
- Not tied to a single transport; any pub/sub with subjects/topics and queue groups works.

## Actors
- **Client**: originates work and reads results from external memory.
- **Gateway**: front-door that validates, writes context to memory, and publishes `JobRequest` packets.
- **Scheduler**: owns the state machine, calls safety, chooses pool/subject, and dispatches jobs.
- **Worker / Pool**: consumes `JobRequest` subjects, executes work, writes results, and emits `JobResult`.
- **Orchestrator**: a specialized worker that spawns child jobs and aggregates results.
- **Safety Kernel**: policy decision point (gRPC or equivalent) invoked before dispatch.
- **Memory Fabric**: external storage for context/result pointers (Redis, object storage, DB).
- **Bus**: pub/sub fabric carrying `BusPacket` envelopes.

## Architecture Diagram

```mermaid
graph TD
    subgraph "External"
        Client
    end

    subgraph "CAP System"
        Gateway
        Bus
        Scheduler
        WorkerPool[Worker Pool]
        Orchestrator
        SafetyKernel[Safety Kernel]
        MemoryFabric[Memory Fabric]
    end

    Client -- "1. Submit Job (with context)" --> Gateway
    Gateway -- "2. Write context to Memory Fabric" --> MemoryFabric
    Gateway -- "3. Publish JobRequest" --> Bus
    Bus -- "4. Forward JobRequest" --> Scheduler
    Scheduler -- "5. Call Safety Kernel" --> SafetyKernel
    SafetyKernel -- "6. Return Decision" --> Scheduler
    Scheduler -- "7. Dispatch Job" --> Bus
    Bus -- "8. Forward Job to Worker" --> WorkerPool
    WorkerPool -- "9. Read context" --> MemoryFabric
    WorkerPool -- "10. Execute Job" --> WorkerPool
    WorkerPool -- "11. Write result" --> MemoryFabric
    WorkerPool -- "12. Publish JobResult" --> Bus
    Bus -- "13. Forward JobResult" --> Scheduler
    Scheduler -- "14. Update Job State" --> Scheduler
    Client -- "15. Read Result" --> MemoryFabric

    Orchestrator -- "Spawns child jobs" --> Bus
    Bus -- "Routes child jobs" --> Scheduler
```

## Core Abstractions
- **BusPacket**: envelope carrying trace metadata and one payload (`JobRequest`, `JobResult`, `Heartbeat`, `SystemAlert`).
- **JobRequest / JobResult**: submission and completion messages for a schedulable unit of work.
- **Pointers**: `context_ptr`, `result_ptr`, and `redacted_context_ptr` keep large payloads off the bus.
- **Heartbeats**: liveness + capacity signals for schedulers.
- **Workflow metadata**: `workflow_id`, `parent_job_id`, `step_index` enable hierarchical orchestration.

## Versioning Expectations
- Protobuf fields are append-only; never renumber existing fields.
- `protocol_version` in `BusPacket` allows producers/consumers to negotiate or reject incompatible traffic.
- Backward compatibility is required for stable interop; new fields must be optional-safe.
