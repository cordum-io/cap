"""Mutation-resistant tests for the Python worker-trust client core."""

from dataclasses import replace
from datetime import datetime, timedelta, timezone

import pytest
from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
from cap.trust_signing import sign_trust_packet
from cap.worker_trust import (
    WORKER_HANDSHAKE_AUDIENCE,
    WorkerHandshakeBindingError,
    WorkerHandshakeExpiredError,
    WorkerHandshakePacketError,
    WorkerHandshakeRejectionError,
    WorkerHandshakeRequestOptions,
    WorkerTrustConfig,
    WorkerTrustConfigError,
    WorkerTrustMode,
    WorkerTrustModeError,
    build_authenticate,
    build_challenge_request,
    parse_worker_trust_mode,
    verify_challenge,
    verify_result,
)


NOW = datetime(2026, 7, 18, 12, tzinfo=timezone.utc)
ISSUE = handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE
RENEW = handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW


def _timestamp(value: datetime) -> timestamp_pb2.Timestamp:
    result = timestamp_pb2.Timestamp()
    result.FromDatetime(value)
    return result


class TrustFixture:
    def __init__(self) -> None:
        self.worker_key = ec.generate_private_key(ec.SECP256R1())
        self.scheduler_key = ec.generate_private_key(ec.SECP256R1())
        self.config = WorkerTrustConfig(
            worker_id="worker-1",
            expected_agent_id="agent-1",
            tenant_id="tenant-1",
            audience=WORKER_HANDSHAKE_AUDIENCE,
            proof_key_id="worker-key-1",
            proof_private_key=self.worker_key,
            expected_scheduler_id="scheduler-1",
            scheduler_public_keys={"scheduler-key-1": self.scheduler_key.public_key()},
            sdk_version="v2.14.1",
        )

    def request(self, purpose: int = ISSUE) -> buspacket_pb2.BusPacket:
        return build_challenge_request(
            self.config,
            WorkerHandshakeRequestOptions(
                request_id="request-1",
                trace_id="trace-1",
                purpose=purpose,
                client_nonce=b"B" * 32,
                created_at=NOW,
            ),
        )

    def challenge(self, request: buspacket_pb2.BusPacket) -> buspacket_pb2.BusPacket:
        source = request.worker_handshake_challenge_request
        challenge = handshake_pb2.WorkerHandshakeChallenge(
            request_id=source.request_id,
            challenge_id="challenge-1",
            trace_id=source.trace_id,
            worker_id=source.worker_id,
            agent_id=self.config.expected_agent_id,
            tenant_id=self.config.tenant_id,
            proof_key_id=source.proof_key_id,
            proof_algorithm=source.proof_algorithm,
            server_key_id="scheduler-key-1",
            audience=source.audience,
            purpose=source.purpose,
            client_nonce=source.client_nonce,
            server_nonce=b"$" * 32,
            protocol_version=source.protocol_version,
            sdk_version=source.sdk_version,
            issued_at=_timestamp(NOW),
            expires_at=_timestamp(NOW + timedelta(seconds=30)),
        )
        packet = _packet("scheduler-1", challenge.trace_id)
        packet.worker_handshake_challenge.CopyFrom(challenge)
        sign_trust_packet(packet, self.scheduler_key)
        return packet

    def result(self, challenge, token: str, accepted: bool = True):
        reason = handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED
        if not accepted:
            reason = handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED
        result = handshake_pb2.WorkerHandshakeResult(
            challenge=challenge,
            accepted=accepted,
            rejection_reason=reason,
            issued_at=_timestamp(NOW),
        )
        if accepted:
            result.token_expires_at.CopyFrom(_timestamp(NOW + timedelta(hours=1)))
        packet = _packet("scheduler-1", challenge.trace_id)
        packet.auth_token = token
        packet.worker_handshake_result.CopyFrom(result)
        sign_trust_packet(packet, self.scheduler_key)
        return packet


def _packet(sender: str, trace: str) -> buspacket_pb2.BusPacket:
    return buspacket_pb2.BusPacket(
        sender_id=sender,
        trace_id=trace,
        protocol_version=1,
        created_at=_timestamp(NOW),
    )


def _capability(config: WorkerTrustConfig) -> handshake_pb2.Handshake:
    return handshake_pb2.Handshake(
        component_id=config.worker_id,
        role=handshake_pb2.COMPONENT_ROLE_WORKER,
        supported_versions=[1],
        capabilities={"progress": True},
        sdk_version=config.sdk_version,
        ready_topics=["job.allowed"],
    )


def test_mode_parser_requires_an_explicit_known_value() -> None:
    assert parse_worker_trust_mode(" WARN ") is WorkerTrustMode.WARN
    assert parse_worker_trust_mode("off") is WorkerTrustMode.OFF
    with pytest.raises(WorkerTrustModeError, match="optional"):
        parse_worker_trust_mode("optional")
    with pytest.raises(WorkerTrustModeError):
        parse_worker_trust_mode("")


