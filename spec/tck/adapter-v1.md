# CAP TCK Adapter Protocol — adapter-v1

The Technical Conformance Kit (TCK) drives conformance scenarios against
**adapters**: external processes that translate the harness's requests into
calls on a specific CAP implementation. The runner links no SDK directly, so one
suite exercises Go, Python, Node, or any future implementation identically.

This document is normative for protocol version **1**.

> The reference adapter bundled with the harness exists to test the harness
> itself. Passing against it is **not** evidence of an independent conformant
> implementation.

## Transport

- The runner launches the adapter with an explicit argument vector. **No shell
  is involved**; no argument is ever interpreted as a command line.
- Messages flow as **line-delimited JSON (JSONL)**: one UTF-8 JSON object per
  line, terminated by `\n`.
  - Runner → adapter on the adapter's **stdin**.
  - Adapter → runner on the adapter's **stdout**.
- `stderr` is free-form diagnostic text. The runner captures a bounded prefix
  (64 KiB) and never parses it.
- Each message line is bounded at **1 MiB**. A longer line is a protocol
  violation and the run for that adapter is aborted.
- Blank lines are ignored.

## Message types

Every message carries a `"type"`. Unknown fields are rejected — a drifted
message is a violation, not something to tolerate.

### Runner → adapter

| type | fields | meaning |
|------|--------|---------|
| `hello` | — | Requests the adapter's handshake. Sent once, first. |
| `run` | `id`, `scenario`, `params` | Execute one scenario case. `id` is the stable case identifier and must be echoed. `params` is a bounded string→string map. |
| `bye` | — | Shut down cleanly. |

### Adapter → runner

| type | fields | meaning |
|------|--------|---------|
| `handshake` | `protocolVersion`, `role`, `profiles`, `features`, `sdk`, `transport` | Capability declaration, sent once in response to `hello`. |
| `result` | `id`, `status`, `detail`, `evidence` | Outcome for exactly one `run`. `id` must equal the request `id`. |

## Handshake

The adapter's first stdout line MUST be a `handshake`:

```json
{"type":"handshake","protocolVersion":1,"role":"worker",
 "profiles":["core","cap-production"],"features":["cancel","artifacts"],
 "sdk":"go","transport":"nats"}
```

- `protocolVersion` MUST equal `1`. Any other value is rejected and the adapter
  is terminated.
- `role` is `worker` or `control-plane`. A worker adapter runs worker-owned
  behavior; a control-plane adapter runs scheduler-owned transitions. The runner
  marks a scenario whose role the adapter does not own as **N/A** — it is never
  sent, and never counts as a pass.
- A missing or late handshake fails closed after the handshake deadline.

## Running a case

For each applicable scenario the runner sends:

```json
{"type":"run","id":"lifecycle/happy-path","scenario":"lifecycle/happy-path","params":{"attempt":"1"}}
```

The adapter executes the case as a black box — it alone knows the expected
behavior — and replies with exactly one `result` bearing the same `id`:

```json
{"type":"result","id":"lifecycle/happy-path","status":"PASS","evidence":{"terminal":"SUCCEEDED"}}
```

A `result` whose `id` does not match the outstanding request (a stale or
duplicate response) is a protocol violation.

## Status semantics

| status | meaning | counts as proof? |
|--------|---------|------------------|
| `PASS` | the implementation satisfied the case | yes |
| `FAIL` | the implementation violated the case | no — fails the run |
| `ERROR` | adapter/harness fault, not a conformance result | no — fails the run |
| `UNSUPPORTED` | the implementation does not support the behavior | no — a **required** case reporting this fails the run |
| `N/A` | not applicable to this adapter | no, and never counts as proof |

`FAIL` and `ERROR` are deliberately distinct: `FAIL` is a conformance verdict
the adapter reached; `ERROR` is a fault reaching that verdict.

An applicable **required** case must report `PASS`. Any other status — including
`N/A` or `UNSUPPORTED` — makes the run non-conformant and the runner exits
non-zero. This closes the loophole where a suite passes by declining its own
hardest cases.

## Deadlines, cancellation, and cleanup

- Handshake and per-case deadlines are enforced by the runner. A hung adapter is
  killed; the case is recorded as `ERROR` (timeout), never silently skipped.
- On interrupt the runner sends `bye`, waits a short grace period, then kills the
  process. Every child process, pipe, and temporary directory is released on
  pass, fail, or interrupt. Release is **bounded**: if an adapter spawns a
  grandchild that inherits its stderr and lingers, the runner force-closes the
  inherited handle after a short delay rather than waiting on it, so teardown
  cannot hang.

## Reports

The runner emits deterministic JSON and JUnit XML: exact scenario, profile,
role, transport, per-case status, bounded diagnostic/evidence, timings, and tool
and SDK versions. Reports carry **no** secret, private-key, or token material.
Reports are local/CI evidence — not certification or an external
interoperability claim.
