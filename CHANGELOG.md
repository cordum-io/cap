# Changelog

All notable changes to the Cordum Agent Protocol and its SDKs are documented here.

Entries are grouped by SDK release tag. Wire schema changes (protobuf field additions or semantic changes) are prefixed with **[WIRE]**. See [spec/17-versioning-policy.md](spec/17-versioning-policy.md) for the full versioning policy.

## Development

- **cordum-guard closes governance bypasses (security, behavior change).**
  - Verdicts: the client now reads the gateway's protojson names
    (`DECISION_TYPE_REQUIRE_HUMAN`, ...). Previously every real verdict, DENY included, parsed
    as ALLOW outside CAP-PRODUCTION. Missing verdicts (e.g. an empty 200 body) and unknown
    verdicts, including `DECISION_TYPE_UNSPECIFIED`, now fail closed as DENY in every profile.
    Outside CAP-PRODUCTION they used to default to ALLOW.
  - Tool guards: the LangChain and LlamaIndex guards treat everything except
    `allow` / `allow_with_constraints` as blocking. `require_approval` (or
    `approvalRequired: true`) used to run the tool; it now raises the new
    `CordumApprovalRequiredError` (a `CordumBlockedError`) with `reason`, `rule_id`,
    `policy_snapshot`, `approval_ref` and `job_id`. Unknown verdicts return `[BLOCKED]`. The
    LlamaIndex guard now also guards `acall()` and `real_fn` (used by CodeActAgent), which
    ran unguarded. The LangChain guard keeps the wrapped `_run` / `_arun` signature, so
    `@tool` / `StructuredTool` tools receive LangChain's `config` argument again instead of
    failing with `TypeError`.
  - Labels: guards now send labels at the top level, where the Safety Kernel matches label
    rules. Before, they were sent only under `meta.labels` and no label rule ever matched
    guard traffic. `meta.labels` is still sent.
  - Arguments: every guard sends scalar call arguments as bounded `arg.<name>` labels (16
    per call, 256 chars per value, never for secret-like names), so rules such as
    `label_threshold: {arg.amount: 10000}` work. The policy cache key now includes labels.
    `@guard` approval jobs carry the same labels and mask secret-like arguments in the
    prompt. Pass `include_args=False` to a guard to send no argument labels. For `@guard`
    this also keeps argument values out of the approval prompt.
  - Added `Decision.ALLOW_WITH_CONSTRAINTS` and the `SafetyDecision` fields `rule_id`,
    `policy_snapshot`, `approval_required`, `approval_ref` and `constraints`. Constraints are
    not enforced in-process.
  - `@guard` approvals: the decorator waited for its approval job to finish, but no worker
    serves `job.guard`, so an approved call never ran. After approval Cordum's scheduler
    cannot route the job, re-checks the policy on the retry and puts the job back in the
    approval queue. The call then failed with `CordumTimeoutError`. `@guard` now waits for
    the approval outcome with the new `CordumClient.wait_for_approval()`, which reads
    `approval_decision` from `GET /api/v1/jobs/{id}`. Approved runs the function. Rejected,
    expired or invalidated raises `CordumBlockedError`. No verdict within `timeout` still
    raises `CordumTimeoutError`. Afterwards the guard cancels the approval job. The async
    decorator now polls in a worker thread instead of blocking the event loop. `JobStatus`
    gains `approval_status`, `approval_decision`, `approval_by` and `approval_reason`, and
    `MockCordumClient` gains `wait_for_approval()`.
- **cordum-guard: `CordumClient` takes `ca_cert` and `verify`.** `ca_cert` (default
  `$CORDUM_TLS_CA`, the variable `cordumctl` reads) is a PEM bundle the client trusts for the
  gateway's TLS certificate, for gateway calls only. `verify` is passed to httpx. Verification is off only with an explicit
  `verify=False`, which logs a warning. Before, a gateway behind a private CA, such as the
  Compose stack on `https://localhost:8081`, needed `SSL_CERT_FILE` pointed at a merged
  bundle, which changed every other HTTPS client in the process.
- **Go SDK requires Go 1.26.0 or newer (was 1.25.12).** `golang.org/x/net` v0.60.0, the
  first release that fixes four reachable advisories govulncheck reports against v0.55.0
  (GO-2026-6603, GO-2026-6611, GO-2026-6612, GO-2026-6617), itself declares `go 1.26.0`.
  Go 1.25 left upstream support when Go 1.27 shipped. Repository builds and CI now use the
  `go1.26.9` toolchain through the `toolchain` directive, which also clears the standard
  library advisories fixed in 1.26.9; the toolchain line does not apply to consumers.
