import asyncio
from typing import Callable, Awaitable

from google.protobuf import timestamp_pb2
from cap.pb.cortex.agent.v1 import buspacket_pb2, job_pb2

DEFAULT_PROTOCOL_VERSION = 1
SUBJECT_RESULT = "sys.job.result"


async def run_worker(nats_url: str, subject: str, handler: Callable[[job_pb2.JobRequest], Awaitable[job_pb2.JobResult]]):
    import nats

    nc = await nats.connect(servers=nats_url, name="cap-worker")

    async def on_msg(msg):
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(msg.data)
        req = packet.job_request
        if not req.job_id:
            return
        try:
            res = await handler(req)
        except Exception as exc:  # noqa: BLE001
            res = job_pb2.JobResult(
                job_id=req.job_id,
                status=job_pb2.JOB_STATUS_FAILED,
                error_message=str(exc),
            )
        ts = timestamp_pb2.Timestamp()
        ts.GetCurrentTime()
        out = buspacket_pb2.BusPacket()
        out.trace_id = packet.trace_id
        out.sender_id = "cap-worker"
        out.protocol_version = DEFAULT_PROTOCOL_VERSION
        out.created_at.CopyFrom(ts)
        out.job_result.CopyFrom(res)
        await nc.publish(SUBJECT_RESULT, out.SerializeToString())

    await nc.subscribe(subject, queue=subject, cb=on_msg)
    try:
        while True:
            await asyncio.sleep(1)
    finally:
        await nc.drain()
