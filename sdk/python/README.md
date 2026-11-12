# CAP Python SDK

Asyncio-first SDK with NATS helpers for CAP workers and clients. The supported
and CI-verified interpreter range is CPython 3.9 through 3.14.

## Compatibility

- Python: `>=3.9` (tested on every minor release from 3.9 through 3.14).
- Protobuf runtime: `protobuf>=6.31.1,<7`.
- gRPC runtime: `grpcio>=1.76.0,<2`.

The protobuf modules are included in the wheel and sdist; consumers do not need
`protoc` or `grpcio-tools`. The minimum runtime versions match the headers in
the checked-in generated modules.

## Consumer Quickstart (echo over NATS)

From an empty directory, install the released package and run an echo round-trip
against a running NATS server (`nats://127.0.0.1:4222`, or set `CAP_NATS_URL`).
This is the **direct-pool development-lab** wiring (worker on the submit subject,
no Scheduler/Safety Kernel); production submits through the governed
Scheduler/Safety path (see `docs/reference.md`).

```bash
python -m venv .venv && . .venv/bin/activate
pip install cap-sdk-python==2.17.0 nats-py
# add echo.py below, then:
python echo.py
```

<!-- cap-release:snippet:python-echo:python -->
```python
import asyncio
import os

import nats
from cap import client, worker, SUBJECT_RESULT
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2

JOB_ID = "echo-1"


async def handle(req: job_pb2.JobRequest) -> job_pb2.JobResult:
    return job_pb2.JobResult(
        job_id=req.job_id,
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        result_ptr=f"echo://{req.job_id}",
        worker_id="echo-worker",
    )


async def main() -> None:
    url = os.environ.get("CAP_NATS_URL", "nats://127.0.0.1:4222")

    # Dev-lab worker: consume submitted jobs directly (no Scheduler/Safety).
    worker_task = asyncio.create_task(
        worker.run_worker(nats_url=url, subject="sys.job.submit", handler=handle, sender_id="echo-worker")
    )
    await asyncio.sleep(0.5)  # allow the worker subscription to register

    nc = await nats.connect(url)
    fut: asyncio.Future = asyncio.get_running_loop().create_future()

    async def on_result(msg):
        pkt = buspacket_pb2.BusPacket()
        pkt.ParseFromString(msg.data)
        if pkt.HasField("job_result") and pkt.job_result.job_id == JOB_ID and not fut.done():
            fut.set_result(pkt.job_result)

    await nc.subscribe(SUBJECT_RESULT, cb=on_result)
    await nc.flush()

    await client.submit_job(nc, job_pb2.JobRequest(job_id=JOB_ID, topic="job.echo"), JOB_ID, "echo-client", None)

    try:
        result = await asyncio.wait_for(fut, timeout=10)
    except asyncio.TimeoutError:
        raise SystemExit("timed out waiting for JobResult (no worker?)")
    if result.status != job_pb2.JOB_STATUS_SUCCEEDED:
        raise SystemExit(f"job {result.job_id} ended {result.status}")
    print(f"job {result.job_id}: SUCCEEDED payload={result.result_ptr}")
    await nc.drain()
    worker_task.cancel()
    try:
        await worker_task
    except asyncio.CancelledError:
        pass


if __name__ == "__main__":
    asyncio.run(main())
```
<!-- cap-release:snippet-end -->

## Quick Start
1. Install:
   ```bash
   pip install -e .
   ```

2. Run a worker:
   ```python
   import asyncio
   from cap import worker
   from cap.pb.cordum.agent.v1 import job_pb2

   async def handle(req: job_pb2.JobRequest):
       return job_pb2.JobResult(
           job_id=req.job_id,
           status=job_pb2.JOB_STATUS_SUCCEEDED,
           result_ptr=f"redis://res/{req.job_id}",
           worker_id="worker-echo-1",
       )

   asyncio.run(worker.run_worker("nats://127.0.0.1:4222", "job.echo", handle))
   ```

3. Submit a job (client):
   ```python
   import asyncio
   from cryptography.hazmat.primitives.asymmetric import ec
   from cap import client
   from cap.pb.cordum.agent.v1 import job_pb2
   import nats

   async def main():
       nc = await nats.connect("nats://127.0.0.1:4222")
       priv = ec.generate_private_key(ec.SECP256R1())
       req = job_pb2.JobRequest(
           job_id="job-echo-1",
           topic="job.echo",
           context_ptr="redis://ctx/job-echo-1",
       )
       await client.submit_job(nc, req, "trace-1", "client-py", priv)
       await nc.drain()

   asyncio.run(main())
   ```

