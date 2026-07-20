# Conformance & the TCK

CAP conformance is **executable**. Beyond the binary decode fixtures in
`spec/conformance/`, the Technical Conformance Kit (TCK) drives behavioral
scenarios against real implementations over a versioned adapter protocol.

Start at [`spec/tck/README.md`](../spec/tck/README.md).

## What the TCK proves — and what it does not

- It proves that an **adapter** (a process wrapping one SDK) satisfies a suite of
  lifecycle, cancellation, negotiation, malformed-packet, bounds, and safety
  scenarios, and — for stable SDKs — that they interoperate byte-for-byte in the
  cross-language fixture/signature matrix over a real embedded NATS transport.
- It does **not** issue certification. The bundled reference adapter tests the
  harness, not an implementation. JSON/JUnit reports are local/CI evidence, not
  an external interoperability or compliance claim.

## Honest coverage today

Some behaviors are authored and enforced now; others are **gated** on
predecessor work and are recorded as such rather than omitted:

- Enforced now: legal lifecycle transitions and rejections (spec/07),
  cancellation from non-terminal states, version negotiation (spec/14),
  malformed/oversize packet drops (job.proto ERROR_CODE_PROTOCOL_*), and safety
  decision ordering (spec/06); the Go→Go fixture/signature edge; mandatory
  embedded real-NATS.
- Gated (see `spec/tck/coverage.json` and `spec/tck/MATRIX-STATUS.md`): duplicate
  suppression, retry attempt-fencing, and the signature/identity/session cases —
  they require the CAP-PRODUCTION dispatch-identity model; and the Python/Node
  matrix edges and installed-artifact tests, which require those SDKs' packaging
  and signing predecessors.

## Commands

```
go build -o cap-tck ./cmd/cap-tck
cap-tck list --suite spec/tck/scenarios/core-lifecycle.json
cap-tck run  --suite spec/tck/scenarios/core-lifecycle.json --adapter "ref=./cap-tck|__adapter|--role|worker"
go test ./internal/tck/...    # includes the mandatory embedded real-NATS tests
```

CI runs these plus the manifest checks in `.github/workflows/sdk-gates.yml`.
