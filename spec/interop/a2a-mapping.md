# A2A 1.0 to CAP Mapping (v1, draft design)

Status: **draft design**. This document fixes how an A2A 1.0 server-side bridge
maps A2A tasks onto CAP jobs. No CAP component implements it yet; the bridge,
its egress validation, and its end-to-end proof are separate, later work. The
machine-readable cases live in [vectors/a2a-v1.json](vectors/a2a-v1.json).

The key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## 1. Scope and versions

| Item | Pinned version | Source |
|---|---|---|
| A2A protocol | 1.0.0 (tag `v1.0.0`, commit `173695755607e884aa9acf8ce4feed90e32727a1`, released 2026-03-12) | https://github.com/a2aproject/A2A/blob/v1.0.0/specification/a2a.proto, https://github.com/a2aproject/A2A/blob/v1.0.0/docs/specification.md, https://a2a-protocol.org/v1.0.0/specification/ |
| a2a-go | `github.com/a2aproject/a2a-go/v2` v2.6.0 (`go 1.26.0`, `google.golang.org/grpc v1.83.2`) | module proxy download, read with `go doc` and source review only |
| CAP | this repository's `JobStatus` enum ([job.proto](../../proto/cordum/agent/v1/job.proto) lines 52-65) and the subjects of [spec/09](../09-transport-profile.md) | — |

Section numbers written as "A2A §x.y" refer to the v1.0.0 `docs/specification.md`.
The heading of that file at the tag still reads "Release Candidate v1.0"; the tag
and the 1.0.0 CHANGELOG entry are the version of record.

