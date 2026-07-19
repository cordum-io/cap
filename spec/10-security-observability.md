# Security and Observability

Security and visibility are mandatory for production deployments of CAP. See
[11 Security Best Practices](11-security-best-practices.md) for the broader
deployment checklist and [14 Capability Negotiation](14-capability-negotiation.md)
for the normative worker trust transcript.

## Authentication Layers

CAP uses distinct, cumulative controls:

1. The transport authenticates and authorizes a bus connection.
2. A registered P-256 proof key authenticates the worker trust exchange.
3. A short-lived `BusPacket.auth_token` binds subsequent packets to the issued
   worker session.
4. Packet signatures protect integrity and sender binding.

None of these controls makes a caller-provided `sender_id`, `agent_name`,
capability, ready topic, tenant, agent ID, token, or public key authoritative.

### Legacy heartbeat credential versus bound session

`Heartbeat.auth_token` is the legacy worker-attestation credential. It is a
bearer value provisioned out of band and checked against the credential store.
It does not prove possession of a private key, negotiate capabilities, or bind
later packets to one authenticated session.

`BusPacket.auth_token` is different. It is an opaque, short-lived session token
issued only after the signed challenge/authenticate exchange. It is bound by
server authority to the worker, agent, tenant, exact audience
`cordum-scheduler`, active proof key, and session lifecycle. Runtime code, not
an application payload, attaches it before signing outbound packets. A gateway
or scheduler MUST NOT copy an untrusted token into this field.

Legacy heartbeat credentials MAY exist during migration, but MUST NOT be
reported as proof-bound sessions. WARN may observe a tokenless capability or
heartbeat as explicitly unauthenticated telemetry, but it MUST NOT refresh
authenticated liveness, readiness, pool/capacity, or dispatch authority and
MUST NOT mint a session. A presented invalid credential is hard-rejected; it is
not equivalent to legacy omission.

## Proof-key Enrollment

Enrollment is an authenticated control-plane operation, not a CAP bus message:

1. Generate the worker's P-256 proof private key in the worker's protected key
   store. Do not send the private key to the scheduler.
2. Through an authenticated administrative path, register the public key and a
   stable `proof_key_id` against an existing worker record.
3. The control plane derives and persists the worker's agent and tenant binding
   from authoritative records. The worker cannot self-select or change them in
   a trust packet.
4. Provision the worker with its expected worker/agent/tenant identities, exact
   audience, proof key ID/private key, expected scheduler identity, and pinned
   scheduler public keys.

A packet-supplied public key MUST never be used to verify the packet that
supplied it. Unknown, disabled, revoked, wrong-curve, or identity-mismatched keys
fail as `AUTHENTICATION_FAILED` without revealing which record existed.

## Rotation, Renewal, and Revocation

- Rotate worker proof keys by registering a new active key ID, deploying that
  ID/private key to the worker, observing successful authenticated sessions,
  and then revoking the old record. A revoked proof key cannot ISSUE or RENEW.
- Rotate scheduler signing keys by distributing the new public key pin before
  the scheduler begins signing with its new `server_key_id`. Remove the old pin
  only after the overlap window ends.
- ISSUE carries no prior token. RENEW MUST sign and present the current live
  token; it MUST NOT fall back to tokenless ISSUE.
- A successful renewal installs a new session and supersedes the prior session
  according to the authoritative session store. Expired, revoked, superseded,
  audience-mismatched, or identity-mismatched tokens are invalid.
- Credential, proof-key, challenge/replay, revocation, or session-store
  unavailability fails closed. WARN may keep an already verified old token only
  until its original expiry; it cannot extend it locally.

Revocation is a server-side state transition. Deleting a local token or key is
necessary for worker cleanup but is not a substitute for revoking the
authoritative credential, proof-key, or session record.

## Migration Modes

| Mode | Authenticated exchange | Admission after trust failure | Authority |
|---|---|---|---|
| `off` | disabled | legacy behavior only | no proof-bound session |
| `warn` | attempted and strictly verified | worker runtime may remain available for migration; tokenless registry input is telemetry-only | never refresh authority from tokenless input |
| `enforce` | required before admission | blocked; expired/failed renewal stops admission | only a live verified session |

Mode selection MUST be explicit once trust configuration or tuning is present.
`off` MUST reject dormant proof keys, pins, or trust retry/timeout settings so a
configuration typo cannot silently disable authentication. WARN changes only
admission policy after an operational failure; it does not relax packet shape,
unknown-field, signature, identity, audience, freshness, challenge, result, or
token verification.

## Data Protection and Safety

- Keep content behind access-controlled pointers; do not place secrets in
  `context_ptr`, `result_ptr`, labels, `env`, or capability maps.
- Encrypt state at rest and in transit.
- Apply the Safety Kernel before dispatch and preserve the recorded decision.
- Treat private keys, credentials, session tokens, signatures, nonces, complete
  trust packets, and raw rejection inputs as secret security material.

## Safe Observability

Operators SHOULD measure handshake attempts, accepted/rejected/error outcomes,
phase, mode, coarse rejection reason, latency, renewal timing, and whether a
live authenticated session exists. Readiness views MUST distinguish:

- connected but legacy/untrusted;
- authenticated but not ready for a topic;
- authenticated and authorized for a ready topic;
- expired, revoked, superseded, or renewal-failed.

Logs and traces MAY include an already-authenticated worker ID, trace ID, SDK
version, and configured scheduler ID when operationally necessary. Values from
an unauthenticated packet must first be bounded and sanitized or replaced with
a one-way correlation value.

Logs, metrics, traces, alerts, errors, and reports MUST NOT contain raw
`Heartbeat.auth_token`, `BusPacket.auth_token`, credentials, private keys,
public-key material, packet signatures, nonces, complete trust packets, or
attacker-controlled exception text. Trust-handshake metrics MUST use bounded
enums rather than worker IDs, key IDs, tokens, subjects, or rejection messages
as labels.

External responses use only `WorkerHandshakeRejectionReason`. More detailed
internal audit reasons may be retained in access-controlled storage, but they
must remain bounded and secret-free and MUST NOT turn the endpoint into an
identity, key, or session oracle.

## Compliance and Retention

- Configure TTLs for challenge, replay, session, context, result, and audit
  records. Expiry MUST be enforced by logic, not only eventual storage cleanup.
- Retain security decisions and lifecycle transitions for a policy-defined
  window without retaining bearer tokens or proof material.
- Audit proof-key enrollment, activation, rotation, and revocation; session
  issue, renewal, supersession, expiry, and revocation; and migration-mode
  changes.
