# Changelog

# Changelog

## v2.0.0 — 2025-12-12
- Clarified versioning (protocol wire 1.0.0 with `protocol_version=1`; repo/SDK release 2.0.0).
- Added first-class context/memory semantics: `memory_id`, `context_hints`, and a dedicated spec page.
- Added budgeting and multi-tenant metadata to JobRequest (budget, tenant/principal, labels) and Safety inputs.
- Expanded observability/tracing guidance (workflow parent/child semantics, stable `trace_id`) and transport/idempotency recommendations.
- Regenerated Go/Python/C++ stubs with new fields; fixed Node proto loading and E2E tests; C++ build uses vendored stubs.

## v0.1.0 — 2025-12-11
- First public draft of the coretex Agent Protocol (CAP): BusPacket, JobRequest/JobResult, Heartbeat, SafetyKernel, and Alert protobuf contracts under `proto/coretex/agent/v1`.
- Transport profile documented for NATS with canonical subjects (`sys.job.submit`, `sys.job.result`, `sys.heartbeat`) and pointer semantics (`context_ptr`, `result_ptr`, `redacted_context_ptr`).
- SDKs: Go (`github.com/coretexos/cap/go/coretex/agent/v1` import path via `/go` stubs), Python (asyncio + NATS), and Node/TypeScript (protobufjs loader) aligned to the same contracts.
- Tooling: `tools/make_protos.sh` to generate Go/Python stubs into `/go` and `/python`; Python virtualenv support via `PYTHON_BIN`.
- Examples: simple echo, workflow repo review, and heartbeat samples called out from the README for quick starts.
