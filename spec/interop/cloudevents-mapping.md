# CloudEvents 1.0 Mapping for CAP Packets

This document maps four kinds of CAP packet on the NATS reference transport to
[CloudEvents 1.0][ce-spec] events in the JSON event format. RFC 2119/8174 key
words are normative. Sections marked informative are not.

## Status and scope

- This is a documented mapping only. No CAP SDK, CAP command, or Cordum
  component implements it or emits these events. A case-insensitive search for
  `cloudevent` finds no code in CAP at commit `85097a1` or in Cordum at commit
  `ebc0dd6`; the only hits are prose in CAP's `docs/ecosystem.md`.
- It defines no exporter and no CloudEvents transport binding, and it assigns
  no NATS subject to CloudEvents. A deployment that emits these events chooses
  its own transport; the mapping does not depend on it.
- Event types end in `.v1`. A change to any rule here that alters an event's
  `id`, `data`, or another attribute for the same input requires a new type
  version.
- Wire impact: none. This document adds no `BusPacket` field, no enum value,
  and no generated code, and CAP-PRODUCTION v1 is unchanged. Trace context is
  carried as [trace-context.md](trace-context.md) defines.

## Mapping input

A mapper takes these inputs:

1. The actual NATS subject the packet arrived on. It is never read from the
   packet.
