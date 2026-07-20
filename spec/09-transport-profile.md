# Transport Profile

CAP is transport-agnostic, but assumes a pub/sub fabric with subjects and
competing-consumer groups. NATS is the reference transport profile.

## Subject/Topic Conventions

| Subject | Payload | Direction | Delivery |
|---|---|---|---|
| `sys.job.submit` | `JobRequest` | Gateway -> Scheduler | durable when required |
| `sys.job.result` | `JobResult` | Worker -> Scheduler | durable when required |
| `sys.heartbeat` | `Heartbeat` | Worker -> Schedulers | broadcast |
| `sys.alert` | `SystemAlert` | Any -> Controllers | broadcast |
| `sys.job.progress` | `JobProgress` | Worker -> Scheduler | profile-specific |
| `sys.job.cancel` | `JobCancel` | Scheduler -> Worker | profile-specific |
| `sys.job.dlq` | `BusPacket` | Scheduler -> DLQ | durable |
| `sys.workflow.event` | `BusPacket` | Orchestrator -> Scheduler | profile-specific |
| `sys.handshake` | `Handshake` | Component -> Schedulers/Controllers | broadcast, legacy capability only |
| `sys.worker.handshake.challenge` | `WorkerHandshakeChallengeRequest` | Worker -> Scheduler | core request/reply only |
| `sys.worker.handshake.authenticate` | `WorkerHandshakeAuthenticate` | Worker -> Scheduler | core request/reply only |
| `job.<pool>` | `JobRequest` | Scheduler -> Workers | queue group |

All `sys.*` subjects are protocol-defined. The `job.<pool>` pattern is
application-defined.

## NATS Profile

- Use queue groups for `job.<pool>` so workers share load.
- Do not use queue groups for `sys.heartbeat` or `sys.handshake`; each registry
  replica that depends on those broadcasts must observe them.
- Use JetStream where job durability is required and assume at-least-once
  delivery. Consumers MUST fence retries and suppress duplicate side effects.
- Prefer bounded packets and pointers to large content.

### Authenticated worker trust

The worker trust exchange uses **core NATS request/reply**, not publish-only
advisories:

- Requests go only to `sys.worker.handshake.challenge` or
  `sys.worker.handshake.authenticate`.
- Replies go only to the reply inbox supplied by the NATS request. A scheduler
  MUST NOT construct a reply subject from a worker, request, challenge, tenant,
  or agent identifier.
- `WorkerHandshakePurpose` inside the request selects ISSUE or RENEW. There are
  no alternate renewal subjects and no legacy JSON fallback subjects.
- Trust requests and replies MUST NOT be persisted in JetStream, mirrored,
  replayed, or forwarded to a durable audit stream. Accepted results contain a
  session token in `BusPacket.auth_token`.
- Request packet size, reply packet size, request deadline, and retry count MUST
  be bounded. A timeout is failure, never implicit acceptance.

A scheduler queue group MAY consume the two trust request subjects only when
every member shares the same authoritative identity, proof-key, challenge,
replay, revocation, and session state. Per-process challenge or replay maps are
not sufficient: a challenge issued by one replica can be authenticated at
another, and concurrent authentication must still have one winner.

Cordum's NATS profile uses shared Redis as that authority. Challenge creation,
single-use consumption, session mint/supersession/revocation, credential
status, proof-key status, and replay fences MUST use the shared Redis records
and atomic operations. A different CAP implementation may use an equivalent
shared fail-closed transactional store, but MUST NOT treat NATS delivery or
replica-local memory as authoritative. Store unavailability rejects the
exchange.

## Delivery and Idempotency

- At-least-once delivery MUST be assumed for durable job subjects.
- Ordering is not guaranteed across subjects or partitions.
- Workers and schedulers MUST key retry/duplicate fences to the canonical job
  and dispatch identity, not only a result pointer.
- `meta.idempotency_key`, when present, SHOULD short-circuit duplicate work.
- A trust challenge is single-use. The scheduler MUST validate the authenticate
  packet, atomically consume the live challenge, and only then mint a token.
  Redelivery therefore cannot mint a second session.

## Kafka Profile

Kafka deployments map subjects to prefixed topics (for example,
`cap.sys.job.submit`) and consumer groups to pools. They still require a
separate request/reply mechanism and shared atomic authority for the worker
trust exchange; replaying trust messages from a Kafka log is forbidden.

## TLS, Authentication, and ACLs

- Production bus connections MUST use TLS and authenticated NATS identities
  (for example mTLS, NKeys/JWT, or scoped user credentials). Transport identity
  is required defense in depth; it does not replace the signed CAP proof.
- ACLs MUST be least privilege. Workers may publish the two trust request
  subjects and receive only their broker-granted request replies. Schedulers may
  subscribe to those two subjects and publish only to the broker-provided reply
  inbox. Neither role needs wildcard publish/subscribe access to all `_INBOX`
  traffic.
- Workers subscribe only to allowed pool/cancel subjects and publish only their
  result/progress/heartbeat/capability subjects. Gateways, schedulers, and
  controllers receive only their role-specific subjects.
- A principal-to-component binding MUST be checked against `sender_id`; a
  caller-selected `sender_id` is not authenticated identity.
- Broker credentials, private proof keys, session tokens, raw trust packets,
  and signatures MUST NOT be recorded in NATS monitoring payloads, logs, or
  durable streams.
