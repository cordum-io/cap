# Changelog

All notable changes to the Cordum Agent Protocol and its SDKs are documented here.

Entries are grouped by SDK release tag. Wire schema changes (protobuf field additions or semantic changes) are prefixed with **[WIRE]**. See [spec/17-versioning-policy.md](spec/17-versioning-policy.md) for the full versioning policy.

## v2.9.0 — 2026-04-07

- **[WIRE]** Added `auth_token` field (18) to `Heartbeat` for worker attestation.
- **[WIRE]** Added `ready_topics` field (6) to `Handshake` for readiness declaration.
- Added automatic handshake publish in `Agent.Start()` across Go, Python, and Node SDKs — all workers now send handshake at startup with zero code changes.
- Added `publishHandshake()` to Python SDK (`cap.handshake`) and Node SDK (`handshake.ts`) — previously only Go SDK had this.
- Updated heartbeat payload builders in all 3 SDKs to accept `auth_token` parameter.
- Updated handshake construction in all 3 SDKs to include `ready_topics` from registered handler topics.
- Updated spec docs: 05-heartbeats.md (auth_token), 14-capability-negotiation.md (ready_topics), 10-security-observability.md (attestation flow).
- Regenerated conformance fixtures for new proto fields.

## Unreleased — Protocol Hardening

