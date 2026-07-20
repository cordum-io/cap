"""RED-turned-GREEN evidence for task-a13f83fa step-7 (Python runtime admission)."""

import asyncio
import unittest
from datetime import datetime, timedelta, timezone

from cryptography.hazmat.primitives.asymmetric import ec

from cap.production_replay import InMemoryReplayStore, ReplayOutcome
from cap.production_signing import (
    MAX_PRODUCTION_RAW_BYTES,
    ProductionSignatureError,
    ProductionTrust,
    extract_signature,
    sign_production_packet,
    verify_production_packet,
)
from cap.runtime import Agent
from tests.production_admission_support import (
    RecordingReplayStore as _RecordingReplayStore,
    activate_production_session as _activate_production_session,
    make_packet as _make_packet,
    production_agent as _agent,
    signed_packet_without_validation as _signed_packet_without_validation,
)


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
        self.assertIsNone(second)

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

    def test_rejects_invalid_envelope_before_replay(self):
        replay = _RecordingReplayStore()
        agent = _agent({"k1": self.key.public_key()}, replay)
        packet = _make_packet()
        packet.protocol_version = 999
        raw = _signed_packet_without_validation(self.key, packet)

        self.assertIsNone(agent._decode_production_packet(raw))
        self.assertEqual(replay.digests, [])

    def test_replay_digest_binds_exact_signed_body(self):
        replay = _RecordingReplayStore()
        agent = _agent({"k1": self.key.public_key()}, replay)
        packet = _make_packet()
        first = sign_production_packet(packet, self.key)
        second = sign_production_packet(packet, self.key)

        self.assertIsNotNone(agent._decode_production_packet(first))
        self.assertIsNotNone(agent._decode_production_packet(second))
        self.assertEqual(len(replay.digests), 2)
        self.assertEqual(replay.digests[0], replay.digests[1])

    def test_actor_mirror_binds_authoritative_actor(self):
        agent = _agent({"k1": self.key.public_key()})
        packet = _make_packet()
        packet.identity.actor_id = "actor-a"
        packet.job_request.identity.actor_id = "actor-a"
        packet.job_request.meta.actor_id = "principal-a"
        raw = sign_production_packet(packet, self.key)

        self.assertIsNone(agent._decode_production_packet(raw))

    def test_runtime_path_requires_authenticated_session(self):
        agent = _agent({"k1": self.key.public_key()})
        raw = sign_production_packet(_make_packet(), self.key)

        self.assertIsNone(agent._decode_admitted_packet(raw, "worker-pool-a"))

    def test_partial_production_configuration_is_rejected(self):
        trust = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
        )
        with self.assertRaises(ValueError):
            Agent(production_trust=trust)
        with self.assertRaises(ValueError):
            Agent(replay_store=InMemoryReplayStore())

    def test_unknown_replay_outcome_fails_closed(self):
        replay = _RecordingReplayStore(outcome=None)
        agent = _agent({"k1": self.key.public_key()}, replay)
        raw = sign_production_packet(_make_packet(), self.key)

        self.assertIsNone(agent._decode_production_packet(raw))

    def test_actual_subject_is_authoritative_audience(self):
        agent = _agent({"k1": self.key.public_key()})
        raw = sign_production_packet(_make_packet(), self.key)

        self.assertIsNone(agent._decode_production_packet(raw, "other.subject"))

    def test_identity_rejection_does_not_poison_replay_id(self):
        agent = _agent({"k1": self.key.public_key()})
        invalid = _make_packet()
        invalid.job_request.env["tenant_id"] = "tenant-b"
        valid = _make_packet()

        self.assertIsNone(
            agent._decode_production_packet(sign_production_packet(invalid, self.key))
        )
        self.assertIsNotNone(
            agent._decode_production_packet(sign_production_packet(valid, self.key))
        )

    def test_rejects_expiry_beyond_default_lifetime(self):
        agent = _agent({"k1": self.key.public_key()})
        packet = _make_packet()
        packet.signature_metadata.expires_at.FromDatetime(
            datetime.now(timezone.utc) + timedelta(minutes=10)
        )

        self.assertIsNone(
            agent._decode_production_packet(
                _signed_packet_without_validation(self.key, packet)
            )
        )

    def test_signer_rejects_expiry_beyond_default_lifetime(self):
        packet = _make_packet()
        expiry = datetime.now(timezone.utc) + timedelta(minutes=10)
        packet.signature_metadata.expires_at.FromDatetime(expiry)

        with self.assertRaisesRegex(ProductionSignatureError, "lifetime"):
            sign_production_packet(packet, self.key)

    def test_signing_rejects_non_p256_key(self):
        p384_key = ec.generate_private_key(ec.SECP384R1())

        with self.assertRaises(ValueError):
            sign_production_packet(_make_packet(), p384_key)

    def test_oversize_wire_is_rejected_before_parsing(self):
        with self.assertRaises(ProductionSignatureError):
            extract_signature(b"x" * (MAX_PRODUCTION_RAW_BYTES + 1))

    def test_malformed_embedded_protobuf_is_normalized(self):
        raw = bytes([0x52, 0x01, 0x80, 0x72, 0x01, 0x01])
        trust = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
        )

        with self.assertRaises(ProductionSignatureError):
            verify_production_packet(raw, trust)

    def test_nonminimal_outer_wire_is_rejected(self):
        cases = (
            bytes([0x40, 0x81, 0x00, 0x72, 0x01, 0x01]),
            bytes([0x4A, 0x81, 0x00, 0x00, 0x72, 0x01, 0x01]),
        )
        for raw in cases:
            with self.subTest(raw=raw), self.assertRaises(ProductionSignatureError):
                extract_signature(raw)

    def test_unexpected_replay_backend_error_fails_closed_without_detail(self):
        class ExplodingReplayStore:
            def admit(self, *_args):
                raise RuntimeError("secret backend detail")

        agent = _agent({"k1": self.key.public_key()}, ExplodingReplayStore())
        raw = sign_production_packet(_make_packet(), self.key)

        with self.assertLogs("cap.runtime", level="WARNING") as logs:
            self.assertIsNone(agent._decode_production_packet(raw))
        self.assertNotIn("secret backend detail", " ".join(logs.output))

    def test_runtime_binds_trust_to_authenticated_session_authority(self):
        replay = _RecordingReplayStore()
        agent = _agent({"k1": self.key.public_key()}, replay)
        _activate_production_session(agent)
        packet = _make_packet()
        packet.identity.tenant_id = "tenant-b"
        packet.job_request.tenant_id = "tenant-b"
        packet.job_request.identity.tenant_id = "tenant-b"

        result = agent._decode_admitted_packet(
            sign_production_packet(packet, self.key), "worker-pool-a"
        )
        self.assertIsNone(result)
        self.assertEqual(replay.digests, [])

    def test_authenticated_session_allows_runtime_admission(self):
        agent = _agent({"k1": self.key.public_key()})
        _activate_production_session(agent)
        raw = sign_production_packet(_make_packet(), self.key)

        self.assertIsNotNone(
            agent._decode_admitted_packet(raw, "worker-pool-a")
        )

    def test_start_rejects_production_without_enforce_mode(self):
        connected = False

        async def forbidden_connect(**_kwargs):
            nonlocal connected
            connected = True
            raise AssertionError("NATS connect must not run")

        agent = Agent(
            production_trust=ProductionTrust(
                audience="worker-pool-a",
                tenant="tenant-a",
                sender="scheduler-1",
                public_keys={"k1": self.key.public_key()},
            ),
            replay_store=InMemoryReplayStore(),
            connect_fn=forbidden_connect,
        )

        @agent.job("job.test")
        def _handler(_context, _input):
            return {}

        with self.assertRaisesRegex(RuntimeError, "ENFORCE"):
            asyncio.run(agent.start())
        self.assertFalse(connected)

    def test_replay_store_purges_expired_entries(self):
        store = InMemoryReplayStore()
        now = datetime.now(timezone.utc)
        old_id = b"0123456789abcdef"
        store.admit("tenant-a", "audience-a", "sender-a", old_id, b"old", now + timedelta(minutes=1))
        old_key = ("tenant-a", "audience-a", "sender-a", old_id)
        store._entries[old_key].expires_at = now - timedelta(minutes=1)

        store.admit(
            "tenant-a", "audience-a", "sender-a", b"fedcba9876543210",
            b"new", now + timedelta(minutes=1),
        )
        self.assertNotIn(old_key, store._entries)

    def test_replay_store_tuple_key_is_unambiguous(self):
        store = InMemoryReplayStore()
        expires = datetime.now(timezone.utc) + timedelta(minutes=1)
        message_id = b"0123456789abcdef"

        self.assertEqual(
            ReplayOutcome.FIRST,
            store.admit("a\x00b", "c", "d", message_id, b"one", expires),
        )
        self.assertEqual(
            ReplayOutcome.FIRST,
            store.admit("a", "b", "c\x00d", message_id, b"two", expires),
        )


if __name__ == "__main__":
    unittest.main()
