"""Bounded deterministic codec for worker trust-handshake packets."""

from cap.pb.cordum.agent.v1 import buspacket_pb2
from cap.worker_trust import WORKER_HANDSHAKE_MAX_PACKET_SIZE, WorkerHandshakePacketError
from cap.worker_trust_validate import validate_worker_trust_packet


def marshal_worker_trust_packet(packet: buspacket_pb2.BusPacket) -> bytes:
    """Validate and deterministically encode one trust phase."""

    validate_worker_trust_packet(packet)
    data = packet.SerializeToString(deterministic=True)
    if len(data) > WORKER_HANDSHAKE_MAX_PACKET_SIZE:
        raise WorkerHandshakePacketError("packet size exceeds 65536 bytes")
    return data


def unmarshal_worker_trust_packet(data: bytes) -> buspacket_pb2.BusPacket:
    """Reject empty, oversized, malformed, unknown-field, or invalid input."""

    if not isinstance(data, bytes) or not 0 < len(data) <= WORKER_HANDSHAKE_MAX_PACKET_SIZE:
        raise WorkerHandshakePacketError("packet size must be 1..65536 bytes")
    packet = buspacket_pb2.BusPacket()
    try:
        consumed = packet.ParseFromString(data)
    except Exception as exc:
        raise WorkerHandshakePacketError("cannot decode worker trust packet") from exc
    if consumed is not None and consumed != len(data):
        raise WorkerHandshakePacketError("worker trust packet has trailing bytes")
    validate_worker_trust_packet(packet)
    return packet


__all__ = ["marshal_worker_trust_packet", "unmarshal_worker_trust_packet"]
