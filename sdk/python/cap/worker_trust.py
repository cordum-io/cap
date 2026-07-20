"""Authenticated worker trust-handshake client API and immutable models."""

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from enum import Enum
from types import MappingProxyType
from typing import Mapping

from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2


WORKER_HANDSHAKE_AUDIENCE = "cordum-scheduler"
WORKER_HANDSHAKE_PROTOCOL_VERSION = 1
WORKER_HANDSHAKE_NONCE_SIZE = 32
WORKER_HANDSHAKE_MAX_PACKET_SIZE = 64 * 1024
WORKER_HANDSHAKE_MAX_IDENTITY_LENGTH = 256
WORKER_HANDSHAKE_MAX_SESSION_TOKEN_SIZE = 16 * 1024
WORKER_HANDSHAKE_MAX_SKEW = timedelta(minutes=1)
WORKER_HANDSHAKE_MAX_LIFETIME = timedelta(minutes=1)


class WorkerTrustMode(str, Enum):
    """Admission policy after an authenticated handshake failure."""

    OFF = "off"
    WARN = "warn"
    ENFORCE = "enforce"


class WorkerTrustError(ValueError):
    """Base class for worker-trust client failures."""


class WorkerTrustModeError(WorkerTrustError):
    """Raised for an unknown or implicit trust mode."""


class WorkerTrustConfigError(WorkerTrustError):
    """Raised for partial or unsafe trust configuration."""


class WorkerHandshakePacketError(WorkerTrustError):
    """Raised for a malformed trust packet."""


class WorkerHandshakeBindingError(WorkerTrustError):
    """Raised when authenticated correlation differs."""


class WorkerHandshakeExpiredError(WorkerTrustError):
    """Raised when a signed packet falls outside its validity window."""


class WorkerHandshakeRejectionError(WorkerTrustError):
    """Opaque scheduler rejection carrying only the protocol reason enum."""

    def __init__(self, reason: int) -> None:
        super().__init__("worker handshake rejected")
        self.reason = reason


@dataclass(frozen=True)
class WorkerTrustConfig:
    worker_id: str
    expected_agent_id: str
    tenant_id: str
    audience: str
    proof_key_id: str
    proof_private_key: ec.EllipticCurvePrivateKey
    expected_scheduler_id: str
    scheduler_public_keys: Mapping[str, ec.EllipticCurvePublicKey]
    sdk_version: str

    def __post_init__(self) -> None:
        source = self.scheduler_public_keys
        copied = MappingProxyType(dict(source)) if isinstance(source, Mapping) else MappingProxyType({})
        object.__setattr__(self, "scheduler_public_keys", copied)


@dataclass(frozen=True)
class WorkerHandshakeRequestOptions:
    request_id: str
    trace_id: str
    purpose: int
    client_nonce: bytes
    created_at: datetime


@dataclass(frozen=True)
class WorkerHandshakeSession:
    token: str
    issued_at: datetime
    expires_at: datetime


@dataclass(frozen=True, init=False)
class VerifiedWorkerHandshakeChallenge:
    """Opaque scheduler-authenticated challenge with defensive-copy access."""

    _request_bytes: bytes
    _response_bytes: bytes

    @classmethod
    def _create(cls, request, response):
        value = object.__new__(cls)
        object.__setattr__(value, "_request_bytes", _serialize(request))
        object.__setattr__(value, "_response_bytes", _serialize(response))
        return value

    def message(self) -> handshake_pb2.WorkerHandshakeChallenge:
        return _verified_packets(self)[1].worker_handshake_challenge


def parse_worker_trust_mode(raw: str) -> WorkerTrustMode:
    """Parse only explicit off, warn, or enforce values."""

    normalized = raw.strip().lower() if isinstance(raw, str) else ""
    try:
        return WorkerTrustMode(normalized)
    except ValueError as exc:
        raise WorkerTrustModeError("invalid worker trust mode: {!r}".format(raw)) from exc


