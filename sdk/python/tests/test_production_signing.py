import base64
import hashlib
import json
from pathlib import Path

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils

from cap.production_signing import DOMAIN, extract_signature


def test_go_raw_wire_vector_verifies_without_reserialization() -> None:
    fixture = json.loads(
        (Path(__file__).parents[3] / "test/fixtures/production-signing-v1.json").read_text()
    )
    raw = base64.b64decode(fixture["raw_base64"])
    unsigned, signature = extract_signature(raw)
    digest = hashlib.sha256(DOMAIN + unsigned).digest()

    assert unsigned == base64.b64decode(fixture["unsigned_base64"])
    assert digest.hex() == fixture["preimage_digest_hex"]
    key = serialization.load_pem_public_key(fixture["public_key_pem"].encode())
    assert isinstance(key, ec.EllipticCurvePublicKey)
    key.verify(signature, digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))
