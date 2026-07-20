import hashlib
import importlib
import json
from pathlib import Path
from types import ModuleType

import pytest
from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils
from google.protobuf.timestamp_pb2 import Timestamp

from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2, job_pb2


DOMAINS = {
    "challenge_request": b"CAP-WORKER-HANDSHAKE-CHALLENGE-REQUEST-V1",
    "challenge": b"CAP-WORKER-HANDSHAKE-CHALLENGE-V1",
    "authenticate": b"CAP-WORKER-HANDSHAKE-AUTHENTICATE-V1",
    "result": b"CAP-WORKER-HANDSHAKE-RESULT-V1",
}
FIXTURES = (
    Path(__file__).resolve().parents[3]
    / "spec"
    / "conformance"
    / "fixtures"
    / "handshake"
)


def _api() -> ModuleType:
    return importlib.import_module("cap.trust_signing")


def _challenge() -> handshake_pb2.WorkerHandshakeChallenge:
    return handshake_pb2.WorkerHandshakeChallenge(
        request_id="request-1",
        challenge_id="challenge-1",
        trace_id="trace-1",
        worker_id="worker-1",
        agent_id="agent-1",
        tenant_id="tenant-1",
        proof_key_id="worker-key-1",
        proof_algorithm=handshake_pb2.WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256,
        server_key_id="scheduler-key-1",
        audience="cordum-scheduler",
        purpose=handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE,
        client_nonce=b"c" * 32,
        server_nonce=b"s" * 32,
        protocol_version=1,
        sdk_version="python-test",
        issued_at=Timestamp(seconds=1_893_456_000),
        expires_at=Timestamp(seconds=1_893_456_060),
    )


def _challenge_request() -> handshake_pb2.WorkerHandshakeChallengeRequest:
    return handshake_pb2.WorkerHandshakeChallengeRequest(
        request_id="request-1",
        trace_id="trace-1",
        worker_id="worker-1",
        proof_key_id="worker-key-1",
        proof_algorithm=handshake_pb2.WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256,
        audience="cordum-scheduler",
        purpose=handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE,
        client_nonce=b"c" * 32,
        protocol_version=1,
        sdk_version="python-test",
    )


def _packet(phase: str) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket(
        trace_id="trace-1",
        sender_id="worker-1",
        created_at=Timestamp(seconds=1_893_456_000),
        protocol_version=1,
    )
    if phase == "challenge_request":
        packet.worker_handshake_challenge_request.CopyFrom(_challenge_request())
    elif phase == "challenge":
        packet.sender_id = "scheduler-1"
        packet.worker_handshake_challenge.CopyFrom(_challenge())
    elif phase == "authenticate":
        packet.worker_handshake_authenticate.CopyFrom(
            handshake_pb2.WorkerHandshakeAuthenticate(
                challenge=_challenge(),
                capability_handshake=handshake_pb2.Handshake(
                    component_id="worker-1",
                    role=handshake_pb2.COMPONENT_ROLE_WORKER,
                    supported_versions=[1],
                    capabilities={"progress": True},
                    sdk_version="python-test",
                ),
            )
        )
    elif phase == "result":
        packet.sender_id = "scheduler-1"
        packet.auth_token = "session-token"
        packet.worker_handshake_result.CopyFrom(
            handshake_pb2.WorkerHandshakeResult(
                challenge=_challenge(),
                accepted=True,
                issued_at=Timestamp(seconds=1_893_456_001),
                token_expires_at=Timestamp(seconds=1_893_456_301),
            )
        )
    else:
        raise ValueError("unsupported test phase")
    return packet


def _manual_digest(packet: buspacket_pb2.BusPacket, domain: bytes) -> bytes:
    clone = buspacket_pb2.BusPacket()
    clone.CopyFrom(packet)
    clone.ClearField("signature")
    unsigned = clone.SerializeToString(deterministic=True)
    return hashlib.sha256(domain + b"\x00" + unsigned).digest()


@pytest.mark.parametrize("phase", sorted(DOMAINS))
def test_digest_uses_exact_phase_domain_without_mutating_packet(phase: str) -> None:
    api = _api()
    packet = _packet(phase)
    packet.signature = b"existing-signature"
    before = packet.SerializeToString(deterministic=True)

    assert api.trust_packet_digest(packet) == _manual_digest(packet, DOMAINS[phase])
    assert packet.SerializeToString(deterministic=True) == before


@pytest.mark.parametrize("phase", sorted(DOMAINS))
def test_sign_and_verify_select_the_payload_key_id(phase: str) -> None:
    api = _api()
    packet = _packet(phase)
    signer = ec.generate_private_key(ec.SECP256R1())
    key_id = "worker-key-1" if phase in {"challenge_request", "authenticate"} else "scheduler-key-1"

    signature = api.sign_trust_packet(packet, signer)

    assert packet.signature == signature
    assert api.verify_trust_packet(packet, {key_id: signer.public_key()}) is None


