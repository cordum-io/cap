# Capability Negotiation

CAP components advertise their role, supported protocol versions, and feature capabilities with a `Handshake`. A standalone `Handshake` broadcast is a legacy capability advertisement, not proof of identity. The authenticated worker trust exchange below binds the same capability payload to a registered worker proof key and a short-lived session.

## Capability Advertisement Flow

1. Component connects to the message bus.
2. Component publishes `BusPacket{Handshake}` to `sys.handshake`.
3. Schedulers and controllers consume `sys.handshake` and record the role, versions, and capabilities as legacy/untrusted unless an authenticated session already binds the same record.
4. An advertisement MAY narrow a candidate set (for example `compensation` or `progress`), but it MUST NOT grant or refresh dispatch authority.

Components SHOULD publish a Handshake immediately after establishing a bus connection. This standalone broadcast is informational; no explicit acknowledgment is defined. It MUST NOT create a proof-bound registry record or mint a session. In authenticated modes the worker includes the capability payload in `WorkerHandshakeAuthenticate`; a later `sys.handshake` broadcast cannot expand or override the authenticated record.

## Handshake Message

```proto
enum ComponentRole {
    COMPONENT_ROLE_UNSPECIFIED = 0;
    COMPONENT_ROLE_GATEWAY = 1;
    COMPONENT_ROLE_SCHEDULER = 2;
    COMPONENT_ROLE_WORKER = 3;
    COMPONENT_ROLE_ORCHESTRATOR = 4;
    COMPONENT_ROLE_CONTROLLER = 5;
}

message Handshake {
    string component_id = 1;
    ComponentRole role = 2;
    repeated int32 supported_versions = 3;
    map<string, bool> capabilities = 4;
    string sdk_version = 5;
    repeated string ready_topics = 6;
    string agent_name = 7;
}
```

See `proto/cordum/agent/v1/handshake.proto` for the canonical definition.

## Capability Registry

The `capabilities` map uses string keys with boolean values. A key set to `true` indicates the component supports that feature. A missing key or `false` value indicates the feature is not supported.

### Initial Capability Keys

| Key             | Description                                                        |
|-----------------|--------------------------------------------------------------------|
| `signatures`    | Component signs outgoing BusPackets and verifies incoming signatures. |
| `compensation`  | Worker supports compensation/rollback actions.                     |
| `progress`      | Worker emits `JobProgress` messages during execution.              |
| `cancel`        | Worker handles `JobCancel` requests.                               |
| `error_codes`   | Component populates `error_code_enum` in JobResult.                |
| `heartbeats`    | Component emits periodic Heartbeat messages.                       |
| `workflows`     | Component supports multi-step workflow orchestration.              |

Implementations MAY define additional capability keys. Custom keys SHOULD use a namespaced prefix (e.g., `x-myorg-feature`) to avoid collisions with future protocol-defined keys.

## Readiness Topics

`ready_topics` lists the concrete job subjects/topics the worker is currently ready to serve. This is distinct from `capabilities`:

- `capabilities` answers **what** the component can do.
- `ready_topics` answers **where** the worker is currently available to receive work.

Schedulers MAY use `ready_topics` as an additional dispatch filter before selecting a worker. This lets a worker advertise broad static capabilities while temporarily narrowing the set of routed topics during startup or reconfiguration. In an authenticated session, effective readiness is the intersection of the advertised topics and the server-authoritative topics allowed for that worker/agent/tenant. A worker-controlled topic can only narrow authority; it can never grant a subject absent from the authoritative session or enrollment record.

Workers SHOULD publish `ready_topics` in a stable order so registries and tests can compare handshake payloads deterministically. Older workers that omit `ready_topics` remain wire-compatible; schedulers SHOULD treat the field as unknown/unspecified rather than as an error.

## Agent Display Name

`agent_name` (field 7) is an optional human-facing **display label** for the component (e.g., `Claude Code — Billing Bot`), surfaced in registries, dashboards, and audit summaries so operators can attribute activity to a recognizable name rather than an opaque `component_id`. SDKs sanitize and bound it (trim, collapse internal whitespace, drop control characters, cap at 128 characters).

`agent_name` is **not an authentication authority**. It is self-reported and spoofable, so schedulers, registries, and audit pipelines MUST prefer authenticated identity records (worker credential / Agent Identity) over it and MUST NOT use it for authorization or identity resolution. It MUST NOT carry secrets, tokens, or PII. Older components that omit `agent_name` remain wire-compatible; consumers SHOULD treat the field as unknown/unspecified rather than as an error.

