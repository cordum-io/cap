# Authenticated worker trust handshake (Go runtime)

`Agent.Start()` accepts `HandshakeMode` values `off`, `warn`, or `enforce`.
An unset mode preserves legacy `off`; an unknown non-empty code or environment
value fails startup. In `warn` and `enforce`, it completes the signed protobuf
challenge/authenticate exchange before subscribing to job subjects.

## Configuration

`warn` and `enforce` require a complete `capsdk.WorkerTrustConfig` in
`Agent.WorkerTrust`:

- `WorkerID`, `ExpectedAgentID`, and `TenantID`
- exact audience `capsdk.WorkerHandshakeAudience`
- registered `ProofKeyID` and its P-256 `ProofPrivateKey`
- `ExpectedSchedulerID` and one or more pinned P-256
  `SchedulerPublicKeys`
- `SDKVersion`

The trust proof key is intentionally separate from `Agent.PrivateKey`, which
signs ordinary CAP packets. `SenderID`, when set, must equal
`WorkerTrust.WorkerID`; otherwise the runtime derives it from the trust
configuration. Partial configuration fails before the runtime opens NATS or
Redis. `off` accepts no trust configuration or trust retry/timeout tuning, so
an operator cannot silently disable a configured trust exchange. Off mode also
requires ordinary packet signing/verification keys unless the caller sets
`AllowUnsigned: true` as an explicit legacy opt-in.

`CORDUM_SDK_HANDSHAKE` may provide the explicit mode when `HandshakeMode` is
empty. `Tenant` and `SDKVersion` remain legacy generic-advertisement settings
only in `off`; they are not authentication authority.

## Enrollment and key rotation

Generate the P-256 proof private key in the worker's protected key store and
register only its public key and `ProofKeyID` through the control plane's
authenticated enrollment path. Enrollment also binds the worker to the
expected agent and tenant. CAP trust packets cannot self-enroll a key or choose
those identities; the runtime never verifies a response with packet-supplied
key material.

To rotate a worker proof key, register a new active ID, deploy its private key
and ID, confirm successful sessions, then revoke the old ID. To rotate the
scheduler signing key, add the new public-key pin before the scheduler changes
`server_key_id`; remove the old pin after the overlap window. Revoking a proof
key or session is a server-side control-plane operation, not merely deletion of
the worker's local copy.

## Flow and renewal

The runtime uses core NATS request/reply on
`sys.worker.handshake.challenge` and `sys.worker.handshake.authenticate`.
Every phase carries one stable, nonempty trace ID and protocol version 1. The
same capability `Handshake` is embedded in authenticate and cloned for the
later `sys.handshake` broadcast. Only a verified signed result is installed,
and the resulting token is attached before validation and signing of outbound
packets.

A renewal sets purpose `RENEW` and signs the current unexpired token into the
authenticate envelope. Renewal never falls back to a tokenless `ISSUE`, which
would bypass session revocation or supersession. On an operational renewal
failure, `warn` may retain the old token only until its existing expiry.
Malformed, unpinned, tampered, mismatched, or rejected renewal responses clear
the session and admissions in both trust modes; `enforce` also clears on
operational failure.
Expired tokens are never returned by `SessionToken` or attached to packets.
The token is opaque and valid only for the exact `cordum-scheduler` audience
and worker/agent/tenant/key bindings verified during the exchange. Successful
renewal installs the new token; the authoritative issuer supersedes the old
session. Expired, revoked, superseded, or binding-mismatched sessions cannot be
recovered locally.

`warn` is an admission migration mode only: an operational trust failure may
allow subscriptions without a token, but it never accepts or installs an
unsigned, malformed, mismatched, or unpinned result and does not make tokenless
registry input dispatch authority. Privileged job results are not published
without a live session in either trust mode; tokenless `warn` output is limited
to capability/heartbeat telemetry. `enforce` blocks startup before subscriptions.

`Close()` cancels and waits for the renewal loop, unsubscribes tracked job
subscriptions, drains a native NATS connection, and closes the blob store.
Startup failures clean only resources opened by the runtime; injected
connections and stores remain caller-owned.

## Public helper boundary

The package-level builders, codec, validators, transcript/signature helpers,
and challenge/result verifiers are public for custom runtime adapters and
cross-SDK compatibility tests. They are client-side composition primitives,
not an enrollment API, scheduler issuer, credential/replay/session store, or
authorization decision. Calling `BuildWorkerHandshakeChallengeRequest`,
`SignTrustHandshake`, or `VerifyWorkerHandshakeResult` alone does not register
a key, revoke/supersede a session, or make a legacy `sys.handshake` broadcast
authenticated.

For ordinary workers, prefer `runtime.Agent` or `worker.ManagedWorker`, which
perform bounded core-NATS request/reply before admission and attach the live
token before packet validation/signing. The standalone capability broadcast
remains compatibility/registry fanout only; it cannot expand the topics allowed
by the control plane.

Never log or report proof/private/public key bytes, session tokens, signatures,
nonces, complete trust packets, or raw rejection errors. Record only bounded
mode/phase/outcome/coarse-reason fields and authenticated identifiers when
needed.
