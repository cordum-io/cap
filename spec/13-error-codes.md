# Error Codes

CAP defines a structured error code registry for machine-readable failure classification. Error codes are carried in the `error_code_enum` field of `JobResult` (see [03 Job Protocol](03-job-protocol.md)).

## Motivation

The original `error_code` string field in `JobResult` is freeform, making automated error handling fragile. The `ErrorCode` enum provides a canonical taxonomy that tooling, dashboards, and retry logic can depend on without string parsing.

## Error Code Enum

The `ErrorCode` enum is defined in `proto/cordum/agent/v1/job.proto`. Values are organized into numeric ranges by category.

### Categories and Ranges

| Range     | Prefix       | Category                              |
|-----------|--------------|---------------------------------------|
| 0         | (none)       | Unspecified / unknown                 |
| 100–199   | `PROTOCOL_`  | Protocol-level errors                 |
| 200–299   | `JOB_`       | Job execution errors                  |
| 300–399   | `SAFETY_`    | Safety policy errors                  |
| 400–499   | `TRANSPORT_` | Transport / message bus errors        |
| 1000–9999 | (varies)     | Reserved for application-specific use |

The protocol MUST NOT assign values in the 1000–9999 range. Implementations MAY define application-specific codes in that range; these SHOULD be documented per-deployment.

### Protocol Errors (100–199)

| Value | Name                             | When to Use                                                                 |
|-------|----------------------------------|-----------------------------------------------------------------------------|
| 100   | `ERROR_CODE_PROTOCOL_VERSION_MISMATCH` | Peer advertises an incompatible `protocol_version`.                         |
| 101   | `ERROR_CODE_PROTOCOL_MALFORMED_PACKET` | `BusPacket` fails structural validation (missing required fields, bad wire bytes). |
| 102   | `ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD`  | `payload` oneof variant is unrecognized by the receiver.                    |
| 103   | `ERROR_CODE_PROTOCOL_SIGNATURE_INVALID`| Packet signature fails verification.                                        |
| 104   | `ERROR_CODE_PROTOCOL_SIGNATURE_MISSING`| Packet requires a signature but none is present.                            |

### Job Errors (200–299)

| Value | Name                              | When to Use                                                                |
|-------|-----------------------------------|----------------------------------------------------------------------------|
| 200   | `ERROR_CODE_JOB_TIMEOUT`          | Worker exceeded `budget.deadline_ms` or an external timeout fired.         |
| 201   | `ERROR_CODE_JOB_RESOURCE_EXHAUSTED`| Token budget, memory, or other resource limit reached.                     |
| 202   | `ERROR_CODE_JOB_PERMISSION_DENIED`| Caller lacks required permissions for the requested capability.            |
| 203   | `ERROR_CODE_JOB_INVALID_INPUT`    | `context_ptr` payload fails validation (schema mismatch, missing fields).  |
| 204   | `ERROR_CODE_JOB_NOT_FOUND`        | Referenced job, memory corpus, or resource does not exist.                 |
| 205   | `ERROR_CODE_JOB_DUPLICATE`        | Duplicate `job_id` or `idempotency_key` detected.                          |
| 206   | `ERROR_CODE_JOB_WORKER_UNAVAILABLE`| No worker is available to handle the job in the target pool.               |

### Safety Errors (300–399)

| Value | Name                              | When to Use                                                                |
|-------|-----------------------------------|----------------------------------------------------------------------------|
| 300   | `ERROR_CODE_SAFETY_DENIED`        | SafetyKernel returned `DENY` for the request.                             |
| 301   | `ERROR_CODE_SAFETY_POLICY_VIOLATION`| Request violates a specific policy rule (use with `error_message` detail). |
| 302   | `ERROR_CODE_SAFETY_RISK_TAG_BLOCKED`| One or more `risk_tags` on the job are blocked by current policy.          |

### Transport Errors (400–499)

| Value | Name                                | When to Use                                                              |
|-------|-------------------------------------|--------------------------------------------------------------------------|
| 400   | `ERROR_CODE_TRANSPORT_PUBLISH_FAILED` | Publishing a message to the bus failed.                                  |
| 401   | `ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED`| Subscribing to a bus subject failed.                                     |
| 402   | `ERROR_CODE_TRANSPORT_CONNECTION_LOST` | Connection to the message bus was lost during operation.                  |

## Reserved Ranges

| Range     | Owner                | Notes                                          |
|-----------|----------------------|------------------------------------------------|
| 0         | Protocol             | `ERROR_CODE_UNSPECIFIED` only                  |
| 1–99      | Protocol (reserved)  | Reserved for future protocol-level categories  |
| 100–199   | Protocol             | Protocol errors                                |
| 200–299   | Protocol             | Job errors                                     |
| 300–399   | Protocol             | Safety errors                                  |
| 400–499   | Protocol             | Transport errors                               |
| 500–999   | Protocol (reserved)  | Reserved for future protocol categories        |
| 1000–9999 | Application          | Application-specific codes                     |

## Backward Compatibility

The `error_code` string field (field 6 in `JobResult`) is retained for backward compatibility. New implementations SHOULD populate both fields during a migration period:

1. Set `error_code_enum` to the appropriate `ErrorCode` value.
2. Set `error_code` to a string representation for older consumers (e.g., `"TIMEOUT"`, `"PERMISSION_DENIED"`).

Consumers SHOULD prefer `error_code_enum` when present. If `error_code_enum` is `ERROR_CODE_UNSPECIFIED` (0) and `error_code` is non-empty, the string field remains authoritative.

### Legacy String Mapping

| Legacy `error_code` string | `ErrorCode` enum value                  |
|----------------------------|------------------------------------------|
| `"TIMEOUT"`                | `ERROR_CODE_JOB_TIMEOUT` (200)           |
| `"DENIED"`                 | `ERROR_CODE_SAFETY_DENIED` (300)         |
| `"PERMISSION_DENIED"`      | `ERROR_CODE_JOB_PERMISSION_DENIED` (202) |

Implementations MAY extend this mapping table for additional legacy strings used in their deployments.

## Example: JobResult with Both Fields

```json
{
  "job_id": "job-abc-123",
  "status": "JOB_STATUS_FAILED",
  "worker_id": "worker-7",
  "execution_ms": 4500,
  "error_code": "TIMEOUT",
  "error_message": "Worker exceeded deadline_ms budget of 3000ms",
  "error_code_enum": "ERROR_CODE_JOB_TIMEOUT"
}
```