- Dependency maintenance: bumped `google.golang.org/grpc` 1.82.1 to 1.83.1 (GO-2026-6348)
  and `golang.org/x/net` 0.55.0 to 0.60.0 (with `x/crypto` 0.57.0, `x/sys` 0.48.0 and
  `x/text` 0.42.0); bumped sdk/node dev dependencies `js-yaml` 4.3.1, `markdown-it` 14.3.2
  and `brace-expansion` 2.1.7, and tools/codegen dev dependencies `markdown-it` 14.3.2 and
  `brace-expansion` 2.1.7.

## v2.17.0 — 2026-07-22

- **Go SDK ResourceRef registry:** added an immutable registry of operator-installed
  resolver/externalizer backends. Resolution and externalization require authenticated
  tenant/job context, enforce per-backend streaming size bounds, independently verify media
  type, size, SHA-256, purpose, and expiry, and expose no generic URL/file fallback or
  embedded-credential path. Resolved bytes are opaque and clone-returning; cancellation and
  backend failures fail closed with bounded typed errors.
- Dependency maintenance: bumped `google.golang.org/grpc` 1.79.3 to 1.82.1; bumped
  `linkify-it` 5.0.1 to 5.0.2 (sdk/node dev dependency); kept `setuptools` pinned to 82.0.1
  in sdk/python (the 83.0.0 bump was reverted).

## v2.16.1 — 2026-07-22

- **CI (no source change):** fixed two publish-workflow bugs that had been dormant since v2.14.0 (neither `publish-go.yml` nor `publish-python.yml` runs outside an actual GitHub Release, and none had been created since). `publish-python.yml`'s mandatory real-NATS gate failed to even collect `test_production_admission_nats.py` (`from tests.docker_nats_support import ...` only resolved when invoked with `sdk/python` as the working directory, not from the repo root as the workflow does) — added `sdk/python/tests/conftest.py` to fix path resolution regardless of invocation directory. `publish-go.yml`'s stub-regeneration step called the pre-hermetic-codegen `tools/make_protos.sh` invocation (no mode flag, custom `CAP_OUT_GO` output directory) that PR #81's rewrite made incompatible; replaced it with a direct test of the already-tracked, already-freshness-checked `cordum/agent/v1/*.pb.go` output.
- v2.16.0's Go and Python SDK publish jobs failed on these two bugs after the Node SDK and cordum-guard published successfully; this release completes publication of the same v2.16.0 content for cap-go and cap-python.

## v2.16.0 — 2026-07-22

