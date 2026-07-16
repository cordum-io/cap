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

### Failure and shutdown contract

- In both `run_worker()` and the high-level `Agent`, an ordinary handler
  `Exception` produces exactly one `JOB_STATUS_FAILED` result with the generic
  external message `handler failed`. Diagnostics use bounded, newline-safe
  identifiers and the exception type without exposing exception text. Logging
  and metrics hooks are best-effort and cannot block the terminal result; the
  worker remains available for later jobs.
- `asyncio.CancelledError`, `KeyboardInterrupt`, and `SystemExit` remain control
  flow and are not converted into job results.
- Cancelling `run_worker()` drains its NATS connection before exit.
- `Agent.run()` always enters cleanup without letting a cleanup error replace
  cancellation or another primary failure. `Agent.close()` stops intake, stops
  the heartbeat, waits for tracked handlers, drains NATS, and closes the blob
  store. Each asynchronous cleanup stage has the `shutdown_timeout` deadline
  (30 seconds by default), and later stages are still attempted after a timeout.
  Pass `None` only to opt out of deadlines; zero and negative values are
  rejected. Repeated or concurrent calls share the same cleanup operation.

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
explicit broker URL (CI uses NATS 2.10.29):

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
