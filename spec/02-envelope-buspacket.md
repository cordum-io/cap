# Envelope - BusPacket

All CAP traffic is wrapped in a `BusPacket`. The envelope provides tracing, sender identity, and protocol negotiation around a single payload.

## Required Fields
- `trace_id`: correlates all packets for a request or workflow.
- `sender_id`: stable identifier for the emitting component (gateway, scheduler, worker, orchestrator, controller).
- `created_at`: timestamp of emission.
- `protocol_version`: CAP wire version. Consumers MAY reject packets with unsupported versions.
- `payload`: exactly one of `JobRequest`, `JobResult`, `Heartbeat`, or `SystemAlert`.

## Canonical Proto (see `proto/coretex/agent/v1/buspacket.proto`)
```proto
message BusPacket {
  string trace_id = 1;
  string sender_id = 2;
  google.protobuf.Timestamp created_at = 3;
  int32 protocol_version = 4;

  oneof payload {
    JobRequest job_request = 10;
    JobResult job_result = 11;
    Heartbeat heartbeat = 12;
    SystemAlert alert = 13;
  }
}
```

## Subject Recommendations
- Submission: publish `BusPacket{JobRequest}` to `sys.job.submit`.
- Results: publish `BusPacket{JobResult}` to `sys.job.result`.
- Heartbeats: publish `BusPacket{Heartbeat}` to `sys.heartbeat` (often with queue groups disabled so all schedulers can see them).
- Alerts: publish `BusPacket{SystemAlert}` to `sys.alert`.
- Work pools: workers subscribe to `job.<pool>` subjects (e.g., `job.code.llm`, `job.tools`, `job.image`).

## Envelope Rules
- Field numbers MUST NOT be renumbered; evolve by adding new fields.
- All timestamps SHOULD be UTC.
- Producers SHOULD set `protocol_version = 1` until a new major is defined.
- Consumers SHOULD treat unknown fields as optional and ignore them.
- Bus-level metadata (headers) MAY be used for auth or routing, but message-level fields remain canonical.
