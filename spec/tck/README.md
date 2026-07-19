# CAP Technical Conformance Kit (TCK)

The TCK is a runnable, transport-agnostic harness that drives conformance
scenarios against **adapters** — external processes that translate the harness's
requests into calls on a specific CAP implementation. The runner links no SDK
directly, so one suite exercises Go, Python, Node, or any future implementation
identically.

> The reference adapter bundled with `cap-tck` (`cap-tck __adapter`) exists to
> test the harness itself. It implements no CAP behavior and is **not** an
> independent conformant implementation. The JSON/JUnit reports are local/CI
> evidence, **not** certification or an external interoperability claim.

## Layout

| path | what |
|------|------|
| `adapter-v1.md` | normative adapter protocol (JSONL over stdin/stdout) |
| `schema/*.schema.json` | JSON Schemas for scenarios, adapter messages, reports |
| `scenarios/*.json` | declarative scenario suites (lifecycle, cancellation, negotiation, malformed, safety) |
| `coverage.json` | DoD-behavior → case-id map, including gated behaviors |
| `MATRIX-STATUS.md` | cross-language fixture/signature matrix status |
| `../../internal/tck` | the harness (codec, adapter driver, runner, reporters, matrix, embedded NATS) |
| `../../cmd/cap-tck` | the CLI + built-in reference adapter |

## Running (from a clean checkout)

```
go build -o cap-tck ./cmd/cap-tck

# list a suite
cap-tck list --suite spec/tck/scenarios/core-lifecycle.json

# run one adapter against a profile (exit 0 conformant, 1 not, 2 usage)
cap-tck run   --suite spec/tck/scenarios/core-lifecycle.json \
              --adapter "ref=./cap-tck|__adapter|--role|worker" --json -

# run several adapters (conformance matrix over adapters)
cap-tck matrix --suite spec/tck/scenarios/core-safety.json \
               --adapter "a=./cap-tck|__adapter|--role|control-plane"
```

An adapter is given as `label=argv0|arg1|...` — executed directly, **no shell**,
so paths and arguments may contain spaces.

## Status semantics (failure triage)

| status | meaning | effect on the run |
|--------|---------|-------------------|
| `PASS` | the implementation satisfied the case | counts as proof |
| `FAIL` | the implementation violated the case | run non-conformant |
| `ERROR` | adapter/harness fault reaching a verdict | run non-conformant |
| `UNSUPPORTED` | behavior not supported | non-conformant if the case is required |
| `N/A` | not applicable to this adapter's role | never proof |

An applicable **required** case must report `PASS`. Any other status — including
`N/A` or `UNSUPPORTED` — makes the run non-conformant. A scenario whose `role`
the adapter does not own is recorded as a non-applicable `N/A` and never sent.

## Adding a new stable SDK producer/consumer

1. Build an adapter executable that speaks `adapter-v1` using only that SDK's
   public encode/decode/validate/sign/verify/runtime APIs.
2. Declare its role, supported profiles, SDK id, and transport in the handshake.
3. For the fixture matrix, implement `tck.Consumer` (`Name`, `Inspect`,
   `VerifySignature`) over that SDK and add it alongside the Go reference in
   `internal/tck/matrix_test.go`; matrix completeness is computed, so a missing
   producer→consumer edge fails rather than being silently skipped.
4. Add the SDK to `sdk/support-tiers.json` with real gate evidence and run
   `cap-support check`.

## Scenario ↔ spec coverage

`coverage.json` maps each DoD behavior to its case ids and cites the normative
spec clause (e.g. `spec/07-state-machine.md#L48` for backwards-transition
rejection). Behaviors that cannot be authored yet are listed with a `gated`
marker and the blocking task, so a deferred behavior can never read as covered.

## Real transport

`internal/tck` starts an isolated, loopback-only embedded NATS server with
JetStream on an ephemeral port for its transport tests — there is **no** skip
path, so a missing broker is a failure, not a silent pass.