## Version Negotiation

Components advertise the wire versions they support via `supported_versions`. Schedulers SHOULD pick the highest version common to both the scheduler and the target component. If no common version exists, the scheduler SHOULD reject jobs to that component with `ERROR_CODE_PROTOCOL_VERSION_MISMATCH`.

Currently, the only defined wire version is `1`. Components SHOULD include `1` in `supported_versions`. The authenticated worker trust profile does not negotiate a range: the envelope, trust payload, and embedded capability handshake MUST meet the exact v1 rules below.

## Reconnection and Liveness

- Components SHOULD re-publish their Handshake on every reconnect to the bus.
- Schedulers MAY expire legacy registry entries after a configured timeout. Authenticated liveness/readiness expiry and refresh MUST use a live bound session; tokenless Handshake or Heartbeat traffic cannot refresh it.
- In `off` mode or an explicitly configured legacy migration path, components that never send a Handshake may be treated as CORE-only for compatibility. That fallback is not authenticated identity, readiness, or session evidence.
- If `ready_topics` is absent, legacy policy may use existing routing/heartbeat signals. An authenticated registry MUST NOT infer additional authorized topics from absence.
- After reconnect, a worker that requires authenticated admission MUST re-establish and verify its live trust state before it is counted as authenticated or receives session-required dispatch. A prior connection's token does not make the new transport authenticated by itself.

## Security

- A legacy `Handshake` is a capability advertisement, not an authentication exchange and not authority to mint a session.
- Handshake packets SHOULD be signed like any other BusPacket when signatures are enabled, but a generic packet signature alone does not perform proof-key enrollment or create a session.
- Schedulers SHOULD verify Handshake signatures before recording the advertisement. Only a successfully verified trust authenticate phase may make the capability record proof-bound.
- The `component_id` in a Handshake MUST match the `sender_id` in the enclosing BusPacket.

## Authenticated Worker Trust Handshake

The worker trust handshake establishes a short-lived runtime session. It is distinct from the legacy capability advertisement above, while reusing the existing `Handshake` message as the single capability payload inside `WorkerHandshakeAuthenticate`.

### Subjects and phases

All four messages are carried in `BusPacket` and use NATS request/reply. The reply MUST use the transport-provided reply inbox; implementations MUST NOT derive a reply subject from caller-controlled IDs.

| Subject | Request payload | Reply payload |
|---|---|---|
| `sys.worker.handshake.challenge` | `WorkerHandshakeChallengeRequest` | `WorkerHandshakeChallenge` |
| `sys.worker.handshake.authenticate` | `WorkerHandshakeAuthenticate` | `WorkerHandshakeResult` |

`WorkerHandshakePurpose` selects `ISSUE` or `RENEW`; there are no purpose-specific subjects. A deployment MAY use a scheduler queue group for the two request subjects only when all members share the authoritative credential, challenge, replay, and session stores. Trust-handshake traffic MUST NOT be persisted in JetStream or another durable log because result packets carry session tokens.

The older `sys.worker.handshake` and `sys.worker.handshake.renew` JSON exchanges are not CAP trust-handshake subjects. A conforming issuer MUST NOT mint or renew a session from those unsigned payloads.

### Enrollment boundary

Proof-key enrollment happens through an authenticated administrative control-plane path, not through these NATS subjects. The control plane registers an active P-256 public key and `proof_key_id` against an existing worker and derives the agent and tenant bindings from authoritative records. A worker retains the private key and pins the scheduler identity and scheduler signing public keys. An issuer MUST NOT trust a public key, agent ID, tenant ID, allowed topic, or scheduler key supplied by the packet being authenticated.

Rotation activates a new registered proof-key ID before workers switch to it, then revokes the old ID after the rollout. Scheduler signing-key rotation distributes a new pin before `server_key_id` changes. A revoked worker proof key cannot ISSUE or RENEW, and an unpinned scheduler key cannot authenticate a challenge or result.

### Required bindings

For protocol version 1, implementations MUST enforce all of the following before accepting a phase:

