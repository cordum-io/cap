"""Builders for signed worker trust-handshake requests."""

from datetime import datetime
from typing import Optional

from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
from cap.trust_signing import sign_trust_packet
from cap.worker_trust import (
    WORKER_HANDSHAKE_NONCE_SIZE,
    WORKER_HANDSHAKE_PROTOCOL_VERSION,
    WorkerHandshakeBindingError,
    WorkerHandshakePacketError,
    WorkerHandshakeRequestOptions,
    VerifiedWorkerHandshakeChallenge,
    WorkerTrustConfig,
    _canonical_text,
    _issue_purpose,
    _p256_algorithm,
    _renew_purpose,
    _utc,
    _valid_session_token,
    _verified_packets,
    validate_worker_trust_config,
)
from cap.worker_trust_validate import (
    validate_worker_capability,
    validate_worker_trust_packet,
)


def build_challenge_request(
    config: WorkerTrustConfig,
    options: WorkerHandshakeRequestOptions,
) -> buspacket_pb2.BusPacket:
    """Build, sign, and structurally validate a challenge request."""

    validate_worker_trust_config(config)
    _validate_options(options)
    request = handshake_pb2.WorkerHandshakeChallengeRequest(
        request_id=options.request_id, trace_id=options.trace_id,
        worker_id=config.worker_id, proof_key_id=config.proof_key_id,
        proof_algorithm=_p256_algorithm(), audience=config.audience,
        purpose=options.purpose, client_nonce=bytes(options.client_nonce),
        protocol_version=WORKER_HANDSHAKE_PROTOCOL_VERSION,
        sdk_version=config.sdk_version,
    )
    packet = _new_packet(options.trace_id, config.worker_id, options.created_at)
    packet.worker_handshake_challenge_request.CopyFrom(request)
    sign_trust_packet(packet, config.proof_private_key)
    validate_worker_trust_packet(packet)
    return packet


def build_authenticate(
    config: WorkerTrustConfig,
    verified: VerifiedWorkerHandshakeChallenge,
    capability: handshake_pb2.Handshake,
    current_session_token: str,
    created_at: datetime,
) -> buspacket_pb2.BusPacket:
    """Sign a verified challenge and exact worker capability declaration."""

    validate_worker_trust_config(config)
    challenge = _verified_packets(verified)[1].worker_handshake_challenge
    _bind_challenge_config(config, challenge)
    validate_worker_capability(capability, challenge)
    _validate_session_for_purpose(challenge.purpose, current_session_token)
    packet = _new_packet(challenge.trace_id, config.worker_id, created_at)
    packet.auth_token = current_session_token
    packet.worker_handshake_authenticate.challenge.CopyFrom(challenge)
    packet.worker_handshake_authenticate.capability_handshake.CopyFrom(capability)
    sign_trust_packet(packet, config.proof_private_key)
    validate_worker_trust_packet(packet)
    return packet


def _bind_challenge_config(
    config: WorkerTrustConfig,
    challenge: handshake_pb2.WorkerHandshakeChallenge,
) -> None:
    values = (
        challenge.worker_id == config.worker_id,
        challenge.agent_id == config.expected_agent_id,
        challenge.tenant_id == config.tenant_id,
        challenge.proof_key_id == config.proof_key_id,
        challenge.audience == config.audience,
        challenge.sdk_version == config.sdk_version,
    )
    if not all(values):
        raise WorkerHandshakeBindingError("challenge identity changed")


def _validate_options(options: WorkerHandshakeRequestOptions) -> None:
    if not isinstance(options, WorkerHandshakeRequestOptions):
        raise WorkerHandshakePacketError("request options are required")
    if not _canonical_text(options.request_id) or not _canonical_text(options.trace_id):
        raise WorkerHandshakePacketError("request and trace IDs are invalid")
    if options.purpose not in (_issue_purpose(), _renew_purpose()):
        raise WorkerHandshakePacketError("handshake purpose is unsupported")
    if len(options.client_nonce) != WORKER_HANDSHAKE_NONCE_SIZE:
        raise WorkerHandshakePacketError("client nonce must be 32 bytes")
    _utc(options.created_at)


def _validate_session_for_purpose(purpose: int, token: Optional[str]) -> None:
    if purpose == _issue_purpose() and token:
        raise WorkerHandshakePacketError("issue must not carry a session")
    if purpose == _renew_purpose() and not _valid_session_token(token):
        raise WorkerHandshakePacketError("renew requires the current session")


def _new_packet(
    trace_id: str,
    sender_id: str,
    created_at: datetime,
) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id, sender_id=sender_id,
        protocol_version=WORKER_HANDSHAKE_PROTOCOL_VERSION,
    )
    try:
        packet.created_at.FromDatetime(_utc(created_at))
    except (ValueError, OverflowError) as exc:
        raise WorkerHandshakePacketError("created_at is invalid") from exc
    return packet


__all__ = ["build_authenticate", "build_challenge_request"]
