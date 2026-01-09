import asyncio

from cap import worker
from cap.pb.cordum.agent.v1 import job_pb2


async def handle(req: job_pb2.JobRequest) -> job_pb2.JobResult:
    return job_pb2.JobResult(
        job_id=req.job_id,
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        result_ptr=f"redis://res/{req.job_id}",
        worker_id="worker-echo-py",
    )


async def main():
    await worker.run_worker(
        nats_url="nats://127.0.0.1:4222",
        subject="job.echo",
        handler=handle,
        sender_id="worker-echo-py",
    )


if __name__ == "__main__":
    asyncio.run(main())
