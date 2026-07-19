"""Verification and correlation for worker trust-handshake responses."""

from cap.trust_signing import TrustSigningError, verify_trust_packet
from cap.worker_trust import (
    WORKER_HANDSHAKE_MAX_SKEW,
    VerifiedWorkerHandshakeChallenge,
    WorkerHandshakeBindingError,
    WorkerHandshakeExpiredError,
    WorkerHandshakePacketError,
    WorkerHandshakeRejectionError,
    WorkerHandshakeSession,
    _renew_purpose,
    _serialize,
    _timestamp_datetime,
    _utc,
    _verified_packets,
    validate_worker_trust_config,
)
from cap.worker_trust_validate import validate_worker_trust_packet


def verify_challenge(config, request, response, now):
    """Authenticate and correlate one scheduler challenge response."""

    validate_worker_trust_config(config)
    validate_worker_trust_packet(request)
    try:
        verify_trust_packet(
            request,
            {config.proof_key_id: config.proof_private_key.public_key()},
        )
    except TrustSigningError as exc:
        raise WorkerHandshakePacketError("request signature is invalid") from exc
    _validate_skew(request.created_at, now, "request.created_at")
    _verify_scheduler_packet(config, response, now)
    _correlate_challenge(config, request, response)
    _validate_challenge_freshness(response.worker_handshake_challenge, now)
    return VerifiedWorkerHandshakeChallenge._create(request, response)


def verify_result(config, verified, authenticate, response, now):
    """Authenticate a correlated result and return only a live session."""

    validate_worker_trust_config(config)
    _verified_packets(verified)
    validate_worker_trust_packet(authenticate)
    try:
        verify_trust_packet(
            authenticate,
            {config.proof_key_id: config.proof_private_key.public_key()},
        )
    except TrustSigningError as exc:
        raise WorkerHandshakePacketError("authenticate signature is invalid") from exc
    _verify_scheduler_packet(config, response, now)
    _correlate_result(verified, authenticate, response)
    result = response.worker_handshake_result
    _validate_challenge_freshness(result.challenge, now)
    _validate_skew(result.issued_at, now, "result.issued_at")
    if not result.accepted:
        raise WorkerHandshakeRejectionError(result.rejection_reason)
    expires_at = _timestamp_datetime(result.token_expires_at, "token_expires_at")
    if expires_at <= _utc(now):
        raise WorkerHandshakeExpiredError("result token is already expired")
    if result.challenge.purpose == _renew_purpose() and response.auth_token == authenticate.auth_token:
        raise WorkerHandshakeBindingError("renewal did not rotate session token")
    issued_at = _timestamp_datetime(result.issued_at, "issued_at")
    return WorkerHandshakeSession(response.auth_token, issued_at, expires_at)


def _verify_scheduler_packet(config, packet, now):
    validate_worker_trust_packet(packet)
    if packet.sender_id != config.expected_scheduler_id:
        raise WorkerHandshakeBindingError("scheduler sender differs")
    _validate_skew(packet.created_at, now, "created_at")
    try:
        verify_trust_packet(packet, config.scheduler_public_keys)
    except TrustSigningError as exc:
        raise WorkerHandshakePacketError("scheduler signature is invalid") from exc


def _correlate_challenge(config, request_packet, response_packet):
    request = request_packet.worker_handshake_challenge_request
    challenge = response_packet.worker_handshake_challenge
    fields = (
        "request_id", "trace_id", "worker_id", "proof_key_id",
        "proof_algorithm", "audience", "purpose", "client_nonce",
        "protocol_version", "sdk_version",
    )
    if any(getattr(challenge, field) != getattr(request, field) for field in fields):
        raise WorkerHandshakeBindingError("challenge does not match request")
    if (challenge.agent_id != config.expected_agent_id or
            challenge.tenant_id != config.tenant_id):
        raise WorkerHandshakeBindingError("challenge identity differs")


def _correlate_result(verified, authenticate, response):
    wanted = _serialize(_verified_packets(verified)[1].worker_handshake_challenge)
    got = _serialize(response.worker_handshake_result.challenge)
    sent = _serialize(authenticate.worker_handshake_authenticate.challenge)
    if got != wanted or sent != wanted:
        raise WorkerHandshakeBindingError("result challenge differs")


def _validate_challenge_freshness(challenge, now):
    _validate_skew(challenge.issued_at, now, "challenge.issued_at")
    expires_at = _timestamp_datetime(challenge.expires_at, "challenge.expires_at")
    if _utc(now) >= expires_at:
        raise WorkerHandshakeExpiredError("challenge expired")


def _validate_skew(timestamp, now, field):
    delta = _timestamp_datetime(timestamp, field) - _utc(now)
    if delta > WORKER_HANDSHAKE_MAX_SKEW or delta < -WORKER_HANDSHAKE_MAX_SKEW:
        raise WorkerHandshakeExpiredError("{} outside allowed skew".format(field))


__all__ = ["verify_challenge", "verify_result"]