def test_signing_uses_prehashed_sha256() -> None:
    api = _api()
    packet = _packet("challenge_request")
    signer = ec.generate_private_key(ec.SECP256R1())
    api.sign_trust_packet(packet, signer)
    digest = api.trust_packet_digest(packet)

    signer.public_key().verify(
        packet.signature,
        digest,
        ec.ECDSA(utils.Prehashed(hashes.SHA256())),
    )
    with pytest.raises(InvalidSignature):
        signer.public_key().verify(packet.signature, digest, ec.ECDSA(hashes.SHA256()))


def test_signature_covers_auth_token_and_payload() -> None:
    api = _api()
    signer = ec.generate_private_key(ec.SECP256R1())
    result = _packet("result")
    api.sign_trust_packet(result, signer)
    result.auth_token = "tampered-token"
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(result, {"scheduler-key-1": signer.public_key()})

    request = _packet("challenge_request")
    api.sign_trust_packet(request, signer)
    request.worker_handshake_challenge_request.audience = "other-scheduler"
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(request, {"worker-key-1": signer.public_key()})


def test_verifier_never_tries_unselected_or_untrusted_keys() -> None:
    api = _api()
    trusted = ec.generate_private_key(ec.SECP256R1())
    attacker = ec.generate_private_key(ec.SECP256R1())
    packet = _packet("authenticate")
    api.sign_trust_packet(packet, attacker)

    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(
            packet,
            {
                "worker-key-1": trusted.public_key(),
                "attacker-key": attacker.public_key(),
            },
        )
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(packet, {"attacker-key": attacker.public_key()})


def test_missing_key_signature_and_wrong_payload_fail_closed() -> None:
    api = _api()
    signer = ec.generate_private_key(ec.SECP256R1())
    packet = _packet("challenge_request")
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(packet, {"worker-key-1": signer.public_key()})

    packet.worker_handshake_challenge_request.proof_key_id = ""
    packet.signature = signer.sign(
        api.trust_packet_digest(packet),
        ec.ECDSA(utils.Prehashed(hashes.SHA256())),
    )
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(packet, {"": signer.public_key()})

    wrong = buspacket_pb2.BusPacket(job_request=job_pb2.JobRequest(job_id="job-1"))
    with pytest.raises(api.TrustSigningError):
        api.trust_packet_digest(wrong)
    wrong.signature = packet.signature
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(wrong, {"": signer.public_key()})


def test_non_p256_and_non_der_signatures_are_rejected() -> None:
    api = _api()
    packet = _packet("challenge_request")
    p384 = ec.generate_private_key(ec.SECP384R1())
    with pytest.raises(api.TrustSigningError):
        api.sign_trust_packet(packet, p384)

    p256 = ec.generate_private_key(ec.SECP256R1())
    api.sign_trust_packet(packet, p256)
    valid = packet.signature
    for malformed in (valid + b"\x00", b"\x01" * 64):
        packet.signature = malformed
        with pytest.raises(api.TrustSigningError):
            api.verify_trust_packet(packet, {"worker-key-1": p256.public_key()})

    packet.signature = valid
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(packet, {"worker-key-1": p384.public_key()})


@pytest.mark.parametrize("phase", sorted(DOMAINS))
def test_payload_proof_algorithm_must_be_p256(phase: str) -> None:
    api = _api()
    packet = _packet(phase)
    signer = ec.generate_private_key(ec.SECP256R1())
    if phase == "challenge_request":
        packet.worker_handshake_challenge_request.proof_algorithm = 0
    elif phase == "authenticate":
        packet.worker_handshake_authenticate.challenge.proof_algorithm = 0
    elif phase == "challenge":
        packet.worker_handshake_challenge.proof_algorithm = 0
    else:
        packet.worker_handshake_result.challenge.proof_algorithm = 0

    with pytest.raises(api.TrustSigningError):
        api.sign_trust_packet(packet, signer)
    packet.signature = signer.sign(
        api.trust_packet_digest(packet),
        ec.ECDSA(utils.Prehashed(hashes.SHA256())),
    )
    key_id = "worker-key-1" if phase in {"challenge_request", "authenticate"} else "scheduler-key-1"
    with pytest.raises(api.TrustSigningError):
        api.verify_trust_packet(packet, {key_id: signer.public_key()})


def test_reference_vectors_reproduce_digests_and_verify_signatures() -> None:
    api = _api()
    manifest = json.loads((FIXTURES / "manifest.json").read_text(encoding="utf-8"))
    for phase, vector in sorted(manifest["positive"].items()):
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString((FIXTURES / vector["packet"]).read_bytes())
        signer = manifest["keys"][vector["signer"]]
        public_key = serialization.load_pem_public_key(
            (FIXTURES / signer["public_key"]).read_bytes()
        )
        assert isinstance(public_key, ec.EllipticCurvePublicKey)
        assert api.trust_packet_digest(packet).hex() == vector["digest_sha256"], phase
        assert api.verify_trust_packet(packet, {signer["id"]: public_key}) is None


def test_public_package_exports_trust_signing_helpers() -> None:
    cap = importlib.import_module("cap")
    assert callable(cap.trust_packet_digest)
    assert callable(cap.sign_trust_packet)
    assert callable(cap.verify_trust_packet)
