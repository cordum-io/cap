"""CAP Job Submitter — sends a job to job.echo, waits for the result, and exits."""

import asyncio
import os
import logging
import uuid

import nats
from google.protobuf import timestamp_pb2
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2

logging.basicConfig(level=logging.INFO, format="%(asctime)s [submit] %(message)s")
log = logging.getLogger("cap.playground.submit")

NATS_URL = os.environ.get("NATS_URL", "nats://127.0.0.1:4222")
RESULT_SUBJECT = "sys.job.result"
TIMEOUT_SECONDS = 15


async def main():
    job_id = f"playground-{uuid.uuid4().hex[:8]}"
    trace_id = f"trace-{uuid.uuid4().hex[:8]}"

    # Retry connection — NATS and worker may not be ready yet
    nc = None
    for attempt in range(1, 16):
        try:
            log.info("Connecting to NATS at %s (attempt %d)", NATS_URL, attempt)
            nc = await nats.connect(NATS_URL)
            break
        except Exception as exc:
            if attempt == 15:
                raise
            log.warning("Connection failed: %s — retrying in 2s", exc)
            await asyncio.sleep(2)

    # Give the worker a moment to be ready
    await asyncio.sleep(2)

    # Subscribe to results before submitting
    result_future = asyncio.get_event_loop().create_future()

    async def on_result(msg):
        pkt = buspacket_pb2.BusPacket()
        pkt.ParseFromString(msg.data)
        res = pkt.job_result
        if res.job_id == job_id:
            result_future.set_result(res)

    sub = await nc.subscribe(RESULT_SUBJECT, cb=on_result)

    # Build and send the job request
    req = job_pb2.JobRequest(
        job_id=job_id,
        topic="job.echo",
        context_ptr="inline://hello-from-cap-playground",
    )

    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id
    packet.sender_id = "playground-submitter"
    packet.protocol_version = 1
    packet.created_at.CopyFrom(ts)
    packet.job_request.CopyFrom(req)

    log.info("Submitting job %s (trace=%s)", job_id, trace_id)
    log.info("  topic:       %s", req.topic)
    log.info("  context_ptr: %s", req.context_ptr)

    # Publish directly to job.echo (no scheduler in playground)
    await nc.publish("job.echo", packet.SerializeToString(deterministic=True))
    await nc.flush()

    # Wait for result
    log.info("Waiting for result...")
    try:
        res = await asyncio.wait_for(result_future, timeout=TIMEOUT_SECONDS)
        log.info("Result received!")
        log.info("  job_id:     %s", res.job_id)
        log.info("  status:     %s", job_pb2.JobStatus.Name(res.status))
        log.info("  result_ptr: %s", res.result_ptr)
        log.info("  worker_id:  %s", res.worker_id)
        print()
        print("=" * 60)
        print("  CAP Playground Demo Complete!")
        print()
        print(f"  Job {res.job_id}")
        print(f"  Status: {job_pb2.JobStatus.Name(res.status)}")
        print(f"  Worker: {res.worker_id}")
        print("=" * 60)
    except asyncio.TimeoutError:
        log.error("Timed out after %ds waiting for result", TIMEOUT_SECONDS)

    await sub.unsubscribe()
    await nc.drain()


if __name__ == "__main__":
    asyncio.run(main())
