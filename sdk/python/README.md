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
