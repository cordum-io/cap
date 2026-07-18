# Authenticated worker trust handshake (Go runtime)

`Agent.Start()` requires an explicit `HandshakeMode`: `off`, `warn`, or
`enforce`. In `warn` and `enforce`, it completes the signed protobuf
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
an operator cannot silently disable a configured trust exchange.

`CORDUM_SDK_HANDSHAKE` may provide the explicit mode when `HandshakeMode` is
empty. `Tenant` and `SDKVersion` remain legacy generic-advertisement settings
only in `off`; they are not authentication authority.

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
would bypass session revocation or supersession. On renewal failure, `warn`
may retain the old token only until its existing expiry. `enforce` clears it.
Expired tokens are never returned by `SessionToken` or attached to packets.

`warn` is an admission migration mode only: an operational trust failure may
allow subscriptions without a token, but it never accepts or installs an
unsigned, malformed, mismatched, or unpinned result. `enforce` blocks startup
before subscriptions.

`Close()` cancels and waits for the renewal loop, unsubscribes tracked job
subscriptions, drains a native NATS connection, and closes the blob store.
Startup failures clean only resources opened by the runtime; injected
connections and stores remain caller-owned.
