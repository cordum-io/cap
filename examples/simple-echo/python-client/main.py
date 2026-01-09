import asyncio

import nats
from cap import client
from cap.pb.cordum.agent.v1 import job_pb2


async def main():
    nc = await nats.connect("nats://127.0.0.1:4222")
    req = job_pb2.JobRequest(
        job_id="job-echo-1",
        topic="job.echo",
        context_ptr="redis://ctx/job-echo-1",
    )
    await client.submit_job(nc, req, "trace-1", "client-py", None)
    await nc.drain()


if __name__ == "__main__":
    asyncio.run(main())