## Files
- `cap/bus.py` — NATS connector.
- `cap/worker.py` — worker skeleton with handler hook.
- `cap/client.py` — publish JobRequest to `sys.job.submit`.
- `cap/pb/` — protobuf stubs (generated).

## Defaults
- Subjects: `sys.job.submit`, `sys.job.result`, `sys.heartbeat`.
- Protocol version: `1`.
- Signing: `submit_job` and `run_worker` sign envelopes when given an `ec.EllipticCurvePrivateKey`. Signatures use deterministic protobuf serialization (map entries ordered by key) for cross-SDK verification. Generate a keypair with `cryptography`:
  ```python
  from cryptography.hazmat.primitives.asymmetric import ec
  priv = ec.generate_private_key(ec.SECP256R1())
  pub = priv.public_key()
  ```
- Set `public_keys` on `run_worker` to verify incoming packets.
- Omit `public_keys` to accept unsigned packets.
- Pass `private_key=None` to `submit_job` if you want to send unsigned envelopes.

Swap out `cap.bus` if you need a different transport.

## Testing

The `cap.testing` module lets you test handlers without running NATS or Redis.

```python
from cap.testing import run_handler
from cap.pb.cordum.agent.v1 import job_pb2

async def test_echo():
    result = await run_handler(
        lambda ctx, data: {"echo": data["prompt"]},
        {"prompt": "hello"},
        topic="job.echo",
    )
    assert result.status == job_pb2.JOB_STATUS_SUCCEEDED
```

- `run_handler(handler, input, **options)` — runs a single handler invocation and returns the `JobResult`.
- `create_test_agent(**options)` — returns `(agent, mock_nats, store)` pre-wired with `MockNATS` + `InMemoryBlobStore`.
- `MockNATS` — in-memory NATS mock for custom test setups.

## Runtime (High-Level SDK)
The runtime hides NATS/Redis plumbing and gives you typed handlers.

```python
import asyncio
from pydantic import BaseModel
from cap.runtime import Agent, Context

class Input(BaseModel):
    prompt: str

class Output(BaseModel):
    summary: str

agent = Agent(retries=2)

@agent.job("job.summarize", input_model=Input, output_model=Output)
async def summarize(ctx: Context, data: Input) -> Output:
    return Output(summary=data.prompt[:140])

asyncio.run(agent.run())
```

### Authenticated worker trust

- `Agent` accepts `worker_trust_mode`, `worker_trust`, bounded timeout/retry/renewal tuning, or the `CORDUM_SDK_HANDSHAKE` mode. `warn`/`enforce` require a complete `WorkerTrustConfig` with enrolled identities, exact audience, active P-256 proof key, scheduler identity/pins, and SDK version.
- Omitting every trust option retains legacy `off` for source compatibility. Once configuration or tuning is present, mode is explicit and `off` rejects dormant material; use `warn` only for visible migration and `enforce` for fail-closed admission.
- Before handlers, the runtime uses bounded protobuf request/reply on `sys.worker.handshake.challenge` and `sys.worker.handshake.authenticate`, verifies the pinned scheduler, installs and attaches the opaque token, and renews with the current token. It never falls back to ISSUE; expired, revoked, superseded, wrong-audience, or binding-mismatched sessions are invalid.
- Enroll/rotate through an authenticated control plane: register only the public key, retain the private key in the worker, overlap scheduler pins, then revoke old authoritative key/session records.
- `build_challenge_request`, `build_authenticate`, trust codecs/signers/verifiers, and `handshake_payload` / `publish_handshake` support adapters and compatibility; they do not enroll keys, issue/revoke sessions, grant topics, or authenticate `sys.handshake`.
- Low-level `run_worker(..., session_token=...)` is caller-managed static compatibility only: it does not mint, renew, reauthenticate after reconnect, or observe revocation. Use high-level `Agent` with `warn`/`enforce` for authenticated worker trust. `warn` fails open only for transport availability errors; malformed, rejected, unpinned, expired, or otherwise invalid proofs stop trust admission.
- Never log key material, tokens, signatures, nonces, complete trust packets, or raw rejections; use bounded mode/phase/outcome/coarse-reason telemetry.

### Failure and shutdown contract

- In both `run_worker()` and the high-level `Agent`, an ordinary handler
  `Exception` produces exactly one `JOB_STATUS_FAILED` result with the generic
  external message `handler failed`. In the `Agent`, a `cap.JobFailure`
  publishes its own status and code instead (see below). Diagnostics use
  bounded, newline-safe identifiers and the exception type without exposing
  exception text. Logging and metrics hooks are best-effort and cannot block
  the terminal result; the worker remains available for later jobs.
