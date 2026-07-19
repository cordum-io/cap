# Changelog

All notable changes to the Cordum Agent Protocol and its SDKs are documented here.

Entries are grouped by SDK release tag. Wire schema changes (protobuf field additions or semantic changes) are prefixed with **[WIRE]**. See [spec/17-versioning-policy.md](spec/17-versioning-policy.md) for the full versioning policy.

## Unreleased

- Release-truth foundation (not yet released): added `release/manifest.json` as the single machine-readable source of release, wire, SDK, transport, toolchain, and security-support truth; a strict, network-free Go validator (`internal/releasetruth`); and the `cap-release` tool (`check`, `render --write|--check`, `links`). README, the spec index, `SECURITY.md`, and the SDK/transport tables now generate their factual blocks from the manifest.
- **Go SDK:** `Client.Submit` and package `Submit` now fail fast with `ctx.Err()` before signing or publishing when the context is already canceled or past its deadline (CRD-15). Previously a canceled context still published the job packet.
- Dependency and CI maintenance: bumped `golang.org/x/crypto` 0.51.0 to 0.52.0, `golang.org/x/net` 0.51.0 to 0.55.0, `protobufjs` 7.5.8 to 7.6.3, `js-yaml` 4.1.1 to 4.2.0, and `markdown-it` 14.1.1 to 14.2.0 (sdk/node); bumped the Go CI toolchain for GO-2026-5856.

## v2.14.0 — 2026-06-02

- **Node & Python SDKs:** stamp `trace_id` and `created_at` in heartbeat/progress payload builders so helper-built `BusPacket`s pass SDK validation, matching the Go SDK. Not a [WIRE] change.
- De-flaked the handshake test and documented the `created_at` note.
- Bumped `protobufjs` 7.5.6 to 7.5.8 (sdk/node).

## v2.13.4 — 2026-05-31

- **Go SDK:** stamp worker payload envelopes so freshly built packets pass `ValidateBusPacket` (`HeartbeatPayloadWithProgress` sets `TraceId`=worker id and `CreatedAt`; `ProgressPayload`/`CancelPayload` set `TraceId`=job id). Not a [WIRE] change.

## v2.13.3 — 2026-05-25

- Continued the `agent_name` display-label rollout across the SDKs (PR #53).

## v2.13.2 — 2026-05-25

- **[WIRE]** Added `agent_name` — `Heartbeat` field 19 and `Handshake` field 7 — an optional, additive, human-facing display label. It is a DISPLAY label only and **not an authentication authority**: consumers MUST prefer authenticated identity records and MUST NOT use it for authorization. Added `agent_name` support across the Go/Python/Node SDKs with a shared sanitizer (trim, collapse whitespace, drop control characters, cap at 128).

## v2.13.1 — 2026-05-13

- Merged the 2026-05-09 CAP train (#49), which consolidated onto the main release line the work previously tagged on side branches v2.11.0–v2.13.0:
  - **[WIRE]** Added `cordum/agent/v1/policy.proto` with the unified Policy Studio Rule/Decision/Bundle/RuleScope/AuditMetadata shapes and enums.
  - **[WIRE]** Extended `DecisionType` (`safety.proto`) with `DECISION_TYPE_QUARANTINE = 6` and `DECISION_TYPE_REDACT = 7` (existing values 0–5 unchanged per the append-only rule).
  - **[WIRE]** Declared `BusPacket.auth_token` at field 18 across the Go/Python/Node descriptors.
  - Added the policy evaluator service proto, `spec/18-policy-shapes.md`, cross-SDK regeneration, and `policy_rule.bin`/`policy_decision.bin`/`policy_bundle.bin` conformance fixtures with Go/Python/Node decode tests.
  - **Go SDK:** added `sdk/go/agent.go` — `AgentClient.Register/Lookup/SetScope` wrapping the Cordum control-plane agent endpoints; added `docs/agent-registration.md`.
- Bumped `protobufjs` to 7.5.6 (sdk/node).
- Note: tags v2.11.0, v2.12.0, v2.12.1, and v2.13.0 were cut on side branches and are not ancestors of the main line; their content reached main consolidated in this release.

## v2.10.0 — 2026-04-22

- **Go SDK:** added SDK handshake, session-renew, and attach primitives.
- CI hardening: pinned actions to commit SHAs, added explicit wiki-sync permissions, narrowed the CodeQL matrix, and bumped the Go directive to 1.25.9 for stdlib vulnerability fixes.
- Bumped `protobufjs` 7.5.4 to 7.5.5 (sdk/node).

## v2.9.0 — 2026-04-07

- **[WIRE]** Added `auth_token` field (18) to `Heartbeat` for worker attestation.
- **[WIRE]** Added `ready_topics` field (6) to `Handshake` for readiness declaration.
- Added automatic handshake publish in `Agent.Start()` across Go, Python, and Node SDKs — all workers now send handshake at startup with zero code changes.
- Added `publishHandshake()` to Python SDK (`cap.handshake`) and Node SDK (`handshake.ts`) — previously only Go SDK had this.
- Updated heartbeat payload builders in all 3 SDKs to accept `auth_token` parameter.
- Updated handshake construction in all 3 SDKs to include `ready_topics` from registered handler topics.
- Updated spec docs: 05-heartbeats.md (auth_token), 14-capability-negotiation.md (ready_topics), 10-security-observability.md (attestation flow).
- Regenerated conformance fixtures for new proto fields.
- Consolidated note: the following protocol features shipped by 2.9.0 but were previously tracked only in an unreleased section — the `Handshake` message and capability negotiation (spec/14); the `ErrorCode` enum on `JobResult` and the structured `SystemAlert` severity fields; specs 13–17 (error codes, capability negotiation, conformance levels, protocol errors, versioning policy); the Go/Node/Python input-validation helpers (`ValidateJobRequest`/`ValidateJobResult`/`ValidateBusPacket`); and the `cordum-guard` extension.

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