- **CAP-PRODUCTION cross-language conformance vectors:** `test/fixtures/production-signing-v1.json` grows from one positive vector to a normative set (schema 2) of 9 signing vectors, 3 replay sequences, and 3 identity-binding vectors, generated by the new deterministic `tools/conformance/prodsigning`. Negatives cover tampered body, audience mismatch, expiry, unknown `key_id`, both halves of a key-rotation overlap window, and identical-message-id-with-different-digest. The Go, Python, and Node consumers all read the same file and assert the same verdicts. `spec/19-cap-production-profile.md` now defines the signed-body digest explicitly as `SHA-256(exact_unsigned_packet_wire)` with no domain prefix — a distinct value from the signature preimage over the same bytes — and states the replay outcomes and append-only evolution posture at MUST level.
- **Python SDK (security fix):** `admit_production_event` computed its replay arguments independently of every other admission path and got both wrong. It hashed the domain-separated *signature preimage* instead of the signed-body digest, so identical wire bytes hashed differently than in `cap/runtime.py`, the Go SDK, and the Node SDK, and a shared replay store rejected a valid redelivery as a conflict. It also retained the replay entry only until `expires_at` rather than expiry plus clock skew, leaving a window in which a packet still verified but no replay record survived. Both paths now match the profile.
- **Go SDK:** `ManagedWorker` now has an explicit fail-closed CAP-PRODUCTION mode: exact raw request verification uses the delivered NATS subject; durable claim/renew/complete/abort settlement prevents acknowledged work from being silently lost; and result/progress events echo the admitted identity and dispatch fence with fresh subject-bound production signatures. The profile requires mutually authenticated NATS TLS and canonical resource-resolver configuration.
- **Node & Python SDKs (security fix):** managed-worker production results and progress events were built and dispatched without ever being signed, so a peer verifying CAP-PRODUCTION signatures on inbound `JobResult`/`JobProgress` traffic would reject every managed-worker outcome (or, under permissive settings, accept it unauthenticated). Both runtimes now sign every managed-production result/progress event with the same subject-bound production signature the Go SDK already produces, with new cross-language echo/signing test coverage.
- **TCK non-vacuity, cancel fencing, and resource-matrix hardening (no wire change):** the behavioral TCK's non-vacuity is now proven against a real reference model (`internal/tck/refadapter_test.go`) that computes each outcome from a scenario's input params rather than self-reporting; per-invariant mutation adapters break exactly the cases that depend on each check, and removing any load-bearing param fails its case — so a gutted scenario can no longer show green. Added JobCancel attempt/identity fencing (stale and wrong-worker cancels must not tear down the active attempt), and the cross-language fixture matrix corpus now exercises the structured `ResourceRef` surface (`context_ref`, `result_ref`, `artifact_refs`) across all nine producer→consumer edges. Matrix driver child stdout/stderr are bounded so a huge or crash-looping child cannot exhaust the harness.
- **Hermetic codegen image fully pinned (supply-chain):** the base image is pinned by an immutable `@sha256` digest and every direct `protoc`/`protobuf-javascript`/`node` download by a committed per-arch sha256 (replacing the mutable `setup_20.x | bash` Node install); `tools/codegen/pins_test.go` fails in CI (`ci.yml`, `sdk-gates.yml`) if a pin or checksum is dropped. Added a Windows `mutation_check.ps1` wrapper (`mutation_check.sh` redirects to it on MSYS) and a windows-latest CI check. Docs in `docs/code-generation.md` and `docs/conformance-tck.md` reconciled to reality — the image is digest-pinned and the generated tree is reconciled (74 outputs across six targets).
- Note: this is the first release actually published (tagged, released, and pushed to PyPI/npm/the Go proxy) since v2.14.0. v2.15.0 and v2.15.1 were tagged in git but never published as GitHub Releases; their CHANGELOG sections below remain as an accurate record of when that work landed.

## v2.15.0 — 2026-07-20

