# Changelog

# Changelog

## v2.0.7 — 2026-01-03
- Added policy budget constraint `max_concurrent_jobs`.
- Confirmed CAP bus payloads include `JobProgress` and `JobCancel` for worker control events.

## v2.0.6 — 2026-01-03
- Added `JobRequest.meta` for structured pack-ready identity/capability metadata.
- SafetyKernel: expanded PolicyCheckResponse with policy snapshots, rule IDs, and structured constraints; added Evaluate/Explain/Simulate/ListSnapshots RPCs.
- Go: canonical protobuf import path `github.com/coretexos/cap/v2/coretex/agent/v1`; removed duplicate `/go` stubs and updated generation defaults.
- Go SDK moved under the root module (`github.com/coretexos/cap/v2/sdk/go`) for unified versioning.
- Regenerated stubs across Go/C++/Python/Node for the updated contracts.

## v2.0.5 — 2026-01-02
- Fixed Python signing/verification to use raw packet bytes for ECDSA, matching Go/Node behavior.
- Node SDK: corrected proto root resolution, handled handler errors, and defaulted missing `jobId`/`workerId` in results.
- Go SDK: allow unsigned submits and handle nil handler results without panicking.
- Python SDK: allow unsigned submits, fill missing `job_id`/`worker_id`, lazy-load NATS, and shim protobuf runtime checks for older installs.
- Expanded examples and SDK docs, including Python/Node simple-echo walkthroughs.

## v2.0.0 — 2025-12-12
- Clarified versioning (protocol wire 1.0.0 with `protocol_version=1`; repo/SDK release 2.0.0).
- Added first-class context/memory semantics: `memory_id`, `context_hints`, and a dedicated spec page.
- Added budgeting and multi-tenant metadata to JobRequest (budget, tenant/principal, labels) and Safety inputs.
- Expanded observability/tracing guidance (workflow parent/child semantics, stable `trace_id`) and transport/idempotency recommendations.
- Regenerated Go/Python/C++ stubs with new fields; fixed Node proto loading and E2E tests; C++ build uses vendored stubs.

## v0.1.0 — 2025-12-11
- First public draft of the coretex Agent Protocol (CAP): BusPacket, JobRequest/JobResult, Heartbeat, SafetyKernel, and Alert protobuf contracts under `proto/coretex/agent/v1`.
- Transport profile documented for NATS with canonical subjects (`sys.job.submit`, `sys.job.result`, `sys.heartbeat`) and pointer semantics (`context_ptr`, `result_ptr`, `redacted_context_ptr`).
- SDKs: Go (`github.com/coretexos/cap/v2/coretex/agent/v1` import path via `/coretex/agent/v1` stubs), Python (asyncio + NATS), and Node/TypeScript (protobufjs loader) aligned to the same contracts.
- Tooling: `tools/make_protos.sh` to generate Go/Python stubs into `/coretex/agent/v1` and `/python`; Python virtualenv support via `PYTHON_BIN`.
- Examples: simple echo, workflow repo review, and heartbeat samples called out from the README for quick starts.
