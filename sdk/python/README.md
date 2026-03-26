# CAP Python SDK

Asyncio-first SDK with NATS helpers for CAP workers and clients.

## Quick Start
1. Generate protobuf stubs into this SDK (one-time per proto change):
   ```bash
   python -m grpc_tools.protoc \
     -I../../proto \
     --python_out=./cap/pb \
     --grpc_python_out=./cap/pb \
     ../../proto/cordum/agent/v1/*.proto
   ```
   (Or run `./tools/make_protos.sh` from repo root with `CAP_RUN_PY=1` and copy `/python` into `sdk/python/cap/pb` if you want vendored stubs.)

2. Install:
   ```bash
   pip install -e .
   ```

3. Run a worker:
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

4. Submit a job (client):
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
