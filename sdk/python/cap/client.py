from google.protobuf import timestamp_pb2
from cap.pb.cortex.agent.v1 import buspacket_pb2


DEFAULT_PROTOCOL_VERSION = 1
SUBJECT_SUBMIT = "sys.job.submit"


async def submit_job(nc, job_request, trace_id: str, sender_id: str):
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id
    packet.sender_id = sender_id
    packet.created_at.CopyFrom(ts)
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.job_request.CopyFrom(job_request)
    await nc.publish(SUBJECT_SUBMIT, packet.SerializeToString())
