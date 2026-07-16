import asyncio
import os

import cap
from cap.pb.cordum.agent.v1 import job_pb2


WORKER_ID = "worker-echo-py"
WORKER_SUBJECT = "job.echo"


async def handle(request: job_pb2.JobRequest) -> job_pb2.JobResult:
    return job_pb2.JobResult(
        job_id=request.job_id,
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        result_ptr=f"demo://result/{request.job_id}",
        worker_id=WORKER_ID,
    )


async def main() -> None:
    print(f"CAP_SIMPLE_ECHO_WORKER_STARTING subject={WORKER_SUBJECT}", flush=True)
    await cap.run_worker(
        nats_url=os.getenv("CAP_NATS_URL", "nats://127.0.0.1:4222"),
        subject=WORKER_SUBJECT,
        handler=handle,
        sender_id=WORKER_ID,
    )


if __name__ == "__main__":
    asyncio.run(main())
