# CAP Architecture

This document provides an overview of the coretex Agent Protocol (CAP) system architecture.

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

## Actors

*   **Client**: originates work and reads results from external memory.
*   **Gateway**: front-door that validates, writes context to memory, and publishes `JobRequest` packets.
*   **Scheduler**: owns the state machine, calls safety, chooses pool/subject, and dispatches jobs.
*   **Worker / Pool**: consumes `JobRequest` subjects, executes work, writes results, and emits `JobResult`.
*   **Orchestrator**: a specialized worker that spawns child jobs and aggregates results.
*   **Safety Kernel**: policy decision point (gRPC or equivalent) invoked before dispatch.
*   **Memory Fabric**: external storage for context/result pointers (Redis, object storage, DB).
*   **Bus**: pub/sub fabric carrying `BusPacket` envelopes.
