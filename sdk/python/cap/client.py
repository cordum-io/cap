from google.protobuf import timestamp_pb2
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from typing import Optional


from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.packet_boundary import Publisher, finalize_packet
from cap.subjects import SUBJECT_SUBMIT


async def submit_job(
    nc: Publisher,
    job_request: job_pb2.JobRequest,
    trace_id: str,
    sender_id: str,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    *,
    session_token: Optional[str] = None,
) -> None:
    """Publish a JobRequest onto the CAP submit subject.

    Args:
        nc: An active NATS connection.
        job_request: A protobuf JobRequest message.
        trace_id: Distributed trace identifier propagated through the bus.
        sender_id: Identity of the sender (used in the BusPacket envelope).
        private_key: Optional ECDSA private key for signing the packet.
        session_token: Optional trusted runtime token for the outer envelope.
    """
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id
    packet.sender_id = sender_id
    packet.created_at.CopyFrom(ts)
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.job_request.CopyFrom(job_request)

    outgoing = finalize_packet(
        packet, private_key, session_token=session_token
    )
    await nc.publish(
        SUBJECT_SUBMIT, outgoing.SerializeToString(deterministic=True)
    )
