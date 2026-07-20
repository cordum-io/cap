"""CAP-PRODUCTION exact-wire signing and verification."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Mapping

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils

from .pb.cordum.agent.v1.buspacket_pb2 import BusPacket

DOMAIN = b"CAP-PRODUCTION-SIGNATURE-V1\x00"
PROFILE_VERSION = "cap-production-v1"
ALGORITHM = "ECDSA-P256-SHA256"


class ProductionSignatureError(ValueError):
    """Raised when production wire admission fails closed."""


@dataclass(frozen=True)
class ProductionTrust:
    audience: str
    public_keys: Mapping[str, ec.EllipticCurvePublicKey]
    tenant: str = ""
    sender: str = ""


def sign_production_packet(packet: BusPacket, key: ec.EllipticCurvePrivateKey) -> bytes:
    _validate_metadata(packet)
    clone = BusPacket()
    clone.CopyFrom(packet)
    clone.signature = b""
    unsigned = clone.SerializeToString()
    signature = key.sign(_digest(unsigned), ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    return unsigned + _field(14, signature)


def verify_production_packet(raw: bytes, trust: ProductionTrust) -> BusPacket:
    unsigned, signature = extract_signature(raw)
    packet = BusPacket()
    packet.ParseFromString(unsigned)
    _validate_metadata(packet, trust.audience)
    if trust.tenant and packet.identity.tenant_id != trust.tenant:
        raise ProductionSignatureError("tenant mismatch")
    if trust.sender and packet.sender_id != trust.sender:
        raise ProductionSignatureError("sender mismatch")
    key = trust.public_keys.get(packet.signature_metadata.key_id)
    if key is None:
        raise ProductionSignatureError("unknown key id")
    try:
        key.verify(signature, _digest(unsigned), ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    except Exception as exc:
        raise ProductionSignatureError("invalid signature") from exc
    packet.signature = signature
    return packet


def extract_signature(raw: bytes) -> tuple[bytes, bytes]:
    unsigned = bytearray()
    signature: bytes | None = None
    offset = 0
    while offset < len(raw):
        start = offset
        tag, offset = _varint(raw, offset)
        number, wire_type = tag >> 3, tag & 7
        value_end = _skip_value(raw, offset, wire_type)
        if number == 14:
            if signature is not None or wire_type != 2:
                raise ProductionSignatureError("duplicate or malformed signature field")
            length, data_start = _varint(raw, offset)
            if data_start + length != value_end or length == 0:
                raise ProductionSignatureError("malformed signature")
            signature = raw[data_start:value_end]
        else:
            unsigned.extend(raw[start:value_end])
        offset = value_end
    if signature is None:
        raise ProductionSignatureError("missing signature")
    return bytes(unsigned), signature


def _validate_metadata(packet: BusPacket, audience: str = "") -> None:
    if not packet.HasField("signature_metadata"):
        raise ProductionSignatureError("missing signature metadata")
    metadata = packet.signature_metadata
    if metadata.profile_version != PROFILE_VERSION or metadata.algorithm != ALGORITHM:
        raise ProductionSignatureError("unsupported production signature profile")
    if len(metadata.message_id) != 16 or not metadata.audience or not metadata.key_id:
        raise ProductionSignatureError("invalid signature metadata")
    if audience and metadata.audience != audience:
        raise ProductionSignatureError("audience mismatch")
    expiry = metadata.expires_at.ToDatetime(tzinfo=timezone.utc)
    if expiry <= datetime.now(timezone.utc):
        raise ProductionSignatureError("signature expired")


def _digest(unsigned: bytes) -> bytes:
    return hashlib.sha256(DOMAIN + unsigned).digest()


def _field(number: int, value: bytes) -> bytes:
    return _encode_varint(number << 3 | 2) + _encode_varint(len(value)) + value


def _varint(raw: bytes, offset: int) -> tuple[int, int]:
    value = 0
    for index in range(10):
        if offset >= len(raw):
            raise ProductionSignatureError("truncated varint")
        byte = raw[offset]
        offset += 1
        value |= (byte & 0x7F) << (index * 7)
        if byte < 0x80:
            if _encode_varint(value) != raw[offset - index - 1 : offset]:
                raise ProductionSignatureError("non-minimal varint")
            return value, offset
    raise ProductionSignatureError("oversize varint")


def _encode_varint(value: int) -> bytes:
    output = bytearray()
    while value >= 0x80:
        output.append((value & 0x7F) | 0x80)
        value >>= 7
    output.append(value)
    return bytes(output)


def _skip_value(raw: bytes, offset: int, wire_type: int) -> int:
    if wire_type == 0:
        _, offset = _varint(raw, offset)
        return offset
    if wire_type == 1:
        end = offset + 8
    elif wire_type == 2:
        length, offset = _varint(raw, offset)
        end = offset + length
    elif wire_type == 5:
        end = offset + 4
    else:
        raise ProductionSignatureError("unsupported wire type")
    if end > len(raw):
        raise ProductionSignatureError("truncated field")
    return end
