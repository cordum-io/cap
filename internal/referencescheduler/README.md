# internal/referencescheduler

A single-process reference scheduler runtime for CAP over NATS. It exists to
exercise CAP-PRODUCTION workers and the framework TCK against a small, readable
control plane. It is an internal package with no API stability promise. It
imports no Cordum module and, like the rest of this repository, is Apache-2.0.

This is reference code for interoperability testing. It makes no claim of
production readiness, endorsement or adoption.

## Single replica only

Run exactly one process per set of subjects. The worker-trust issuer keeps its
single-use challenges, its sessions and its one-winner authentication in process
memory, and the scheduler subscribes to the trust subjects without a queue
group. Queue-group replicas need shared authoritative trust state
(spec/09-transport-profile.md:53-57), which this package does not have. A second
replica on the same subjects is unsupported
(docs/adoption/framework-adapters-design.md section 7, item 7).

## State and transport

- State lives in memory for the lifetime of the process. There is no
  persistence, no restart recovery, no HA and no JetStream: the runtime uses
  core NATS publish/subscribe and request/reply.
- `MaxLiveJobs` bounds the jobs admitted over the process lifetime, not the
  jobs in flight. Finished jobs are not evicted. Once the bound is reached, a
  new job is refused with `ErrRuntimeCapacity` and dropped.

## Subjects

Section 2.2 of
[the design note](../../docs/adoption/framework-adapters-design.md#22-subjects)
fixes the subjects and the message flow.

| subject | payload | direction |
|---|---|---|
| `sys.job.submit` | JobRequest | client → scheduler |
| `sys.job.progress` | JobProgress | worker → scheduler |
| `sys.job.result` | JobResult | worker → scheduler |
| `sys.worker.handshake.challenge`, `sys.worker.handshake.authenticate` | worker-trust handshake, request/reply | worker → scheduler, reply to the worker |
| `worker.<id>.jobs` | JobRequest, the scheduler-signed dispatch | scheduler → one worker |
| `sys.job.cancel` | JobCancel, fenced to the job's dispatch | scheduler → worker |
| `refsched.job.cancel` | JobCancel | client → scheduler |
| `refsched.job.terminal` | JobResult, the scheduler-signed terminal winner | scheduler → clients |

The two `refsched.*` subjects are application subjects that reuse existing
messages. They are not a wire change.

## Admission before any mutation

Every inbound packet passes these steps in order:

1. the size bound (`MaxPacketBytes`) and decode;
2. a trust input built locally: the audience is the actual subject, never a
   packet field; the tenant comes from the worker's live session or from the
   client config, and the verification keys from the operator config;
3. `capsdk.VerifyTrustedProductionPacket`;
4. the replay store;
5. the state core.

A packet that fails steps 1-4 is dropped: `Stats().Dropped` grows by one, and
one payload-free log line names only the subject and the reason. There is no
state change, no publish and no handler call. An identical redelivery of an
admitted packet is dropped the same way, at step 4.

## Worker selection: the tenant-aware topic rule

- A worker can take a job on topic T when the operator config lists it for T
  and it holds a live session of the job's tenant. Selection is round-robin
  over the eligible workers.
- `RuntimeConfig.Resolve` rejects an allow rule (tenant, topic) whose topic
  lists no worker of the same tenant (`ErrConfigTopicWorker`).
- Every scheduler-signed packet (dispatch, terminal and cancel) carries the
  job's identity binding, recorded from its first accepted submit.
- A job with no eligible worker stays SCHEDULED and is retried on the next tick.

## Config fails closed

- TLS (CA, client certificate and key) is required unless the URL host is
  loopback and `DevPlaintextLoopback` is set.
- An empty worker trust store, client set or allowlist is an error.
- Errors are bare `ErrConfig*` sentinels, and no secret value appears in an
  error or a log line.

## Lifecycle

- `NewRuntime(RuntimeOptions{...})`, then `Run(ctx)`. `Run` serves once.
- `Ready()` closes once all six subscriptions are live and flushed. Callbacks
  are bounded by `MaxConcurrency`.
- On context cancel or permanent transport loss, the runtime unsubscribes,
  flushes, closes the connection, then joins every callback and the tick loop.
- `Run` returns nil after cancel, and `ErrRuntimeTransport` after a dial failure
  or a permanent connection loss.
- `Stats()` reports the `Dropped`, `CoreCalls` and `Published` counters.

## Cancel and the terminal winner

- A client cancel on a live job records CANCELLED and publishes it on
  `refsched.job.terminal`. When the job has been dispatched, it also sends one
  `sys.job.cancel` fenced to the job's most recent dispatch.
- A cancel on a finished job republishes the recorded winner and sends no
  `sys.job.cancel`.
- A result after the winner is dropped.

## Caveats

1. The sdk/go `ManagedWorker` in production mode requires `request.topic` to
   equal the delivery subject (sdk/go/worker/managed_production.go:222), so it
   cannot consume this scheduler's direct-subject dispatch. The scheduler
   targets cap-sdk-python's `worker.<id>.jobs` subscription.
2. `MaxChallenges` is global, so one configured worker can hold every challenge
   slot and delay other workers' handshakes for up to `ChallengeTTL`.

## Tests

`go test ./internal/referencescheduler/` runs the E2E flows against a real
embedded nats-server, over plaintext loopback and over TLS with a per-test CA.
The transport tests fail rather than skip. `go test -race` needs CGO
(`CGO_ENABLED=1` and a C compiler).
