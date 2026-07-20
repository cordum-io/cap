"""CAP-PRODUCTION exact-wire signing and verification."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Callable, Mapping

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils
from google.protobuf.message import DecodeError

from .pb.cordum.agent.v1.buspacket_pb2 import BusPacket, SignatureMetadata

DOMAIN = b"CAP-PRODUCTION-SIGNATURE-V1\x00"
PROFILE_VERSION = "cap-production-v1"
ALGORITHM = "ECDSA-P256-SHA256"
DEFAULT_MAX_LIFETIME = timedelta(minutes=5)
MAX_CLOCK_SKEW = timedelta(minutes=1)
MAX_PRODUCTION_RAW_BYTES = 1 << 20
_BUS_PACKET_WIRE_TYPES = {
    **{number: 2 for number in (1, 2, 3, 5, 6)},
    4: 0,
    **{number: 2 for number in range(10, 23)},
}
_BUS_PACKET_PAYLOAD_FIELDS = frozenset(
    {10, 11, 12, 13, 15, 16, 17, 19, 20, 21, 22}
)


class ProductionSignatureError(ValueError):
    """Raised when production wire admission fails closed."""


@dataclass(frozen=True)
class ProductionTrust:
    audience: str
    public_keys: Mapping[str, ec.EllipticCurvePublicKey]
    tenant: str
    sender: str
    max_lifetime: timedelta = DEFAULT_MAX_LIFETIME
    clock_skew: timedelta = timedelta(0)
    now: Callable[[], datetime] = lambda: datetime.now(timezone.utc)


def sign_production_packet(packet: BusPacket, key: ec.EllipticCurvePrivateKey) -> bytes:
    _require_p256_key(key)
    _validate_metadata(packet)
    clone = BusPacket()
    clone.CopyFrom(packet)
    clone.signature = b""
    unsigned = clone.SerializeToString()
    _validate_unsigned_bus_packet(unsigned)
    _reject_unknown_nested_fields(clone)
    signature = key.sign(_digest(unsigned), ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    raw = unsigned + _field(14, signature)
    if len(raw) > MAX_PRODUCTION_RAW_BYTES:
        raise ProductionSignatureError("production packet exceeds size limit")
    return raw


def verify_production_packet(raw: bytes, trust: ProductionTrust) -> BusPacket:
    _validate_trust_authority(trust)
    unsigned, signature = extract_signature(raw)
    packet = BusPacket()
    try:
        packet.ParseFromString(unsigned)
    except DecodeError as exc:
        raise ProductionSignatureError("malformed production protobuf") from exc
    _reject_unknown_nested_fields(packet)
    _validate_metadata(packet, trust.audience, trust)
    if packet.identity.tenant_id != trust.tenant:
        raise ProductionSignatureError("tenant mismatch")
    if packet.sender_id != trust.sender:
        raise ProductionSignatureError("sender mismatch")
    key = trust.public_keys.get(packet.signature_metadata.key_id)
    if key is None:
        raise ProductionSignatureError("unknown key id")
    _require_p256_key(key)
    try:
        key.verify(signature, _digest(unsigned), ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    except Exception as exc:
        raise ProductionSignatureError("invalid signature") from exc
    packet.signature = signature
    return packet


def extract_signature(raw: bytes) -> tuple[bytes, bytes]:
    if len(raw) > MAX_PRODUCTION_RAW_BYTES:
        raise ProductionSignatureError("production packet exceeds size limit")
    unsigned = bytearray()
    signature: bytes | None = None
    seen: set[int] = set()
    payload_field: int | None = None
    offset = 0
    while offset < len(raw):
        start = offset
        tag, offset = _varint(raw, offset)
        number, wire_type = tag >> 3, tag & 7
        payload_field = _validate_bus_packet_field(
            number, wire_type, seen, payload_field
        )
        if number == 4:
            protocol_version, value_end = _varint(raw, offset)
            if protocol_version != 1 or raw[offset:value_end] != b"\x01":
                raise ProductionSignatureError("invalid protocol version wire")
        else:
            value_end = _skip_value(raw, offset, wire_type)
        if number == 14:
            length, data_start = _varint(raw, offset)
            if data_start + length != value_end or length == 0:
                raise ProductionSignatureError("malformed signature")
            signature = raw[data_start:value_end]
        else:
            unsigned.extend(raw[start:value_end])
        offset = value_end
    if 4 not in seen:
        raise ProductionSignatureError("invalid protocol version wire")
    if signature is None:
        raise ProductionSignatureError("missing signature")
    return bytes(unsigned), signature


def _validate_trust_authority(trust: ProductionTrust) -> None:
    authorities = (trust.audience, trust.tenant, trust.sender)
    if any(
        not isinstance(value, str)
        or not value.strip()
        or value != value.strip()
        for value in authorities
    ):
        raise ProductionSignatureError("production trust authority required")


def _validate_unsigned_bus_packet(unsigned: bytes) -> None:
    """Apply the verifier's exact-wire rules before emitting a signature."""
    extract_signature(unsigned + _field(14, b"\x01"))


