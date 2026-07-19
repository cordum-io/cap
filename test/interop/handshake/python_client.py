"""Installed-wheel CAP worker-trust interop client; emits only bounded status JSON."""

import asyncio
import json
import os
import secrets
import sys
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path

import cap
import nats
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec
from nats.errors import TimeoutError as NATSTimeoutError

from cap.pb.cordum.agent.v1 import handshake_pb2


@dataclass(frozen=True)
class Settings:
    case: str
    nats_url: str
    trust: cap.WorkerTrustConfig


@dataclass(frozen=True)
class MutationProof:
    signature_valid: bool = False
    tamper_rejected: bool = False


def required(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise RuntimeError("missing required environment: {}".format(name))
    return value


def load_private(path: str) -> ec.EllipticCurvePrivateKey:
    value = serialization.load_pem_private_key(Path(path).read_bytes(), password=None)
    if not isinstance(value, ec.EllipticCurvePrivateKey):
        raise RuntimeError("worker private key is not EC")
    return value


def load_public(path: str) -> ec.EllipticCurvePublicKey:
    value = serialization.load_pem_public_key(Path(path).read_bytes())
    if not isinstance(value, ec.EllipticCurvePublicKey):
        raise RuntimeError("scheduler public key is not EC")
    return value


def load_settings() -> Settings:
    scheduler_key_id = required("CAP_HANDSHAKE_SCHEDULER_KEY_ID")
    trust = cap.WorkerTrustConfig(
        worker_id=required("CAP_HANDSHAKE_WORKER_ID"),
        expected_agent_id=required("CAP_HANDSHAKE_AGENT_ID"),
        tenant_id=required("CAP_HANDSHAKE_TENANT_ID"),
        audience=cap.WORKER_HANDSHAKE_AUDIENCE,
        proof_key_id=required("CAP_HANDSHAKE_PROOF_KEY_ID"),
        proof_private_key=load_private(required("CAP_HANDSHAKE_WORKER_PRIVATE_KEY")),
        expected_scheduler_id=required("CAP_HANDSHAKE_SCHEDULER_ID"),
        scheduler_public_keys={
            scheduler_key_id: load_public(required("CAP_HANDSHAKE_SCHEDULER_PUBLIC_KEY"))
        },
        sdk_version=required("CAP_HANDSHAKE_SDK_VERSION"),
    )
    cap.validate_worker_trust_config(trust)
    return Settings(required("CAP_HANDSHAKE_CASE"), required("CAP_HANDSHAKE_NATS_URL"), trust)


def request_options(purpose: int, created_at: datetime) -> cap.WorkerHandshakeRequestOptions:
    return cap.WorkerHandshakeRequestOptions(
        request_id=secrets.token_hex(16), trace_id=secrets.token_hex(16),
        purpose=purpose, client_nonce=secrets.token_bytes(32), created_at=created_at,
    )


def capability(trust: cap.WorkerTrustConfig) -> handshake_pb2.Handshake:
    return handshake_pb2.Handshake(
        component_id=trust.worker_id, role=handshake_pb2.COMPONENT_ROLE_WORKER,
        supported_versions=[1], capabilities={"progress": True},
        sdk_version=trust.sdk_version, ready_topics=["job.interop"],
        agent_name=trust.worker_id,
    )


async def request_packet(connection, subject: str, packet):
    response = await connection.request(
        subject, cap.marshal_worker_trust_packet(packet), timeout=3.0
    )
    return cap.unmarshal_worker_trust_packet(response.data)


async def exchange(connection, trust: cap.WorkerTrustConfig, purpose: int, token: str):
    request = cap.build_challenge_request(
        trust, request_options(purpose, datetime.now(timezone.utc))
    )
    challenge = await request_packet(connection, cap.SUBJECT_WORKER_HANDSHAKE_CHALLENGE, request)
    verified = cap.verify_challenge(trust, request, challenge, datetime.now(timezone.utc))
    authenticate = cap.build_authenticate(
        trust, verified, capability(trust), token, datetime.now(timezone.utc)
    )
    result = await request_packet(
        connection, cap.SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE, authenticate
    )
    return cap.verify_result(
        trust, verified, authenticate, result, datetime.now(timezone.utc)
    )


def mutate(case: str, packet) -> None:
    request = packet.worker_handshake_challenge_request
    if case == "wrong_audience":
        request.audience = "other-scheduler"
    elif case == "missing_identity":
        packet.sender_id = ""
    elif case == "missing_trace":
        packet.trace_id = ""
    elif case == "unsupported_version":
        packet.protocol_version = 2
    elif case == "tamper":
        request.audience = "tampered-after-signing"
    elif case != "skew":
        raise RuntimeError("unsupported negative case: {}".format(case))


def prove_mutation_signature(
    case: str, packet, trust: cap.WorkerTrustConfig
) -> MutationProof:
    keys = {trust.proof_key_id: trust.proof_private_key.public_key()}
    if case == "wrong_audience":
        cap.verify_trust_packet(packet, keys)
        return MutationProof(signature_valid=True)
    if case != "tamper":
        return MutationProof()
    try:
        cap.verify_trust_packet(packet, keys)
    except cap.TrustSigningError:
        return MutationProof(tamper_rejected=True)
    raise RuntimeError("tampered packet retained a valid signature")


async def expect_rejected(connection, packet) -> None:
    try:
        await connection.request(
            cap.SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
            packet.SerializeToString(deterministic=True), timeout=0.75,
        )
    except NATSTimeoutError:
        return
    raise RuntimeError("negative request received a reply")


async def exercise_negative(connection, settings: Settings) -> MutationProof:
    created_at = datetime.now(timezone.utc)
    if settings.case == "skew":
        created_at += timedelta(seconds=61)
    request = cap.build_challenge_request(
        settings.trust,
        request_options(handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE, created_at),
    )
    if settings.case == "replay":
        await request_packet(connection, cap.SUBJECT_WORKER_HANDSHAKE_CHALLENGE, request)
        await expect_rejected(connection, request)
        return MutationProof()
    if settings.case != "impersonation":
        mutate(settings.case, request)
        if settings.case == "wrong_audience":
            cap.sign_trust_packet(request, settings.trust.proof_private_key)
    proof = prove_mutation_signature(settings.case, request, settings.trust)
    await expect_rejected(connection, request)
    return proof


async def run() -> dict:
    settings = load_settings()
    connection = await asyncio.wait_for(
        nats.connect(servers=[settings.nats_url], name="cap-python-handshake-interop"),
        timeout=3.0,
    )
    result = {"language": "python", "case": settings.case, "status": "PASS",
              "issue": False, "renew": False, "rotated": False,
              "mutation_signature_valid": False,
              "tamper_signature_rejected": False}
    try:
        if settings.case == "valid":
            issued = await exchange(
                connection, settings.trust,
                handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE, "",
            )
            renewed = await exchange(
                connection, settings.trust,
                handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW, issued.token,
            )
            result.update(issue=True, renew=True, rotated=bool(
                issued.token and renewed.token and issued.token != renewed.token
            ))
            if not result["rotated"]:
                raise RuntimeError("session did not rotate")
        else:
            proof = await exercise_negative(connection, settings)
            result.update(
                mutation_signature_valid=proof.signature_valid,
                tamper_signature_rejected=proof.tamper_rejected,
            )
        return result
    finally:
        await asyncio.wait_for(connection.drain(), timeout=3.0)


def main() -> int:
    try:
        result = asyncio.run(run())
    except Exception as error:
        print("handshake interop client failed: {}".format(type(error).__name__), file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
