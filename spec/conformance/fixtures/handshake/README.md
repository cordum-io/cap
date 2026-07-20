# Worker trust handshake conformance vectors

These fixtures freeze the CAP v1 worker trust signing contract from
`spec/14-capability-negotiation.md`.

## Positive packets

`challenge_request.bin`, `challenge.bin`, `authenticate.bin`, and `result.bin`
are complete, signed `BusPacket` values. `manifest.json` records the expected
domain, signer, and SHA-256 digest of each transcript after clearing **only**
`BusPacket.signature`. Each stable SDK must reproduce those digests and verify
the stored ASN.1 DER signatures with `worker_public.pem` or
`scheduler_public.pem`.

The ECDSA signatures are reference values, not reproducible build output.
ECDSA implementations may produce different valid signatures for the same
digest. Tests compare the deterministic digest and verify the reference
signature; they never compare freshly generated signature bytes.

The PEM files contain public test keys only. No private or production key
material belongs in this directory. The visible `fixture-session-token-v1`
string is inert conformance data, not a usable Cordum session.

## Security vectors

`negative_vectors` is a versioned, machine-readable mutation catalogue. A
consumer starts with the named positive `base`, applies `mutation`, and checks
the exact `expected` outcome. The catalogue covers identity impersonation,
exact and concurrent replay, clock boundaries, audience and signed-field
tampering, missing bindings, unsupported versions, expiry, response
correlation, unknown keys, session-claim mismatches, superseded renewal,
unknown protobuf fields, and unavailable state.

Boundary vectors at exactly plus or minus 60 seconds are intentionally
accepted. The corresponding one-nanosecond-over vectors are rejected. Replay
vectors permit exactly one mint/install, never one per concurrent request.

All 38 vectors are executable: 19 run against stable SDK validation/signature
APIs, and 19 stateful vectors run by ID against Cordum's scheduler test
harness. No vector in this manifest is merely declarative.

The fixed timestamps are protocol fixtures. They are not evaluated against the
wall clock; harnesses use `state.now_unix_nanos` from the manifest.