def _reject_unknown_nested_fields(packet: BusPacket) -> None:
    probe = BusPacket()
    probe.CopyFrom(packet)
    with_unknown = probe.SerializeToString(deterministic=True)
    probe.DiscardUnknownFields()
    without_unknown = probe.SerializeToString(deterministic=True)
    if with_unknown != without_unknown:
        raise ProductionSignatureError("unknown nested protobuf field")


def _validate_bus_packet_field(
    number: int,
    wire_type: int,
    seen: set[int],
    payload_field: int | None,
) -> int | None:
    expected = _BUS_PACKET_WIRE_TYPES.get(number)
    if expected is None:
        raise ProductionSignatureError("unknown BusPacket field")
    if wire_type != expected:
        raise ProductionSignatureError("wrong BusPacket field wire type")
    if number in seen:
        raise ProductionSignatureError("duplicate singular BusPacket field")
    seen.add(number)
    if number not in _BUS_PACKET_PAYLOAD_FIELDS:
        return payload_field
    if payload_field is not None:
        raise ProductionSignatureError("multiple BusPacket payload fields")
    return number


def _validate_metadata(
    packet: BusPacket,
    audience: str = "",
    trust: ProductionTrust | None = None,
) -> None:
    if not packet.HasField("signature_metadata"):
        raise ProductionSignatureError("missing signature metadata")
    metadata = packet.signature_metadata
    if metadata.profile_version != PROFILE_VERSION or metadata.algorithm != ALGORITHM:
        raise ProductionSignatureError("unsupported production signature profile")
    if len(metadata.message_id) != 16 or not metadata.audience or not metadata.key_id:
        raise ProductionSignatureError("invalid signature metadata")
    if audience and metadata.audience != audience:
        raise ProductionSignatureError("audience mismatch")
    expiry, now, skew, max_lifetime = _validated_time_bounds(metadata, trust)
    try:
        if expiry <= now - skew:
            raise ProductionSignatureError("signature expired")
        if max_lifetime is not None and expiry > now + max_lifetime + skew:
            raise ProductionSignatureError("signature lifetime exceeds bound")
    except (OverflowError, TypeError) as exc:
        raise ProductionSignatureError("invalid production lifetime bounds") from exc


def _validated_time_bounds(
    metadata: SignatureMetadata,
    trust: ProductionTrust | None,
) -> tuple[datetime, datetime, timedelta, timedelta | None]:
    try:
        expiry = metadata.expires_at.ToDatetime(tzinfo=timezone.utc)
    except (OverflowError, ValueError) as exc:
        raise ProductionSignatureError("invalid signature expiry") from exc
    if trust is None:
        return (
            expiry,
            datetime.now(timezone.utc),
            timedelta(0),
            DEFAULT_MAX_LIFETIME,
        )
    try:
        now = trust.now()
    except Exception as exc:
        raise ProductionSignatureError("invalid production lifetime bounds") from exc
    skew, max_lifetime = trust.clock_skew, trust.max_lifetime
    if (
        not isinstance(now, datetime)
        or not isinstance(skew, timedelta)
        or not isinstance(max_lifetime, timedelta)
        or skew < timedelta(0)
        or max_lifetime <= timedelta(0)
        or max_lifetime > DEFAULT_MAX_LIFETIME
        or skew > MAX_CLOCK_SKEW
        or skew > max_lifetime
    ):
        raise ProductionSignatureError("invalid production lifetime bounds")
    try:
        if now.tzinfo is None or now.utcoffset() is None:
            raise ProductionSignatureError("invalid production lifetime bounds")
    except ProductionSignatureError:
        raise
    except Exception as exc:
        raise ProductionSignatureError("invalid production lifetime bounds") from exc
    return expiry, now, skew, max_lifetime


def _digest(unsigned: bytes) -> bytes:
    return hashlib.sha256(DOMAIN + unsigned).digest()


def _require_p256_key(
    key: ec.EllipticCurvePrivateKey | ec.EllipticCurvePublicKey,
) -> None:
    if not isinstance(key.curve, ec.SECP256R1):
        raise ProductionSignatureError(
            "production signatures require ECDSA P-256"
        )


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