- `BusPacket.protocol_version` and the payload `protocol_version` MUST each equal `1`. `Handshake.supported_versions` MUST contain exactly one entry, also `1`. Unknown enum values, duplicates, or any other version are rejected.
- A version-1 trust envelope and its selected trust-handshake message, including nested messages, MUST contain no unknown protobuf fields. Implementations MUST reject unknown fields before transcript verification so parsers from different versions cannot derive different deterministic bytes.
- `request_id`, `trace_id`, and, after challenge creation, `challenge_id` MUST be non-empty. The inner `trace_id` MUST equal the enclosing `BusPacket.trace_id`, and one trace ID MUST be retained across all four phases.
- `client_nonce` and `server_nonce` MUST each contain exactly 32 cryptographically random bytes. They are raw bytes, not hex or base64 text.
- `audience` MUST equal the exact ASCII string `cordum-scheduler`. Implementations MUST compare it byte-for-byte without case folding or trimming.
- `proof_algorithm` MUST be `ECDSA_P256_SHA256`. `proof_key_id` MUST identify the active, non-revoked P-256 key already registered for `worker_id`. A public key supplied by the requester MUST NOT be used.
- The server MUST derive `agent_id` and `tenant_id` from authoritative records linked to `worker_id`. The client cannot select either value. Credential, worker, agent, tenant, and proof-key records MUST agree before a challenge is returned.
- The request and authenticate envelopes MUST have `sender_id == worker_id`. Challenge and result envelopes MUST use the configured scheduler identity. `capability_handshake.component_id` MUST equal `worker_id`, its role MUST be `COMPONENT_ROLE_WORKER`, and its `sdk_version` MUST equal the challenge `sdk_version`.
- Authenticated `capability_handshake.capabilities` entries MUST use keys matching `^[a-z][a-z0-9_.-]{0,63}$` and MUST have value `true`. Builders MUST omit false entries and validators MUST reject them. This true-only form prevents protobuf implementations from disagreeing about whether a default `false` map value is emitted in the signed bytes.
- `created_at`, challenge `issued_at`, challenge `expires_at`, result `issued_at`, and `token_expires_at` MUST be valid protobuf timestamps. The absolute request-envelope clock skew MUST NOT exceed 60 seconds. A challenge lifetime MUST be positive and no greater than 60 seconds; it is expired when current time is equal to or later than `expires_at`. Deployments MAY enforce lower limits.
- Every field in the returned challenge MUST exactly match the signed request or an authoritative server-derived value. `WorkerHandshakeAuthenticate.challenge` MUST be byte-for-byte equivalent under deterministic protobuf encoding to the issued challenge.

The enclosing `BusPacket.auth_token` has a single meaning in this exchange:

| Packet | `auth_token` requirement |
|---|---|
| Challenge request, challenge, ISSUE authenticate | MUST be empty |
| RENEW authenticate | MUST contain the current active session token |
| Accepted result | MUST contain the newly issued session token |
| Rejected result | MUST be empty |

Because `auth_token` is part of the signed envelope transcript, stripping, substituting, or copying a prior token invalidates the phase signature. Session tokens are opaque to CAP clients. An accepted result MUST set `accepted=true`, `rejection_reason=UNSPECIFIED`, and a future `token_expires_at`. A rejected result MUST set `accepted=false`, a non-zero rejection reason, no token, and no token expiry.

ISSUE and RENEW have different authority:

- ISSUE proves the registered worker key with an empty prior token and may mint the first short-lived session only after the challenge is atomically consumed.
- RENEW proves the same bindings and MUST present the current active token in the signed authenticate envelope. Success returns a newly issued token and supersedes the prior session in the authoritative store.
- RENEW MUST NOT fall back to tokenless ISSUE. Expired, revoked, superseded, malformed, wrong-audience, wrong-worker, wrong-agent, wrong-tenant, or wrong-key sessions fail as `SESSION_INVALID`.
- Revocation and expiry are server-authoritative. A cached client token cannot extend its own lifetime, and challenge/session-store unavailability cannot be treated as acceptance.
- An implementation claiming `enforce` MUST apply authenticated admission to
  worker reports, capability/readiness updates, session issuance/renewal, and
  governed `JobRequest` submissions. Securing only worker output while
  admitting tokenless job producers is not enforce mode.

### Signing transcript

Every trust-handshake packet MUST carry a `BusPacket.signature`. The signature key is the registered worker proof key for challenge requests and authenticate packets, and the configured pinned scheduler key identified by `server_key_id` for challenge and result packets.

The phase-specific ASCII domain strings are:

