# Conformance Levels

CAP defines three conformance levels that specify which protocol features are required, recommended, or optional for each component role. Implementations declare their conformance level to communicate interoperability expectations.

## Levels

### CORE

The minimum viable implementation. A CORE-conformant component can participate in basic job submission and execution.

- BusPacket envelope (`trace_id`, `sender_id`, `created_at`, `protocol_version`)
- `JobRequest` / `JobResult` message handling
- State machine basics (`PENDING` → `SCHEDULED` → `DISPATCHED` → `RUNNING` → `SUCCEEDED` | `FAILED`)
- Subject constants (`sys.job.submit`, `sys.job.result`)

### STANDARD

Recommended for production deployments. Adds security, observability, and structured error handling on top of CORE.

- All CORE features
- Digital signatures (sign outgoing BusPackets, verify incoming)
- Heartbeats (`Heartbeat` message, `sys.heartbeat` subject)
- Safety kernel integration (`SafetyVerdict` evaluation)
- Error codes (`ErrorCode` enum in `JobResult.error_code_enum`)
- `SystemAlert` messages (`sys.alert` subject)
- Idempotency (`meta.idempotency_key` deduplication)

### FULL

Complete protocol support including orchestration, lifecycle management, and capability negotiation.

- All STANDARD features
- Workflows (`workflow_id`, `parent_job_id`, `step_index`)
- Compensation and rollback (`Compensation` message)
- Progress reporting (`JobProgress` message)
- Cancellation (`JobCancel` message)
- Handshake and capability negotiation (`Handshake` message, `sys.handshake` subject)
- Dead-letter queue (DLQ) handling

## Conformance Matrix

Each cell indicates the requirement level for the feature per role: **MUST**, **SHOULD**, **MAY**, or **N/A**.

### CORE Features

| Feature                     | Gateway | Scheduler | Worker | Orchestrator | Controller |
|-----------------------------|---------|-----------|--------|--------------|------------|
| BusPacket envelope          | MUST    | MUST      | MUST   | MUST         | MUST       |
| `JobRequest` publish        | MUST    | MUST      | N/A    | MUST         | N/A        |
| `JobRequest` consume        | N/A     | MUST      | MUST   | N/A          | N/A        |
| `JobResult` publish         | N/A     | N/A       | MUST   | N/A          | N/A        |
| `JobResult` consume         | MAY     | MUST      | N/A    | MUST         | MAY        |
| State machine enforcement   | N/A     | MUST      | N/A    | SHOULD       | N/A        |
| `sys.job.submit` subject    | MUST    | MUST      | N/A    | MUST         | N/A        |
| `sys.job.result` subject    | MAY     | MUST      | MUST   | MUST         | MAY        |
| Pool subjects (`job.<pool>`)| N/A     | MUST      | MUST   | N/A          | N/A        |

### STANDARD Features

| Feature                     | Gateway | Scheduler | Worker | Orchestrator | Controller |
|-----------------------------|---------|-----------|--------|--------------|------------|
| Digital signatures          | SHOULD  | SHOULD    | SHOULD | SHOULD       | SHOULD     |
| Heartbeat publish           | MAY     | MAY       | SHOULD | MAY          | MAY        |
| Heartbeat consume           | N/A     | MUST      | N/A    | MAY          | SHOULD     |
| Safety kernel integration   | N/A     | SHOULD    | N/A    | SHOULD       | N/A        |
| `error_code_enum` populate  | N/A     | SHOULD    | SHOULD | SHOULD       | N/A        |
| `error_code_enum` consume   | MAY     | MUST      | N/A    | MUST         | MAY        |
| `SystemAlert` publish       | MAY     | SHOULD    | SHOULD | MAY          | SHOULD     |
| `SystemAlert` consume       | N/A     | SHOULD    | N/A    | MAY          | MUST       |
| Idempotency deduplication   | N/A     | SHOULD    | SHOULD | SHOULD       | N/A        |

### FULL Features

| Feature                     | Gateway | Scheduler | Worker | Orchestrator | Controller |
|-----------------------------|---------|-----------|--------|--------------|------------|
| Workflow fields             | MAY     | SHOULD    | MAY    | MUST         | N/A        |
| Compensation                | N/A     | MAY       | MAY    | MUST         | N/A        |
| `JobProgress` publish       | N/A     | N/A       | MAY    | N/A          | N/A        |
| `JobProgress` consume       | MAY     | SHOULD    | N/A    | SHOULD       | MAY        |
| `JobCancel` publish         | MAY     | MAY       | N/A    | MAY          | MAY        |
| `JobCancel` consume         | N/A     | SHOULD    | MAY    | N/A          | N/A        |
| Handshake publish           | SHOULD  | SHOULD    | SHOULD | SHOULD       | SHOULD     |
| Handshake consume           | N/A     | SHOULD    | N/A    | MAY          | SHOULD     |
| DLQ handling                | N/A     | SHOULD    | N/A    | MAY          | MAY        |

## Declaring Conformance

Components SHOULD advertise their conformance level in their `Handshake` message using the `capabilities` map. The following keys are reserved for conformance declaration:

| Key                    | Value  | Meaning                            |
|------------------------|--------|------------------------------------|
| `conformance.core`     | `true` | Component meets CORE requirements  |
| `conformance.standard` | `true` | Component meets STANDARD requirements |
| `conformance.full`     | `true` | Component meets FULL requirements  |

A component declaring `conformance.full` implicitly satisfies `conformance.standard` and `conformance.core`.

## Backward Compatibility

Components that do not send a Handshake or do not declare a conformance level are assumed to be CORE-conformant. Schedulers and controllers MUST NOT reject components that lack conformance declarations.
