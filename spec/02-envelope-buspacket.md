# Envelope - BusPacket

All CAP traffic is wrapped in a `BusPacket`. The envelope provides tracing, sender identity, and protocol negotiation around a single payload.

## Envelope Fields
- `trace_id`: correlates all packets for a request or workflow.
- `sender_id`: stable identifier for the emitting component (gateway, scheduler, worker, orchestrator, controller).
- `created_at`: timestamp of emission.
- `protocol_version`: CAP wire version. Consumers MAY reject packets with unsupported versions.
- `payload`: exactly one of `JobRequest`, `JobResult`, `Heartbeat`, `SystemAlert`, `JobProgress`, `JobCancel`, or `Handshake`. Old consumers that do not recognize a variant will ignore it per standard protobuf oneof behavior.
- `signature` (optional but recommended): digital signature of the serialized `BusPacket` for authenticity and integrity. Producers SHOULD sign; consumers SHOULD verify when configured with public keys.
- `auth_token` (optional): trusted runtime session token attached by CAP SDK/runtime code after handshake or scheduler-issued session establishment. Implementations MUST treat it as sensitive and MUST NOT populate it from untrusted client input.

## Canonical Proto (see `proto/cordum/agent/v1/buspacket.proto`)
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
    JobProgress job_progress = 15;
    JobCancel job_cancel = 16;
    Handshake handshake = 17;
  }

  bytes signature = 14; // digital signature of the serialized BusPacket
  string auth_token = 18; // trusted runtime session token
}
```

## Subject Recommendations
- Submission: publish `BusPacket{JobRequest}` to `sys.job.submit`.
- Results: publish `BusPacket{JobResult}` to `sys.job.result`.
- Heartbeats: publish `BusPacket{Heartbeat}` to `sys.heartbeat` (often with queue groups disabled so all schedulers can see them).
- Alerts: publish `BusPacket{SystemAlert}` to `sys.alert`.
- Handshake: publish `BusPacket{Handshake}` to `sys.handshake` on connect or reconnect (see [14 Capability Negotiation](14-capability-negotiation.md)).
- Work pools: workers subscribe to `job.<pool>` subjects (e.g., `job.code.llm`, `job.tools`, `job.image`).

## Envelope Rules
- Field numbers MUST NOT be renumbered; evolve by adding new fields.
- All timestamps SHOULD be UTC.
- Producers SHOULD set `protocol_version = 1` until a new major is defined.
- Consumers SHOULD treat unknown fields as optional and ignore them.
- Bus-level metadata (headers) MAY be used for auth or routing, but message-level fields remain canonical.
- `auth_token` is trusted runtime context, not an application payload field. Gateways and schedulers MUST NOT copy caller-supplied values into it unless the caller is already authenticated as trusted control-plane/runtime code.
- When signatures are enabled, verify the `signature` against the serialized packet with the field zeroed; drop or flag packets that fail verification.
- Signatures MUST be computed over deterministic protobuf serialization. Map entries MUST be ordered by key. Implementations SHOULD use the SDK signing helpers when available. In particular, packets that carry both a oneof `payload` and `auth_token` MUST use the CAP unsigned BusPacket signing order with `signature` cleared and the payload serialized before `auth_token` so Go, Python, and Node verify the same bytes.

## Protocol Error Handling
- When a consumer receives a BusPacket with an unsupported `protocol_version`, it SHOULD publish a `SystemAlert` with `error_code_enum = PROTOCOL_VERSION_MISMATCH` and `severity = ERROR` to `sys.alert`.
- When signature verification fails, the consumer SHOULD publish a `SystemAlert` with `error_code_enum = PROTOCOL_SIGNATURE_INVALID` and `severity = CRITICAL`.
- When a BusPacket fails to deserialize, the consumer SHOULD publish a `SystemAlert` with `error_code_enum = PROTOCOL_MALFORMED_PACKET` and `severity = ERROR`.
- See [16 Protocol Errors](16-protocol-errors.md) for the full error reporting specification.