- **[WIRE] CAP-PRODUCTION profile:** added the normative profile (`spec/19-cap-production-profile.md`) and append-only production wire identities (`SignatureMetadata`, `IdentityBinding`, `DispatchIdentity`, structured `ResourceRef`). Go enforces raw-wire signing and replay contracts, and raw-packet admission is wired into the Go, Python, and Node `Agent` runtimes with exact-wire conformance vectors.
- **Guard fails closed (security):** `cordum-guard` gains a `production_profile` option. Under CAP-PRODUCTION a missing, null, non-string, or unknown gateway verdict now fails **closed** (DENY) instead of defaulting to ALLOW; constructing a client with `on_error="open"` is rejected; and a positive ALLOW is never cached without a signed lease. A `{"decision": null}` response previously raised `AttributeError` instead of denying.
- **CPython 3.9 support restored:** asyncio primitives (dispatch semaphore, reconnect/exchange locks, mock queue) are created inside the running loop rather than at `Agent.__init__`, which raised `RuntimeError: There is no current event loop` on 3.9.
- Release-truth foundation (not yet released): added `release/manifest.json` as the single machine-readable source of release, wire, SDK, transport, toolchain, and security-support truth; a strict, network-free Go validator (`internal/releasetruth`); and the `cap-release` tool (`check`, `render --write|--check`, `links`). README, the spec index, `SECURITY.md`, and the SDK/transport tables now generate their factual blocks from the manifest.
- **Go SDK:** `Client.Submit` and package `Submit` now fail fast with `ctx.Err()` before signing or publishing when the context is already canceled or past its deadline (CRD-15). Previously a canceled context still published the job packet.
- Dependency and CI maintenance: bumped `golang.org/x/crypto` 0.51.0 to 0.52.0, `golang.org/x/net` 0.51.0 to 0.55.0, `protobufjs` 7.5.8 to 7.6.3, `js-yaml` 4.1.1 to 4.2.0, and `markdown-it` 14.1.1 to 14.2.0 (sdk/node); bumped the Go CI toolchain for GO-2026-5856.
- **[WIRE] Authenticated worker trust handshake (merged to main, not yet in a release tag):** added the version-1 worker-trust messages (`WorkerHandshakeChallengeRequest`, `WorkerHandshakeChallenge`, `WorkerHandshakeAuthenticate`, `WorkerHandshakeResult`) and append-only `BusPacket` oneof tags 19–22. Exact-v1, unknown-field-free packets, P-256 proof keys, phase-domain signing, bounded core-NATS request/reply, and a short-lived session token carried in the existing `BusPacket.auth_token` (tag 18).
- **Worker-trust runtimes (Go/Python/Node):** pinned client/runtime lifecycles with `off`/`warn`/`enforce` admission modes, proof-key and scheduler-key binding, exact `cordum-scheduler` audience validation, ISSUE-before-admission, token-covering RENEW, reconnect handling, and expiry-safe token attachment. WARN only changes compatibility admission after an operational failure; it never weakens packet, signature, identity, result, or session verification.
- **Breaking (Go SDK):** `ValidateBusPacket` now requires `protocol_version == 1` and binds `BusPacket.sender_id` to `JobResult.worker_id` / `Heartbeat.worker_id` / `Handshake.component_id`; `warn`/`enforce` require a complete `WorkerTrustConfig`; unsigned transport is no longer inferred from missing keys — it requires an explicit `AllowUnsigned: true` (or the named `SubmitUnsigned`).
- **Security documentation:** out-of-band proof public-key enrollment, overlap-based worker/scheduler key rotation, authoritative session supersession/revocation, shared challenge/replay/session authority, safe coarse rejections, and secret-free observability.
- **CI/release:** pinned the mandatory Python and Node real-NATS gates to NATS 2.12.6 by immutable image digest; Node CI/publish extract and verify the exact `nats-server` binary via `CAP_NATS_SERVER_BIN`.
- **Node SDK (non-wire bugfix):** the published npm artifact now bundles all runtime protobuf schemas; corrected NATS callback dispatch and shutdown/reconnect handling; configured inbound signature verification fails closed before handlers run.
- **Python SDK:** corrected the P0 low-level worker and high-level Agent failure paths (one generic `JOB_STATUS_FAILED` result without leaking exception text, bounded failure context, worker stays available); declared and verified CPython 3.9–3.14 support and added pinned codegen-drift, typing, real-NATS, clean wheel/sdist, metadata, and tag-to-version release gates.
- **Onboarding (no wire change):** reworked the local playground and simple-echo docs to distinguish direct dev-only `job.echo` publishing from governed `sys.job.submit` routing, with deterministic readiness/deadlines/exit-codes/cleanup and opaque `demo://` pointers.

## v2.15.1 — 2026-07-20

- **Behavioral TCK, hermetic codegen, and honest SDK tiers (no wire change):** the TCK now
  ships 45 scenarios / 46 graded cases over 13 behaviors, adding exact-redelivery duplicate
  suppression and retry attempt-fencing (monotonic attempt; stale, future and wrong-worker
  rejection) on top of the merged CAP-PRODUCTION `DispatchIdentity` model. Coverage is
  machine-checked: a gated behavior must state a reason and list no cases, and the five that
  remain gated say why. Non-vacuity is tested rather than assumed — every suite is run in
  both roles and both profiles against a mutation adapter that must fail, so a suite that
  graded nothing can no longer look green. The cross-language fixture/signature matrix now
  covers all **nine** producer→consumer edges (Go, Python, Node) from installed artifacts —
  a wheel, a packed tarball and a module proxy — comparing normalized-semantic and
  unsigned-preimage digests, verifying signatures bidirectionally, and requiring every
  consumer to reject bit-flips and wrong keys; completeness is computed, so a new stable SDK
  fails the matrix until its driver exists. Code generation runs in a pinned,
  network-disabled container that reproduces all **74** tracked artifacts across six
  languages, with a mutation probe proving the output derives from the `.proto` sources and
  a manifest that now covers the 14 gRPC stubs it previously missed. SDK tiers are
  machine-readable and gate-enforced (Go/Python/Node stable; C++ and Python Guard community;
  Java/.NET/Rust/PHP/Ruby experimental). The TCK issues no certification: the bundled
  reference adapter tests the harness, not an independent implementation, and its reports
  are local/CI evidence rather than an interoperability claim.

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
