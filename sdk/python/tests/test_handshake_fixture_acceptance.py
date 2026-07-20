from pathlib import Path

import pytest

from cap.pb.cordum.agent.v1 import buspacket_pb2
from cap.worker_trust import WorkerHandshakePacketError
from cap.worker_trust_validate import validate_worker_trust_packet


FIXTURE_DIR = (
    Path(__file__).resolve().parents[3]
    / "spec"
    / "conformance"
    / "fixtures"
    / "handshake"
)


def load_handshake_fixture(name: str) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket()
    packet.ParseFromString((FIXTURE_DIR / name).read_bytes())
    return packet


@pytest.mark.parametrize(
    "name",
    ["challenge_request.bin", "challenge.bin", "authenticate.bin", "result.bin"],
)
def test_validator_accepts_published_handshake_fixture(name: str) -> None:
    validate_worker_trust_packet(load_handshake_fixture(name))


def test_validator_rejects_unsupported_fixture_version() -> None:
    packet = load_handshake_fixture("challenge_request.bin")
    packet.protocol_version = 2
    with pytest.raises(WorkerHandshakePacketError):
        validate_worker_trust_packet(packet)