def validate_worker_trust_config(config: WorkerTrustConfig) -> None:
    """Reject partial identities and unpinned/non-P256 trust roots."""

    if not isinstance(config, WorkerTrustConfig):
        raise WorkerTrustConfigError("worker trust configuration is required")
    values = (
        config.worker_id, config.expected_agent_id, config.tenant_id,
        config.audience, config.proof_key_id, config.expected_scheduler_id,
        config.sdk_version,
    )
    if not all(_canonical_text(value) for value in values):
        raise WorkerTrustConfigError("worker trust identity is invalid")
    if config.audience != WORKER_HANDSHAKE_AUDIENCE:
        raise WorkerTrustConfigError("worker trust audience is invalid")
    _require_p256_private(config.proof_private_key)
    if not config.scheduler_public_keys:
        raise WorkerTrustConfigError("scheduler trust keys are required")
    for key_id, key in config.scheduler_public_keys.items():
        if not _canonical_text(key_id):
            raise WorkerTrustConfigError("scheduler key ID is invalid")
        _require_p256_public(key)


def _verified_packets(verified):
    if not isinstance(verified, VerifiedWorkerHandshakeChallenge):
        raise WorkerHandshakeBindingError("challenge is unverified")
    request = buspacket_pb2.BusPacket.FromString(verified._request_bytes)
    response = buspacket_pb2.BusPacket.FromString(verified._response_bytes)
    return request, response


def _timestamp_datetime(value, field):
    try:
        value.ToDatetime()
        return value.ToDatetime(tzinfo=timezone.utc)
    except (AttributeError, ValueError, OverflowError) as exc:
        raise WorkerHandshakePacketError("{} is invalid".format(field)) from exc


def _utc(value):
    if not isinstance(value, datetime) or value.tzinfo is None:
        raise WorkerHandshakePacketError("timestamp must be timezone-aware")
    return value.astimezone(timezone.utc)


def _canonical_text(value):
    return (isinstance(value, str) and
            0 < len(value) <= WORKER_HANDSHAKE_MAX_IDENTITY_LENGTH and
            all(0x21 <= ord(char) <= 0x7e for char in value))


def _valid_session_token(value):
    return (isinstance(value, str) and
            0 < len(value) <= WORKER_HANDSHAKE_MAX_SESSION_TOKEN_SIZE and
            all(0x21 <= ord(char) <= 0x7e for char in value))


def _require_p256_private(key):
    if (not isinstance(key, ec.EllipticCurvePrivateKey) or
            not isinstance(key.curve, ec.SECP256R1)):
        raise WorkerTrustConfigError("worker proof key must use P-256")


def _require_p256_public(key):
    if (not isinstance(key, ec.EllipticCurvePublicKey) or
            not isinstance(key.curve, ec.SECP256R1)):
        raise WorkerTrustConfigError("scheduler trust key must use P-256")


def _serialize(packet):
    return packet.SerializeToString(deterministic=True)


def _p256_algorithm():
    return handshake_pb2.WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256


def _issue_purpose():
    return handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE


def _renew_purpose():
    return handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW


from cap.worker_trust_build import build_authenticate, build_challenge_request  # noqa: E402
from cap.worker_trust_verify import verify_challenge, verify_result  # noqa: E402


__all__ = [
    "WORKER_HANDSHAKE_AUDIENCE", "WorkerHandshakeBindingError",
    "WorkerHandshakeExpiredError", "WorkerHandshakePacketError",
    "WorkerHandshakeRejectionError", "WorkerHandshakeRequestOptions",
    "WorkerHandshakeSession", "WorkerTrustConfig", "WorkerTrustConfigError",
    "WorkerTrustError", "WorkerTrustMode", "WorkerTrustModeError",
    "build_authenticate", "build_challenge_request", "parse_worker_trust_mode",
    "validate_worker_trust_config", "verify_challenge", "verify_result",
]
