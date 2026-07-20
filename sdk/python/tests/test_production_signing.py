"""Python consumer for the cross-language CAP-PRODUCTION conformance vectors.

Reads the SAME test/fixtures/production-signing-v1.json as
sdk/go/production_signing_vectors_test.go and sdk/node/test/production-signing.test.ts.
All three must reach identical verdicts on every vector -- that agreement is
what makes the file a conformance artifact instead of three unrelated local
suites. A vector no SDK reads is not a conformance vector.
"""

from __future__ import annotations

import base64
import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path

import pytest
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils

from cap.pb.cordum.agent.v1.job_pb2 import IdentityBinding, JobRequest
from cap.production_replay import (
    InMemoryReplayStore,
    ReplayConflictError,
    ReplayOutcome,
)
from cap.production_signing import (
    DOMAIN,
    ProductionSignatureError,
    ProductionTrust,
    extract_signature,
    verify_production_packet,
)
from cap.production_validation import IdentityMismatchError, validate_identity_binding

FIXTURE_PATH = Path(__file__).parents[3] / "test/fixtures/production-signing-v1.json"

# The fixture's language-neutral reject reason -> the substring Python uses.
# Go collapses an identity mismatch into "unknown key id" so the error cannot be
# used as an oracle; Python names the specific mirror. The fixture records the
# OUTCOME, and each SDK maps it to its own wording.
REASON_MESSAGES = {
    "invalid_signature": "invalid signature",
    "audience_mismatch": "audience mismatch",
    "signature_expired": "signature expired",
    "unknown_key_id": "unknown key id",
    "identity_mismatch": ("tenant mismatch", "sender mismatch"),
}


@pytest.fixture(scope="module")
def fixture() -> dict:
    data = json.loads(FIXTURE_PATH.read_text())
    assert data["schema_version"] >= 2, "fixture predates the multi-vector schema"
    # A shrinking fixture weakens all three SDKs at once, so the floor is asserted.
    assert len(data["vectors"]) >= 9
    assert len(data["replay_vectors"]) >= 3
    assert len(data["identity_binding_vectors"]) >= 3
    return data


def _verify_at(fixture: dict) -> datetime:
    return datetime.fromisoformat(fixture["verify_at_rfc3339"].replace("Z", "+00:00"))


def _trust(fixture: dict, vector: dict) -> ProductionTrust:
    installed = vector.get("trust_key_ids") or list(fixture["trust"]["public_keys"])
    keys = {}
    for key_id in installed:
        pem = fixture["trust"]["public_keys"][key_id]
        loaded = serialization.load_pem_public_key(pem.encode())
        assert isinstance(loaded, ec.EllipticCurvePublicKey)
        keys[key_id] = loaded
    verify_at = _verify_at(fixture)
    return ProductionTrust(
        audience=fixture["trust"]["audience"],
        public_keys=keys,
        tenant=fixture["trust"]["tenant"],
        sender=fixture["trust"]["sender"],
        now=lambda: verify_at,
    )


def _vector_ids(fixture_data: dict) -> list:
    return [v["name"] for v in fixture_data["vectors"]]


def _load_vectors() -> list:
    return json.loads(FIXTURE_PATH.read_text())["vectors"]


@pytest.mark.parametrize("vector", _load_vectors(), ids=lambda v: v["name"])
def test_vector_reaches_the_expected_verdict(fixture: dict, vector: dict) -> None:
    raw = base64.b64decode(vector["raw_base64"])
    trust = _trust(fixture, vector)

    if vector["expect"] == "accept":
        packet = verify_production_packet(raw, trust)
        assert packet.signature_metadata.audience == vector["audience"]
        assert packet.signature_metadata.key_id == vector["key_id"]
        assert packet.signature_metadata.message_id.hex() == vector["message_id_hex"]
        return

    with pytest.raises(ProductionSignatureError) as caught:
        verify_production_packet(raw, trust)
    expected = REASON_MESSAGES[vector["reject_reason"]]
    candidates = (expected,) if isinstance(expected, str) else expected
    assert any(c in str(caught.value) for c in candidates), (
        f"reason {vector['reject_reason']} expected one of {candidates}, got {caught.value}"
    )


