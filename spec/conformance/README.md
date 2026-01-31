# CAP Conformance Fixtures
Tags: `conformance`, `fixtures`, `testing`, `signing`, `deterministic`.

This directory contains binary fixtures used by SDK conformance tests. Fixtures are serialized `cordum.agent.v1.BusPacket` messages that cover each payload type.

## Fixtures
- `buspacket_job_request.bin`
- `buspacket_job_result.bin`
- `buspacket_job_progress.bin`
- `buspacket_job_cancel.bin`
- `buspacket_heartbeat.bin`
- `buspacket_alert.bin`

## Signature
Each fixture includes a `signature` computed over the serialized `BusPacket` with the `signature` field cleared. The public key for verification is stored in `public_key.pem`.

Fixtures are signed deterministically (RFC 6979 via Go's `crypto.Signer` path) to keep fixture bytes stable across Go patch releases.

## Regenerating
To regenerate fixtures (and public key), run:

```bash
go run tools/conformance/generate_fixtures.go
```

Regeneration overwrites the binary fixture files in this folder.
