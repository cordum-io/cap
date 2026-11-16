# cap-reference-scheduler

`cap-reference-scheduler` runs the Apache-2.0 CAP reference scheduler
(`internal/referencescheduler`) as a standalone control plane over NATS. It
needs no Cordum module or service, and it sends no telemetry.

```
go build ./cmd/cap-reference-scheduler
cap-reference-scheduler -config scheduler.json
```

## Flags

| Flag | Config field | Meaning |
|------|--------------|---------|
| `-config` | | JSON configuration file (below). Flags that are set override its values. |
| `-nats-url` | `NATSURL` | One NATS endpoint, `tls://host:port`; `nats://` only with `-dev-plaintext-loopback`. |
| `-tls-ca`, `-tls-cert`, `-tls-key` | `TLSCAFile`, `TLSCertFile`, `TLSKeyFile` | PEM CA bundle, client certificate and its private key. Set all three or none. |
| `-dev-plaintext-loopback` | `DevPlaintextLoopback` | Allow plaintext NATS to a loopback host, for development only. |
| `-scheduler-id` | `SchedulerID` | The sender of dispatches and terminal results. |
| `-signing-key-id`, `-signing-key-file` | `SigningKeyID`, `SigningKeyFile` | The production signing key (P-256 private PEM). |
| `-trust-key-id`, `-trust-key-file` | `TrustKeyID`, `TrustKeyFile` | The worker-trust handshake signing key (P-256 private PEM). |

## Config file

The file is one JSON object whose keys are the field names of
`referencescheduler.RuntimeConfig`. Unknown keys are refused, and durations
are strings such as `"30s"`. The workers, clients, allowlist and topic routing
exist only in the file:

```json
{
  "NATSURL": "tls://nats.example:4222",
  "TLSCAFile": "ca.pem", "TLSCertFile": "client.pem", "TLSKeyFile": "client.key",
  "SchedulerID": "refsched-1",
  "SigningKeyID": "s1", "SigningKeyFile": "signing.key",
  "TrustKeyID": "t1", "TrustKeyFile": "trust.key",
  "Workers": [{"WorkerID": "worker-1", "TenantID": "tenant-a", "AgentID": "agent-1",
    "ProofKeyID": "p1", "ProofKeyFile": "worker-1-proof.pub",
    "SigningKeyFiles": {"w1": "worker-1-sign.pub"}}],
  "Clients": [{"SenderID": "client-1", "TenantID": "tenant-a",
    "SigningKeyFiles": {"c1": "client-1-sign.pub"}}],
  "Allow": [{"TenantID": "tenant-a", "Topic": "job.echo", "Capability": "echo"}],
  "TopicWorkers": {"job.echo": ["worker-1"]},
  "Limits": {"MaxInputBytes": 1048576, "MaxOutputBytes": 1048576, "MaxArtifacts": 8,
    "MaxAttempts": 1, "DispatchTimeout": "30s", "RunTimeout": "5m", "MaxRunTimeout": "30m"},
  "MaxPacketBytes": 65536, "MaxConcurrency": 16, "MaxLiveJobs": 10000, "MaxChallenges": 64,
  "ChallengeTTL": "30s", "SessionTTL": "10m"
}
```

## Trust files

- `SigningKeyFile` signs dispatches and the terminal results published on
  `refsched.job.terminal`. Give its public half to clients, which verify
  terminals with it.
- `TrustKeyFile` signs worker handshake replies. Workers trust its public half.
- `Workers[].ProofKeyFile` and `SigningKeyFiles` are each worker's public keys;
  `Clients[].SigningKeyFiles` are the public keys of the clients allowed to
  submit and cancel jobs for their tenant.

Private key files (`SigningKeyFile`, `TrustKeyFile`, `TLSKeyFile`) are refused
when they are a symlink or, on POSIX, readable or writable by group or others.
Use mode `0600`.

## TLS

NATS needs TLS with a client certificate (`tls://` and all three TLS files).
The only exception is a loopback host with `-dev-plaintext-loopback`, for
development.

## Signals and exit codes

SIGINT and SIGTERM (`os.Interrupt` on Windows) drain the runtime, which then
exits `0`. A NATS transport failure exits `1`, and a usage or configuration
error exits `2`. Messages name the failing flag or a fixed error, never key
material or packet payloads.

## Not included

- No persistence: job state lives in memory, and a restart forgets it.
  `MaxLiveJobs` caps the jobs one process ever accepts.
- One NATS endpoint; no server list and no JetStream.
- No adapters or conformance harness, and no Cordum integration.
