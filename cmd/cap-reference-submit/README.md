# cap-reference-submit

`cap-reference-submit` submits one job to the Apache-2.0 CAP reference
scheduler (`cmd/cap-reference-scheduler`) and waits for its result. It needs
no Cordum module or service, and it sends no telemetry.

```
go build ./cmd/cap-reference-submit
cap-reference-submit -nats-url tls://nats.example:4222 -tls-ca ca.pem -tls-cert client.pem -tls-key client.key \
  -sender-id client-1 -tenant-id tenant-a -signing-key-id c1 -signing-key-file client-1-sign.key \
  -scheduler-id refsched-1 -scheduler-key-id s1 -scheduler-key-file refsched-1-signing.pub \
  -topic job.echo -capability echo -kv-bucket refsched-inputs -input question.json
```

The client stores the input in the nats-kv bucket and signs one CAP-PRODUCTION
`JobRequest` whose `context_ref` names it. It publishes the request on
`sys.job.submit` and waits on `refsched.job.terminal`, taking the first
verified terminal for its own job. It then prints one JSON summary on stdout.

## Flags

| Flag | Meaning |
|------|---------|
| `-nats-url` | One NATS endpoint, `tls://host:port`; `nats://` only with `-dev-plaintext-loopback`. |
| `-tls-ca`, `-tls-cert`, `-tls-key` | PEM CA bundle, client certificate and its private key. Set all three or none. |
| `-dev-plaintext-loopback` | Allow plaintext NATS to a loopback host, for development only. |
| `-sender-id`, `-tenant-id` | This client, as one `Clients` entry of the scheduler's configuration. |
| `-signing-key-id`, `-signing-key-file` | This client's production signing key (P-256 private PEM) and the id that entry's `SigningKeyFiles` gives it. |
| `-scheduler-id`, `-scheduler-key-id`, `-scheduler-key-file` | The scheduler's `SchedulerID`, its `SigningKeyID`, and the public half of its `SigningKeyFile` (P-256 PEM). |
| `-topic`, `-capability` | The job's topic and `meta.capability`. The scheduler's allowlist must hold the tenant, topic and capability. |
| `-job-id` | The job id. Default: a random id. |
| `-input`, `-media-type` | The file holding the job input, and its media type (default `application/json`). |
| `-kv-bucket` | The JetStream KV bucket of the nats-kv resource backend. |
| `-deadline` | How long to wait for the terminal result (default `30s`). |

Flags with no default, other than the TLS flags, are required. Their values
and those of `-job-id` and `-media-type` must not have surrounding spaces.

## Trust files

- `-signing-key-file` is this client's private key. Its public half goes into
  the scheduler's `Clients[].SigningKeyFiles`, under the `-signing-key-id`.
- `-scheduler-key-file` is the public half of the scheduler's
  `SigningKeyFile`. A terminal counts only when it verifies against this key,
  names `-scheduler-id` as its sender and `-tenant-id` as its tenant, and
  carries this job's id; every other message on the subject is skipped.

Private key files (`-signing-key-file`, `-tls-key`) are refused when they are
a symlink or, on POSIX, readable or writable by group or others. Use mode
`0600`.

## Input and result

The input goes into the operator-provisioned bucket through
`internal/resourcekv`, as `nats-kv://<bucket>/<tenant>/<sha256>`, and it
expires at the deadline. Workers resolve it under the trusted tenant. The
client never fetches the job's output: the summary names it in `result_uri`,
`result_sha256`, `result_media_type` and `result_size_bytes`.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | The verified terminal is `JOB_STATUS_SUCCEEDED`. |
| `1` | The verified terminal has any other status (`FAILED`, `FAILED_FATAL`, `DENIED`, `TIMEOUT` or `CANCELLED`), or submitting failed: NATS, the bucket or the input. |
| `2` | A usage or configuration error, found before connecting. |
| `3` | No verified terminal for the job before `-deadline`. The scheduler publishes none for a packet it drops at admission, an invalid job, a job id reused for another request, or its lifetime job cap. |

Messages name the failing flag or a fixed error, never key material or the
input.

## Not included

- No cancel: nothing is sent on `refsched.job.cancel`.
- One job per run, and no output download.
- No Cordum integration.
