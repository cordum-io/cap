# Capability Negotiation

CAP components advertise their role, supported protocol versions, and feature capabilities via a `Handshake` message published on connect. This enables schedulers and controllers to build a live registry of the topology and tailor behavior to each component's capabilities.

## Handshake Flow

1. Component connects to the message bus.
2. Component publishes `BusPacket{Handshake}` to `sys.handshake`.
3. Schedulers and controllers consume `sys.handshake` and record the component's role, supported versions, and capabilities in their component registry.
4. Schedulers MAY use the registry to route jobs only to workers that advertise required capabilities (e.g., `compensation`, `progress`).

Components SHOULD publish a Handshake immediately after establishing a bus connection. The Handshake is informational; no explicit acknowledgment is defined. Schedulers that need confirmation of capability support SHOULD use the registry rather than waiting for a response.

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

Schedulers MAY use `ready_topics` as an additional dispatch filter before selecting a worker. This lets a worker advertise broad static capabilities while temporarily narrowing the set of routed topics during startup or reconfiguration.

Workers SHOULD publish `ready_topics` in a stable order so registries and tests can compare handshake payloads deterministically. Older workers that omit `ready_topics` remain wire-compatible; schedulers SHOULD treat the field as unknown/unspecified rather than as an error.

## Agent Display Name

`agent_name` (field 7) is an optional human-facing **display label** for the component (e.g., `Claude Code — Billing Bot`), surfaced in registries, dashboards, and audit summaries so operators can attribute activity to a recognizable name rather than an opaque `component_id`. SDKs sanitize and bound it (trim, collapse internal whitespace, drop control characters, cap at 128 characters).

`agent_name` is **not an authentication authority**. It is self-reported and spoofable, so schedulers, registries, and audit pipelines MUST prefer authenticated identity records (worker credential / Agent Identity) over it and MUST NOT use it for authorization or identity resolution. It MUST NOT carry secrets, tokens, or PII. Older components that omit `agent_name` remain wire-compatible; consumers SHOULD treat the field as unknown/unspecified rather than as an error.

## Version Negotiation

Components advertise the wire versions they support via `supported_versions`. Schedulers SHOULD pick the highest version common to both the scheduler and the target component. If no common version exists, the scheduler SHOULD reject jobs to that component with `ERROR_CODE_PROTOCOL_VERSION_MISMATCH`.

Currently, the only defined wire version is `1`. Components SHOULD include `1` in `supported_versions`.

## Reconnection and Liveness

- Components SHOULD re-publish their Handshake on every reconnect to the bus.
- Schedulers MAY expire registry entries for components that have not sent a Heartbeat or Handshake within a configured timeout.
- Components that never send a Handshake are assumed to support CORE-level capabilities only (backward compatibility with pre-handshake deployments). Schedulers SHOULD NOT require Handshake for basic job dispatch.
- If `ready_topics` is absent, schedulers SHOULD fall back to legacy behavior and rely on the existing routing/heartbeat signals rather than rejecting the worker outright.

## Security

- A legacy `Handshake` is a capability advertisement, not an authentication exchange and not authority to mint a session.
- Handshake packets SHOULD be signed like any other BusPacket when signatures are enabled.
- Schedulers SHOULD verify Handshake signatures before trusting the advertised capabilities.
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

Challenge, replay, credential, or session-store unavailability MUST fail closed. WARN or migration modes MAY admit a legacy worker without a session according to local policy, but MUST NOT mint a token without the authenticated exchange.

### Rejections

`WorkerHandshakeRejectionReason` is intentionally coarse. `AUTHENTICATION_FAILED` covers unknown, missing, revoked, or mismatched identities and keys as well as bad proof, so the wire response does not become an identity oracle. `SESSION_INVALID` likewise covers expired, revoked, superseded, mismatched, or malformed prior sessions. Operators SHOULD record a more detailed internal audit reason, but logs, metrics, traces, and rejection payloads MUST NOT contain proof material, session tokens, private keys, or raw credentials.

## Subject

Handshake messages are published to `sys.handshake`. Schedulers and controllers SHOULD subscribe to this subject. Queue groups SHOULD NOT be used for `sys.handshake` so that all schedulers receive every Handshake.
