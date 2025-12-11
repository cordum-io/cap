# Core Concepts

## Agent
- Any process that produces or consumes jobs, emits heartbeats, or participates in workflows.
- Categories:
  - **Leaf worker**: consumes a job subject and returns a result.
  - **Orchestrator**: consumes a parent job and spawns child jobs while aggregating results.
  - **Gateway**: front-door that accepts client requests and publishes jobs into the bus.

## Job
- Smallest schedulable unit of work.
- Identified by `job_id`, targeting a `topic` (bus subject) with an optional `priority` and `adapter_id`.
- Carries a `context_ptr` to input stored in external memory; results are written to `result_ptr`.

## Bus
- Pub/sub fabric used to exchange `BusPacket` messages.
- Subjects (topics) and queue groups enable routing and load balancing (e.g., `sys.job.submit`, `job.chat.simple`).
- CAP is transport-agnostic: any bus with subjects/topics and competing consumers works.

## Memory Fabric
- External store holding job contexts and results (e.g., Redis, object storage).
- Pointers (`context_ptr`, `result_ptr`) keep large payloads off the bus.

## Safety Kernel
- Out-of-band policy decision point that can allow, deny, require human review, or throttle a job before dispatch.
- Exposed as a logical service (commonly gRPC) invoked by the scheduler or gateway.

## JobStore
- Logical state machine that records job lifecycle transitions and pointers.
- Can be implemented on Redis, SQL, or any durable store; CAP defines the states and recommended transitions.

## Pools
- Logical groups of workers behind a bus subject (e.g., `job.chat.simple`, `job.code.llm`).
- Heartbeats advertise pool membership, capacity, and load so a scheduler can pick the least-loaded worker in a pool.
