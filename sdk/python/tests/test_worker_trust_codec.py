"""Codec and installed public-surface checks for worker trust."""

import pytest

from cap.worker_trust import WorkerHandshakePacketError, WorkerTrustMode
from cap.worker_trust_codec import (
    marshal_worker_trust_packet,
    unmarshal_worker_trust_packet,
)
from test_worker_trust import TrustFixture


def test_codec_round_trip_rejects_unknown_and_size_bounds() -> None:
    fixture = TrustFixture()
    request = fixture.request()
    encoded = marshal_worker_trust_packet(request)
    assert unmarshal_worker_trust_packet(encoded) == request
    nested = request.worker_handshake_challenge_request.SerializeToString() + b"\xa0\x06\x01"
    request.worker_handshake_challenge_request.ParseFromString(nested)
    with pytest.raises(WorkerHandshakePacketError, match="unknown"):
        unmarshal_worker_trust_packet(request.SerializeToString(deterministic=True))
    for invalid in (b"", b"x" * (64 * 1024 + 1)):
        with pytest.raises(WorkerHandshakePacketError, match="size"):
            unmarshal_worker_trust_packet(invalid)


def test_public_package_exports_worker_trust_core() -> None:
    import cap

    assert cap.WorkerTrustMode is WorkerTrustMode
    assert cap.build_challenge_request is not None
    assert cap.unmarshal_worker_trust_packet is unmarshal_worker_trust_packet