- **Python SDK:** Corrected the P0 low-level worker and high-level Agent failure paths so an ordinary handler exception emits one generic `JOB_STATUS_FAILED` result without leaking exception text, records bounded newline-safe failure context, and leaves the worker available for subsequent jobs even when logging or metrics hooks fail. Cancellation and process-exit signals remain control flow, while worker/agent shutdown now drains intake, applies deadlines to every cleanup stage, preserves the primary error, and attempts later resources after a timeout.
- **Python SDK:** Declared and verified CPython 3.9–3.14 support, aligned the runtime floors with generated code (`protobuf>=6.31.1,<7`, `grpcio>=1.76.0,<2`), and added pinned codegen drift, typing, real-NATS, clean wheel/sdist import, metadata, checksum, and tag-to-version release gates. These are unreleased correctness and provenance changes; no wire schema changed.
- **Go SDK:** Fixed worker heartbeat/progress/cancel payload builders so freshly-built `BusPacket`s pass the SDK's own `ValidateBusPacket`: `HeartbeatPayloadWithProgress` now sets `TraceId=workerID` and `CreatedAt`, and `ProgressPayload` / `CancelPayload` now set `TraceId=jobID`. This is not a [WIRE] change; it populates existing envelope fields without changing protobuf schema or protocol version.
- **Python/Node SDKs:** Matched Go SDK parity for heartbeat/progress/cancel payload builders by setting heartbeat `trace_id`/`traceId` to the worker id, progress/cancel `trace_id`/`traceId` to the job id, and stamping `created_at` in these helper-built envelopes so packets pass SDK `BusPacket` validation. This is not a [WIRE] change; it populates existing envelope fields without changing protobuf schema or protocol version.
- **[WIRE]** Added `agent_name` field (19) to `Heartbeat` and `agent_name` field (7) to `Handshake` — an optional human-facing display label (e.g. `Claude Code — Billing Bot`) for dashboard and audit attribution. Additive/append-only; existing clients stay wire-compatible (old clients send empty). It is a DISPLAY label only and **not an authentication authority**: consumers MUST prefer authenticated identity records (worker credential / Agent Identity) over this self-reported value and MUST NOT use it for authorization. Regenerated Go and Python descriptors; C++/Node-generated stubs unchanged (additive field — regenerate via the pinned toolchain in CI).
- Added the `agent_name` display label across SDKs: Go `worker.WithAgentName` heartbeat option, `worker.ManagedConfig.AgentName`, and `runtime.Agent.AgentName`; Python `agent_name` parameter on `cap.heartbeat` payload builders and `cap.handshake` builders; Node `agentName` parameter on `heartbeat.ts` and `handshake.ts` builders. All SDKs sanitize and bound the label (trim, collapse whitespace, drop control characters, cap at 128 chars) via a shared helper — `capsdk.SanitizeAgentName`, `cap.heartbeat.sanitize_agent_name`, `protos.sanitizeAgentName`.
- Updated spec docs: 05-heartbeats.md (`agent_name` semantics + non-authority warning) and 14-capability-negotiation.md (new Agent Display Name section).
- **[WIRE]** Declared `BusPacket.auth_token` at field 18 to match the pre-existing session-token wire convention and expose the field consistently across Go, Python, and Node descriptors.
- **Go SDK:** `SignPacket`/`VerifyPacketSignature` now use the CAP cross-SDK unsigned BusPacket signing order when both `auth_token` and a oneof payload are present, so tokens remain covered by signatures without breaking Python/Node verification.
- **Docs:** Added [`docs/agent-registration.md`](docs/agent-registration.md) — surface description of `AgentClient.Register/Lookup/SetScope`, the create-time vs `SetScope` preapproval split, and the canonical real-world consumer (Cordum's own LLM chat copilot). Cross-links to the bi-directional [governance senior review](https://github.com/cordum-io/cordum/blob/main/docs/llmchat/governance-review.md) in the cordum repo so the dogfooding evidence is reachable from both sides.
- **Go SDK:** Added `sdk/go/agent.go` — `AgentClient` with `Register(ctx, AgentSpec) (id, error)`, `Lookup(ctx, name, tenant) (*AgentIdentity, error)`, and `SetScope(ctx, AgentScopeUpdate) error` wrapping the Cordum control-plane endpoints `POST /api/v1/agents`, `GET /api/v1/agents`, `PUT /api/v1/agents/{id}`. Bearer-token-supplants-X-API-Key auth, configurable tenant header, idempotency-key support on SetScope. Source-of-truth wrappers so service bootstraps (cordum-llm-chat phase 3, future services) stop rolling their own MCP-fallback registration paths.
- **Go SDK:** `Register` payload deliberately omits `preapproved_mutating_tools` per the gateway's `registerAgentArgs`/`updateAgentRequest` split — preapproved mutations are a post-registration `SetScope` privilege, never a create-time grant.
- **Go SDK:** `SetScope` always sends `preapproved_mutating_tools` (including empty `[]`) so operators have a deterministic revoke path for chat-assistant-style agents.
- **Go SDK:** `Client.Submit` and package `Submit` now fail fast with `ctx.Err()` before signing/publishing when the context is already canceled or past its deadline; previously a canceled context still published the job packet.
- **[WIRE]** Added `ErrorCode` enum and `error_code_enum` field to `JobResult` for structured error classification.
- **[WIRE]** Enhanced `SystemAlert` with `AlertSeverity` enum and structured fields (`severity`, `source_component`, `details`, `trace_id`).
- **[WIRE]** Added `Handshake` message for capability negotiation.
- **[WIRE]** Extended `DecisionType` (`safety.proto`) with `DECISION_TYPE_QUARANTINE = 6` and `DECISION_TYPE_REDACT = 7` for the unified Policy Studio shapes (Cordum epic-d9a6c0a1). Existing values 0-5 retain their numbers per the CAP append-only enum rule.
- **[WIRE]** Added `cordum/agent/v1/policy.proto` with the unified Policy Studio Rule / Decision / Bundle / RuleScope / AuditMetadata shapes plus enums `RuleType`, `RuleStatus`, `DecisionSource`, `RuleScopeKind`. Field-tag policy: 1-20 reserved for envelope, 50+ for type-specific extensions. Imports `safety.proto` for `DecisionType`.
- Added error code registry (spec/13), capability negotiation (spec/14), conformance levels (spec/15), protocol errors (spec/16), and versioning policy (spec/17).
- Added input validation helpers to Go, Node, and Python SDKs (`ValidateJobRequest`, `ValidateJobResult`, `ValidateBusPacket`).
- Added `cordum-guard` Python SDK with `@guard` decorator for LangChain, LlamaIndex, and plain Python functions.
- Hardened SDK packaging: metadata, test exclusion, dependency separation, publish workflows.
- Added python-guard CI test job.
- Added `spec/18-policy-shapes.md` documenting the unified Policy Studio Rule/Decision/Bundle shapes + extended DecisionType enum (QUARANTINE/REDACT) introduced in PR #46. Cross-references existing safety/conformance/versioning specs.
- Added `spec/conformance/fixtures/policy_rule.bin`, `policy_decision.bin`, `policy_bundle.bin` — standalone-message fixtures for cross-SDK decode parity (the existing `buspacket_*.bin` fixtures are signed bus payloads; policy shapes are higher-layer API messages, not bus payloads). Generated by extended `tools/conformance/generate_fixtures.go`.
- Added `TestPolicyShapesConformance` in `sdk/go/conformance_test.go` covering Rule decode, Decision decode with QUARANTINE wire value, Bundle decode with EdgeMode=ENTERPRISE_STRICT, and DecisionType=REDACT round-trip.
- Regenerated `sdk/python/cap/pb/cordum/agent/v1/policy_pb2.py` (NEW) + `safety_pb2.py` (extended for QUARANTINE/REDACT enum values).
- Added `EXTRA_PROTO_INCLUDE` env hook to `tools/make_protos.sh` for cross-platform host setups whose protoc install lacks the Google well-known protos in standard `-I` paths (e.g. Windows binary releases). Documented in `CONTRIBUTING.md` § Proto Changes.
- Cordum cross-ref: epic-d9a6c0a1 / Backend 1 (cordum task-3bf37e32, this cap PR #46) / Backend 1.5 (cordum task-e38d99a5 yaml + dashboard regen, separate cordum PR) / Backend 1.6 (this follow-up).

## v2.0.19 — 2026-01-31

- Stabilized conformance fixture signatures across Go patch releases.
- Updated Go toolchain to 1.24.12 for stdlib security fixes.
- Bumped go-redis to v9.7.3 for vulnerability remediation.

## v2.0.18 — 2026-01-31

- Added high-level runtime layers for Go/Node/Python SDKs with typed handlers, Redis pointer hydration, retries, and size/timeouts defaults.
- Added runtime docs and tests across SDKs.
- Added runtime dependencies (go-redis, zod, redis, pydantic) for developer-friendly validation and storage access.
- Regenerated conformance fixtures with deterministic signing.

## v2.0.17 — 2026-01-30

- Added deterministic signing helpers and conformance fixtures for cross-SDK verification (Go/Node/Python).
- Standardized signature serialization rules in spec/wiki and SDK docs.
- Added CI workflow to run Go/Node/Python tests, govulncheck, and fixture drift checks.

## v2.0.16 — 2026-01-26

- **[WIRE]** Added `FAILED_RETRYABLE` and `FAILED_FATAL` to `JobStatus` for explicit retry vs rollback handling.
- Updated state machine/spec/docs and examples for the new failure semantics.

## v2.0.15 — 2026-01-26

- Updated Go toolchain to 1.24.11 and bumped Go deps (grpc/x/*) to address vulnerability reports.
- Fixed Node SDK proto path resolution for built artifacts and added npm override to remediate diff advisory.

## v2.0.14 — 2026-01-26

- **[WIRE]** Added `JobRequest.compensation` template for rollback semantics.
- **[WIRE]** Added heartbeat checkpoint fields `progress_pct` and `last_memo`.
- Regenerated stubs across Go/C++/Node/Python and updated Go heartbeat helper.

## v2.0.13 — 2026-01-23

- **[WIRE]** Added `memory_load` to worker Heartbeats for memory utilization telemetry.
- Regenerated Go/C++ stubs and added Go SDK heartbeat helper with memory.

## v2.0.9 — 2026-01-09

- Release bump for published tag.

## v2.0.8 — 2026-01-09

- Rebranded CAP to Cordum: module path, proto package, and namespace updates across SDKs.
- Regenerated Go/C++/Node/Python stubs under `cordum/agent/v1`.

## v2.0.7 — 2026-01-03

- **[WIRE]** Added policy budget constraint `max_concurrent_jobs`.
- Confirmed CAP bus payloads include `JobProgress` and `JobCancel` for worker control events.

## v2.0.6 — 2026-01-03

- **[WIRE]** Added `JobRequest.meta` for structured pack-ready identity/capability metadata.
- SafetyKernel: expanded PolicyCheckResponse with policy snapshots, rule IDs, and structured constraints; added Evaluate/Explain/Simulate/ListSnapshots RPCs.
- Go: canonical protobuf import path `github.com/cordum-io/cap/v2/cordum/agent/v1`; removed duplicate `/go` stubs and updated generation defaults.
- Go SDK moved under the root module (`github.com/cordum-io/cap/v2/sdk/go`) for unified versioning.
- Regenerated stubs across Go/C++/Python/Node for the updated contracts.

## v2.0.5 — 2026-01-02

- Fixed Python signing/verification to use raw packet bytes for ECDSA, matching Go/Node behavior.
- Node SDK: corrected proto root resolution, handled handler errors, and defaulted missing `jobId`/`workerId` in results.
- Go SDK: allow unsigned submits and handle nil handler results without panicking.
- Python SDK: allow unsigned submits, fill missing `job_id`/`worker_id`, lazy-load NATS, and shim protobuf runtime checks for older installs.
- Expanded examples and SDK docs, including Python/Node simple-echo walkthroughs.

## v2.0.0 — 2025-12-12

- Clarified versioning (protocol wire 1.0.0 with `protocol_version=1`; repo/SDK release 2.0.0).
- **[WIRE]** Added first-class context/memory semantics: `memory_id`, `context_hints`, and a dedicated spec page.
- **[WIRE]** Added budgeting and multi-tenant metadata to JobRequest (budget, tenant/principal, labels) and Safety inputs.
- Expanded observability/tracing guidance (workflow parent/child semantics, stable `trace_id`) and transport/idempotency recommendations.
- Regenerated Go/Python/C++ stubs with new fields; fixed Node proto loading and E2E tests; C++ build uses vendored stubs.

## v0.1.0 — 2025-12-11

- **[WIRE]** First public draft of the Cordum Agent Protocol (CAP): BusPacket, JobRequest/JobResult, Heartbeat, SafetyKernel, and Alert protobuf contracts under `proto/cordum/agent/v1`.
- Transport profile documented for NATS with canonical subjects (`sys.job.submit`, `sys.job.result`, `sys.heartbeat`) and pointer semantics (`context_ptr`, `result_ptr`, `redacted_context_ptr`).
- SDKs: Go (`github.com/cordum-io/cap/v2/cordum/agent/v1` import path via `/cordum/agent/v1` stubs), Python (asyncio + NATS), and Node/TypeScript (protobufjs loader) aligned to the same contracts.
- Tooling: `tools/make_protos.sh` to generate Go/Python stubs into `/cordum/agent/v1` and `/python`; Python virtualenv support via `PYTHON_BIN`.
- Examples: simple echo, workflow repo review, and heartbeat samples called out from the README for quick starts.
