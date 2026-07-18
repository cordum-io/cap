"""Domain-separated signing helpers for CAP worker trust packets."""

import hashlib
from typing import Mapping, Tuple

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils

from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2


_DOMAINS = {
    "worker_handshake_challenge_request": b"CAP-WORKER-HANDSHAKE-CHALLENGE-REQUEST-V1",
    "worker_handshake_challenge": b"CAP-WORKER-HANDSHAKE-CHALLENGE-V1",
    "worker_handshake_authenticate": b"CAP-WORKER-HANDSHAKE-AUTHENTICATE-V1",
    "worker_handshake_result": b"CAP-WORKER-HANDSHAKE-RESULT-V1",
}


class TrustSigningError(ValueError):
    """Raised when a trust packet cannot be signed or verified safely."""


def _phase(packet: buspacket_pb2.BusPacket) -> Tuple[str, bytes]:
    payload = packet.WhichOneof("payload")
    if payload not in _DOMAINS:
        raise TrustSigningError("packet is not a worker trust-handshake phase")
    return payload, _DOMAINS[payload]


def _key_id(packet: buspacket_pb2.BusPacket, payload: str) -> str:
    if payload == "worker_handshake_challenge_request":
        trust_payload = packet.worker_handshake_challenge_request
        key_id = trust_payload.proof_key_id
    elif payload == "worker_handshake_authenticate":
        trust_payload = packet.worker_handshake_authenticate.challenge
        key_id = trust_payload.proof_key_id
    elif payload == "worker_handshake_challenge":
        trust_payload = packet.worker_handshake_challenge
        key_id = trust_payload.server_key_id
    else:
        trust_payload = packet.worker_handshake_result.challenge
        key_id = trust_payload.server_key_id
    expected = handshake_pb2.WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256
    if trust_payload.proof_algorithm != expected:
        raise TrustSigningError("trust packet proof algorithm is not P-256 SHA-256")
    if not key_id:
        raise TrustSigningError("trust packet key ID is missing")
    return key_id


def _require_p256_private(key: ec.EllipticCurvePrivateKey) -> None:
    if not isinstance(key, ec.EllipticCurvePrivateKey):
        raise TrustSigningError("trust signing requires an EC private key")
    if not isinstance(key.curve, ec.SECP256R1):
        raise TrustSigningError("trust signing requires a P-256 private key")


def _require_p256_public(key: ec.EllipticCurvePublicKey) -> None:
    if not isinstance(key, ec.EllipticCurvePublicKey):
        raise TrustSigningError("trusted key must be an EC public key")
    if not isinstance(key.curve, ec.SECP256R1):
        raise TrustSigningError("trusted key must use P-256")


def _require_strict_der(signature: bytes) -> None:
    try:
        r_value, s_value = utils.decode_dss_signature(signature)
    except ValueError as exc:
        raise TrustSigningError("trust signature is not strict DER") from exc
    if r_value <= 0 or s_value <= 0:
        raise TrustSigningError("trust signature has invalid integers")
    if utils.encode_dss_signature(r_value, s_value) != signature:
        raise TrustSigningError("trust signature is not canonical DER")


def trust_packet_digest(packet: buspacket_pb2.BusPacket) -> bytes:
    """Return SHA-256(domain || NUL || deterministic unsigned packet)."""

    _, domain = _phase(packet)
    unsigned = buspacket_pb2.BusPacket()
    unsigned.CopyFrom(packet)
    unsigned.ClearField("signature")
    encoded = unsigned.SerializeToString(deterministic=True)
    return hashlib.sha256(domain + b"\x00" + encoded).digest()


def sign_trust_packet(
    packet: buspacket_pb2.BusPacket,
    private_key: ec.EllipticCurvePrivateKey,
) -> bytes:
    """Sign a trust packet with P-256 ECDSA over its precomputed digest."""

    payload, _ = _phase(packet)
    _key_id(packet, payload)
    _require_p256_private(private_key)
    signature = private_key.sign(
        trust_packet_digest(packet),
        ec.ECDSA(utils.Prehashed(hashes.SHA256())),
    )
    _require_strict_der(signature)
    packet.signature = signature
    return signature


def verify_trust_packet(
    packet: buspacket_pb2.BusPacket,
    trusted_keys: Mapping[str, ec.EllipticCurvePublicKey],
) -> None:
    """Verify using only the configured key selected by the signed payload."""

    payload, _ = _phase(packet)
    if not packet.signature:
        raise TrustSigningError("trust signature is missing")
    key_id = _key_id(packet, payload)
    public_key = trusted_keys.get(key_id)
    if public_key is None:
        raise TrustSigningError("trust packet key ID is not configured")
    _require_p256_public(public_key)
    _require_strict_der(packet.signature)
    try:
        public_key.verify(
            packet.signature,
            trust_packet_digest(packet),
            ec.ECDSA(utils.Prehashed(hashes.SHA256())),
        )
    except (InvalidSignature, ValueError) as exc:
        raise TrustSigningError("trust signature is invalid") from exc


__all__ = [
    "TrustSigningError",
    "sign_trust_packet",
    "trust_packet_digest",
    "verify_trust_packet",
]