In scope for v1: the JSON-RPC methods `SendMessage`, `GetTask` and `CancelTask`;
text and data parts; inline artifacts; one CAP job per A2A turn. Out of scope:
see [section 6](#6-excluded-in-v1).

## 2. State mapping

A2A 1.0 `TaskState` has nine values (a2a.proto lines 187-208). `COMPLETED`,
`FAILED`, `CANCELED` and `REJECTED` are terminal. `INPUT_REQUIRED` and
`AUTH_REQUIRED` are interrupted (non-terminal). `UNSPECIFIED` is never emitted by
the bridge; vectors use it only to mean "no task is visible to the caller".

Each of the 11 non-UNSPECIFIED CAP `JobStatus` values maps to exactly one
`TaskState`:

| CAP `JobStatus` | A2A `TaskState` | Terminal |
|---|---|---|
| `JOB_STATUS_PENDING` | `TASK_STATE_SUBMITTED` | no |
| `JOB_STATUS_SCHEDULED` | `TASK_STATE_SUBMITTED` | no |
| `JOB_STATUS_DISPATCHED` | `TASK_STATE_WORKING` | no |
| `JOB_STATUS_RUNNING` | `TASK_STATE_WORKING` | no |
| `JOB_STATUS_FAILED_RETRYABLE` | `TASK_STATE_WORKING` | no |
| `JOB_STATUS_SUCCEEDED` | `TASK_STATE_COMPLETED`, or the envelope's interrupted state (below) | yes, unless interrupted |
| `JOB_STATUS_DENIED` | `TASK_STATE_REJECTED` | yes |
| `JOB_STATUS_CANCELLED` | `TASK_STATE_CANCELED` | yes |
| `JOB_STATUS_FAILED` | `TASK_STATE_FAILED` | yes |
| `JOB_STATUS_FAILED_FATAL` | `TASK_STATE_FAILED` | yes |
| `JOB_STATUS_TIMEOUT` | `TASK_STATE_FAILED` | yes |

`TASK_STATE_INPUT_REQUIRED` and `TASK_STATE_AUTH_REQUIRED` come **only** from a
valid turn-result envelope ([section 4](#4-turn-result-envelope)) carried by a
`JOB_STATUS_SUCCEEDED` result. No `JobStatus` value is added for them; the CAP
job for that turn ends `SUCCEEDED`, and the A2A task stays open.

A `JOB_STATUS_SUCCEEDED` result whose envelope is missing or invalid is treated
exactly as `JOB_STATUS_FAILED` (so the task becomes `TASK_STATE_FAILED`). This
is fail-closed: a worker that does not speak the envelope cannot complete an A2A
task.

### 2.1 Reference state machine

The bridge keeps one record per task. Events are applied in arrival order:

1. **Publish.** When the bridge publishes the `JobRequest` for turn N, the task
   enters `SUBMITTED` and turn N becomes the current turn. This is allowed only
   when no task exists yet (turn 1) or the task is `INPUT_REQUIRED` or
   `AUTH_REQUIRED` (turn N > 1).
2. **Progress** (`sys.job.progress`, `JobProgress.status`). Only events for the
   current turn's job count. Only `PENDING`, `SCHEDULED`, `DISPATCHED`,
   `RUNNING` and `FAILED_RETRYABLE` are applied; a progress hint never makes a
   task terminal. Within a turn, the state never moves from `WORKING` back to
   `SUBMITTED`.
3. **Result** (`sys.job.result`, or `sys.job.dlq` for scheduler-side outcomes
   such as `DENIED`). Only events for the current turn's job count. The first
   terminal `JobStatus` observed for a job wins; later results for the same job,
   including exact redeliveries, are ignored. `FAILED_RETRYABLE` is applied as
   `WORKING` and does not finish the turn.
4. **Terminal is final.** Once the task is `COMPLETED`, `FAILED`, `CANCELED` or
   `REJECTED`, every later CAP event for it is ignored, and a `SendMessage` that
   targets it fails with `UNSUPPORTED_OPERATION` (A2A §3.1.1).

Each applied state change that differs from the current state is one A2A status
update, in order. Duplicates and ignored events produce none.

## 3. Identity mapping

| A2A | CAP `JobRequest` field | Rule |
|---|---|---|
| `Task.id` (server-generated, A2A §3.4.2) | `workflow_id` | `a2a-task-` followed by 32 lowercase hex characters from a CSPRNG (41 bytes). Clients cannot create task ids. |
| one turn (one accepted `Message`) | one `job_id` | `a2a-` followed by the 64-hex-character `idempotency_key` below (68 bytes, charset `a-z0-9-`). |
| previous turn | `parent_job_id` | empty for turn 1, otherwise the previous turn's `job_id`. |
| turn number | `step_index` | 1 for the first turn, incremented per accepted turn. At most 64 turns per task. |
| `Message.messageId` | `meta.idempotency_key` (via the key below) | 1-128 bytes, charset `A-Za-z0-9._:-`; anything else is `INVALID_PARAMS`. |
| `Task.contextId` | none (bridge record only) | Server-generated `a2a-ctx-` plus 32 hex when absent. A client value must be 1-128 bytes of `A-Za-z0-9._:-` and, for an existing task, equal the task's context (A2A §3.4.3), else `INVALID_PARAMS`. |
| authenticated caller | `tenant_id`, `meta.tenant_id`, `principal_id`, `meta.actor_id` | Taken **only** from the authenticated transport identity. Message metadata, parts, `contextId`, trace headers and baggage never set or change them. `env["tenant_id"]`, if set, equals `tenant_id`. |
| agent skill | `topic` | From operator configuration only; never from client input. |

### 3.1 Idempotency key

```
idempotency_key = lowercase_hex( SHA-256(
    "cap-a2a-turn-v1"
    || u32be(len(tenant))     || tenant
    || u32be(len(principal))  || principal
    || u32be(len(task_ref))   || task_ref
    || u32be(len(message_id)) || message_id ) )
```

`len` is the UTF-8 byte length and `u32be` is a 4-byte big-endian unsigned
integer, so field boundaries are unambiguous: `("ab","c")` and `("a","bc")`
hash different inputs. `task_ref` is the `taskId` named by the message, or the
empty string for a message that creates a task.

The key includes `principal` as well as `tenant`, `taskId` and `messageId`.
Message ids are only unique per client, so a key without the principal would let
two principals of one tenant collide on `msg-1` and land in each other's task.

A `SendMessage` whose key matches an accepted turn returns that turn's task and
publishes nothing (A2A §3.3.1 permits messageId deduplication). If its parts
differ from the accepted turn's parts, it fails with `INVALID_PARAMS`. Because
`job_id` is derived from the key, a retried publish also lands on the
scheduler's existing job record.

## 4. Turn-result envelope

Media type: **`application/vnd.cordum.cap.a2a-turn-result.v1+json`**.

On the compat profile the worker stores the envelope as the job's result
payload: `JobResult.result_ptr` MUST be exactly `redis://res:<job_id>` for the
same job. Any other pointer, a `result_ref` without a pointer, or a payload that
fails the schema or the byte bounds makes the envelope invalid, which the bridge
treats as `JOB_STATUS_FAILED`.

The turn input that the bridge writes for the worker uses the same part grammar
and bounds, at `redis://ctx:<job_id>`, as a JSON object
`{"task_id", "context_id", "turn", "message_id", "parts"}`.

### 4.1 Bounds

| Bound | Value |
|---|---|
| Parts per message (input message or envelope `message`) | 1-16 |
| Parts per artifact | 1-16 |
| Artifacts per envelope | 0-8 (0 for `auth-required`) |
| Text part, UTF-8 bytes | at most 65 536 |
| Data part, bytes of its compact JSON serialization | at most 65 536 |
| Data part JSON nesting depth | at most 32 |
| Whole envelope or turn input, bytes | at most 262 144 |
| `artifact_id` | 1-128 bytes, `A-Za-z0-9._:-` |
| Artifact `name` | at most 256 characters |
| Part `media_type` | at most 127 characters, `type/subtype` grammar |

Every bound is a finite number and is enforced on both the input and the result
path. An input that breaks a bound is rejected before any publish
(`INVALID_PARAMS`, or `CONTENT_TYPE_NOT_SUPPORTED` for a part kind outside
`text` and `data`).

### 4.2 Credentials travel only out of band

`TASK_STATE_AUTH_REQUIRED` follows A2A §7.6 (In-Task Authorization). The worker
explains what authorization it needs in a text-only `message`. Credentials reach
the worker **only out of band** over a secure channel that the worker arranges
(A2A §7.6.1, §7.6.3). This v1 mapping negotiates no in-band credential
mechanism:

- the envelope has no token, secret or credential field, and every object in the
  schema is closed (`additionalProperties: false`), so an envelope that adds one
  is invalid and the task fails;
- an `auth-required` envelope may contain only text parts and no artifacts;
- an A2A message sent to a task in `AUTH_REQUIRED` is an ordinary new turn. The
  bridge never treats it as a credential and never forwards anything to the
  worker except the bounded turn input.

### 4.3 Schema

The JSON Schema below (draft 2020-12) is normative together with the byte bounds
in section 4.1, which JSON Schema cannot express exactly. `maxLength` counts
characters and is only a coarse upper guard.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:cordum:cap:a2a-turn-result:v1",
  "type": "object",
  "additionalProperties": false,
  "required": ["media_type", "a2a_state"],
  "properties": {
    "media_type": { "const": "application/vnd.cordum.cap.a2a-turn-result.v1+json" },
    "a2a_state": { "enum": ["completed", "input-required", "auth-required"] },
    "message": { "$ref": "#/$defs/message" },
    "artifacts": {
      "type": "array",
      "maxItems": 8,
      "items": { "$ref": "#/$defs/artifact" }
    }
  },
  "allOf": [
    {
      "if": { "properties": { "a2a_state": { "const": "input-required" } } },
      "then": { "required": ["message"] }
    },
    {
      "if": { "properties": { "a2a_state": { "const": "auth-required" } } },
      "then": {
        "required": ["message"],
        "properties": {
          "message": {
            "properties": {
              "parts": { "items": { "$ref": "#/$defs/text_part" } }
            }
          },
          "artifacts": { "maxItems": 0 }
        }
      }
    }
  ],
  "$defs": {
    "media_type": {
      "type": "string",
      "maxLength": 127,
      "pattern": "^[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]*/[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]*$"
    },
    "text_part": {
      "type": "object",
      "additionalProperties": false,
      "required": ["kind", "text"],
      "properties": {
        "kind": { "const": "text" },
        "text": { "type": "string", "maxLength": 65536 },
        "media_type": { "$ref": "#/$defs/media_type" }
      }
    },
    "data_part": {
      "type": "object",
      "additionalProperties": false,
      "required": ["kind", "data"],
      "properties": {
        "kind": { "const": "data" },
        "data": {},
        "media_type": { "$ref": "#/$defs/media_type" }
      }
    },
    "part": { "oneOf": [{ "$ref": "#/$defs/text_part" }, { "$ref": "#/$defs/data_part" }] },
    "parts": {
      "type": "array",
      "minItems": 1,
      "maxItems": 16,
      "items": { "$ref": "#/$defs/part" }
    },
    "message": {
      "type": "object",
      "additionalProperties": false,
      "required": ["parts"],
      "properties": { "parts": { "$ref": "#/$defs/parts" } }
    },
    "artifact": {
      "type": "object",
      "additionalProperties": false,
      "required": ["artifact_id", "parts"],
      "properties": {
        "artifact_id": { "type": "string", "pattern": "^[A-Za-z0-9._:-]{1,128}$" },
        "name": { "type": "string", "maxLength": 256 },
        "parts": { "$ref": "#/$defs/parts" }
      }
    }
  }
}
```

The bridge maps an envelope part `{"kind":"text","text":…}` to A2A
`Part.text`, and `{"kind":"data","data":…}` to `Part.data`. `media_type` maps
to `Part.media_type`. The envelope `message` becomes the `TaskStatus.message`
(role `ROLE_AGENT`), and each artifact becomes an A2A `Artifact` on the task.

## 5. Cancel semantics

`CancelTask` (A2A §3.1.5) on a task whose current turn has a job in flight:

1. The bridge publishes **one** `JobCancel` for that job on `sys.job.cancel`. A
   repeated `CancelTask` publishes nothing more and waits for the same outcome.
2. The bridge does not set `CANCELED` itself. The task takes the **actual
   winning terminal state**: the first terminal `JobResult` for the job observed
   after the cancel. `CANCELLED` gives `CANCELED`. A `SUCCEEDED` (or `FAILED`,
   `DENIED`, `TIMEOUT`) result that wins the race gives that result's state, and
   `CancelTask` then fails with `TASK_NOT_CANCELABLE`.
3. If no terminal result arrives within `cancel_grace_ms` (default 30 000,
   bounded to 1 000-60 000), the task becomes `CANCELED`. This rule exists
   because, on the compat profile, the Cordum scheduler records `CANCELLED` for
   a bus `JobCancel` without publishing a `JobResult`, and the CAP Go worker
   runtime does not consume `JobCancel` (see [section 8](#8-bridge-placement-and-admission)).
   A result that arrives after this point is ignored (rule 4 of section 2.1).

`CancelTask` on an `INPUT_REQUIRED` or `AUTH_REQUIRED` task has no job in
flight: the task becomes `CANCELED` at once and nothing is published.
`CancelTask` on a terminal task fails with `TASK_NOT_CANCELABLE` and publishes
nothing.

## 6. Excluded in v1

- `ResourceRef` inputs and results, and dereferencing any URL. A `url` or `raw`
  part is `CONTENT_TYPE_NOT_SUPPORTED`.
- Streaming: `SendStreamingMessage` and `SubscribeToTask` return
  `UNSUPPORTED_OPERATION`, and the Agent Card declares `streaming: false`.
- Push notifications (`PUSH_NOTIFICATION_NOT_SUPPORTED`), `ListTasks`
  (`UNSUPPORTED_OPERATION`) and the extended Agent Card.
- OIDC and any other identity federation. The bridge authenticates callers with
  its own configured scheme.
- Baggage and trace data as authority. Trace headers are never used to select
  tenant, principal, topic or policy.

`SendMessage` honours `return_immediately`. A blocking call waits for a terminal
or interrupted state for at most `max_blocking_wait_ms` (default 60 000, bounded
to 1 000-300 000) and then returns the current task. That cap is a deliberate,
documented deviation from the unbounded wait in A2A §3.2.2, made to bound server
resources.

### 6.1 Errors

All task lookups use the composite key (tenant, principal, taskId) in a single
store operation. Another caller's task and an unknown id take the same code
path and return the same error: `TASK_NOT_FOUND` with the same message and the
same `google.rpc.ErrorInfo` (reason `TASK_NOT_FOUND`, domain
`a2a-protocol.org`), so neither the error code nor the response timing reveals
whether the task exists (A2A §3.3.2). Authentication failures are rejected before
any lookup.

| Vector `expected_error` | JSON-RPC code (A2A §5.4, §9.5) |
|---|---|
| `TASK_NOT_FOUND` | -32001 |
| `TASK_NOT_CANCELABLE` | -32002 |
| `UNSUPPORTED_OPERATION` | -32004 |
| `CONTENT_TYPE_NOT_SUPPORTED` | -32005 |
| `INVALID_PARAMS` | -32602 |

## 7. a2a-go v2.6.0 notes

Symbols verified with `go doc` against the downloaded module:

| Role | Symbol |
|---|---|
| JSON-RPC server handler | `a2asrv.NewJSONRPCHandler(handler RequestHandler, options ...TransportOption) http.Handler` |
| Request handler | `a2asrv.NewHandler(executor AgentExecutor, options ...RequestHandlerOption) RequestHandler`; `a2asrv.RequestHandler` |
| Executor | `a2asrv.AgentExecutor` with `Execute` and `Cancel(ctx, *ExecutorContext) iter.Seq2[a2a.Event, error]`; `a2asrv.ExecutorContext` |
| Event queue | `eventqueue.Manager`, `eventqueue.NewInMemoryManager`; option `a2asrv.WithEventQueueManager` |
| Task store | `taskstore.Store` (`Create`, `Update`, `Get`, `List`); `taskstore.InMemory`, `taskstore.NewInMemory(*InMemoryStoreConfig)`; `taskstore.Authenticator`; option `a2asrv.WithTaskStore` |
| Caller identity | `a2asrv.CallContext`, `a2asrv.User`, `a2asrv.NewTaskStoreAuthenticator` |
| Client | `a2aclient.Client`, `a2aclient.Factory` |

Packages: `github.com/a2aproject/a2a-go/v2/{a2a,a2asrv,a2asrv/taskstore,a2asrv/eventqueue,a2aclient}`.

**Store-scoping verdict: partial, and not sufficient as the bridge's
authorization boundary.** Unlike the v2.3.1 finding of the 2026-07-15 audit,
v2.6.0 does tag each task with an owner string:

- `InMemory.Get` and `Update` return `ErrTaskNotFound` on an owner mismatch
  (`a2asrv/taskstore/inmemory.go` lines 143-145 and 171-178), and `List` filters
  by owner (lines 191-194 and 235).
- `NewHandler`'s default store uses `NewTaskStoreAuthenticator`
  (`a2asrv/handler.go` lines 233-236, `a2asrv/middleware.go` lines 106-113), whose
  owner is `CallContext.User.Name`. An unauthenticated call has name `""`
  (`middleware.go` line 37), and `NewInMemory(nil)` returns `""` for everyone
  (`inmemory.go` lines 78-82). All such callers share one bucket.
- The owner is a single name string with no tenant component.
- `Get` looks the task up before it calls the authenticator (lines 163-178). An
  authenticator error is therefore returned only for existing tasks, which
  reveals existence.
- `CancelTask` reaches the store through the executor factory's `Get`
  (`a2asrv/agentexec.go` lines 281-290), but in local mode a concurrent cancel
  for the same task id joins the in-flight cancel and receives its result
  without its own ownership check (`internal/taskexec/local_manager.go` lines
  232-256).

So a bridge built on a2a-go MUST supply its own `taskstore.Store` keyed by the
length-prefixed (tenant, principal, taskId) composite, MUST authenticate before
any store access, and MUST check ownership in `RequestHandler` before it
delegates `CancelTask`.

## 8. Bridge placement and admission

Evidence was read at Cordum `origin/main`
`e5111643ee2e31d4467a518f972a6222fc3e151c`. Paths are in the cordum repository.

**(a) Admission.** The scheduler gates `sys.job.submit` with
`verifyJobSubmissionAuthority` (`core/controlplane/scheduler/engine.go:931`),
which calls `verifyReservedServiceAuthority`
(`core/controlplane/scheduler/engine_active_authority.go:104-138`):

- With handshake mode `off` it returns true at once (lines 109-110).
- In `warn` mode a missing or invalid token is admitted (lines 124-132).
- Otherwise it requires a verified token whose subject is a reserved identity
  equal to the packet's `sender_id` (lines 116-120). The reserved identities are
  `cordum-scheduler`, `api-gateway` and `workflow-engine`
  (`core/auth/servicetoken/servicetoken.go:64-74`).
- The compose defaults are `CORDUM_CAP_PROFILE=compat` and
  `CORDUM_SDK_HANDSHAKE=off` (`docker-compose.yml:185-186`).
- Bus `JobCancel` uses the session-token gate
  (`engine.go:1047-1057`, `engine_session_boundary.go:24-38`), which is a no-op
  when handshake mode is `off` (`token_middleware.go:9`, `:116`).

So a bridge with its own `sender_id` is admitted on compat (off or warn) and
rejected under `enforce` and on CAP-PRODUCTION.

**(b) Input path.** On compat the job input is a legacy pointer:

- `readContextResource` (`core/controlplane/scheduler/engine_resource.go:25-41`,
  called for schema validation at `engine.go:2167-2179`; the d2f6ec2b reference
  `engine.go:2171-2179` now holds that caller) reads `context_ref` through the
  registry, or `context_ptr` through the legacy loader.
- The legacy loader binds the pointer to the job: the key MUST equal
  `ctx:<job_id>` (`core/infra/resourceio/legacy_key.go:21-41`,
  `core/infra/store/redis_store.go:222-224`), written as `redis://ctx:<job_id>`.
- Legacy pointers are enabled only off the production profile
  (`cmd/cordum-scheduler/main.go:481-484`, and the safety client at
  `:405-410`).
- The CAP Go worker runtime reads the same pointer from the shared Redis
  (`sdk/go/runtime/runtime.go` lines 405 and 466-484 in this repository).

So the bridge delivers input by writing `ctx:<job_id>` to the deployment's Redis
and publishing `context_ptr = redis://ctx:<job_id>`. `ResourceRef` stays excluded
in v1.

**(c) Governance.** A bridge-published job takes the same path as a gateway job
once admitted:

- `handleJobRequest` (`engine.go:1356`) takes the job lock, reads the state, and
  records `PENDING` for a new job (`engine.go:1457-1461`). A redelivered
  request for a job that is already dispatched, running or terminal is
  dropped (`engine.go:1390-1416`).
- `processJob` (`engine.go:1489`) runs the safety check
  `checkSafetyDecisionTraced` (`engine.go:1652`).
- A deny records `DENIED` and emits a `DENIED` `JobResult` on the DLQ subject
  (`engine.go:1755-1767`, `emitDLQ` at `engine.go:3406-3458`).
  `RequireApproval` parks the job (`engine.go:1743-1751`); it has no `JobStatus`,
  so the A2A task stays `SUBMITTED` until the job moves on.
- Before `handleJobRequest`, `handlePacket` validates the request and rejects a
  tenant mismatch between `tenant_id` and `env["tenant_id"]` (`engine.go:946-973`).

**(d) Packaging.** The bridge goes in `internal/a2abridge` with a
`cmd/cap-a2a-bridge` binary, beside the existing `internal/governance`,
`internal/releasetruth` and `internal/tck`, so the `cap/v2` module gains no
public API.

On a copy of this module's `go.mod`, `go get github.com/a2aproject/a2a-go/v2@v2.6.0`
adds three modules and changes no existing version:

- `github.com/a2aproject/a2a-go/v2` v2.6.0;
- `github.com/a2aproject/a2a-go` v0.3.15;
- `go.uber.org/goleak` v1.3.0.

a2a-go requires `go 1.26.0` and `grpc v1.83.2`, which equal this module's current
`go` directive and grpc requirement.

**Verdict: KEEP-D3.** The bridge is a CAP-native gateway in this repository.

- The raw `sys.job.submit` path delivers input on compat (b).
- It is governed by the scheduler's job-state record and safety check (c).
- It needs no new public API (d).

A Cordum-side bridge over the Gateway REST API
(`core/controlplane/gateway/gateway.go:1509-1512` and `:1574`) is therefore not
required for the governed proof.

**Production limitation, stated plainly.** The bridge is admitted only while the
scheduler runs with handshake mode `off` or `warn`. That covers the compat
profile and the compose defaults. Under `enforce` or CAP-PRODUCTION it is
rejected (a), and legacy input pointers are disabled (b).

On compat, trust rests on network position:

- any NATS client can publish a `JobRequest` with any `tenant_id`;
- any NATS client can publish a `JobResult` for a known `job_id`, and the bridge
  cannot tell it apart from a worker's result;
- cancel outcomes are inferred, as section 5 describes.

Fronting the bridge through the Cordum gateway, or giving it a reserved
service identity, before any production use is a documented follow-up. It is not
current behavior.

## 9. Vector file

[vectors/a2a-v1.json](vectors/a2a-v1.json) has top-level `schema_version` (1),
`a2a_version` (`1.0.0`), `envelope_media_type` and `cases`. Each case requires
`id`, `description`, `cap_events`, `expected_a2a_state` and `expected_terminal`.
The optional fields are:

- `caller` and `owner`: `{tenant, principal}`. `owner` defaults to `caller`.
- `turns`: the A2A messages, with `message_id`, `task_ref` and `parts`.
- `a2a_call`: the A2A request under test, applied after `cap_events`.
- `expected_event_order`: the A2A status updates in order.
- `expected_publishes`: the counts `{job_request, job_cancel}`.
- `expected_error`: a reason from the section 6.1 table.

`cap_events[].kind` is one of:

- `submit_published`: the bridge published turn N's `JobRequest`;
- `progress`;
- `result`;
- `cancel_published`;
- `cancel_grace_expired`.

Each event may carry `turn`, `job_status`, `envelope` and `duplicate`.

When `expected_error` is set, `expected_a2a_state` is the state visible to the
caller. It is `TASK_STATE_UNSPECIFIED` when no task is visible.
