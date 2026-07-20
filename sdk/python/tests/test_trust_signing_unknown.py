"""Unknown protobuf fields must never enter cross-language transcripts."""

import pytest
from cryptography.hazmat.primitives.asymmetric import ec

from test_trust_signing import _api, _packet


def test_unknown_fields_are_rejected_without_mutating_the_packet() -> None:
    api = _api()
    key = ec.generate_private_key(ec.SECP256R1())
    packet = _packet("challenge_request")
    api.sign_trust_packet(packet, key)
    nested = packet.worker_handshake_challenge_request.SerializeToString() + b"\xa0\x06\x01"
    packet.worker_handshake_challenge_request.ParseFromString(nested)
    before = packet.SerializeToString(deterministic=True)
    operations = (
        lambda: api.trust_packet_digest(packet),
        lambda: api.sign_trust_packet(packet, key),
        lambda: api.verify_trust_packet(packet, {"worker-key-1": key.public_key()}),
    )
    for operation in operations:
        with pytest.raises(api.TrustSigningError, match="unknown"):
            operation()
        assert packet.SerializeToString(deterministic=True) == before
