# Protocol Errors

This section specifies when and how CAP components report protocol-level errors using `SystemAlert` messages published to `sys.alert`.

## Overview

Protocol errors occur when a component receives a `BusPacket` that violates wire-level expectations: unsupported versions, failed signatures, malformed data, or unrecognized payload types. These errors are distinct from job-level failures (which are reported via `JobResult`).

Components SHOULD report protocol errors by publishing a `SystemAlert` to `sys.alert` so that controllers, schedulers, and observability backends can detect and respond to issues.

## When to Emit

| Condition | ErrorCode | Severity | Description |
|-----------|-----------|----------|-------------|
| Unsupported `protocol_version` | `PROTOCOL_VERSION_MISMATCH` (100) | `ERROR` | Peer advertises a version the consumer does not support. |
| Packet fails deserialization | `PROTOCOL_MALFORMED_PACKET` (101) | `ERROR` | Raw bytes cannot be parsed as a valid `BusPacket`. |
| Unknown `payload` oneof variant | `PROTOCOL_UNKNOWN_PAYLOAD` (102) | `WARNING` | Payload variant is unrecognized (expected with older consumers). |
| Signature verification fails | `PROTOCOL_SIGNATURE_INVALID` (103) | `CRITICAL` | Signature is present but does not verify against the sender's public key. |
| Signature required but missing | `PROTOCOL_SIGNATURE_MISSING` (104) | `CRITICAL` | Component policy requires signatures but the packet has none. |

## Who Emits

Any component that detects a protocol error SHOULD publish a `SystemAlert` to `sys.alert`. This includes gateways, schedulers, workers, orchestrators, and controllers.

Components MUST NOT crash on protocol errors. The offending packet SHOULD be dropped after the alert is published.

## Who Listens

- **Controllers**: SHOULD consume `sys.alert` and surface protocol errors in dashboards and alerting systems.
- **Schedulers**: SHOULD consume `sys.alert` to detect misbehaving components and MAY suspend dispatch to offending `sender_id` values.
- **Observability backends**: SHOULD consume `sys.alert` for logging, metrics, and tracing correlation.

## Severity Mapping

| Severity | Meaning | Recommended Response |
|----------|---------|---------------------|
| `CRITICAL` | Security-relevant error (signature failure) | Alert on-call, log for audit, consider blocking sender |
| `ERROR` | Protocol violation (version mismatch, malformed data) | Log, increment error metrics, drop packet |
| `WARNING` | Non-critical issue (unknown payload) | Log, increment metrics; no immediate action required |
| `INFO` | Informational (reserved for future use) | Log only |

## Recommended Actions

1. **Log**: Every protocol error SHOULD be logged with `trace_id`, `sender_id`, `error_code_enum`, and `message` for post-incident analysis.
2. **Metrics**: Components SHOULD increment a counter keyed by `error_code_enum` and `source_component` for monitoring.
3. **Block**: For `CRITICAL` severity (signature failures), operators MAY configure automatic blocking of the offending `sender_id` after a threshold of violations.
4. **Notify**: Controllers SHOULD forward `CRITICAL` alerts to external notification channels (PagerDuty, Slack, etc.).

## SystemAlert Fields

When emitting a protocol error alert, populate the following fields:

| Field | Usage |
|-------|-------|
| `severity` | Set to the appropriate `AlertSeverity` value per the mapping above. |
| `error_code_enum` | Set to the corresponding `ErrorCode` value (e.g., `PROTOCOL_SIGNATURE_INVALID`). |
| `message` | Human-readable description of the error. |
| `source_component` | The `sender_id` of the component emitting the alert. |
| `trace_id` | The `trace_id` from the offending `BusPacket`, if available. |
| `details` | Additional context as key-value pairs (e.g., `{"offending_sender": "worker-5", "expected_version": "1"}`). |

Legacy fields (`level`, `component`, `code`) MAY be populated for backward compatibility with older consumers but SHOULD NOT be relied upon for new implementations.

## Example

A scheduler detects a signature failure from `worker-5`:

```json
{
  "severity": "ALERT_SEVERITY_CRITICAL",
  "error_code_enum": "ERROR_CODE_PROTOCOL_SIGNATURE_INVALID",
  "message": "Signature verification failed for BusPacket from worker-5",
  "source_component": "scheduler-1",
  "trace_id": "trace-abc-789",
  "details": {
    "offending_sender": "worker-5",
    "subject": "sys.job.result"
  },
  "level": "CRITICAL",
  "component": "scheduler-1",
  "code": "PROTOCOL_SIGNATURE_INVALID"
}
```