- `asyncio.CancelledError`, `KeyboardInterrupt`, and `SystemExit` remain control
  flow and are not converted into job results.
- Cancelling `run_worker()` drains its NATS connection before exit.
- `Agent.run()` always enters cleanup without letting a cleanup error replace
  cancellation or another primary failure. `Agent.close()` stops intake, stops
  the heartbeat, waits for tracked handlers (retaining the live session for their
  terminal results), drains NATS, and closes the blob
  store. Each asynchronous cleanup stage has the `shutdown_timeout` deadline
  (30 seconds by default), and later stages are still attempted after a timeout.
  Pass `None` only to opt out of deadlines; zero and negative values are
  rejected. Repeated or concurrent calls share the same cleanup operation.

#### Typed failures and error codes

In an `Agent` handler or middleware, raise `cap.JobFailure` to publish a
[spec/13](https://github.com/cordum-io/cap/blob/main/spec/13-error-codes.md)
error code instead of `handler failed`. For example, the handler above can
reject an empty prompt:

```python
from cap import JobFailure

@agent.job("job.summarize", input_model=Input, output_model=Output)
async def summarize(ctx: Context, data: Input) -> Output:
    if not data.prompt:
        raise JobFailure("EMPTY_PROMPT", retryable=False)
    return Output(summary=data.prompt[:140])
```

- An `int` code sets `error_code_enum`. It must be an `ErrorCode` value other
  than `ERROR_CODE_UNSPECIFIED`, or an application code in 1000-9999.
  `error_code` stays empty, except for the spec/13 legacy strings:
  - `ERROR_CODE_JOB_TIMEOUT`: `TIMEOUT`;
  - `ERROR_CODE_SAFETY_DENIED`: `DENIED`;
  - `ERROR_CODE_JOB_PERMISSION_DENIED`: `PERMISSION_DENIED`.
- A `str` code such as `"FRAMEWORK_ERROR"` sets `error_code` and leaves
  `error_code_enum` at `ERROR_CODE_UNSPECIFIED`. It must match
  `[A-Z][A-Z0-9_]{0,63}`. Any other code or `retryable` value, `bool` codes
  included, raises `TypeError` or `ValueError` when the `JobFailure` is built;
  inside a handler, that error is an ordinary exception.
- `retryable` selects the status:
  - `retryable=None` (the default): `JOB_STATUS_FAILED`;
  - `retryable=True`: `JOB_STATUS_FAILED_RETRYABLE`;
  - `retryable=False`: `JOB_STATUS_FAILED_FATAL`.
- `error_message` is always `handler reported failure`, and the metrics hook's
  `on_job_failed` gets the same text. The exception's arguments, its cause and
  the input are never published. A subclass must call `JobFailure.__init__`;
  one that does not publishes `handler failed`, still without a retry.
- The SDK never retries after a `JobFailure`: it publishes that outcome at
  once, whatever `retries` says. An ordinary exception, an `output_model`
  validation error included, still runs the handler up to `retries + 1` times;
  if every attempt fails, the result is `handler failed`.
- The low-level `run_worker()` is unchanged. Its handlers build their own
  `JobResult`, and a `JobFailure` raised there is an ordinary exception.

Failures the `Agent` detects itself publish `JOB_STATUS_FAILED` with an empty
`error_code` and fixed text, never exception text, input bytes or pointer text:

| Condition | `error_code_enum` | `error_message` |
|---|---|---|
| `context_ptr` is not a valid pointer | `ERROR_CODE_JOB_INVALID_INPUT` | `context pointer invalid` |
| a `resource_registry` is set and `context_ref` is missing | `ERROR_CODE_JOB_INVALID_INPUT` | `context ref required` |
| the registry refused `context_ref`, or `context_ptr` names other bytes | `ERROR_CODE_JOB_INVALID_INPUT` | `context ref refused` |
| context fetch raised or timed out | `ERROR_CODE_UNSPECIFIED` | `context fetch failed` |
| no context blob at the pointer | `ERROR_CODE_JOB_NOT_FOUND` | `context not found` |
| context over `max_context_bytes` | `ERROR_CODE_JOB_RESOURCE_EXHAUSTED` | `context exceeds max size` |
| context is not UTF-8 JSON | `ERROR_CODE_JOB_INVALID_INPUT` | `context decode failed` |
| `input_model` rejects the input | `ERROR_CODE_JOB_INVALID_INPUT` | `input validation failed` |
| handler, middleware or `output_model` raised on every attempt | `ERROR_CODE_UNSPECIFIED` | `handler failed` |
| output is not JSON-serializable | `ERROR_CODE_UNSPECIFIED` | `result encode failed` |
| result over `max_result_bytes` | `ERROR_CODE_JOB_RESOURCE_EXHAUSTED` | `result exceeds max size` |
| result write raised or timed out | `ERROR_CODE_UNSPECIFIED` | `result write failed` |

#### ResourceRef input and output

Pass an operator-built `cap.ResourceRegistry` to read job input from
`JobRequest.context_ref` and publish results as `JobResult.result_ref`:

```python
from cap import Agent, InMemoryResourceBackend, ResourceBackendRegistration, ResourceRegistry

backend = InMemoryResourceBackend()  # tests only: install a durable backend in production
registry = ResourceRegistry([ResourceBackendRegistration("memory", 2 * 1024 * 1024, backend, backend)])
agent = Agent(
    resource_registry=registry,
    result_backend_id="memory",  # required with a registry
    result_ref_ttl=3600.0,  # seconds; the default
    worker_trust_mode="enforce",
    worker_trust=trust,  # a cap.WorkerTrustConfig
)
```

- Every resolve and externalize runs under the worker-trust tenant
  (`worker_trust.tenant_id`) and the admitted job id. Neither comes from the
  ref. `start()` raises `RuntimeError` before it connects when there is no
  worker-trust configuration (mode `warn` or `enforce`). No Redis store is opened.
- Input comes only from `context_ref`. A `context_ptr` sent with it must name
  the same Redis key (spec/19). A ref whose `media_type` is not
  `application/json`, or whose `size_bytes` is over `max_context_bytes`, is
  rejected before any fetch. The handler never runs for a missing, refused or
  oversize ref, and `retries` never repeats the resolve.
- A ref over the backend's `max_bytes` publishes `context exceeds max size`. A
  backend error or timeout publishes `context fetch failed`, and so does another
  tenant's ref, which the backend cannot find. A non-JSON ref publishes
  `context decode failed`, and any other registry error `context ref refused`.
- A SUCCEEDED result is externalized as `application/json` with purpose
  `job-result`, expiring `result_ref_ttl` seconds later. It carries
  `result_ref`, and `result_ptr` stays empty. A result over the backend's
  `max_bytes` publishes `result exceeds max size`; any other failure
  `result write failed`.
- Without `resource_registry`, `context_ptr` and `result_ptr` work as before.

#### Cancellation and deadlines

Tags: cancellation, deadlines, worker-contract.

`Agent` subscribes to `sys.job.cancel` without a queue group or a handler slot.
It first checks whether the unverified job id is running locally (routing only,
not authority), then admits the cancel exactly like a request. CAP-PRODUCTION
requires the `sys.job.cancel` audience, session-bound trust and replay admission;
a set cancel identity must equal the verified envelope identity. The admitted
job id and the whole dispatch (dispatch id, attempt, assigned worker) must match
the running attempt. Empty dispatch matches only empty dispatch. Refused cancels
change nothing, and non-target workers do not consume shared replay entries.

`Agent(max_deadline=None)` keeps unbounded execution by default. A positive
`Budget.deadline_ms` gives `min(deadline_ms / 1000, max_deadline)` seconds, or the
budget alone when the maximum is `None`. A zero budget uses the maximum.
The maximum must be positive and finite; booleans and non-numbers are rejected.
The clock starts after input resolution, at handler start, and includes
middleware and retries. Result writes and publication still use `io_timeout`.
`Context.deadline` is that `time.monotonic()` instant, or `None`;
`Context.cancelled` is a read-only, thread-safe boolean set when cancel or
deadline stops the handler.

Cancellation publishes `JOB_STATUS_CANCELLED`, an empty `error_code`, enum
`UNSPECIFIED`, and `job cancelled`. Deadline expiry publishes `JOB_STATUS_FAILED`
(never status TIMEOUT), `ERROR_CODE_JOB_TIMEOUT`, `TIMEOUT`, and
`job deadline exceeded`. Cancel reason and requester text are never published or
logged. Exactly one terminal result is selected: the first observed event wins,
with completion before cancel before deadline when observed together. A cancel
before handler start skips the handler with execution time zero; an input
failure keeps its own outcome.

A synchronous handler blocks the event loop and cannot be pre-empted; its result
wins once it returns. Blocking work in `asyncio.to_thread` gets its terminal
result at the bound, while the thread finishes independently. A handler that
suppresses `CancelledError` keeps its slot until it returns; that outcome is
discarded. Once `close()` drains subscriptions, no further cancels are received.
The low-level `cap.worker.run_worker` is unchanged.

### Middleware

Add cross-cutting concerns (logging, auth, metrics) without modifying handlers:

```python
from cap.middleware import logging_middleware

# Built-in logging middleware
agent.use(logging_middleware())

# Custom middleware
async def timing(ctx, data, next_fn):
    import time
    start = time.monotonic()
    result = await next_fn(ctx, data)
    elapsed = time.monotonic() - start
    print(f"job {ctx.job_id} took {elapsed:.3f}s")
    return result

agent.use(timing)
```

Middleware executes in registration order (FIFO). Each can inspect context,
measure timing, or short-circuit by returning without calling `next_fn`.

### Redis TLS
The Python SDK provides `redis_ssl_context_from_env()` to build an `SSLContext` for secure Redis connections. It reads:
- `REDIS_TLS_CA` (or `SSL_CERT_FILE` fallback): Path to CA certificate.
- `REDIS_TLS_CERT` / `REDIS_TLS_KEY`: Path to client certificate/key pair.
- `REDIS_TLS_SERVER_NAME`: SNI server name override.
- `REDIS_TLS_INSECURE`: Set to `1` or `true` to skip certificate verification (dev only).

### Environment
- `NATS_URL` (default `nats://127.0.0.1:4222`)
- `REDIS_URL` (default `redis://127.0.0.1:6379/0`)

## Contributor and Release Verification

Run these commands from the repository root. Generated Python modules are
checked, never rewritten, by the pinned toolchain:

```bash
python -m pip install -e "sdk/python[dev]"
python -m pytest -q sdk/python/tests --ignore=sdk/python/tests/integration

python -m pip install -r sdk/python/requirements-codegen.txt
python sdk/python/scripts/generate_protos.py --check
```

The real-NATS test is mandatory in CI and never skips. Run it against an
explicit broker URL (CI and publishing pin NATS 2.12.6 by image digest):

```bash
CAP_TEST_NATS_URL=nats://127.0.0.1:4222 \
  python -m pytest -q sdk/python/tests/integration/test_worker_nats.py
```

Build and verify the exact wheel/sdist pair in clean consumer environments:

```bash
rm -rf dist/python
python -m build --outdir dist/python sdk/python
python -m twine check dist/python/*
python sdk/python/scripts/verify_artifacts.py \
  --wheel dist/python/*.whl \
  --sdist dist/python/*.tar.gz \
  > dist/python/artifact-verification.json
```

Before creating a release, bump the checked-in package version and require an
exact lowercase `v<version>` tag. Replace `<version>` below with the version in
`sdk/python/pyproject.toml`:

```bash
python sdk/python/scripts/validate_release.py \
  --tag "v<version>" \
  --artifact-report dist/python/artifact-verification.json
```

The publish workflow exports the tagged commit, builds once, records the exact
artifact inventory and SHA-256 checksums, and publishes only that verified
pair. Do not mutate generated code or package metadata during publishing, and
do not use a manual or `skip-existing` fallback.

## Generating API Docs

Generate HTML API reference locally using [pdoc](https://pdoc.dev/):

```bash
pip install cap-sdk-python[dev]
pdoc ./cap --output-dir docs
```

Output is written to `docs/` (gitignored). Open `docs/index.html` to browse.

## Observability

### Structured Logging
The runtime Agent and Worker use `logging.Logger` (stdlib) for structured logging. All log calls include contextual fields (`job_id`, `trace_id`, `topic`, `sender_id`). Pass a custom logger or leave as default:

```python
import logging
from cap.runtime import Agent

logger = logging.getLogger("my-agent")
logger.setLevel(logging.DEBUG)
agent = Agent(logger=logger)
```

### MetricsHook
Implement the `MetricsHook` protocol to integrate with Prometheus, OpenTelemetry, or any metrics system:

```python
from cap.metrics import MetricsHook

class MetricsHook(Protocol):
    def on_job_received(self, job_id: str, topic: str) -> None: ...
    def on_job_completed(self, job_id: str, duration_ms: int, status: str) -> None: ...
    def on_job_failed(self, job_id: str, error_msg: str) -> None: ...
    def on_heartbeat_sent(self, worker_id: str) -> None: ...
```

The default is `NoopMetrics` (zero overhead). Example Prometheus integration:

```python
from cap.runtime import Agent

class PromMetrics:
    def on_job_received(self, job_id, topic):
        jobs_received.labels(topic=topic).inc()

    def on_job_completed(self, job_id, duration_ms, status):
        job_duration.labels(status=status).observe(duration_ms)

    def on_job_failed(self, job_id, error_msg):
        jobs_failed.inc()

    def on_heartbeat_sent(self, worker_id):
        pass

agent = Agent(metrics=PromMetrics())
```

The `trace_id` is propagated through all log and metrics calls for distributed tracing correlation.
