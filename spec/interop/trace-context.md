# W3C Trace Context for CAP over NATS

This contract defines how CAP participants carry
[W3C Trace Context](https://www.w3.org/TR/trace-context-1/) on the NATS
transport profile and which OpenTelemetry messaging spans they record.
RFC 2119/8174 key words are normative. Sections marked informative are not.

## Status and scope

- Scope: CAP packets on the NATS reference transport
  ([09](../09-transport-profile.md)). Other transport profiles are out of scope.
- Wire impact: none. This contract adds no `BusPacket` field, no enum value,
  and no generated code. CAP-PRODUCTION v1 is unchanged.
- Span names and attributes follow the OpenTelemetry semantic conventions for
  messaging spans at release
  [v1.44.0](https://github.com/open-telemetry/semantic-conventions/blob/v1.44.0/docs/messaging/messaging-spans.md).
  Upstream, those conventions have **Development** status. This contract pins
  v1.44.0 and changes only together with a new vector `schema_version`.
- This document defines a contract. It does not claim that any CAP SDK or
  Cordum implements it today; [Current Cordum behavior](#current-cordum-behavior-informative)
  records the known differences.

## Carrier

1. The carrier is the NATS message header block. A producer with a valid span
   context MUST write it as exactly one `traceparent` header and, when the trace
   state is non-empty, exactly one `tracestate` header. Header keys MUST be the
   exact lowercase strings `traceparent` and `tracestate`.
2. Header values follow the Trace Context Level 1 grammar. A producer MUST emit
   `traceparent` version `00`.
3. A consumer MUST look up both keys by exact case. A key with any other casing,
   for example `Traceparent`, is treated as absent. The NATS clients CAP pins
   all look headers up by exact case: nats.go v1.49.0 `Header.Get` is
   case-sensitive, nats.js 2.29.3 `MsgHdrs.get` defaults to `Match.Exact`, and
   nats-py 2.16.0 exposes headers as a plain `dict`. Trace Context §3.2.1 asks vendors
   to accept any casing because HTTP field names are case-insensitive. NATS
   header keys are case-sensitive, so CAP fixes one spelling instead.
4. A producer MUST NOT emit more than one field for either key. The vectors
   cover single-valued headers only. However a consumer treats a repeated
   field, the result MUST NOT affect admission or authority.
5. Trace headers are outside the signed bytes. CAP-PRODUCTION signs the
   `BusPacket` bytes only ([19](../19-cap-production-profile.md)). An
   intermediary or publisher can add, change, or remove these headers without
   invalidating a signature, so they are untrusted correlation hints even on a
   verified packet.

## Why not a BusPacket field

Trace context travels in headers, not in a `BusPacket` field:

- CAP-PRODUCTION v1 uses a closed outer grammar
  ([sdk/go/production_wire.go:57-58](../../sdk/go/production_wire.go#L57-L58)).
  `productionBusPacketWireType`
  ([sdk/go/production_wire.go:80-89](../../sdk/go/production_wire.go#L80-L89))
  accepts only tags 1-6 and 10-22. A new outer field, such as tag 7, is
  therefore malformed wire to every deployed verifier, and
  [19](../19-cap-production-profile.md#admission-order) requires unknown input
  to be rejected, fail closed. A trace-context field would need a new profile
  version.
- `BusPacket.trace_id`
  ([buspacket.proto:20](../../proto/cordum/agent/v1/buspacket.proto#L20)) is
  the CAP correlation ID, not a W3C trace-id. It MUST NOT be parsed as a W3C
  trace-id, and MUST NOT be generated from, or overwritten by, header values.
- Cordum already carries trace context in NATS headers. It injects at
  [core/infra/bus/nats.go:411](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/bus/nats.go#L411)
  and extracts at
  [core/infra/bus/nats.go:516](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/bus/nats.go#L516)
  and
  [core/infra/bus/nats.go:557](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/bus/nats.go#L557).
- Earlier planning reserved `BusPacket` tag 7 and oneof tags 28-30 for trace
  context. That reservation is released.

## Baggage

- An implementation MUST NOT inject a `baggage` header and MUST NOT extract one
  by default. The propagator for this contract is the W3C trace-context
  propagator alone, not a composite that also includes W3C Baggage.
- A consumer MUST ignore a received `baggage` header unless an operator
  explicitly enables baggage. Enabled baggage is still subject to
  [Authority](#authority) and MUST NOT be copied into span attributes.

Informative: Cordum differs today. Its global propagator combines
TraceContext and Baggage
([core/infra/otel/tracer.go:113-116](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/otel/tracer.go#L113-L116)),
and its NATS header helpers apply that global propagator
([core/infra/otel/nats_propagator.go:31-52](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/otel/nats_propagator.go#L31-L52)).
Cordum therefore forwards any baggage present in a context. This contract
records that difference; it does not change Cordum.

## Processing order

A consumer MUST handle each message in this order:

1. **Admit.** Apply CAP-PRODUCTION admission
   ([19](../19-cap-production-profile.md#admission-order)) to a production
   packet, or ordinary validation
   ([16](../16-protocol-errors.md#ordinary-buspacket-errors)) to a legacy
   packet. Neither step reads trace headers.
2. **Extract.** For an admitted packet only, extract `traceparent` and
   `tracestate` from the headers.
3. **Process.** Start the CONSUMER `process` span as
   [Linking and parenting](#linking-and-parenting) describes, and invoke the
   handler inside it.

Rules:

- Trace headers MUST NOT be read, parsed, or used before admission or
  validation succeeds.
- A rejected packet is dropped, as [16](../16-protocol-errors.md) requires. Its
  headers MUST NOT be extracted and MUST NOT become a parent or a link. An
  implementation MAY record a `process` span for the rejection. That span MUST
  be a new root, MUST set `error.type`, and MUST NOT carry
  `messaging.message.id` or `messaging.message.conversation_id`.
- A missing or invalid creation context MUST NOT reject, delay, or reroute the
  packet. Processing continues and starts a new root: no part of the invalid
  context is used as a parent or a link. Invalid means that the native W3C
  propagator yields no valid remote span context, as in the `invalid`
  [vectors](#vectors).
- Trace-state handling never changes whether `traceparent` is valid
  (Trace Context §3.3).
- The sampled flag is a hint. A consumer MAY apply its own sampler and MAY
  ignore the remote sampled flag.

## Authority

Trace data is never authority. `traceparent`, `tracestate`, `baggage`, the
trace-id, parent-id, and flags they carry, and any span or link built from
them MUST NOT select or influence any of the following:

- tenant, principal, or sender identity;
- a signing or verification key;
- policy, safety decision, or approval;
- admission, replay fencing, idempotency, routing, or dispatch.

These decisions use only the authenticated transport session and the verified
or validated packet ([19](../19-cap-production-profile.md#identity-binding)).
Header values MUST NOT be written into a `BusPacket`, job metadata, or an
audit record as if they were authenticated.

## Spans

| Span | Kind | Name | Created |
|---|---|---|---|
| publish | `PRODUCER` | `publish <destination>` | once per published packet; its context is the creation context injected into the headers |
| process | `CONSUMER` | `process <destination>` | once per admitted packet, covering handler execution; a rejection span is optional, as [Processing order](#processing-order) describes |

- `<destination>` is the actual subject when that subject is in the
  [09 subject table](../09-transport-profile.md#subjecttopic-conventions):
  either a fixed `sys.*` subject or `job.<pool>`. The operator configures the
  pools, so `job.<pool>` stays low-cardinality.
- For a broker-generated reply inbox, for example the reply to a worker-trust
  request, the span name omits the destination and is just `publish` or
  `process`, following the semconv cardinality rule.
- Spans are per message. This contract defines no batch, `create`, `receive`,
  or `settle` spans.

## Attributes

Values are exact strings unless shown as booleans. Requirement levels use the
semconv vocabulary.

### Publish span

| Attribute | Value | Requirement |
|---|---|---|
| `messaging.system` | `nats` | Required |
| `messaging.operation.name` | `publish` | Required |
| `messaging.operation.type` | `send` | Required |
| `messaging.destination.name` | the actual subject | Required for a 09 subject; MUST NOT be set for a reply inbox |
| `messaging.destination.temporary` | `true` (boolean) | Required for a reply inbox; otherwise not set |
| `messaging.destination.anonymous` | `true` (boolean) | Required for a reply inbox; otherwise not set |
| `messaging.message.conversation_id` | `BusPacket.trace_id` of the published packet | Required |
| `error.type` | a value from [Error type values](#error-type-values) | Conditionally Required: if and only if the publish failed |

`messaging.message.id` is not set on publish spans.

### Process span

| Attribute | Value | Requirement |
|---|---|---|
| `messaging.system` | `nats` | Required |
| `messaging.operation.name` | `process` | Required |
| `messaging.operation.type` | `process` | Required |
| `messaging.destination.name` | the actual subject | Required for a 09 subject; MUST NOT be set for a reply inbox |
| `messaging.destination.temporary` | `true` (boolean) | Required for a reply inbox; otherwise not set |
| `messaging.destination.anonymous` | `true` (boolean) | Required for a reply inbox; otherwise not set |
| `messaging.consumer.group.name` | the NATS queue group | Conditionally Required: when the subscription uses a queue group |
| `messaging.destination.subscription.name` | the JetStream durable consumer name | Conditionally Required: when the message is consumed through a JetStream durable consumer |
| `messaging.message.id` | the 16-byte `SignatureMetadata.message_id` ([buspacket.proto:54](../../proto/cordum/agent/v1/buspacket.proto#L54)) as 32 lowercase hex characters | Conditionally Required: if and only if the packet passed CAP-PRODUCTION admission |
| `messaging.message.conversation_id` | `BusPacket.trace_id` | Required for a packet that passed CAP-PRODUCTION admission; Recommended for a legacy packet, under the rules below |
| `error.type` | a value from [Error type values](#error-type-values) | Conditionally Required: if and only if processing failed or the packet was rejected |

Rules for both spans:

- `messaging.system` is `nats`, a custom value: semconv v1.44.0 lists no
  well-known value for NATS.
- `messaging.message.id` MUST NOT be set for a legacy or rejected packet,
  because only CAP-PRODUCTION admission verifies the message ID.
- `messaging.message.conversation_id` MUST NOT be set for a rejected packet. A
  legacy packet is not authenticated, so its `trace_id` MUST be bounded and
  sanitized as [10](../10-security-observability.md) requires before it is
  recorded, or the attribute is omitted.
- An implementation MAY add `server.address`, `server.port`,
  `network.peer.address`, and `network.peer.port`. It MUST NOT add any other
  attribute to these spans.
- When `error.type` is set, the span status is `ERROR`. The status description
  is empty or equal to the `error.type` value. Implementations MUST NOT record
  exception events, stack traces, or error messages on these spans.

### Error type values

`error.type` is a closed enum. An implementation MUST use exactly one of these
values and MUST NOT emit free text.

| Value | Span | Meaning |
|---|---|---|
| `PROTOCOL_VERSION_MISMATCH` | process | unsupported `protocol_version` ([16](../16-protocol-errors.md#ordinary-buspacket-errors)) |
| `PROTOCOL_MALFORMED_PACKET` | process | the packet fails bounded deserialization, including CAP-PRODUCTION raw-wire checks |
| `PROTOCOL_UNKNOWN_PAYLOAD` | process | no recognized payload |
| `PROTOCOL_SIGNATURE_INVALID` | process | the signature fails verification |
| `PROTOCOL_SIGNATURE_MISSING` | process | a required signature is missing |
| `ADMISSION_REJECTED` | process | CAP-PRODUCTION admission rejected the packet for any other reason, such as replay, expiry, an audience or identity mismatch, an unknown key, or an unavailable replay store. This is deliberately coarse, so telemetry cannot reveal which check failed. |
| `HANDLER_FAILED` | process | the packet was admitted and the handler returned an error or raised one |
| `TRANSPORT_PUBLISH_FAILED` | publish | publishing to NATS failed; this is the name of ErrorCode 400 in [13](../13-error-codes.md) |
| `_OTHER` | both | any other failure; this is the semconv fallback value |

The `PROTOCOL_*` values are the [16](../16-protocol-errors.md) ErrorCode names
verbatim.

### Forbidden attributes

The following MUST NOT appear in span attributes, span names, link
attributes, span events, or status descriptions:

- `auth_token` from `BusPacket` or `Heartbeat`, and any other session token or
  credential;
- `signature` bytes, and every `SignatureMetadata` field except the message ID
  defined above;
- identity values: `sender_id`, worker, agent, and actor IDs, and key IDs;
- tenant identifiers, such as `tenant_id`;
- principal identifiers, such as `principal_id`;
- payload bytes and any decoded payload field;
- pointers: `context_ptr`, `result_ptr`, `artifact_ptrs`, memory pointers, and
  `ResourceRef` values;
- baggage entries;
- raw error text: exception messages, parse errors, and stack traces;
- copies of the `traceparent` or `tracestate` header values.

## Linking and parenting

- The publish span's context is the creation context. The producer injects it
  and creates no separate `create` span.
- For an admitted packet with a valid extracted creation context, the process
  span MUST link to that context.
- The process span uses the creation context as its parent only when no
  ambient span exists. If no ambient span is active when the process span
  starts, the creation context MUST be its parent. If an ambient span is
  active, the process span is that span's child and keeps the link.
- Without a valid creation context, the process span has no link. It is a new
  root, or the ambient span's child when one is active.

## Vectors

[vectors/trace-context-v1.json](vectors/trace-context-v1.json) has
`schema_version` 1.

- `traceparent_cases`: each case has a unique `name`, `headers`, `verdict`, and
  `expected`. `headers` holds only the lowercase keys `traceparent` and
  `tracestate`, and an absent header is omitted. An adapter test MUST put
  exactly `headers` into a carrier, extract with its native W3C propagator, and
  check the result:
  - `valid`: the result is a valid remote span context. Its lowercase-hex
    trace ID and span ID equal `expected.trace_id` and `expected.parent_id`.
    Its sampled bit equals `expected.sampled`. Its serialized trace state
    equals `expected.tracestate`, which is `""` when no member survives.
  - `invalid`: the result is not a valid remote span context, and `expected`
    is `null`.
- `attribute_maps.publish` and `attribute_maps.process`: given `input`, the
  span MUST have exactly the `span_name` and `span_kind` shown, and MUST carry
  every listed attribute with exactly the listed value. Apart from the optional
  environment attributes above, it MUST carry no other attribute.
  In `input`, `subject` is the actual NATS subject, `queue_group` is the
  subscription's queue group, `admission` value `cap_production_verified`
  means the packet passed CAP-PRODUCTION admission, `bus_packet_trace_id` is
  `BusPacket.trace_id`, and `signature_metadata_message_id_base64` is the
  proto3 JSON (base64) form of the 16-byte message ID.
- All three propagators named in `sources.verified_with` produced the expected
  result for every traceparent case. Any change to the file MUST re-run that
  cross-check and keep only the cases where all three agree.

## Native propagator divergences (informative)

These inputs were measured on 2026-10-26 with go.opentelemetry.io/otel
v1.47.0, opentelemetry-api 1.41.1, and @opentelemetry/core 2.12.0. The
propagators disagree on them, so they are excluded from the vectors. Adapters
MUST NOT depend on the outcome for any of them, and no outcome affects
admission or authority.

| Input | Go | Python | Node |
|---|---|---|---|
| version `00`, flags `09` | invalid | valid, sampled | valid, sampled |
| version `00`, flags `ff` | invalid | valid, sampled | valid, sampled |
| leading space before the `traceparent` value | invalid | valid | valid |
| trailing space after the `traceparent` value | invalid | valid | valid |
| 33 `tracestate` members | trace state dropped | trace state dropped | first 32 members kept |
| one malformed member among valid members | trace state dropped | trace state dropped | valid members kept |
| `tracestate` over 512 characters (9 members, 575 characters) | kept | kept | trailing members removed, leaving 8 (511 characters) |

## Current Cordum behavior (informative)

At Cordum commit `e5111643ee2e31d4467a518f972a6222fc3e151c`:

- Publish injects through the global propagator at
  [core/infra/bus/nats.go:411](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/bus/nats.go#L411).
  That propagator includes Baggage; see [Baggage](#baggage).
- The core subscription callback extracts at
  [core/infra/bus/nats.go:516](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/bus/nats.go#L516-L517)
  before it admits the packet on the next line. The JetStream callback does
  the same at
  [core/infra/bus/nats.go:557](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/bus/nats.go#L557-L558).
  This contract requires admission before extraction.
- A second JetStream callback starts from `context.Background()` and does not
  extract at all
  ([core/infra/bus/nats.go:732-733](https://github.com/cordum-io/cordum/blob/e5111643ee2e31d4467a518f972a6222fc3e151c/core/infra/bus/nats.go#L732-L733)).

This contract records these differences. It does not change Cordum.

## References

- Trace Context Level 1, W3C Recommendation, 23 November 2021:
  <https://www.w3.org/TR/trace-context-1/>
- OpenTelemetry semantic conventions v1.44.0:
  - [messaging spans](https://github.com/open-telemetry/semantic-conventions/blob/v1.44.0/docs/messaging/messaging-spans.md)
  - [messaging attribute registry](https://github.com/open-telemetry/semantic-conventions/blob/v1.44.0/docs/registry/attributes/messaging.md)
  - [`error.type`](https://github.com/open-telemetry/semantic-conventions/blob/v1.44.0/docs/registry/attributes/error.md)
- CAP: [09 Transport Profile](../09-transport-profile.md),
  [10 Security and Observability](../10-security-observability.md),
  [13 Error Codes](../13-error-codes.md),
  [16 Protocol Errors](../16-protocol-errors.md),
  [19 CAP-PRODUCTION Profile](../19-cap-production-profile.md)
