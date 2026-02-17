from google.protobuf import timestamp_pb2
from cap.pb.cordum.agent.v1 import buspacket_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes
from typing import Optional


from cap.subjects import SUBJECT_SUBMIT

DEFAULT_PROTOCOL_VERSION = 1


async def submit_job(
    nc,
    job_request,
    trace_id: str,
    sender_id: str,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
):
    """Publish a JobRequest onto the CAP submit subject.

    Args:
        nc: An active NATS connection.
        job_request: A protobuf JobRequest message.
        trace_id: Distributed trace identifier propagated through the bus.
        sender_id: Identity of the sender (used in the BusPacket envelope).
        private_key: Optional ECDSA private key for signing the packet.
    """
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id
    packet.sender_id = sender_id
    packet.created_at.CopyFrom(ts)
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.job_request.CopyFrom(job_request)

    if private_key:
        unsigned_data = packet.SerializeToString(deterministic=True)
        signature = private_key.sign(unsigned_data, ec.ECDSA(hashes.SHA256()))
        packet.signature = signature

    await nc.publish(SUBJECT_SUBMIT, packet.SerializeToString(deterministic=True))
