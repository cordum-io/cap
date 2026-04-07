"""Handshake helpers for CAP Python SDK.

These helpers build and publish worker handshake BusPacket envelopes.
"""

from typing import Mapping, Optional, Sequence

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
from cap.subjects import SUBJECT_HANDSHAKE


def handshake_payload(
    component_id: str,
    capabilities: Optional[Mapping[str, bool]] = None,
    *,
    sender_id: Optional[str] = None,
    supported_versions: Optional[Sequence[int]] = None,
    ready_topics: Optional[Sequence[str]] = None,
) -> bytes:
    """Build a worker handshake payload."""
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()

    packet = buspacket_pb2.BusPacket()
    packet.sender_id = sender_id or component_id
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.created_at.CopyFrom(ts)
    packet.handshake.CopyFrom(
        handshake_pb2.Handshake(
            component_id=component_id,
            role=handshake_pb2.COMPONENT_ROLE_WORKER,
            supported_versions=list(supported_versions or [DEFAULT_PROTOCOL_VERSION]),
            capabilities=dict(capabilities or {}),
            ready_topics=list(ready_topics or []),
        )
    )
    return packet.SerializeToString(deterministic=True)


async def publish_handshake(
    nc,
    component_id: str,
    capabilities: Optional[Mapping[str, bool]] = None,
    *,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    sender_id: Optional[str] = None,
    supported_versions: Optional[Sequence[int]] = None,
    ready_topics: Optional[Sequence[str]] = None,
) -> None:
    """Publish one worker handshake packet to the CAP handshake subject."""
    payload = handshake_payload(
        component_id=component_id,
        capabilities=capabilities,
        sender_id=sender_id,
        supported_versions=supported_versions,
        ready_topics=ready_topics,
    )
    data = payload
    if private_key is not None:
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)
        packet.ClearField("signature")
        unsigned_data = packet.SerializeToString(deterministic=True)
        packet.signature = private_key.sign(unsigned_data, ec.ECDSA(hashes.SHA256()))
        data = packet.SerializeToString(deterministic=True)

    await nc.publish(SUBJECT_HANDSHAKE, data)
