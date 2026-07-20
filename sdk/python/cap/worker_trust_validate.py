"""Structural validation for worker trust-handshake protobuf packets."""

import re
from datetime import timezone

from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
from cap.worker_trust import (
    WORKER_HANDSHAKE_MAX_LIFETIME,
    WORKER_HANDSHAKE_MAX_SESSION_TOKEN_SIZE,
    WORKER_HANDSHAKE_NONCE_SIZE,
    WORKER_HANDSHAKE_PROTOCOL_VERSION,
    WorkerHandshakePacketError,
)


_CAPABILITY_KEY = re.compile(r"^[a-z][a-z0-9_.-]{0,63}$")
_REJECTIONS = {
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_INVALID_REQUEST,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_REPLAY_DETECTED,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_CLOCK_SKEW,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_CHALLENGE_EXPIRED,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_SESSION_REQUIRED,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_SESSION_INVALID,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_UNSUPPORTED_VERSION,
    handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_INTERNAL_ERROR,
}
_TRUST_PHASES = {
    "worker_handshake_challenge_request",
    "worker_handshake_challenge",
    "worker_handshake_authenticate",
    "worker_handshake_result",
}


def validate_worker_trust_packet(packet: buspacket_pb2.BusPacket) -> None:
    """Enforce exact-v1 structural semantics before cryptographic use."""

    if not isinstance(packet, buspacket_pb2.BusPacket):
        _fail("BusPacket", "must not be nil")
    if packet.WhichOneof("payload") not in _TRUST_PHASES:
        _fail("payload", "must be exactly one worker trust phase")
    _validate_envelope(packet)
    if _has_unknown_fields(packet):
        _fail("unknown_fields", "must be empty recursively")
    phase = packet.WhichOneof("payload")
    if phase == "worker_handshake_challenge_request":
        _validate_request(packet)
    elif phase == "worker_handshake_challenge":
        _validate_challenge(packet, packet.worker_handshake_challenge, True)
    elif phase == "worker_handshake_authenticate":
        _validate_authenticate(packet)
    else:
        _validate_result(packet)


def validate_worker_capability(capability, challenge) -> None:
    """Bind capability identity, version, feature keys, and ready topics."""

    if not isinstance(capability, handshake_pb2.Handshake):
        _fail("capability_handshake", "must not be nil")
    if (capability.component_id != challenge.worker_id or
            capability.role != handshake_pb2.COMPONENT_ROLE_WORKER or
            capability.sdk_version != challenge.sdk_version):
        _fail("capability_handshake", "identity does not match challenge")
    if list(capability.supported_versions) != [WORKER_HANDSHAKE_PROTOCOL_VERSION]:
        _fail("capability_handshake.supported_versions", "must contain exactly v1")
    for key, enabled in capability.capabilities.items():
        if not enabled or _CAPABILITY_KEY.fullmatch(key) is None:
            _fail("capability_handshake.capabilities", "key is invalid or disabled")
    topics = list(capability.ready_topics)
    if any(not topic.strip() for topic in topics) or len(set(topics)) != len(topics):
        _fail("capability_handshake.ready_topics", "topic is empty or duplicated")


def _validate_envelope(packet) -> None:
    if not packet.trace_id:
        _fail("trace_id", "must not be empty")
    if not packet.sender_id:
        _fail("sender_id", "must not be empty")
    if packet.protocol_version != WORKER_HANDSHAKE_PROTOCOL_VERSION:
        _fail("protocol_version", "must equal 1")
    if not packet.HasField("created_at"):
        _fail("created_at", "must not be nil")
    _datetime(packet.created_at, "created_at")


def _validate_request(packet) -> None:
    request = packet.worker_handshake_challenge_request
    required = (request.request_id, request.trace_id, request.worker_id,
                request.proof_key_id, request.audience, request.sdk_version)
    if _missing(required):
        _fail("challenge_request", "required field is empty")
    if packet.trace_id != request.trace_id or packet.sender_id != request.worker_id:
        _fail("challenge_request", "trace or worker does not match envelope")
    if request.protocol_version != 1 or request.purpose not in _purposes():
        _fail("challenge_request", "version or purpose is unsupported")
    if request.proof_algorithm != _p256() or len(request.client_nonce) != 32:
        _fail("challenge_request", "proof algorithm or nonce is invalid")
    if packet.auth_token or not packet.signature:
        _fail("challenge_request", "must be signed without a session token")