@pytest.mark.parametrize(
    "field,value",
    [
        ("worker_id", " worker-1"),
        ("tenant_id", "tenant\nforged"),
        ("proof_key_id", "key\x00forged"),
        ("audience", "other-scheduler"),
        ("sdk_version", ""),
        ("scheduler_public_keys", {}),
        ("scheduler_public_keys", None),
    ],
)
def test_config_rejects_partial_or_unsafe_identity(field: str, value: object) -> None:
    fixture = TrustFixture()
    with pytest.raises(WorkerTrustConfigError):
        build_challenge_request(replace(fixture.config, **{field: value}), _options())


def _options(purpose: int = ISSUE) -> WorkerHandshakeRequestOptions:
    return WorkerHandshakeRequestOptions(
        "request-1", "trace-1", purpose, b"B" * 32, NOW
    )


def test_issue_lifecycle_installs_only_a_signed_correlated_result() -> None:
    fixture = TrustFixture()
    request = fixture.request()
    verified = verify_challenge(fixture.config, request, fixture.challenge(request), NOW)
    authenticate = build_authenticate(
        fixture.config, verified, _capability(fixture.config), "", NOW
    )
    session = verify_result(
        fixture.config,
        verified,
        authenticate,
        fixture.result(verified.message(), "session-token"),
        NOW,
    )
    assert session.token == "session-token"
    assert session.expires_at == NOW + timedelta(hours=1)
    assert authenticate.auth_token == ""


@pytest.mark.parametrize(
    "field,value",
    [
        ("request_id", "other"),
        ("agent_id", "attacker"),
        ("tenant_id", "attacker"),
        ("audience", "other"),
        ("client_nonce", b"X" * 32),
    ],
)
def test_challenge_rejects_each_signed_correlation_mutation(field: str, value: object) -> None:
    fixture = TrustFixture()
    request = fixture.request()
    response = fixture.challenge(request)
    setattr(response.worker_handshake_challenge, field, value)
    response.ClearField("signature")
    sign_trust_packet(response, fixture.scheduler_key)
    with pytest.raises(WorkerHandshakeBindingError):
        verify_challenge(fixture.config, request, response, NOW)


def test_challenge_rejects_unknown_fields_before_signature_verification() -> None:
    fixture = TrustFixture()
    request = fixture.request()
    response = fixture.challenge(request)
    nested = response.worker_handshake_challenge.SerializeToString() + b"\xa0\x06\x01"
    response.worker_handshake_challenge.ParseFromString(nested)
    with pytest.raises(WorkerHandshakePacketError, match="unknown"):
        verify_challenge(fixture.config, request, response, NOW)


def test_renewal_requires_current_session_and_rotates_it() -> None:
    fixture = TrustFixture()
    request = fixture.request(RENEW)
    verified = verify_challenge(fixture.config, request, fixture.challenge(request), NOW)
    with pytest.raises(WorkerHandshakePacketError):
        build_authenticate(fixture.config, verified, _capability(fixture.config), "", NOW)
    authenticate = build_authenticate(
        fixture.config, verified, _capability(fixture.config), "current-token", NOW
    )
    response = fixture.result(verified.message(), "current-token")
    with pytest.raises(WorkerHandshakeBindingError, match="rotate"):
        verify_result(fixture.config, verified, authenticate, response, NOW)


def test_renewal_accepts_a_bounded_token_larger_than_an_identity() -> None:
    fixture = TrustFixture()
    request = fixture.request(RENEW)
    verified = verify_challenge(fixture.config, request, fixture.challenge(request), NOW)
    token = "t" * 300
    authenticate = build_authenticate(
        fixture.config, verified, _capability(fixture.config), token, NOW
    )
    assert authenticate.auth_token == token


def test_result_rejection_is_typed_but_opaque() -> None:
    fixture = TrustFixture()
    request = fixture.request()
    verified = verify_challenge(fixture.config, request, fixture.challenge(request), NOW)
    authenticate = build_authenticate(
        fixture.config, verified, _capability(fixture.config), "", NOW
    )
    with pytest.raises(WorkerHandshakeRejectionError) as caught:
        verify_result(
            fixture.config,
            verified,
            authenticate,
            fixture.result(verified.message(), "", False),
            NOW,
        )
    assert caught.value.reason == handshake_pb2.WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED
    assert "authentication" not in str(caught.value).lower()


def test_result_rejects_an_already_expired_token() -> None:
    fixture = TrustFixture()
    request = fixture.request()
    verified = verify_challenge(fixture.config, request, fixture.challenge(request), NOW)
    authenticate = build_authenticate(
        fixture.config, verified, _capability(fixture.config), "", NOW
    )
    response = fixture.result(verified.message(), "session-token")
    response.worker_handshake_result.issued_at.CopyFrom(
        _timestamp(NOW - timedelta(seconds=30))
    )
    response.worker_handshake_result.token_expires_at.CopyFrom(
        _timestamp(NOW - timedelta(seconds=1))
    )
    response.ClearField("signature")
    sign_trust_packet(response, fixture.scheduler_key)
    with pytest.raises(WorkerHandshakeExpiredError):
        verify_result(fixture.config, verified, authenticate, response, NOW)
