"""CAP Echo Worker — listens on job.echo and returns a result."""

import asyncio
import os
import logging

from cap import worker
from cap.pb.cordum.agent.v1 import job_pb2

logging.basicConfig(level=logging.INFO, format="%(asctime)s [worker] %(message)s")
log = logging.getLogger("cap.playground.worker")

NATS_URL = os.environ.get("NATS_URL", "nats://127.0.0.1:4222")


async def handle(req: job_pb2.JobRequest) -> job_pb2.JobResult:
    log.info("Received job %s (topic=%s, context_ptr=%s)", req.job_id, req.topic, req.context_ptr)
    result = job_pb2.JobResult(
        job_id=req.job_id,
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        result_ptr=f"echo://{req.context_ptr}",
        worker_id="playground-worker",
    )
    log.info("Completed job %s -> SUCCEEDED", req.job_id)
    return result


async def main():
    # Retry connection — NATS may not be ready yet
    for attempt in range(1, 11):
        try:
            log.info("Connecting to NATS at %s (attempt %d)", NATS_URL, attempt)
            await worker.run_worker(
                nats_url=NATS_URL,
                subject="job.echo",
                handler=handle,
                sender_id="playground-worker",
            )
        except Exception as exc:
            if attempt == 10:
                raise
            log.warning("Connection failed: %s — retrying in 2s", exc)
            await asyncio.sleep(2)


if __name__ == "__main__":
    asyncio.run(main())
