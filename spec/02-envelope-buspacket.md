# Envelope - BusPacket

All CAP traffic is wrapped in a `BusPacket`. The envelope provides tracing,
sender identity, protocol selection, a single payload, and optional packet
authentication context.

## Envelope Fields

- `trace_id`: correlates packets for one request, workflow, or trust exchange.
- `sender_id`: stable identifier for the emitting component. It is a claim until
  bound to authenticated transport and, where required, a CAP trust session.
- `created_at`: UTC time of emission.
- `protocol_version`: CAP wire version. Version `1` is the only defined value.
- `payload`: exactly one of the ordinary payloads at tags 10-17 or the worker
  trust-handshake payloads at tags 19-22.
- `signature`: packet signature. Worker trust packets require it; other packet
  classes follow the signature policy for their profile.
- `auth_token`: opaque trusted runtime session token. Runtime code attaches it
  after an authenticated exchange; callers MUST NOT populate it from
  untrusted input.

## Canonical Proto

The source of truth is
`proto/cordum/agent/v1/buspacket.proto`. The v1 field layout is:

```proto
message BusPacket {
  string trace_id = 1;
  string sender_id = 2;
  google.protobuf.Timestamp created_at = 3;
  int32 protocol_version = 4;

  oneof payload {
    JobRequest job_request = 10;
    JobResult job_result = 11;
    Heartbeat heartbeat = 12;
    SystemAlert alert = 13;
    JobProgress job_progress = 15;
    JobCancel job_cancel = 16;
    Handshake handshake = 17;
    WorkerHandshakeChallengeRequest worker_handshake_challenge_request = 19;
    WorkerHandshakeChallenge worker_handshake_challenge = 20;
    WorkerHandshakeAuthenticate worker_handshake_authenticate = 21;
    WorkerHandshakeResult worker_handshake_result = 22;
  }

  bytes signature = 14;
  string auth_token = 18;
}
```

Field numbers MUST NOT be renumbered or reused. Evolution is append-only.

## Version and Unknown-field Rules

- Ordinary v1 producers MUST set `protocol_version = 1`. A v1 consumer MUST
  reject every other value and MUST NOT execute its payload; a future
  multi-version consumer may accept only versions it explicitly implements.
- Generic protobuf compatibility permits an ordinary v1 consumer to preserve
  or ignore unknown fields. An older consumer sees an unknown oneof variant as
  no selected payload and MUST NOT execute it.
- Worker trust packets are stricter: the envelope version and inner version
  MUST both equal `1`, and `Handshake.supported_versions` MUST be exactly
  `[1]`. Unknown fields anywhere in the trust envelope, selected trust message,
  or nested messages MUST cause rejection before signature verification or
  state access. This rule prevents version-skewed parsers from signing different
  transcripts.
- Trust decoders MUST bound the raw packet before parsing. The SDK contract caps
  a trust packet at 65,536 bytes.

## Signing Rules

Ordinary CAP packet signatures clear `signature` and use the CAP deterministic
unsigned-envelope encoding. For ordinary payload tags 10-17, the payload
precedes `auth_token` tag 18. Map entries are ordered by key. Implementations
SHOULD use an SDK helper rather than re-create this encoding.

Worker trust packets MUST NOT use that undomained generic transcript. They use
the phase domain and algorithm defined in
[14 Capability Negotiation](14-capability-negotiation.md):

1. Clone the complete packet and clear only `signature`.
2. Retain `auth_token`, including the current token on RENEW and the new token
   on an accepted result.
3. Deterministically encode known fields in ascending tag order. Consequently,
   `auth_token` tag 18 precedes trust payload tags 19-22.
4. Sign `ASCII(domain) || 0x00 || unsigned_packet` with ECDSA P-256/SHA-256 and
   encode the signature as strict ASN.1 DER.

Signature verification, identity/audience binding, freshness checks, and
session validation MUST succeed before a trust packet changes challenge,
session, registry, readiness, or dispatch state.

## Subject Recommendations

- Submission: `BusPacket{JobRequest}` on `sys.job.submit`.
- Results: `BusPacket{JobResult}` on `sys.job.result`.
- Heartbeats: `BusPacket{Heartbeat}` on `sys.heartbeat`.
- Alerts: `BusPacket{SystemAlert}` on `sys.alert`.
- Legacy capability advertisement: `BusPacket{Handshake}` on `sys.handshake`.
- Authenticated worker trust: core-NATS request/reply on
  `sys.worker.handshake.challenge` and
  `sys.worker.handshake.authenticate`.
- Work pools: `BusPacket{JobRequest}` on application-defined `job.<pool>`.

See [09 Transport Profile](09-transport-profile.md) for queueing, persistence,
and ACL requirements.

## Secret Handling and Rejection

`auth_token`, proof private keys, raw credentials, packet signatures, nonces,
and complete trust packets are security material. They MUST NOT appear in logs,
metrics labels, traces, alerts, rejection payloads, crash reports, or test
artifacts. A rejected `WorkerHandshakeResult` MUST carry no `auth_token` and no
token expiry.

Malformed or unauthenticated traffic MUST be rejected before handler or state
mutation. Implementations MAY emit a bounded protocol alert for ordinary
traffic, but MUST NOT echo attacker-controlled fields. Trust endpoints return
only the coarse `WorkerHandshakeRejectionReason` when it is safe to reply; an
unparseable or unsafe request may be dropped without a reply. See
[16 Protocol Errors](16-protocol-errors.md).
