# Changelog

## v0.1.0 — 2025-12-11
- First public draft of the Cortex Agent Protocol (CAP): BusPacket, JobRequest/JobResult, Heartbeat, SafetyKernel, and Alert protobuf contracts under `proto/cortex/agent/v1`.
- Transport profile documented for NATS with canonical subjects (`sys.job.submit`, `sys.job.result`, `sys.heartbeat`) and pointer semantics (`context_ptr`, `result_ptr`, `redacted_context_ptr`).
- SDKs: Go (`github.com/coretexos/cap/go/cortex/agent/v1` import path via `/go` stubs), Python (asyncio + NATS), and Node/TypeScript (protobufjs loader) aligned to the same contracts.
- Tooling: `tools/make_protos.sh` to generate Go/Python stubs into `/go` and `/python`; Python virtualenv support via `PYTHON_BIN`.
- Examples: simple echo, workflow repo review, and heartbeat samples called out from the README for quick starts.
