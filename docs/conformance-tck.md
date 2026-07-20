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

Some behaviors are authored and enforced now; others are **gated** and recorded
as such in `spec/tck/coverage.json` rather than quietly omitted. The index is
machine-checked: a gated behavior must carry a reason and list no cases, and a
non-gated behavior must list cases that resolve to shipped scenarios, so a
deferred behavior cannot masquerade as covered.

**Enforced now — 13 behaviors, 46 cases across 45 scenarios:**

- Legal lifecycle transitions and rejections (spec/07), including backwards
  transitions, same-state idempotence, conflicting terminals, and every terminal
  status being reachable.
- Cancellation from any non-terminal state, and rejection after terminal.
- Exact-redelivery **duplicate suppression** and **retry attempt-fencing**
  (monotonic attempt; stale, future and wrong-worker rejection), built on the
  merged CAP-PRODUCTION `DispatchIdentity` model.
- Version negotiation (spec/14) and capability/ready-topic dispatch filtering.
- Malformed and oversize packet drops (job.proto `ERROR_CODE_PROTOCOL_*`).
- Safety decision ordering (spec/06).
- All **nine** producer→consumer edges of the cross-language fixture/signature
  matrix, from installed artifacts, plus mandatory embedded real-NATS.

**Still gated — 5 behaviors:** signature validity (expiry/wrong-audience),
identity/attempt/resource mismatch rejection, session binding, stale policy
snapshot reuse, and the concurrent cancel-vs-result race. Each records why in
`coverage.json`. These are the surrounding tasks' conformance surfaces or, in
the race's case, need an interleaving harness beyond adapter-v1's
request/response contract — not omissions of this suite's own scope.

### Why a green run means something

A suite that graded nothing would look exactly as green as one that graded
everything, so non-vacuity is itself tested. `TestEveryShippedSuiteIsNonVacuous`
walks all seven suites in both roles and both profiles and asserts, in every
graded combination, that a well-behaved adapter produces real passes **and** a
mutation adapter that violates the invariant produces real failures and is not
conformant. The count of graded combinations is asserted too, so the check
cannot degenerate into verifying nothing.

The same rule holds for the matrix (`MissingEdges` is computed over the stable
SDK set, not a hard-coded list, so a new stable SDK fails until its driver
exists) and for code generation (a mutation probe changes one `.proto` and
requires every declared language output to change with it).

## Commands

```
go build -o cap-tck ./cmd/cap-tck
cap-tck list --suite spec/tck/scenarios/core-lifecycle.json
cap-tck run  --suite spec/tck/scenarios/core-lifecycle.json --adapter "ref=./cap-tck|__adapter|--role|worker"
go test ./internal/tck/...    # includes the mandatory embedded real-NATS tests
```

The cross-language matrix is gated behind an environment variable because it
builds a real wheel, a packed tarball and a module proxy, which needs `go`,
`python` and `node` present. When the gate is on a missing toolchain is a hard
failure, never a silent skip:

```
CAP_TCK_MATRIX=1 go test ./internal/tck/ -run TestCrossLanguage -v
```

`-v` prints one line per edge plus a totals line, so a passing run leaves its
own evidence rather than only an exit code.

Code generation is reproduced in a pinned, network-disabled container:

```
tools/codegen/codegen.sh --check     # tools/codegen/codegen.ps1 -Check on Windows
tools/codegen/mutation_check.sh      # proves output derives from the .protos
```

CI runs these plus the manifest checks in `.github/workflows/sdk-gates.yml`
(`hermetic-codegen`, `verify-manifests`, `cross-language-matrix`); dedicated
tests assert those jobs still exist, so the gates cannot be quietly dropped.
