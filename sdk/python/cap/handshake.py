"""Handshake helpers for CAP Python SDK.

These helpers build and publish worker handshake BusPacket envelopes.
"""

from typing import Mapping, Optional, Sequence
from uuid import uuid4

from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.heartbeat import sanitize_agent_name
from cap.packet_boundary import Publisher, finalize_packet, parse_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
from cap.subjects import SUBJECT_HANDSHAKE


def handshake_payload(
    component_id: str,
    capabilities: Optional[Mapping[str, bool]] = None,
    *,
    sender_id: Optional[str] = None,
    supported_versions: Optional[Sequence[int]] = None,
    ready_topics: Optional[Sequence[str]] = None,
    agent_name: str = "",
    trace_id: Optional[str] = None,
    session_token: Optional[str] = None,
) -> bytes:
    """Build a worker handshake payload."""
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()

    packet = buspacket_pb2.BusPacket()
    packet.trace_id = trace_id or uuid4().hex
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
            agent_name=sanitize_agent_name(agent_name),
        )
    )
    outgoing = finalize_packet(packet, session_token=session_token)
    return outgoing.SerializeToString(deterministic=True)


async def publish_handshake(
    nc: Publisher,
    component_id: str,
    capabilities: Optional[Mapping[str, bool]] = None,
    *,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    sender_id: Optional[str] = None,
    supported_versions: Optional[Sequence[int]] = None,
    ready_topics: Optional[Sequence[str]] = None,
    agent_name: str = "",
    trace_id: Optional[str] = None,
    session_token: Optional[str] = None,
) -> None:
    """Publish one worker handshake packet to the CAP handshake subject."""
    payload = handshake_payload(
        component_id=component_id,
        capabilities=capabilities,
        sender_id=sender_id,
        supported_versions=supported_versions,
        ready_topics=ready_topics,
        agent_name=agent_name,
        trace_id=trace_id,
        session_token=session_token,
    )
    packet = finalize_packet(
        parse_packet(payload), private_key, session_token=session_token
    )
    data = packet.SerializeToString(deterministic=True)
    await nc.publish(SUBJECT_HANDSHAKE, data)
