# CAP-PRODUCTION Profile

CAP-PRODUCTION is an opt-in security profile layered on CAP, not a fourth
conformance tier. RFC 2119/8174 key words are normative. A participant MUST
NOT advertise this profile until its authenticated worker handshake succeeds
and every mandatory dependency below is ready. Compatibility participants do
not become production participants merely by accepting additive fields.

## Admission order

A receiver MUST apply these checks to the exact received bytes before
protobuf dispatch or any state, resource, policy, audit, workflow, metric, or
dead-letter side effect:

1. authenticate encrypted transport and enforce subject ACLs;
2. strictly extract exactly one signature field and its signed body;
3. validate all bounds and `SignatureMetadata` fields;
4. verify with a locally trusted sender/tenant-scoped key;
5. bind the actual transport subject as expected audience;
6. compare every identity mirror with authenticated session identity; and
7. atomically admit message ID and signed-body digest to replay storage.

Malformed, duplicate, non-minimal, unknown, missing, expired, or unavailable
inputs MUST be rejected. Production admission is fail closed.

## Transport and authorization

Transport MUST provide peer authentication, confidentiality, integrity, and
replay-resistant session establishment. Subject ACLs MUST restrict publishers
and subscribers to assigned CAP subjects. Payload tokens, sender IDs, keys,
tenants, or audiences MUST NOT replace transport/session proof. The actual
subscribed subject is authoritative and MUST be passed to admission logic.

## Versioned raw-wire signatures

`SignatureMetadata` is REQUIRED. `profile_version` and `algorithm` MUST be
recognized; `message_id` MUST be exactly 16 unpredictable bytes; `audience`,
`expires_at`, and `key_id` MUST be nonempty and bounded. Expiry MUST fall
within the operator's maximum lifetime and clock skew.

Version 1 signs:

```
SHA-256("CAP-PRODUCTION-SIGNATURE-V1\0" || exact_unsigned_packet_wire)
```

The producer MUST serialize the unsigned packet once, sign that preimage, and
append field 14 without reserializing the signed body. The receiver MUST
derive identical unsigned bytes by strict raw-wire extraction. Clearing a
parsed signature then reserializing is forbidden because serializers may
normalize different wire bytes. Only a local trust store may resolve `key_id`;
packet-provided keys MUST NOT be trusted. Lookup MUST bind authenticated tenant
and sender.

## Replay and at-least-once delivery

Replay storage MUST atomically key entries by authoritative tenant, actual
audience, authenticated sender, and message ID until expiry plus clock skew,
and retain the signed-body digest. First-seen input is admitted. An identical
redelivery is acknowledged without a second side effect. The same ID with a
different digest is rejected and audited. Store unavailability is rejection.
This permits JetStream at-least-once delivery without mutable replay identity.

## Identity binding

Authenticated session identity is authoritative. Every nonempty tenant,
principal, actor, delegation, sender, metadata, environment, label, and legacy
mirror MUST exactly equal its `IdentityBinding`. Whitespace, case folding,
precedence, or `default` fallback MUST NOT repair a mismatch. Ambiguity MUST
be rejected before job lookup or mutation.

## Dispatch and event fencing

Before publishing a physical dispatch, the control plane MUST atomically
persist an unpredictable `dispatch_id`, monotonically increasing `attempt`,
assigned worker, and authoritative tenant. Broker redelivery retains that
identity; a retry receives a new identity.

Results, progress, cancellation, artifacts, workflow events, and compensation
MUST be accepted only after an atomic comparison with current dispatch,
attempt, worker, tenant, event message ID/digest, and allowed state. Stale,
future, wrong-worker, or duplicate events MUST NOT mutate state. Privileged
all-attempt cancellation is a separate authenticated control-plane operation.

## Safety and decision caching

Policy evaluation and mandatory dependencies MUST fail closed. A cache key
MUST bind the active policy snapshot/version and a canonical encoding of the
complete authoritative decision input, including job ID whenever selection or
evaluation is job scoped. Content-sensitive ALLOW MUST NOT be reused when
content was absent, truncated, unresolved, expired, or not integrity-verified.
Positive client caching requires a signed bounded policy lease; otherwise it
MUST be disabled. DENY remains DENY. Missing, unknown, or malformed decisions
are DENY.

## Compensation

Compensation identity, delegation, capability, topic, adapter, resources,
budget, and risk MUST be no more privileged than the parent authorization.
Empty legacy mirrors inherit; different nonempty values are rejected. Each
execution MUST receive a fresh fenced dispatch and re-evaluate current policy.
Safety unavailability leaves a durable pending/denied outcome and MUST NOT
permit execution.

## Resource references

Production content MUST use `ResourceRef`: a known operator-installed
`resolver_id`, normalized credential-free URI, exactly 32-byte SHA-256,
declared media type/size, unexpired deadline, and purpose. Dual legacy and
structured fields MUST identify identical bytes or be rejected.

Resolvers MUST use local credentials and exact scheme, authority, bucket,
path/key, media, tenant, and maximum-byte allowlists. They MUST reject
userinfo, sensitive query/fragment, encoded traversal, NUL, unapproved
redirect/DNS/IP targets, cross-tenant keys, short/oversize bodies, type/digest
mismatch, and expiry before or during fetch. Unknown resolvers have no generic
HTTP, file, or network fallback.

## Compatibility and migration

Compatibility mode MAY accept legacy unsigned packets, string pointers,
missing identity, or warn-only validation only behind explicit operator
configuration and telemetry. It MUST NOT advertise CAP-PRODUCTION or silently
enable fail-open. Rollout order is schema support, trust/replay/resolver
dependencies, authenticated handshake, producer signing, receiver enforcement,
then profile advertisement. Rollback MUST withdraw advertisement before
compatibility admission is enabled.