| Payload | Domain string |
|---|---|
| `WorkerHandshakeChallengeRequest` | `CAP-WORKER-HANDSHAKE-CHALLENGE-REQUEST-V1` |
| `WorkerHandshakeChallenge` | `CAP-WORKER-HANDSHAKE-CHALLENGE-V1` |
| `WorkerHandshakeAuthenticate` | `CAP-WORKER-HANDSHAKE-AUTHENTICATE-V1` |
| `WorkerHandshakeResult` | `CAP-WORKER-HANDSHAKE-RESULT-V1` |

For a packet `P`, construct and sign the transcript as follows:

1. Clone `P` and clear only `BusPacket.signature`; retain every other field, including `auth_token`.
2. Serialize the clone using deterministic protobuf encoding. Known fields MUST be emitted in ascending field-number order; therefore `auth_token` (tag 18) precedes trust-handshake payload tags 19-22. Map entries MUST be ordered by key. Call these bytes `U`.
3. Construct `T = ASCII(domain) || 0x00 || U`. The domain contains no NUL and no trailing newline.
4. Compute `D = SHA-256(T)`.
5. Sign `D` with ECDSA over NIST P-256. Store the signature as strict ASN.1 DER in `BusPacket.signature`; IEEE P1363 `r || s` encoding is not accepted.

Verifiers MUST select the domain from the concrete oneof payload, reject an unexpected or absent payload, recompute `D`, and verify the signature before using any signed field. The transcript and digest are deterministic; ECDSA signature bytes need not be identical across conforming implementations. Trust-handshake signatures MUST NOT use the undomained generic BusPacket signing transcript.

### State and replay requirements

1. Before returning a challenge, the scheduler validates the envelope, resolves authoritative identity and the registered proof key, and verifies the signed challenge request.
2. The scheduler creates a cryptographically random `challenge_id` and server nonce, then atomically stores the complete challenge binding with an expiry in a shared fail-closed store. Repeated request IDs or client nonces MUST NOT create a second live challenge.
3. The worker verifies the challenge signature using its pinned scheduler key, then checks every request echo, authoritative binding, audience, time, and nonce before signing authenticate.
4. The scheduler validates authenticate structure and identity, verifies the worker signature, and, for RENEW, verifies the active token and all worker/agent/tenant/audience/key bindings.
5. Only after successful signature and token verification, the scheduler atomically compares and consumes the stored challenge. Consumption MUST occur before token minting. Concurrent or repeated authenticate packets therefore produce at most one accepted result.
6. The worker verifies the result signature and all challenge/result correlations before installing the new token. A malformed, unsigned, mismatched, expired, or rejected result MUST NOT change local token state.

Challenge, replay, credential, or session-store unavailability MUST fail closed. In WARN a worker runtime MAY remain locally available without a session, but tokenless registry input is telemetry-only and cannot refresh dispatch authority. No migration mode may mint a token without the authenticated exchange.

### Runtime mode semantics

| Mode | Trust exchange | Behavior after failure |
|---|---|---|
| `off` | disabled | legacy capability/heartbeat behavior only; no proof-bound session |
| `warn` | attempted with the full strict contract | worker runtime may remain available for migration; tokenless registry input is telemetry-only and cannot refresh dispatch authority |
| `enforce` | required | startup/reconnect/renewal failure blocks or stops session-required admission |

WARN does not weaken signature, transcript, unknown-field, identity, audience, freshness, challenge, or result validation. It changes only worker-local admission behavior after an operational failure. Tokenless registry traffic may be observed as unauthenticated telemetry but MUST NOT refresh liveness, readiness, pool/capacity, or dispatch authority. WARN MUST NOT convert a malformed or unsigned response into a session or let worker-advertised topics expand server-authoritative allowed topics. Mode `off` SHOULD reject dormant proof material or trust tuning rather than silently ignoring security configuration.

### Rejections

`WorkerHandshakeRejectionReason` is intentionally coarse. `AUTHENTICATION_FAILED` covers unknown, missing, revoked, or mismatched identities and keys as well as bad proof, so the wire response does not become an identity oracle. `SESSION_INVALID` likewise covers expired, revoked, superseded, mismatched, or malformed prior sessions. Operators SHOULD record a more detailed internal audit reason, but logs, metrics, traces, and rejection payloads MUST NOT contain proof material, session tokens, private keys, or raw credentials.

## Legacy Capability Broadcast Subject

Standalone capability Handshake messages are published to `sys.handshake`. Schedulers and controllers SHOULD subscribe to this subject. Queue groups SHOULD NOT be used so every registry observer receives the broadcast. This subject is never a substitute for the two core-NATS authenticated request/reply subjects.
