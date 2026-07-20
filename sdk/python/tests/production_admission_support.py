"""Shared fixtures for CAP-PRODUCTION Python admission tests."""

from datetime import datetime, timedelta, timezone
from types import SimpleNamespace

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1 import buspacket_pb2
from cap.production_replay import InMemoryReplayStore, ReplayOutcome
from cap.production_signing import DOMAIN, ProductionTrust
from cap.runtime import Agent
from cap.worker_trust import WorkerTrustMode


def make_packet(
    key_id: str = "k1",
    audience: str = "worker-pool-a",
    message_id: bytes = b"0123456789abcdef",
) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket(
        trace_id="trace-1", sender_id="scheduler-1", protocol_version=1
    )
    packet.created_at.GetCurrentTime()
    packet.identity.tenant_id = "tenant-a"
    packet.identity.principal_id = "principal-a"
    metadata = packet.signature_metadata
    metadata.profile_version = "cap-production-v1"
    metadata.algorithm = "ECDSA-P256-SHA256"
    metadata.message_id = message_id
    metadata.audience = audience
    metadata.expires_at.FromDatetime(
        datetime.now(timezone.utc) + timedelta(minutes=2)
    )
    metadata.key_id = key_id
    request = packet.job_request
    request.job_id = "job-1"
    request.topic = "job.test"
    request.tenant_id = "tenant-a"
    request.identity.CopyFrom(packet.identity)
    return packet


class RecordingReplayStore:
    def __init__(self, outcome: ReplayOutcome = ReplayOutcome.FIRST) -> None:
        self.digests: list[bytes] = []
        self.expiries: list[datetime] = []
        self.outcome = outcome

    def admit(self, _tenant, _audience, _sender, _message_id, digest, _expires_at):
        self.digests.append(digest)
        self.expiries.append(_expires_at)
        return self.outcome


def production_agent(public_keys, replay_store=None) -> Agent:
    trust = ProductionTrust(
        audience="worker-pool-a",
        tenant="tenant-a",
        sender="scheduler-1",
        public_keys=public_keys,
    )
    return Agent(
        production_trust=trust,
        replay_store=replay_store or InMemoryReplayStore(),
    )


def activate_production_session(
    agent: Agent, tenant: str = "tenant-a", sender: str = "scheduler-1"
) -> None:
    config = SimpleNamespace(tenant_id=tenant, expected_scheduler_id=sender)
    agent._trust_settings = SimpleNamespace(
        mode=WorkerTrustMode.ENFORCE, config=config
    )
    agent._worker_trust = SimpleNamespace(session_token=lambda: "live-session")
    agent._trust_admitting = True


def encode_varint(value: int) -> bytes:
    output = bytearray()
    while value >= 0x80:
        output.append((value & 0x7F) | 0x80)
        value >>= 7
    output.append(value)
    return bytes(output)


def wire_field(number: int, wire_type: int, value: bytes) -> bytes:
    tag = encode_varint(number << 3 | wire_type)
    if wire_type == 2:
        return tag + encode_varint(len(value)) + value
    return tag + value


def signed_wire_with(
    key: ec.EllipticCurvePrivateKey, extra_field: bytes
) -> bytes:
    unsigned = make_packet().SerializeToString() + extra_field
    return sign_unsigned_wire(key, unsigned)


def signed_packet_without_validation(
    key: ec.EllipticCurvePrivateKey, packet: buspacket_pb2.BusPacket
) -> bytes:
    return sign_unsigned_wire(key, packet.SerializeToString())


def sign_unsigned_wire(
    key: ec.EllipticCurvePrivateKey, unsigned: bytes
) -> bytes:
    signature = key.sign(DOMAIN + unsigned, ec.ECDSA(hashes.SHA256()))
    return unsigned + wire_field(14, 2, signature)
