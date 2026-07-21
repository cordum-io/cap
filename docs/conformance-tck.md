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

**Enforced now — 14 behaviors, 50 cases across 49 scenarios:**

- Legal lifecycle transitions and rejections (spec/07), including backwards
  transitions, same-state idempotence, conflicting terminals, and every terminal
  status being reachable.
- Cancellation from any non-terminal state, and rejection after terminal.
- Exact-redelivery **duplicate suppression**, **retry attempt-fencing**
  (monotonic attempt; stale, future and wrong-worker rejection), and
  **JobCancel attempt/identity fencing** (a stale or wrong-worker cancel must not
  tear down the active attempt), built on the merged CAP-PRODUCTION
  `DispatchIdentity` model.
- Version negotiation (spec/14) and capability/ready-topic dispatch filtering.
- Malformed and oversize packet drops (job.proto `ERROR_CODE_PROTOCOL_*`).
- Safety decision ordering (spec/06).
- All **nine** producer→consumer edges of the cross-language fixture/signature
  matrix, from installed artifacts, plus mandatory embedded real-NATS. The shared
  corpus exercises the CAP-PRODUCTION structured-resource surface (`context_ref`,
  `result_ref`, `artifact_refs`), so every edge builds, decodes, validates and
  signature-verifies real `ResourceRef`s, not just bare payloads.

**Still gated — 5 behaviors:** signature validity (expiry/wrong-audience),
identity/attempt/resource mismatch rejection, session binding, stale policy
snapshot reuse, and the concurrent cancel-vs-result race. Each records why in
`coverage.json`. These are the surrounding tasks' conformance surfaces or, in
the race's case, need an interleaving harness beyond adapter-v1's
request/response contract — not omissions of this suite's own scope.

### Why a green run means something

A suite that graded nothing would look exactly as green as one that graded
everything, so non-vacuity is itself tested — and tested against a real
reference model, not a self-report. `internal/tck/refadapter_test.go` computes,
from each scenario's **input** params (dispatch id, attempt, active attempt,
worker id, decision, from/to state, version sets, ready topics — never from the
expected outcome), the result a conformant implementation must produce, and
grades PASS iff it matches. Each behavioral invariant (order, safety, dedup,
attempt-fence, worker-bind, cancel-fence, negotiation, malformed) is guarded by
one named check that a single-check *mutation* can disable.

`TestEveryShippedSuiteIsNonVacuous` drives the reference model and every mutation
end-to-end through the real adapter protocol and runner and asserts: the
reference PASSes every case, and each mutation breaks **exactly** the cases that
depend on its check while leaving all others green (selectivity). Two companion
tests close the loophole QA found in the earlier blind pass/fail modes:
`TestReferenceConsumesRequiredParams` proves that removing any load-bearing input
param makes a case fail, so a scenario whose semantics were gutted can no longer
show green; `TestReferenceMutationsAreSelective` pins the per-invariant break
set. The reference model tests the harness — it is not an independent conformant
implementation and issues no certification.

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
                                     # (tools/codegen/mutation_check.ps1 on Windows)
```

The image is pinned by an immutable base `@sha256` digest and per-arch download
checksums; `tools/codegen/pins_test.go` fails in CI if any pin is dropped.

CI runs these plus the manifest checks in `.github/workflows/sdk-gates.yml`
(`hermetic-codegen`, `verify-manifests`, `cross-language-matrix`); dedicated
tests assert those jobs still exist, so the gates cannot be quietly dropped.
