# Glossary

This document defines key terms used in the coretex Agent Protocol (CAP) specification.

| Term | Definition |
| --- | --- |
| **Adapter** | A component that adapts a specific model or tool to the CAP worker interface. The `adapter_id` in a `JobRequest` can be used to select a specific adapter. |
| **Bus** | The pub/sub message-passing fabric that transports `BusPacket` envelopes between CAP components. |
| **BusPacket** | The standard envelope for all messages on the bus. It contains metadata like `trace_id` and `sender_id`, and a payload. |
| **Client** | The application or service that originates a job request. |
| **Context Pointer** | An opaque URI (`context_ptr`) that points to the input data for a job, stored in the Memory Fabric. |
| **Gateway** | The entry point for job requests into the CAP system. It validates requests, writes job contexts to the Memory Fabric, and publishes `JobRequest` packets to the bus. |
| **Heartbeat** | A periodic message sent by workers to advertise their liveness, capacity, and capabilities. |
| **Job** | The fundamental unit of work in CAP. It is represented by a `JobRequest` and a `JobResult`. |
| **JobRequest** | A message that initiates a job. It contains metadata about the job, including its topic, priority, and a pointer to its context. |
| **JobResult** | A message that indicates the completion of a job. It contains the job's final status and a pointer to its result. |
| **Memory Fabric** | An external storage system (e.g., Redis, S3, a database) used to store large data payloads (job contexts and results) that are referenced by pointers in CAP messages. |
| **Orchestrator** | A specialized worker that can break down a complex job into a workflow of smaller child jobs. |
| **Pool** | A group of workers that subscribe to the same job topic and have similar capabilities. |
| **Result Pointer** | An opaque URI (`result_ptr`) that points to the output data of a job, stored in the Memory Fabric. |
| **Safety Kernel** | A centralized service that enforces policies on jobs before they are dispatched. It can allow, deny, or throttle jobs based on configurable rules. |
| **Scheduler** | The component responsible for managing the job lifecycle. It consumes `JobRequest` and `JobResult` messages, calls the Safety Kernel, and dispatches jobs to worker pools. |
| **State Machine** | The set of states that a job can be in (e.g., `PENDING`, `RUNNING`, `SUCCEEDED`, `FAILED`). |
| **Worker** | A component that executes jobs. It subscribes to a job topic, processes `JobRequest` messages, and produces `JobResult` messages. |
| **Workflow** | A set of interconnected jobs, managed by an Orchestrator, that together accomplish a larger task. |
