# Python SDK

The Python SDK for CAP is ideal for AI/ML applications, scripting, and rapid prototyping.

## Installation

```bash
pip install -r requirements.txt
```
*(Note: A `requirements.txt` file needs to be created with the necessary dependencies, e.g., `nats-python`, `cryptography`, `protobuf`)*

## Client

The `cap.client` module provides functions for submitting jobs.

### `submit_job`

The `submit_job` function sends a `JobRequest` to the bus.

```python
async def submit_job(nc, job_request, trace_id: str, sender_id: str, private_key: ec.EllipticCurvePrivateKey):
```

**Example:**

```python
import asyncio
import nats
from cryptography.hazmat.primitives.asymmetric import ec
from cap.pb.cortex.agent.v1 import job_pb2
from cap import client

async def main():
    nc = await nats.connect("nats://localhost:4222")
    private_key = ec.generate_private_key(ec.SECP256R1())

    req = job_pb2.JobRequest(
        job_id="my-test-job",
        topic="job.echo"
    )

    await client.submit_job(
        nc,
        req,
        "my-trace-id",
        "my-client",
        private_key
    )

    print("Job submitted")
    await nc.close()

if __name__ == '__main__':
    asyncio.run(main())
```

## Worker

The `cap.worker` module provides tools for building workers.

### `run_worker`

The `run_worker` function starts a worker that listens for jobs on a given subject.

```python
async def run_worker(nats_url: str, subject: str, handler: Callable[[job_pb2.JobRequest], Awaitable[job_pb2.JobResult]],
                     public_keys: Dict[str, ec.EllipticCurvePublicKey] = None,
                     private_key: ec.EllipticCurvePrivateKey = None,
                     sender_id: str = "cap-worker"):
```

**Example:**

```python
import asyncio
from typing import Awaitable
from cryptography.hazmat.primitives.asymmetric import ec
from cap.pb.cortex.agent.v1 import job_pb2
from cap import worker

async def my_handler(req: job_pb2.JobRequest) -> Awaitable[job_pb2.JobResult]:
    print(f"Received job: {req.job_id}")
    return job_pb2.JobResult(
        status=job_pb2.JOB_STATUS_SUCCEEDED
    )

async def main():
    private_key = ec.generate_private_key(ec.SECP256R1())
    await worker.run_worker(
        nats_url="nats://localhost:4222",
        subject="job.echo",
        handler=my_handler,
        sender_id="my-worker",
        private_key=private_key
    )

if __name__ == '__main__':
    asyncio.run(main())
```
