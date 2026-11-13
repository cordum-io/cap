# A2A bridge example

## Status

`cmd/cap-a2a-bridge` is a draft A2A 1.0 JSON-RPC bridge into CAP jobs. It implements
[the A2A mapping](../../spec/interop/a2a-mapping.md) and changes no CAP wire format. It serves
SendMessage, GetTask and CancelTask plus an agent card at `/.well-known/agent-card.json`. Each
authenticated message becomes one signed CAP job on a single operator-configured topic.
`examples/a2a-bridge/worker` is an example worker that answers those jobs.

## CAP-only run (UNGOVERNED)

```sh
go test -count=1 -run TestA2ABridgeE2E ./internal/a2abridge/
```

This test starts an embedded NATS server, the bridge, the example worker logic
(`examples/a2a-bridge/turns`) and an in-memory store. Between them it runs
`ungovernedTestScheduler`. **That scheduler is UNGOVERNED.** It does no policy evaluation and
has no safety kernel and no approvals. It verifies signatures, relays every job to the worker,
and denies only one hard-coded topic. It exists only in that test. The scenarios:

1. A first message goes WORKING, then INPUT_REQUIRED. A follow-up completes it with exactly
   one artifact.
2. CancelTask during a turn publishes one JobCancel. The task is CANCELED after the cancel grace.
3. The text `needs-auth` ends in AUTH_REQUIRED, which is not terminal. No captured CAP packet,
   turn input or result carries the caller's API key.
4. CancelTask on a task waiting for input is CANCELED at once, without a JobCancel.
5. A job on the denied topic ends in REJECTED.

The test also checks that the worker's heartbeats are valid and signed.

## Running the binaries

TLS is the default for every connection. Without it, both binaries exit 2 and name each
plaintext surface. Two development switches exist, and both log a `development only` warning:

- `CAP_A2A_DEV_INSECURE=1` permits plaintext HTTP, `nats://`, `redis://`, an `http` public URL
  and `NATS_TLS_INSECURE` or `REDIS_TLS_INSECURE`.
- `CAP_A2A_DEV_UNSIGNED=1` accepts unsigned packets. On the bridge, `CAP_A2A_RESULT_KEYS_DIR`
  must then be unset, and jobs are published unsigned unless a signing key is set.

Never set either switch outside local development.

### cap-a2a-bridge