@pytest.mark.parametrize("vector", _load_vectors(), ids=lambda v: v["name"])
def test_vector_pins_both_digests(fixture: dict, vector: dict) -> None:
    """The signature preimage and the replay body digest are different values.

    Guards a defect that already shipped once in this SDK: admission code used
    the domain-separated preimage as the replay digest, so identical wire bytes
    hashed differently here than in Go/Node and a valid redelivery looked like a
    conflict.
    """
    raw = base64.b64decode(vector["raw_base64"])
    unsigned, signature = extract_signature(raw)

    assert base64.b64encode(unsigned).decode() == vector["unsigned_base64"]
    assert base64.b64encode(signature).decode() == vector["signature_base64"]
    assert base64.b64decode(fixture["domain_base64"]) == DOMAIN
    assert hashlib.sha256(DOMAIN + unsigned).hexdigest() == vector["preimage_digest_hex"]
    assert hashlib.sha256(unsigned).hexdigest() == vector["body_digest_hex"]
    assert vector["preimage_digest_hex"] != vector["body_digest_hex"]


def test_baseline_signature_verifies_against_the_recorded_public_key(fixture: dict) -> None:
    """Direct cryptographic check, independent of the SDK's verify path."""
    baseline = next(v for v in fixture["vectors"] if v["name"] == "accept/baseline")
    raw = base64.b64decode(baseline["raw_base64"])
    unsigned, signature = extract_signature(raw)
    digest = hashlib.sha256(DOMAIN + unsigned).digest()
    key = serialization.load_pem_public_key(fixture["public_key_pem"].encode())
    assert isinstance(key, ec.EllipticCurvePublicKey)
    key.verify(signature, digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))


def _replay_ids() -> list:
    return json.loads(FIXTURE_PATH.read_text())["replay_vectors"]


@pytest.mark.parametrize("replay", _replay_ids(), ids=lambda r: r["name"])
def test_replay_sequence_matches_at_least_once_contract(fixture: dict, replay: dict) -> None:
    by_name = {v["name"]: v for v in fixture["vectors"]}
    store = InMemoryReplayStore()

    for index, step in enumerate(replay["sequence"]):
        vector = by_name[step["vector"]]
        args = (
            fixture["trust"]["tenant"],
            vector["audience"],
            fixture["trust"]["sender"],
            bytes.fromhex(vector["message_id_hex"]),
            bytes.fromhex(vector["body_digest_hex"]),
            datetime.fromisoformat(vector["expires_at_rfc3339"].replace("Z", "+00:00")),
        )
        if step["expect"] == "conflict":
            with pytest.raises(ReplayConflictError):
                store.admit(*args)
            continue
        outcome = store.admit(*args)
        expected = ReplayOutcome.FIRST if step["expect"] == "first" else ReplayOutcome.DUPLICATE
        assert outcome is expected, f"step {index} of {replay['name']}"


def _identity_vectors() -> list:
    return json.loads(FIXTURE_PATH.read_text())["identity_binding_vectors"]


@pytest.mark.parametrize("vector", _identity_vectors(), ids=lambda v: v["name"])
def test_identity_binding_vector(vector: dict) -> None:
    request = JobRequest()
    request.ParseFromString(base64.b64decode(vector["job_request_base64"]))
    authoritative = IdentityBinding()
    if vector.get("authoritative_base64"):
        authoritative.ParseFromString(base64.b64decode(vector["authoritative_base64"]))

    if vector["expect"] == "accept":
        validate_identity_binding(request, authoritative)
        return
    with pytest.raises(IdentityMismatchError):
        validate_identity_binding(request, authoritative)


def test_legacy_flat_keys_alias_the_baseline_vector(fixture: dict) -> None:
    """Pre-schema-2 readers see the same bytes the baseline vector carries."""
    baseline = next(v for v in fixture["vectors"] if v["name"] == "accept/baseline")
    for key in ("raw_base64", "unsigned_base64", "signature_base64", "preimage_digest_hex"):
        assert fixture[key] == baseline[key], f"legacy {key} drifted from accept/baseline"
    assert fixture["public_key_pem"] == fixture["trust"]["public_keys"][baseline["key_id"]]
