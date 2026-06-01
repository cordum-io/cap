"""Progress and cancel emission helpers for CAP Python SDK.

These helpers build and publish progress/cancel BusPacket envelopes.
"""

from typing import Optional

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.client import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.subjects import SUBJECT_CANCEL, SUBJECT_PROGRESS


def progress_payload(
    sender_id: str,
    job_id: str,
    step_id: str,
    percent: int,
    message: str,
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
    return packet.SerializeToString(deterministic=True)


def cancel_payload(
    sender_id: str,
    job_id: str,
    reason: str,
    requested_by: str,
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
    return packet.SerializeToString(deterministic=True)


async def emit_progress(
    nc,
    payload: bytes,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
) -> None:
    """Publish one progress packet to the progress subject."""
    data = payload
    if private_key is not None:
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)
        packet.ClearField("signature")
        unsigned_data = packet.SerializeToString(deterministic=True)
        packet.signature = private_key.sign(unsigned_data, ec.ECDSA(hashes.SHA256()))
        data = packet.SerializeToString(deterministic=True)

    await nc.publish(SUBJECT_PROGRESS, data)


async def emit_cancel(
    nc,
    payload: bytes,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
) -> None:
    """Publish one cancel packet to the cancel subject."""
    data = payload
    if private_key is not None:
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)
        packet.ClearField("signature")
        unsigned_data = packet.SerializeToString(deterministic=True)
        packet.signature = private_key.sign(unsigned_data, ec.ECDSA(hashes.SHA256()))
        data = packet.SerializeToString(deterministic=True)

    await nc.publish(SUBJECT_CANCEL, data)