| Variable | Default | Meaning |
|---|---|---|
| `CAP_A2A_LISTEN` | `:8443` | HTTP listen address |
| `CAP_A2A_PUBLIC_URL` | required | JSON-RPC URL in the agent card; `https` |
| `CAP_A2A_TLS_CERT_FILE`, `CAP_A2A_TLS_KEY_FILE` | required | Server certificate and key (PEM) |
| `CAP_NATS_URL` | required | Comma-separated `tls://` URLs, or set `NATS_TLS_CA` |
| `CAP_REDIS_URL` | required | `rediss://` URL, or set `REDIS_TLS_CA` |
| `NATS_TLS_*`, `REDIS_TLS_*` | unset | `CA`, `CERT`, `KEY`, `SERVER_NAME`, as in the CAP SDK |
| `CAP_A2A_TOPIC` | required | The one CAP job topic |
| `CAP_A2A_SENDER_ID` | `cap-a2a-bridge` | `sender_id`; never a reserved Cordum service identity |
| `CAP_A2A_API_KEYS_FILE` | required | API key hashes (below) |
| `CAP_A2A_SIGNING_KEY_FILE` | required | P-256 private key (PKCS#8 or SEC1 PEM) |
| `CAP_A2A_RESULT_KEYS_DIR` | required | Public keys of trusted senders (below) |
| `CAP_A2A_INPUT_TTL` | `24h` | TTL of each `ctx:<job_id>` turn input, 1m to 168h |
| `CAP_A2A_MAX_TASKS` | `10000` | Bound of the in-memory task store |
| `CAP_A2A_CANCEL_GRACE` | `30s` | How long a cancel waits for a result, 1s to 60s |
| `CAP_A2A_MAX_BLOCKING_WAIT` | `60s` | Longest blocking SendMessage, 1s to 300s |
| `CAP_A2A_SHUTDOWN_TIMEOUT` | `10s` | Bound on shutdown, 1s to 60s |

### Example worker

The worker registers on its topic and on `worker.<CAP_WORKER_ID>.jobs`, and heartbeats on
`sys.heartbeat`. Cordum's scheduler dispatches only to a heartbeating worker in an eligible pool
([strategy_least_loaded.go](https://github.com/cordum-io/cordum/blob/ee58be379d3927f3aec0cac13ce5e09cf605a67d/core/controlplane/scheduler/strategy_least_loaded.go#L120-L138)).

| Variable | Default | Meaning |
|---|---|---|
| `CAP_NATS_URL`, `CAP_REDIS_URL` | required | Same TLS rules as the bridge |
| `CAP_A2A_TOPIC` | `job.a2a.demo` | Topic to serve |
| `CAP_WORKER_ID` | `a2a-demo-worker` | `sender_id` and heartbeat `worker_id` |
| `CAP_WORKER_POOL` | required | Heartbeat pool; Cordum must map the topic to it |
| `CAP_WORKER_MAX_PARALLEL` | `4` | Advertised parallelism, 1 to 64 |
| `CAP_WORKER_HEARTBEAT_INTERVAL` | `5s` | 1s to 60s |
| `CAP_WORKER_SIGNING_KEY_FILE` | required | P-256 private key for results and heartbeats |
| `CAP_SCHEDULER_ID` | `cordum-scheduler` | Sender whose jobs the worker accepts |
| `CAP_SCHEDULER_PUBLIC_KEY_FILE` | required | That sender's P-256 public key |

With `CAP_A2A_DEV_UNSIGNED=1`, the worker's signing key is optional and its scheduler key must be
unset.

### Keys

```sh
for k in bridge worker; do
  openssl ecparam -name prime256v1 -genkey -noout | openssl pkcs8 -topk8 -nocrypt > "$k.key"
done
mkdir -p result-keys && openssl pkey -in worker.key -pubout > result-keys/a2a-demo-worker.pem
printf '%s' "$KEY" | sha256sum   # an API key's hash
```

`CAP_A2A_API_KEYS_FILE` holds only hashes:
`[{"sha256":"<64 lowercase hex>","tenant":"acme","principal":"alice"}]`. Callers send the key as
`Authorization: Bearer <key>`.

`CAP_A2A_RESULT_KEYS_DIR` holds one `<sender_id>.pem` PKIX public key per sender whose result,
DLQ or progress packets the bridge accepts. For example, `a2a-demo-worker.pem` and
`cordum-scheduler.pem`. Other files are ignored.

## Governed run

Governed operation runs the bridge and the worker against a Cordum compat-profile deployment,
with its scheduler and safety kernel. The governed demo is a separate Cordum CI workflow that is
not yet available, so there is no link to it.

## Production limitation

Cordum admits a raw `sys.job.submit` from a non-reserved sender only while
`CORDUM_SDK_HANDSHAKE` is `off` or `warn`
([verifyReservedServiceAuthority](https://github.com/cordum-io/cordum/blob/ee58be379d3927f3aec0cac13ce5e09cf605a67d/core/controlplane/scheduler/engine_active_authority.go#L104-L138)).
The bridge therefore works only on Cordum's compat profile. Cordum rejects it under `enforce` and
on CAP-PRODUCTION, and fronting it with the Cordum gateway is not supported yet. See
[bridge placement and admission](../../spec/interop/a2a-mapping.md#8-bridge-placement-and-admission).

Cordum's compat profile also publishes unsigned packets
([Publish](https://github.com/cordum-io/cordum/blob/ee58be379d3927f3aec0cac13ce5e09cf605a67d/core/infra/bus/nats.go#L430),
[emitDLQ](https://github.com/cordum-io/cordum/blob/ee58be379d3927f3aec0cac13ce5e09cf605a67d/core/controlplane/scheduler/engine.go#L3406-L3458)).
Running against it today therefore needs `CAP_A2A_DEV_UNSIGNED=1`.

## Known limits

- The task store is in memory, so a restart loses every task.
- Terminal tasks do not expire. After `CAP_A2A_MAX_TASKS` tasks, new tasks are refused until
  restart.
- One topic per process.
- No streaming, push notifications, ListTasks or extended agent card.
- The binary does not wire `ProductionTrust`.
- Credentials travel only out of band
  ([spec §4.2](../../spec/interop/a2a-mapping.md#42-credentials-travel-only-out-of-band)).
