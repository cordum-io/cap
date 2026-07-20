# Protocol Errors

Protocol errors are wire or trust-boundary failures, not job failures. Job
failures use `JobResult`; safe ordinary protocol telemetry may use
`SystemAlert`; worker trust request/reply uses its coarse rejection enum.

## Ordinary BusPacket Errors

| Condition | ErrorCode | Severity |
|---|---|---|
| Unsupported `protocol_version` | `PROTOCOL_VERSION_MISMATCH` (100) | `ERROR` |
| Packet fails bounded deserialization | `PROTOCOL_MALFORMED_PACKET` (101) | `ERROR` |
| No recognized payload | `PROTOCOL_UNKNOWN_PAYLOAD` (102) | `WARNING` |
| Signature fails verification | `PROTOCOL_SIGNATURE_INVALID` (103) | `CRITICAL` |
| Required signature is missing | `PROTOCOL_SIGNATURE_MISSING` (104) | `CRITICAL` |

The consumer MUST drop the packet before invoking a handler or changing job,
registry, readiness, session, or dispatch state. A component MAY publish a
bounded `SystemAlert` to `sys.alert` only when doing so cannot reflect secrets
or amplify attacker traffic. Alert failure does not make the packet valid.

## Trust-handshake Validation Order

Each worker trust phase MUST fail before state use or mutation. The receiver
applies this order, with no later stage allowed to repair an earlier failure:

1. bound raw bytes and decode exactly one `BusPacket`;
2. require the expected trust oneof phase, exact outer and inner v1 values, no
   recursive unknown fields, valid enums, canonical lengths, and valid times;
3. bind request, trace, sender, worker, agent, tenant, audience, purpose, key
   IDs, nonces, SDK version, and capability fields to expected or authoritative
   values;
4. select only a configured active key and verify the phase-domain signature;
5. check challenge freshness, replay state, proof-key/credential status, and,
   for RENEW, the current session and all of its bindings;
6. atomically consume the challenge;
7. mint or supersede a session and update authenticated registry/readiness
   state.

A challenge-request signature MUST verify before a challenge is created or its
request ID/client nonce is reserved. Authenticate verification and live-session
checks MUST succeed before challenge consumption; challenge consumption MUST
win atomically before token minting. No failure path may call application
handlers or partially update state.

## Worker Trust Rejections

`WorkerHandshakeRejectionReason` is deliberately coarse:

| Reason | Safe meaning |
|---|---|
| `INVALID_REQUEST` | malformed or contradictory request |
| `AUTHENTICATION_FAILED` | identity, credential, key, or proof did not authenticate |
| `REPLAY_DETECTED` | one-time request/challenge state was already used |
| `CLOCK_SKEW` | request time is outside the accepted window |
| `CHALLENGE_EXPIRED` | challenge is no longer live |
| `SESSION_REQUIRED` | RENEW omitted its current token |
| `SESSION_INVALID` | prior session is unusable for any reason |
| `UNSUPPORTED_VERSION` | exact v1 contract was not met |
| `INTERNAL_ERROR` | fail-closed authority or internal operation failed |

`AUTHENTICATION_FAILED` MUST cover missing, unknown, disabled, revoked, and
mismatched worker/credential/proof-key records plus invalid proof. Likewise,
`SESSION_INVALID` MUST cover malformed, expired, revoked, superseded,
wrong-audience, and identity/key-mismatched sessions. A caller must not be able
to enumerate which worker, key, or session exists by comparing responses,
latency classes, logs, or retry behavior.

A rejected `WorkerHandshakeResult` MUST set `accepted=false`, a non-zero coarse
reason, an empty `BusPacket.auth_token`, and no token expiry. When a request is
too malformed or unauthenticated to establish a safe reply correlation, the
receiver SHOULD drop it without a reply. A timeout or absent reply is not
acceptance.

## Safe Diagnostics

For ordinary alerts, populate only bounded values:

- `severity` and `error_code_enum` from the fixed enums;
- the emitter's own `source_component`;
- a previously validated `trace_id`, if one exists;
- a generic message and an allowlisted, bounded `details` map.

Do not copy an untrusted `sender_id`, subject, payload field, parse error, or
exception text directly into an alert. A trust rejection normally should be
counted locally rather than echoed to `sys.alert`.

Logs, metrics, traces, alerts, wire rejections, and crash reports MUST NOT
contain credentials, `Heartbeat.auth_token`, `BusPacket.auth_token`, proof
private/public key material, signatures, nonces, raw packets, or complete
challenge/result messages. Trust-rejection metric labels MUST be bounded enums
such as phase, mode, outcome, and coarse reason; never tokens, key IDs,
identities, subjects, or free-form messages.

Components MUST remain available after rejecting protocol traffic. Repeated
invalid traffic SHOULD be rate-limited at an authenticated transport boundary;
it MUST NOT trigger unbounded alerts, allocations, retries, or logs.
