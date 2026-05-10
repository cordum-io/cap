# 18. Policy Shapes

> Status: Normative · Wire format: v1.0.x (additive) · Introduced: cap v2.10 (epic-d9a6c0a1)

## Purpose

Defines the unified `Rule`, `Decision`, and `Bundle` message shapes that
subsume Cordum's previously split job-side authoring surface
(`InputPolicyRule` / `OutputPolicyRule` / `PolicyRule(velocity)`) and
edge-side `EdgeDecision` / `ActionClassification` surface under one
mental model. Consumers: Cordum's `core/policy/` Go package, the
gateway's `/api/v1/policies/*` HTTP routes, the dashboard's Policy Studio
UI, and any external SDK building higher-layer policy authoring tools on
top of cap.

These shapes are **not** part of the cap bus wire format — they are not
`BusPacket` payload variants. They live in higher-layer APIs (HTTP/gRPC
unary calls, dashboard state, audit-chain payloads). They are emitted
from cap so all SDKs can decode them identically when consumers send
them as request/response bodies.

## Append-only relationship to existing shapes

This file documents NEW message + enum types added in cap v2.10. Every
existing `cordum.agent.v1` message and enum is preserved unchanged. The
sole change to a pre-existing type is the **append-only** extension of
the `DecisionType` enum in `safety.proto` with two new values
(`DECISION_TYPE_QUARANTINE = 6`, `DECISION_TYPE_REDACT = 7`); existing
values 0–5 retain their numbers and semantics. Per the CAP append-only
evolution rule (CONTRIBUTING.md § Proto Changes #1), this is a
backward-compatible wire-version increment — clients running cap v2.9.x
that decode `DECISION_TYPE_QUARANTINE` will see it as the protobuf
zero-value (`DECISION_TYPE_UNSPECIFIED`), which is the documented
forward-compat fallback.

## Message catalog (proto/cordum/agent/v1/policy.proto)

### `Rule` — unified authoring surface

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | string | yes | Stable identifier across versions. |
| `name` | string | yes | Author-facing label. |
| `type` | `RuleType` | yes | `INPUT` / `OUTPUT` / `VELOCITY` / `EDGE`. Discriminator. |
| `scope` | `RuleScope` | yes | Where the rule applies. |
| `status` | `RuleStatus` | yes | `DRAFT` / `PUBLISHED` / `DEPRECATED`. Lifecycle. |
| `version` | string | yes | Bundle-version-of-record. |
| `audit` | `AuditMetadata` | yes | Created/updated provenance. |
| `match` | `google.protobuf.Struct` | — | Per-`RuleType` match payload. |
| `decide` | `google.protobuf.Struct` | — | Per-`RuleType` decision payload. |
| `description` | string | optional | Free-form documentation. |

`match` and `decide` are `google.protobuf.Struct` to preserve lossless
round-trip with the Go `json.RawMessage` and orval-generated TypeScript
`Record<string, unknown>` shapes used by Cordum's dashboard. The
per-`RuleType` schema for each is documented in
`cordum/docs/specs/policy-studio-rewrite.md`.

### `Decision` — unified evaluator output

| Field | Type | Required | Notes |
|---|---|---|---|
| `source` | `DecisionSource` | yes | `JOB` (safetykernel) / `EDGE` (classifier). |
| `rule_id` | string | yes | The rule whose evaluation produced this decision. |
| `bundle_id` | string | optional | Bundle that contained the rule. |
| `bundle_version` | string | optional | Snapshot version. |
| `type` | `DecisionType` | yes | The seven canonical outcomes (see § DecisionType). |
| `trace` | repeated `TraceStep` | optional | Multi-step evaluation log. |
| `input_ref` / `output_ref` | string | optional | Pointers into blob store. |
| `audit_hash` | string | optional | Audit-chain hash. |
| `timestamp` | `google.protobuf.Timestamp` | yes | Decision emission time. |

### `TraceStep` — per-rule evaluation record

| Field | Type | Notes |
|---|---|---|
| `rule_id` | string | required |
| `bundle_id` | string | optional |
| `decision_type` | `DecisionType` | required |
| `reason` | string | optional |
| `timestamp` | `google.protobuf.Timestamp` | required |
| `constraints` | `google.protobuf.Struct` | optional — raw constraints payload (e.g. existing safetykernel `BudgetConstraints`/`SandboxProfile` JSON) |

### `Bundle` — deployable rule set

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | string | yes | |
| `name` | string | yes | |
| `rule_ids` | repeated string | optional | References by id, not embed. |
| `scope_binding` | `RuleScope` | yes | Deployment scope. |
| `versions` | repeated `BundleVersion` | optional | Tamper-evident snapshots. |
| `metadata` | `BundleMetadata` | optional | Per-bundle authoring metadata (carries `EdgeMode`). |

### `BundleVersion` — immutable deploy snapshot

| Field | Type | Required |
|---|---|---|
| `version` | string | yes |
| `rule_snapshot` | repeated `Rule` | optional |
| `deployed_at` | `google.protobuf.Timestamp` | yes |
| `audit_hash` | string | optional |

### `BundleMetadata`, `RuleScope`, `AuditMetadata`

- `BundleMetadata { EdgeMode edge_mode }` — replaces the legacy global
  `EdgePolicyMode` switch with per-bundle edge enforcement posture.
- `RuleScope { RuleScopeKind kind; string value }` — `value` is unused
  when `kind=GLOBAL`.
- `AuditMetadata { Timestamp created_at; string created_by; Timestamp
  updated_at; string updated_by }` — `updated_*` are zero-valued on
  first creation.

## Enum catalog

### `RuleType` (policy.proto)
`UNSPECIFIED=0` / `INPUT=1` / `OUTPUT=2` / `VELOCITY=3` / `EDGE=4`.

### `RuleStatus` (policy.proto)
`UNSPECIFIED=0` / `DRAFT=1` / `PUBLISHED=2` / `DEPRECATED=3`.

### `DecisionType` (safety.proto — extended)

| Value | Number | Semantics |
|---|---|---|
| `DECISION_TYPE_UNSPECIFIED` | 0 | (existing) |
| `DECISION_TYPE_ALLOW` | 1 | (existing) |
| `DECISION_TYPE_DENY` | 2 | (existing) |
| `DECISION_TYPE_REQUIRE_HUMAN` | 3 | (existing) |
| `DECISION_TYPE_THROTTLE` | 4 | (existing) |
| `DECISION_TYPE_ALLOW_WITH_CONSTRAINTS` | 5 | (existing) |
| `DECISION_TYPE_QUARANTINE` | **6** | **NEW** — Output-side outcome; value isolated from caller, not deleted. |
| `DECISION_TYPE_REDACT` | **7** | **NEW** — Output-side outcome; sensitive substrings stripped before return. |

### `DecisionSource` (policy.proto)
`UNSPECIFIED=0` / `JOB=1` / `EDGE=2`.

### `RuleScopeKind` (policy.proto)
`UNSPECIFIED=0` / `GLOBAL=1` / `TENANT=2` / `WORKFLOW=3` / `EDGE_FLEET=4` / `EDGE_USER=5`.

### `EdgeMode` (policy.proto)
`UNSPECIFIED=0` / `OBSERVE=1` / `ENFORCE=2` / `ENTERPRISE_STRICT=3`.

## Conformance fixtures

Three binary fixtures are emitted by `tools/conformance/generate_fixtures.go`
and live at `spec/conformance/fixtures/`:

- `policy_rule.bin` — `RuleType=INPUT` instance with populated
  `match`/`decide` Struct payloads.
- `policy_decision.bin` — `Decision` with `Type=QUARANTINE`, multi-step
  `trace`, all optional fields populated.
- `policy_bundle.bin` — `Bundle` with `Metadata.EdgeMode=ENTERPRISE_STRICT`
  and one `BundleVersion`.

Unlike the existing `buspacket_*.bin` fixtures, these are NOT signed
BusPackets — they are standalone proto-marshalled bytes. Cross-SDK
conformance tests verify each SDK can decode them via its native
proto-parsing path:

- Go: `sdk/go/conformance_test.go` — `proto.Unmarshal(data, &agentv1.Rule{})` etc.
- Python: `cap.pb.cordum.agent.v1.policy_pb2.Rule.ParseFromString(data)`.
- Node: `protobufjs.load("proto/cordum/agent/v1/policy.proto").lookup("cordum.agent.v1.Rule").decode(data)`.

## See also

- `spec/06-safety.md` — original `DecisionType` definition (now extended
  per § DecisionType above).
- `spec/15-conformance-levels.md` — cross-SDK conformance contract.
- `spec/17-versioning-policy.md` — wire-version increment semantics for
  append-only enum extensions.
- Cordum's `cordum/docs/specs/policy-studio-rewrite.md` — consuming-side
  spec for the unified policy authoring UX.
