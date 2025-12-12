# CAP Python SDK

Asyncio-first SDK with NATS helpers for CAP workers and clients.

## Quick Start
1. Generate protobuf stubs into this SDK (one-time per proto change):
   ```bash
   python -m grpc_tools.protoc \
     -I../../proto \
     --python_out=./cap/pb \
     --grpc_python_out=./cap/pb \
     ../../proto/coretex/agent/v1/*.proto
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
   from cap.pb.coretex.agent.v1 import job_pb2

   async def handle(req: job_pb2.JobRequest):
       return job_pb2.JobResult(
           job_id=req.job_id,
           status=job_pb2.JOB_STATUS_SUCCEEDED,
           result_ptr=f"redis://res/{req.job_id}",
           worker_id="worker-echo-1",
       )

   asyncio.run(worker.run_worker("nats://127.0.0.1:4222", "job.echo", handle))
   ```

## Files
- `cap/bus.py` — NATS connector.
- `cap/worker.py` — worker skeleton with handler hook.
- `cap/client.py` — publish JobRequest to `sys.job.submit`.
- `cap/pb/` — protobuf stubs (generated).

## Defaults
- Subjects: `sys.job.submit`, `sys.job.result`, `sys.heartbeat`.
- Protocol version: `1`.

Swap out `cap.bus` if you need a different transport.
