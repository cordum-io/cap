# Wire Envelope – BusPacket

All bus traffic in CAP is wrapped in a `BusPacket`. The envelope carries routing and tracing metadata plus a single payload (job request, job result, heartbeat, or alert).

## Semantics
- **trace_id**: correlates all messages for a workflow or client request.
- **sender_id**: identifies the emitting agent (gateway, scheduler, worker, orchestrator).
- **created_at**: timestamp of emission.
- **protocol_version**: CAP wire version. Changes are append-only; fields must not be renumbered.
- **payload**: `oneof` containing exactly one message type.

## Proto snippet (canonical in `proto/cortex/agent/v1/buspacket.proto`)
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

message SystemAlert {
    string level = 1;      // INFO, WARN, CRITICAL
    string message = 2;    // human-readable detail
    string component = 3;  // emitting component
    string code = 4;       // optional machine code
}
```

## Versioning Rules
- Never reuse or renumber fields; add new fields with new IDs.
- `protocol_version` is included so producers/consumers can negotiate behavior or reject incompatible packets.
- Backward/forward compatibility is expected via append-only evolution.
