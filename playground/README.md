# CAP Playground

Run a deterministic CAP job/result round trip with Docker Compose. The stack
contains only a pinned NATS broker, one Python echo worker, and one fail-closed
submitter.

> **Development-only boundary:** the submitter publishes a validated CAP packet
> directly to `job.echo`. This bypasses the Gateway, Scheduler, Safety Kernel,
> policy, authorization, authenticated identity, durable state, and retries.
> The broker and its unauthenticated monitoring endpoint stay inside the Compose
> network and are not published on host interfaces.

## Prerequisite

- Docker with Compose v2.

## Run with an authoritative exit code

From this directory, use the exact attached command below. The `submit` service
owns the verdict: success exits `0`; connection/readiness/result/terminal or
protocol failure exits nonzero. The trap removes containers, networks, volumes,
and orphans while preserving that original verdict.

```bash
(
  bounded_compose_down() {
    docker compose down -v --remove-orphans --timeout 5 >/dev/null 2>&1 &
    down_pid=$!
    (
      sleep 30
      kill -TERM "$down_pid" >/dev/null 2>&1 || true
      sleep 2
      kill -KILL "$down_pid" >/dev/null 2>&1 || true
    ) &
    watchdog_pid=$!
    wait "$down_pid" >/dev/null 2>&1 || true
    kill "$watchdog_pid" >/dev/null 2>&1 || true
    wait "$watchdog_pid" >/dev/null 2>&1 || true
  }
  cleanup() {
    rc=$?
    trap - EXIT INT TERM
    bounded_compose_down
    exit "$rc"
  }
  trap cleanup EXIT INT TERM
  docker compose up --build --abort-on-container-exit --exit-code-from submit
)
```

The submitter first subscribes and flushes `sys.job.result`, then polls NATS
monitoring until `/healthz` is healthy and an object in
`/subsz?subs=1` has `subject == "job.echo"`. Only then does it publish once.
A successful run prints exactly one marker like:

```text
CAP Playground Demo Complete! job_id=playground-... trace_id=trace-...
```

Timeouts and terminal failures never print that marker. Exit codes are stable:

| Code | Meaning |
|---:|---|
| `0` | Correlated, validated `JOB_STATUS_SUCCEEDED` |
| `2` | NATS connect/subscribe/publish/result transport failure |
| `3` | Exact worker subscription was not ready before the deadline |
| `4` | No matching result before the single result deadline |
| `5` | Matching result reported a known non-success terminal status |
| `6` | Matching result or local configuration violated the protocol contract |

## What happened

```text
Submitter                    NATS                    Echo Worker
    |-- subscribe+flush ---->| sys.job.result             |
    |-- monitor readiness -->| job.echo subscription -----|
    |-- BusPacket{JobRequest} -> job.echo ---------------->|
    |<-- BusPacket{JobResult} -- sys.job.result <----------|
```

Both trace ID and job ID must match, but correlation is not authentication. A
governed deployment instead enters through a Gateway or trusted ingress at
`sys.job.submit`; the Scheduler obtains a Safety decision and dispatches to
`job.<pool>`.

## Configuration

Compose supplies bounded defaults. Override only when diagnosing a slow local
machine:

- `CAP_NATS_CONNECT_TIMEOUT_SECONDS` (default `5`)
- `CAP_WORKER_READY_TIMEOUT_SECONDS` (default `20`)
- `CAP_RESULT_TIMEOUT_SECONDS` (default `15`)
- `CAP_CLEANUP_TIMEOUT_SECONDS` (default `2`)

All values must be positive finite numbers. Monitoring is a local readiness
signal only; it is not an authentication or trust mechanism.

## Next steps

- [First CAP Job](../docs/getting-started.md) — run installed Go, Python, or Node artifacts
- [Transport profile](../spec/09-transport-profile.md) — normative subjects and topology
- [Examples](../examples/) — more CAP patterns
