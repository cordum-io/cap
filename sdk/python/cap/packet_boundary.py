"""Shared validation and signing boundary for CAP bus packets."""

import logging
from typing import Dict, Optional, Protocol

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf.message import DecodeError

from cap.errors import InvalidInputError, MalformedPacketError, SignatureInvalidError
from cap.pb.cordum.agent.v1 import buspacket_pb2
from cap.validate import validate_bus_packet


PublicKeyMap = Dict[str, ec.EllipticCurvePublicKey]


class Publisher(Protocol):
    async def publish(self, subject: str, data: bytes) -> None: ...


def parse_packet(payload: bytes) -> buspacket_pb2.BusPacket:
    """Parse a bus packet or raise a typed malformed-packet error."""
    packet = buspacket_pb2.BusPacket()
    try:
        packet.ParseFromString(payload)
    except (DecodeError, TypeError, ValueError) as exc:
        raise MalformedPacketError(str(exc)) from exc
    return packet


def validate_packet(packet: buspacket_pb2.BusPacket) -> None:
    """Raise before a malformed envelope crosses a security boundary."""
    errors = validate_bus_packet(packet)
    if not errors:
        return
    details = "; ".join(f"{error.field}: {error.message}" for error in errors)
    raise InvalidInputError(f"invalid BusPacket: {details}")


def _attach_session_token(
    packet: buspacket_pb2.BusPacket,
    session_token: Optional[str],
) -> bool:
    if session_token is None:
        return False
    if not isinstance(session_token, str):
        raise TypeError("session_token must be a string or None")
    previous = packet.auth_token
    if session_token:
        packet.auth_token = session_token
    else:
        packet.ClearField("auth_token")
    return bool(packet.auth_token != previous)


def finalize_packet(
    packet: buspacket_pb2.BusPacket,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    *,
    session_token: Optional[str] = None,
) -> buspacket_pb2.BusPacket:
    """Clone, attach session proof, validate, then optionally sign a packet."""
    outgoing = buspacket_pb2.BusPacket()
    outgoing.CopyFrom(packet)
    token_changed = _attach_session_token(outgoing, session_token)
    validate_packet(outgoing)
    if token_changed and outgoing.signature and private_key is None:
        raise InvalidInputError("session token change requires re-signing the packet")
    if private_key is None:
        return outgoing
    if not isinstance(private_key, ec.EllipticCurvePrivateKey):
        raise InvalidInputError("packet signing requires an elliptic-curve private key")
    outgoing.ClearField("signature")
    unsigned = outgoing.SerializeToString(deterministic=True)
    outgoing.signature = private_key.sign(unsigned, ec.ECDSA(hashes.SHA256()))
    return outgoing


def verify_packet(
    packet: buspacket_pb2.BusPacket,
    public_keys: Optional[PublicKeyMap],
    logger: logging.Logger,
) -> bool:
    """Verify a configured sender key; ``None`` alone selects legacy mode."""
    if public_keys is None:
        return True
    safe_sender = _safe_log_field(packet.sender_id)
    if not packet.sender_id or not packet.signature:
        _safe_warning(
            logger, "missing signed sender identity", extra={"sender_id": safe_sender}
        )
        return False
    public_key = public_keys.get(packet.sender_id)
    if public_key is None or not isinstance(public_key, ec.EllipticCurvePublicKey):
        _safe_warning(
            logger, "no valid public key found", extra={"sender_id": safe_sender}
        )
        return False
    unsigned = buspacket_pb2.BusPacket()
    unsigned.CopyFrom(packet)
    signature = unsigned.signature
    unsigned.ClearField("signature")
    try:
        public_key.verify(
            signature,
            unsigned.SerializeToString(deterministic=True),
            ec.ECDSA(hashes.SHA256()),
        )
    except (InvalidSignature, TypeError, ValueError, AttributeError):
        error = SignatureInvalidError(f"sender {safe_sender}")
        _safe_warning(
            logger,
            "invalid signature: %s",
            error,
            extra={"sender_id": safe_sender},
        )
        return False
    return True


def decode_packet(
    payload: bytes,
    public_keys: Optional[PublicKeyMap],
    logger: logging.Logger,
) -> Optional[buspacket_pb2.BusPacket]:
    """Parse and validate before attempting signature verification."""
    try:
        packet = parse_packet(payload)
        validate_packet(packet)
    except (MalformedPacketError, InvalidInputError) as exc:
        _safe_warning(logger, "invalid packet: %s", exc)
        return None
    if not verify_packet(packet, public_keys, logger):
        return None
    return packet


def _safe_warning(
    logger: logging.Logger,
    message: str,
    *args: object,
    extra: Optional[Dict[str, str]] = None,
) -> None:
    try:
        logger.warning(message, *args, extra=extra)
    except Exception:
        # Diagnostics must not turn rejected traffic into a handler failure.
        pass


def _safe_log_field(value: str, limit: int = 256) -> str:
    safe = value.replace("\r", "\\r").replace("\n", "\\n")
    return safe if len(safe) <= limit else safe[: limit - 3] + "..."
