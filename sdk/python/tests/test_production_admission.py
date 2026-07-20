"""RED-turned-GREEN evidence for task-a13f83fa step-7 (Python runtime admission)."""

import unittest
from datetime import datetime, timedelta, timezone

from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionTrust, sign_production_packet
from cap.runtime import Agent


def _make_packet(key_id="k1", audience="worker-pool-a", message_id=b"0123456789abcdef"):
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = "trace-1"
    packet.sender_id = "scheduler-1"
    packet.created_at.GetCurrentTime()
    packet.protocol_version = 1
    packet.identity.tenant_id = "tenant-a"
    packet.identity.principal_id = "principal-a"
    packet.signature_metadata.profile_version = "cap-production-v1"
    packet.signature_metadata.algorithm = "ECDSA-P256-SHA256"
    packet.signature_metadata.message_id = message_id
    packet.signature_metadata.audience = audience
    packet.signature_metadata.expires_at.FromDatetime(datetime.now(timezone.utc) + timedelta(hours=1))
    packet.signature_metadata.key_id = key_id
    packet.job_request.job_id = "job-1"
    packet.job_request.topic = "job.test"
    packet.job_request.tenant_id = "tenant-a"
    packet.job_request.identity.tenant_id = "tenant-a"
    packet.job_request.identity.principal_id = "principal-a"
    return packet


def _agent(public_keys):
    trust = ProductionTrust(audience="worker-pool-a", public_keys=public_keys)
    return Agent(production_trust=trust, replay_store=InMemoryReplayStore())


class TestProductionAdmission(unittest.TestCase):
    def setUp(self):
        self.key = ec.generate_private_key(ec.SECP256R1())

    def test_accepts_valid_signed_packet(self):
        agent = _agent({"k1": self.key.public_key()})
        raw = sign_production_packet(_make_packet(), self.key)

        packet = agent._decode_production_packet(raw)

        self.assertIsNotNone(packet)
        self.assertEqual(packet.job_request.job_id, "job-1")

    def test_rejects_unsigned_packet(self):
        agent = _agent({"k1": self.key.public_key()})
        raw = _make_packet().SerializeToString()

        self.assertIsNone(agent._decode_production_packet(raw))

    def test_rejects_tampered_signature(self):
        agent = _agent({"k1": self.key.public_key()})
        raw = bytearray(sign_production_packet(_make_packet(), self.key))
        raw[-1] ^= 0xFF

        self.assertIsNone(agent._decode_production_packet(bytes(raw)))

    def test_identical_redelivery_is_harmless(self):
        agent = _agent({"k1": self.key.public_key()})
        raw = sign_production_packet(_make_packet(), self.key)

        first = agent._decode_production_packet(raw)
        second = agent._decode_production_packet(raw)

        self.assertIsNotNone(first)
        self.assertIsNotNone(second)

    def test_rejects_unknown_key_id(self):
        other_key = ec.generate_private_key(ec.SECP256R1())
        agent = _agent({"k1": self.key.public_key()})
        raw = sign_production_packet(_make_packet(key_id="k1"), other_key)

        self.assertIsNone(agent._decode_production_packet(raw))

    def test_rejects_identity_mirror_mismatch(self):
        agent = _agent({"k1": self.key.public_key()})
        packet = _make_packet()
        packet.job_request.env["tenant_id"] = "tenant-B-DIFFERENT"
        raw = sign_production_packet(packet, self.key)

        self.assertIsNone(agent._decode_production_packet(raw))


if __name__ == "__main__":
    unittest.main()
