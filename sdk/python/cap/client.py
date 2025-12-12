import nats
from google.protobuf import timestamp_pb2
from cap.pb.coretex.agent.v1 import buspacket_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes
import hashlib


DEFAULT_PROTOCOL_VERSION = 1
SUBJECT_SUBMIT = "sys.job.submit"


async def submit_job(nc, job_request, trace_id: str, sender_id: str, private_key: ec.EllipticCurvePrivateKey):
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id
    packet.sender_id = sender_id
    packet.created_at.CopyFrom(ts)
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.job_request.CopyFrom(job_request)

    unsigned_data = packet.SerializeToString()
    digest = hashlib.sha256(unsigned_data).digest()
    signature = private_key.sign(digest, ec.ECDSA(hashes.SHA256()))
    packet.signature = signature

    await nc.publish(SUBJECT_SUBMIT, packet.SerializeToString())
