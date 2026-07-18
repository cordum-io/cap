"""Progress and cancel emission helpers for CAP Python SDK.

These helpers build and publish progress/cancel BusPacket envelopes.
"""

from typing import Optional

from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.client import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.packet_boundary import Publisher, finalize_packet, parse_packet
from cap.subjects import SUBJECT_CANCEL, SUBJECT_PROGRESS


def progress_payload(
    sender_id: str,
    job_id: str,
    step_id: str,
    percent: int,
    message: str,
    *,
    session_token: Optional[str] = None,
) -> bytes:
    """Build a progress payload wrapped in a BusPacket envelope."""
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()

    packet = buspacket_pb2.BusPacket()
    packet.trace_id = job_id
    packet.sender_id = sender_id
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.created_at.CopyFrom(ts)
    packet.job_progress.CopyFrom(
        job_pb2.JobProgress(
            job_id=job_id,
            step_id=step_id,
            percent=percent,
            message=message,
        )
    )
    outgoing = finalize_packet(packet, session_token=session_token)
    return outgoing.SerializeToString(deterministic=True)


def cancel_payload(
    sender_id: str,
    job_id: str,
    reason: str,
    requested_by: str,
    *,
    session_token: Optional[str] = None,
) -> bytes:
    """Build a cancel payload wrapped in a BusPacket envelope."""
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()

    packet = buspacket_pb2.BusPacket()
    packet.trace_id = job_id
    packet.sender_id = sender_id
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.created_at.CopyFrom(ts)
    packet.job_cancel.CopyFrom(
        job_pb2.JobCancel(
            job_id=job_id,
            reason=reason,
            requested_by=requested_by,
        )
    )
    outgoing = finalize_packet(packet, session_token=session_token)
    return outgoing.SerializeToString(deterministic=True)


async def emit_progress(
    nc: Publisher,
    payload: bytes,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    *,
    session_token: Optional[str] = None,
) -> None:
    """Publish one progress packet to the progress subject."""
    packet = finalize_packet(
        parse_packet(payload), private_key, session_token=session_token
    )
    data = packet.SerializeToString(deterministic=True)
    await nc.publish(SUBJECT_PROGRESS, data)


async def emit_cancel(
    nc: Publisher,
    payload: bytes,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    *,
    session_token: Optional[str] = None,
) -> None:
    """Publish one cancel packet to the cancel subject."""
    packet = finalize_packet(
        parse_packet(payload), private_key, session_token=session_token
    )
    data = packet.SerializeToString(deterministic=True)
    await nc.publish(SUBJECT_CANCEL, data)