def _validate_challenge(packet, challenge, top_level) -> None:
    required = (
        challenge.request_id, challenge.challenge_id, challenge.trace_id,
        challenge.worker_id, challenge.agent_id, challenge.tenant_id,
        challenge.proof_key_id, challenge.server_key_id, challenge.audience,
        challenge.sdk_version,
    )
    if _missing(required):
        _fail("challenge", "required field is empty")
    if packet.trace_id != challenge.trace_id:
        _fail("challenge.trace_id", "must equal envelope trace_id")
    if top_level and packet.auth_token:
        _fail("auth_token", "challenge must not carry a session")
    if challenge.protocol_version != 1 or challenge.purpose not in _purposes():
        _fail("challenge", "version or purpose is unsupported")
    if (challenge.proof_algorithm != _p256() or
            len(challenge.client_nonce) != WORKER_HANDSHAKE_NONCE_SIZE or
            len(challenge.server_nonce) != WORKER_HANDSHAKE_NONCE_SIZE):
        _fail("challenge", "proof algorithm or nonce is invalid")
    issued = _datetime(challenge.issued_at, "challenge.issued_at")
    expires = _datetime(challenge.expires_at, "challenge.expires_at")
    if expires <= issued or expires - issued > WORKER_HANDSHAKE_MAX_LIFETIME:
        _fail("challenge.timestamps", "invalid order or lifetime")
    if top_level and not packet.signature:
        _fail("signature", "challenge must be signed")


def _validate_authenticate(packet) -> None:
    authenticate = packet.worker_handshake_authenticate
    if not authenticate.HasField("challenge") or not authenticate.HasField("capability_handshake"):
        _fail("authenticate", "challenge and capability are required")
    challenge = authenticate.challenge
    _validate_challenge(packet, challenge, False)
    if packet.sender_id != challenge.worker_id or not packet.signature:
        _fail("authenticate", "worker binding or signature is invalid")
    validate_worker_capability(authenticate.capability_handshake, challenge)
    _validate_session(challenge.purpose, packet.auth_token)


def _validate_result(packet) -> None:
    result = packet.worker_handshake_result
    if not result.HasField("challenge") or not packet.signature:
        _fail("result", "challenge and signature are required")
    _validate_challenge(packet, result.challenge, False)
    issued = _datetime(result.issued_at, "result.issued_at")
    if result.accepted:
        _validate_accepted(result, packet.auth_token, issued)
        return
    if packet.auth_token or result.rejection_reason not in _REJECTIONS:
        _fail("result", "rejection must not carry token metadata")
    if result.HasField("token_expires_at"):
        _fail("result", "rejection must not carry token metadata")


def _validate_accepted(result, token, issued) -> None:
    if not _valid_token(token) or not result.HasField("token_expires_at"):
        _fail("result", "accepted result requires a future token expiry")
    expires = _datetime(result.token_expires_at, "result.token_expires_at")
    if expires <= issued:
        _fail("result", "accepted result requires a future token expiry")
    if result.rejection_reason != handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED:
        _fail("result.rejection_reason", "must be unspecified when accepted")


def _validate_session(purpose, token) -> None:
    if purpose == handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE and token:
        _fail("auth_token", "issue must not carry a session")
    if purpose == handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW and not _valid_token(token):
        _fail("auth_token", "renew requires the current session")


def _has_unknown_fields(message) -> bool:
    original = message.SerializeToString(deterministic=True)
    clean = type(message)()
    clean.CopyFrom(message)
    clean.DiscardUnknownFields()
    return original != clean.SerializeToString(deterministic=True)


def _datetime(timestamp, field):
    try:
        timestamp.ToDatetime()
        return timestamp.ToDatetime(tzinfo=timezone.utc)
    except (AttributeError, ValueError, OverflowError) as exc:
        raise WorkerHandshakePacketError("{}: invalid timestamp".format(field)) from exc


def _valid_token(value) -> bool:
    return (isinstance(value, str) and 0 < len(value) <= WORKER_HANDSHAKE_MAX_SESSION_TOKEN_SIZE
            and all(0x21 <= ord(char) <= 0x7e for char in value))


def _missing(values) -> bool:
    return any(not isinstance(value, str) or not value.strip() for value in values)


def _purposes():
    return (handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE,
            handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW)


def _p256():
    return handshake_pb2.WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256


def _fail(field, message):
    raise WorkerHandshakePacketError("{}: {}".format(field, message))


__all__ = ["validate_worker_capability", "validate_worker_trust_packet"]
