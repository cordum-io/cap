"""Standalone CAP echo round-trip using only PUBLIC cap-sdk-python imports.

Reference for the Python registry quickstart snippet. Uses the DIRECT-POOL
"development lab" wiring (worker on the submit subject, no Scheduler/Safety
Kernel). PRODUCTION submits through the governed Scheduler/Safety path.

Exits 0 only after correlating the terminal JobResult by job id and asserting
SUCCEEDED; exits non-zero on timeout, no worker, or error.
"""

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
    worker_task = asyncio.create_task(
        worker.run_worker(nats_url=url, subject="sys.job.submit", handler=handle, sender_id="echo-worker")
    )
    nc = await nats.connect(url)
    fut = asyncio.get_running_loop().create_future()

    async def on_result(msg):
        pkt = buspacket_pb2.BusPacket()
        pkt.ParseFromString(msg.data)
        if pkt.HasField("job_result") and pkt.job_result.job_id == JOB_ID and not fut.done():
            fut.set_result(pkt.job_result)

    await nc.subscribe(SUBJECT_RESULT, cb=on_result)
    await nc.flush()

    # worker.run_worker subscribes on a background task, and core NATS drops a
    # submit published before that subscription exists. Retry the submit until
    # the worker answers instead of relying on a fixed startup delay.
    loop = asyncio.get_running_loop()
    deadline = loop.time() + 10
    while True:
        await client.submit_job(
            nc, job_pb2.JobRequest(job_id=JOB_ID, topic="job.echo"), JOB_ID, "echo-client", None
        )
        try:
            result = await asyncio.wait_for(asyncio.shield(fut), timeout=0.5)
            break
        except asyncio.TimeoutError:
            if loop.time() >= deadline:
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