2. The admitted `BusPacket`.
3. The admission mode:
   - `production`: the packet passed every
     [CAP-PRODUCTION admission check](../19-cap-production-profile.md#admission-order)
     on that subject;
   - `legacy`: it passed ordinary validation
     ([16](../16-protocol-errors.md#ordinary-buspacket-errors)), for example
     [ValidateBusPacket](../../sdk/go/validate.go#L126).

   A mapper MUST NOT infer the mode from packet content. A legacy packet can
   carry `signature_metadata`; that does not make it `production`.
4. The received NATS headers.
5. For job events only, optionally the originating `JobRequest`, as the
   operator's control plane admitted and recorded it. It never comes from the
   packet being mapped. A mapper MUST ignore it unless its `job_id` equals the
   event's job ID.
6. The configured `source` ([Context attributes](#context-attributes)).

Order: admit or validate, then extract trace context as
[trace-context.md](trace-context.md#processing-order) requires, then map. A
packet that failed admission or validation MUST NOT be mapped. Mapping never
changes the admission outcome.

## Mapped packets

| Rule | `job.progress` | `job.result` | `alert` | `workflow.event` |
|---|---|---|---|---|
| Selector (subject, payload) | `sys.job.progress`, `JobProgress` | `sys.job.result`, `JobResult` | `sys.alert`, `SystemAlert` | `sys.workflow.event`, `SystemAlert` |
| `type` | `io.cordum.cap.job.progress.v1` | `io.cordum.cap.job.result.v1` | `io.cordum.cap.alert.v1` | `io.cordum.cap.workflow.event.v1` |
| `source` | the configured source | the configured source | the configured source | the configured source |
| `subject` | `job/<job_id>` of the `JobProgress` | `job/<job_id>` of the `JobResult` | absent | `workflow/<trace_id>` of the `BusPacket` |
| `id` | [Event id](#event-id) | [Event id](#event-id) | [Event id](#event-id) | [Event id](#event-id) |
| `time` | `BusPacket.created_at` | `BusPacket.created_at` | `BusPacket.created_at` | `BusPacket.created_at` |
| `datacontenttype` | `application/json` | `application/json` | `application/json` | `application/json` |
| `data` | the redacted `JobProgress` | the redacted `JobResult` | the redacted `SystemAlert` | the redacted `SystemAlert` |

- A `job.result` event is produced for every `JobResult` status. A consumer
  tells success from failure by `data.status`.
- Every other pair of subject and payload MUST NOT be mapped. That includes
  every other payload kind, a payload on a subject that is not its own, and the
  `SystemAlert` packets Cordum publishes on `sys.audit.export` and
  `sys.config.changed`
  ([Workflow events in Cordum today](#workflow-events-in-cordum-today-informative)).
- A mapper MUST NOT map a packet when:
  - a job event has an empty `job_id`. Legacy validation does not check
    `JobProgress`, so the mapper checks it;
  - `created_at` is unset or outside the proto3 Timestamp range, which is
    `0001-01-01T00:00:00Z` to `9999-12-31T23:59:59.999999999Z`;
  - an attribute value would violate its CloudEvents type, for example a
    forbidden character in a String ([Context attributes](#context-attributes));
  - no `source` is configured.

## Context attributes

- `specversion` is `"1.0"`.
- `source` is an absolute URI ([RFC 3986 §4.3][rfc3986]) that the operator
  configures for the producing deployment.
  - It MUST NOT be derived from packet content: not `sender_id`, not
    `trace_id`, not a header.
  - There is one value per deployment and no default. A mapper without a
    configured `source` emits no events. The vectors use
    `https://cap.example/clusters/dev`, a host under the reserved `.example`
    top-level domain.
  - CloudEvents requires `source` plus `id` to be unique for each distinct
    event, so mappers that share a `source` MUST use the same admission mode and
    the same [redaction list](#redaction-list) ([Event id](#event-id)).
- `time` is `BusPacket.created_at` in proto3 JSON Timestamp form: RFC 3339,
  normalized to `Z`, with 0, 3, 6, or 9 fractional digits. It is the sender's
  claim, not the mapper's clock; a mapper MUST NOT substitute its own time.
- `datacontenttype` is `application/json` and is always present.
- `dataschema` is not set in v1.
- Every string attribute MUST satisfy the CloudEvents String type: no
  U+0000-U+001F, U+007F-U+009F, noncharacters, or unpaired surrogates. `source`
  and `subject` MUST be non-empty. A mapper MUST NOT sanitize a violating
  value; it maps nothing for that packet.

## Event id

The `id` has two branches. Both give lowercase hex.

**Production packet.** The id is the 32-character hex form of
[`SignatureMetadata.message_id`](../../proto/cordum/agent/v1/buspacket.proto#L54),
which is exactly 16 bytes. Worked example, vector `result-succeeded-production`:

```
message_id, proto3 JSON (base64)  ASNFZ4mrze/+3LqYdlQyEA==
message_id, bytes                 01 23 45 67 89 ab cd ef fe dc ba 98 76 54 32 10
id                                0123456789abcdeffedcba9876543210
```

**Legacy packet.** The id is the 64-character hex SHA-256 of the UTF-8
[RFC 8785][rfc8785] (JCS) serialization of an object with the members `type`,
`subject`, `time`, and `data`.

- Each member has exactly the value the event carries. `subject` is omitted
  when the event has none, which is the case for `alert` events.
- Every number in `data` is a 32-bit integer. The mapped payloads have no
  floating-point field, and proto3 JSON writes 64-bit integers as strings, so
  JCS number serialization is exact.

Worked example, vector `progress-legacy-traced`. The canonical string is:

```
{"data":{"artifactPtrs":["art:fake/job-42/log-1"],"dispatch":{"assignedWorkerId":"worker-echo-1","attempt":"1","dispatchId":"disp-0001"},"jobId":"job-42","message":"indexed 40 of 100 files","percent":40,"status":"JOB_STATUS_RUNNING","stepId":"step-2"},"subject":"job/job-42","time":"2026-10-17T12:00:00.500Z","type":"io.cordum.cap.job.progress.v1"}
```

Its SHA-256, and therefore the event `id`, is
`00a2d42c64eb1979dafcc3d3e5382a92a25e8e25347685edc5e0f42a2a738425`.

Rules:

- A legacy packet's `message_id` is unverified and MUST NOT be used. An
  attacker can choose it so that it collides with a genuine event and, at a
  consumer that deduplicates on `source` plus `id`, suppresses it. Vector
  `progress-unverified-metadata` carries the production example's `message_id`
  and still gets a 64-character hash.
- Two distinct legacy events with identical `type`, `subject`, `time`, and
  `data` share an `id`, even when their extensions or headers differ.
  CloudEvents lets a consumer treat events with the same `source` and `id` as
  duplicates, so a consumer might treat them as one. `time` carries up to
  nanosecond precision, which makes this unlikely for honest producers.
- The 32- and 64-character forms never collide with each other. A redelivered
  production packet keeps its `id`, which CloudEvents allows for a re-sent
  duplicate. A byte-identical legacy redelivery hashes to the same `id`.
- CAP-PRODUCTION replay storage keys entries by tenant, audience, sender, and
  message ID ([19](../19-cap-production-profile.md#replay-and-at-least-once-delivery)).
  It does not make a message ID unique across senders. Message IDs are random,
  so a collision between honest senders is negligible. An authenticated sender
  can reuse an ID it has seen only on subjects its ACLs let it read
  ([19](../19-cap-production-profile.md#transport-and-authorization)). The id
  is a deduplication key, not a security boundary.

## Data

`data` is the proto3 JSON form of the payload message only: `JobProgress`,
`JobResult`, or `SystemAlert`. It is never the `BusPacket` envelope. Apply the
[proto3 JSON mapping][proto3-json] with its default options, then the
[redaction list](#redaction-list). The result is a JSON value, not a string.

- Field names are lowerCamelCase and enums are written by name.
- 64-bit integers are decimal strings, and `bytes` are padded base64.
- A `Timestamp` is written as in [Context attributes](#context-attributes).
- A map is an object. Default values and unknown fields are omitted.
- Member order is not significant. The legacy id canonicalizes it.

## Redaction list

A mapper MUST remove every member whose key exactly equals a spelling in this
table, at any depth, including the keys of map fields such as
`SystemAlert.details`. Matching is case-sensitive. Removing a member keeps its
parent object, even when the parent becomes empty.

| Proto name | JSON name | Where the field occurs | Why |
|---|---|---|---|
| `auth_token` | `authToken` | `BusPacket` ([L46](../../proto/cordum/agent/v1/buspacket.proto#L46)) | session token; [02](../02-envelope-buspacket.md#secret-handling-and-rejection) |
| `signature` | `signature` | `BusPacket` ([L45](../../proto/cordum/agent/v1/buspacket.proto#L45)) | security material |
| `signature_metadata` | `signatureMetadata` | `BusPacket` ([L24](../../proto/cordum/agent/v1/buspacket.proto#L24)) | its `message_id` surfaces only as the `id` |
| `identity` | `identity` | `BusPacket` ([L25](../../proto/cordum/agent/v1/buspacket.proto#L25)); `JobProgress` ([L208](../../proto/cordum/agent/v1/job.proto#L208)); `JobResult` ([L193](../../proto/cordum/agent/v1/job.proto#L193)); `JobRequest` ([L176](../../proto/cordum/agent/v1/job.proto#L176)); `Compensation` ([L152](../../proto/cordum/agent/v1/job.proto#L152)) | tenant, principal, actor, and delegation IDs |
| `env` | `env` | `JobRequest` ([L164](../../proto/cordum/agent/v1/job.proto#L164)); `Compensation` ([L144](../../proto/cordum/agent/v1/job.proto#L144)) | Cordum stores `tenant_id` and run IDs here ([engine_job.go:238-246][cordum-job-env]) |
| `labels` | `labels` | `JobRequest` ([L173](../../proto/cordum/agent/v1/job.proto#L173)); `JobMetadata` ([L134](../../proto/cordum/agent/v1/job.proto#L134)); `Compensation` ([L150](../../proto/cordum/agent/v1/job.proto#L150)) | Cordum stores workflow, run, and step IDs here ([engine_job.go:247-252][cordum-job-env]) |
| `error_message` | `errorMessage` | `JobResult` ([L189](../../proto/cordum/agent/v1/job.proto#L189)) | raw error text beyond `error_code` |

- The `BusPacket` rows cannot occur in `data`, because `data` holds the payload
  only. They are listed so that a mapper that serializes more than this
  document specifies still cannot export them. The originating `JobRequest` is
  an input, but its `env`, `labels`, and `identity` values never reach `data`,
  an attribute, or an extension.
- No attribute or extension value is ever taken from a listed field.
- These fields are kept:
  - `error_code` and `error_code_enum`;
  - `JobProgress.message`, and `SystemAlert.message` and `details`, because
    [02](../02-envelope-buspacket.md#secret-handling-and-rejection) already
    forbids security material and attacker-controlled echo in alerts;
  - pointers and `ResourceRef` values, because resolving them needs
    authorization ([19](../19-cap-production-profile.md));
  - `dispatch`, because admission binds it to the authenticated assigned worker.
- A mapper MAY redact more and MUST NOT redact less. Redacting more changes
  `data`, and so changes every legacy `id` that depends on it.
- The list is a field denylist, not a secret scanner. Kept free text is
  exported verbatim.

## Extension attributes

Every extension name matches `^[a-z0-9]{1,20}$`. CloudEvents asks only that
names SHOULD NOT exceed 20 characters; this mapping makes the limit a MUST for
its own names.

| Name | Type | Source | Presence |
|---|---|---|---|
| `captraceid` | String | `BusPacket.trace_id` | always |
| `capworkflowid` | String | originating `JobRequest.workflow_id` | job events, when the request is supplied and the value is non-empty |
| `capstepindex` | Integer | originating `JobRequest.step_index` | only alongside `capworkflowid`; `0` is allowed |
| `capparentjobid` | String | originating `JobRequest.parent_job_id` | job events, when the request is supplied and the value is non-empty |
| `traceparent` | String | [Trace context](#trace-context) | when the received context is valid |
| `tracestate` | String | [Trace context](#trace-context) | only alongside `traceparent`, when non-empty |

- `captraceid` is a CAP correlation ID. It is never a W3C trace ID
  ([trace-context.md](trace-context.md#why-not-a-buspacket-field)).
- `JobProgress` and `JobResult` carry no `workflow_id`, `step_index`, or
  `parent_job_id`
  ([job.proto L165-L167](../../proto/cordum/agent/v1/job.proto#L165-L167)).
  Only the originating `JobRequest` supplies them. Alert and workflow events
  carry no job extensions.
- Extensions are descriptive and never authority. A consumer MUST NOT select a
  tenant, principal, key, policy, or approval from any of them. `baggage`,
  `sender_id`, identity, tenant, and principal are never mapped.

## Trace context

[trace-context.md](trace-context.md) and its
[vectors](vectors/trace-context-v1.json) apply unchanged: exact-case header
lookup, admission before extraction, no context from an invalid header, no
baggage, and trace data is never authority.

- `traceparent` is present only when the received headers yield a valid
  context. Its value is `00-<trace-id>-<parent-id>-<flags>`, where `<flags>` is
  `01` when the extracted sampled flag is set and `00` otherwise.
  - W3C Trace Context Level 1 requires a version-00 emitter to set undefined
    flag bits to zero, and says a parser of a higher version reads only the
    sampled bit. The re-serialized value is therefore the same for flags `03`,
    flags `01`, and a future-version header.
  - It is the packet's creation context: the context the producer injected at
    publish. It is never the mapper's own span. This is the CloudEvents
    [distributed tracing][ce-dt] multi-hop rule, which carries the starting
    trace and no per-hop data.
- `tracestate` is present only alongside `traceparent`, and only when the
  extracted trace state is non-empty. Its value is that serialization.
- A missing or invalid context omits both extensions. It never blocks mapping.
  `baggage` is never mapped, even when present.
- Native propagators disagree on some `tracestate` inputs
  ([trace-context.md](trace-context.md#native-propagator-divergences-informative)),
  so `tracestate` can differ between implementations for those inputs. It is
  descriptive only. Extensions are not part of the legacy `id`, so a divergence
  never changes an `id`.
- The v1.0.2 distributed-tracing extension makes `traceparent` REQUIRED when
  the extension is used. The same document on the CloudEvents `main` branch,
  read on 2026-11-10, adds `MUST be a non-empty string`. A mapper that emits
  `traceparent` only for a valid context satisfies both.

## Workflow events in Cordum today (informative)

At Cordum commit `ebc0dd6ad210a95518efb2d7293a24fb0c1c6080`:

- A workflow `notify` step ("event" step) builds a `SystemAlert` and publishes
  it on `sys.workflow.event`. The `BusPacket` has `trace_id` set to the run ID
  and `sender_id` `workflow-engine`
  ([engine.go:1112-1143][cordum-notify]). So `workflow/<trace_id>` names one
  run.
- The alert fields come from the step input: `message`, a severity derived from
  `level`, `source_component`, the input's `details` object with every value
  stringified, and an optional `trace_id`
  ([engine_events.go:51-133][cordum-alert]). The panic
  recovery path puts the panic text in `message`
  ([engine_events.go:52-63][cordum-alert]). This mapping exports `message`
  verbatim, so that text would appear in `data.message`.
- This is not a typed lifecycle event. CAP defines no `WorkflowEvent` payload;
  [09](../09-transport-profile.md#subjecttopic-conventions) lists
  `sys.workflow.event` as a `BusPacket` subject only. Cordum's run and step
  transitions go to an internal timeline store and are not published on the
  bus ([engine_events.go:418-457][cordum-timeline]).
- Cordum also publishes `SystemAlert` packets on two other subjects, which are
  not mapped. On `sys.audit.export`, `message` holds a whole SIEM event as JSON
  and `details` holds `tenant_id`
  ([audit/nats.go:57-75][cordum-audit]). On `sys.config.changed`, `details`
  holds the config scope, its ID, and a timestamp
  ([handlers_config.go:216-240][cordum-config]).
- A typed workflow lifecycle payload needs a proto change and is outside this
  document.

## Security considerations

- Legacy events are unauthenticated. Forgery and a deliberately colliding `id`
  are possible. Use CAP-PRODUCTION where an `id` or a payload must be
  trustworthy.
- A production `id` is a deduplication key, not proof of origin
  ([Event id](#event-id)).
- `time`, the extensions, and every trace value are hints, even on a verified
  packet. Trace headers are outside the signed bytes
  ([trace-context.md](trace-context.md#carrier)).
- No sensitive value appears in a context attribute, as CloudEvents requires.
  No attribute or extension is taken from a redacted field.
- Kept free text is exported verbatim. Producers MUST follow
  [02](../02-envelope-buspacket.md#secret-handling-and-rejection): no security
  material and no attacker-controlled echo in alerts.

## Vectors

[vectors/cloudevents-v1.json](vectors/cloudevents-v1.json) has
`schema_version` 1. A validator needs only a case and `mapper_config`.

- `mapper_config.source` is the configured `source`. `redacted_keys` is the
  list in [Redaction list](#redaction-list).
- `cases`: each has `name`, `covers`, `input`, and `expected_cloudevent`.
  - `input.nats_subject` is the actual subject and `input.admission` is
    `production` or `legacy`.
  - `input.packet` is the admitted `BusPacket` in proto3 JSON.
    `input.originating_job_request`, when present, is the `JobRequest` in
    proto3 JSON.
  - `input.headers`, when present, holds `traceparent`, `tracestate`, and
    optionally `baggage`. Without `baggage`, the set equals the `headers` of one
    case in [trace-context-v1.json](vectors/trace-context-v1.json), and that
    case's `expected` gives the trace values. A CloudEvents vector therefore
    never depends on a behavior on which the native propagators disagree.
  - A mapper applied to `input` MUST produce exactly `expected_cloudevent`.
    The reference check recomputes every legacy `id` with `rfc8785` 0.1.4
    (Python), a sorted-key serializer, and `canonicalize` 2.1.0 (npm). All three
    give the same canonical string and digest.
- `unmapped_cases`: each has `name`, `input`, `expected` (`not_mapped`), and
  `reason`. A mapper MUST produce no event for them. They are separate from
  `cases` because every `expected_cloudevent` is a valid event.
- The vectors assume admission already succeeded. They are not signing or
  admission vectors, and their values are fake.

| Case | Shows |
|---|---|
| `progress-legacy-traced` | `job.progress`, legacy id, trace extensions with `tracestate` |
| `result-succeeded-production` | `job.result`, production id, job-context extensions, redaction of `identity`, `env`, `labels` |
| `result-failed-legacy-redacted` | failed result; `error_code` kept, `errorMessage` and `identity` removed |
| `alert-legacy` | `alert`, no subject, `Z`-normalized `time`, redaction inside `details` |
| `workflow-event-cordum-notify` | `workflow.event` as Cordum emits it today |
| `progress-unverified-metadata` | a legacy packet's `message_id` is not used; non-ASCII and control characters in `data` |
| `progress-invalid-traceparent` | invalid `traceparent` gives no trace extensions |
| `result-baggage-dropped` | unsampled flag kept as `-00`; `baggage` never mapped |
| `progress-future-version` | future-version header normalized to version `00` |
| `progress-tracestate-without-traceparent` | `tracestate` alone gives no trace extensions |

## References

- CloudEvents v1.0.2:
  - [core specification][ce-spec];
  - [JSON event format][ce-json];
  - [distributed tracing extension][ce-dt].
- [RFC 8785][rfc8785], JSON Canonicalization Scheme.
- [RFC 3339](https://www.rfc-editor.org/rfc/rfc3339) and [RFC 3986][rfc3986].
- [W3C Trace Context Level 1](https://www.w3.org/TR/trace-context-1/).
- [Protocol Buffers JSON mapping][proto3-json].
- CAP: [02 Envelope](../02-envelope-buspacket.md),
  [09 Transport Profile](../09-transport-profile.md),
  [16 Protocol Errors](../16-protocol-errors.md),
  [19 CAP-PRODUCTION Profile](../19-cap-production-profile.md),
  [trace-context.md](trace-context.md).

[ce-spec]: https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/spec.md
[ce-json]: https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/formats/json-format.md
[ce-dt]: https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/extensions/distributed-tracing.md
[rfc8785]: https://www.rfc-editor.org/rfc/rfc8785
[rfc3986]: https://www.rfc-editor.org/rfc/rfc3986#section-4.3
[proto3-json]: https://protobuf.dev/programming-guides/json/
[cordum-notify]: https://github.com/cordum-io/cordum/blob/ebc0dd6ad210a95518efb2d7293a24fb0c1c6080/core/workflow/engine.go#L1112-L1143
[cordum-alert]: https://github.com/cordum-io/cordum/blob/ebc0dd6ad210a95518efb2d7293a24fb0c1c6080/core/workflow/engine_events.go#L51-L133
[cordum-timeline]: https://github.com/cordum-io/cordum/blob/ebc0dd6ad210a95518efb2d7293a24fb0c1c6080/core/workflow/engine_events.go#L418-L457
[cordum-job-env]: https://github.com/cordum-io/cordum/blob/ebc0dd6ad210a95518efb2d7293a24fb0c1c6080/core/workflow/engine_job.go#L238-L252
[cordum-audit]: https://github.com/cordum-io/cordum/blob/ebc0dd6ad210a95518efb2d7293a24fb0c1c6080/core/audit/nats.go#L57-L75
[cordum-config]: https://github.com/cordum-io/cordum/blob/ebc0dd6ad210a95518efb2d7293a24fb0c1c6080/core/controlplane/gateway/handlers_config.go#L216-L240
