# CAP Conformance Fixtures
Tags: `conformance`, `fixtures`, `testing`, `signing`, `deterministic`.

This directory contains binary fixtures used by SDK conformance tests. Fixtures are serialized `cordum.agent.v1.BusPacket` messages that cover each payload type.

## Fixtures
- `buspacket_job_request.bin` — JobRequest with all fields populated (priority, budget, meta, compensation, labels)
- `buspacket_job_result.bin` — JobResult with FAILED_RETRYABLE status, error code, and artifacts
- `buspacket_job_progress.bin` — JobProgress at 50% with RUNNING status
- `buspacket_job_cancel.bin` — JobCancel with reason and requester
- `buspacket_heartbeat.bin` — Heartbeat with capabilities, labels, memory load, and progress checkpoint
- `buspacket_auth_token.bin` — BusPacket-level `auth_token` with a Heartbeat payload to verify declared field 18 and cross-SDK signature coverage
- `buspacket_alert.bin` — SystemAlert with legacy fields (level, component, code)
- `buspacket_handshake.bin` — Handshake with worker role, supported versions, capability flags, and SDK version
- `buspacket_alert_enhanced.bin` — Enhanced SystemAlert with both legacy and new fields (severity enum, error code enum, source component, details map, trace ID)

## Signature
Each fixture includes a `signature` computed over the serialized `BusPacket` with the `signature` field cleared. The public key for verification is stored in `public_key.pem`.

Fixtures are signed deterministically (RFC 6979 via Go's `crypto.Signer` path) to keep fixture bytes stable across Go patch releases.

## Regenerating
To regenerate fixtures (and public key), run:

```bash
go run tools/conformance/generate_fixtures.go
```

Regeneration overwrites the binary fixture files in this folder.
