"""Parser-differential regressions for the exact CAP-PRODUCTION wire."""

import unittest
from datetime import datetime, timedelta, timezone

from cryptography.hazmat.primitives.asymmetric import ec

from cap.production_signing import (
    ProductionSignatureError,
    ProductionTrust,
    extract_signature,
    sign_production_packet,
    verify_production_packet,
)
from cap.runtime import Agent
from tests.production_admission_support import (
    RecordingReplayStore,
    make_packet,
    sign_unsigned_wire,
    signed_packet_without_validation,
    signed_wire_with,
    wire_field,
)


class TestProductionWireParser(unittest.TestCase):
    def setUp(self) -> None:
        self.key = ec.generate_private_key(ec.SECP256R1())

    def assert_wire_rejected(self, extra_field: bytes) -> None:
        raw = signed_wire_with(self.key, extra_field)
        with self.assertRaises(ProductionSignatureError):
            extract_signature(raw)

    def test_rejects_signed_unknown_top_level_field(self) -> None:
        self.assert_wire_rejected(wire_field(99, 2, b"unknown"))

    def test_rejects_duplicate_singular_sender_id(self) -> None:
        self.assert_wire_rejected(wire_field(2, 2, b"other-sender"))

    def test_rejects_more_than_one_oneof_payload(self) -> None:
        self.assert_wire_rejected(wire_field(11, 2, b""))

    def test_rejects_wrong_wire_type_for_known_field(self) -> None:
        self.assert_wire_rejected(wire_field(4, 2, b""))

    def test_rejects_protocol_version_parser_differentials(self) -> None:
        invalid_encodings = (
            b"\x02",
            bytes.fromhex("81 80 80 80 10"),
            bytes.fromhex("ff ff ff ff ff ff ff ff ff 01"),
        )
        for encoded in invalid_encodings:
            with self.subTest(encoded=encoded.hex()):
                packet = make_packet()
                packet.protocol_version = 0
                unsigned = packet.SerializeToString() + wire_field(4, 0, encoded)
                raw = sign_unsigned_wire(self.key, unsigned)
                with self.assertRaisesRegex(
                    ProductionSignatureError, "invalid protocol version wire"
                ):
                    extract_signature(raw)

    def test_signer_rejects_retained_unknown_top_level_field(self) -> None:
        packet = make_packet()
        tainted = type(packet).FromString(
            packet.SerializeToString() + wire_field(99, 2, b"unknown")
        )

        with self.assertRaisesRegex(
            ProductionSignatureError, "unknown BusPacket field"
        ):
            sign_production_packet(tainted, self.key)

    def test_rejects_signed_nested_unknown_fields(self) -> None:
        trust = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
        )
        for field_name in ("signature_metadata", "job_request"):
            with self.subTest(field_name=field_name):
                packet = make_packet()
                nested = getattr(packet, field_name)
                tainted = type(nested).FromString(
                    nested.SerializeToString() + wire_field(99, 2, b"unknown")
                )
                nested.CopyFrom(tainted)
                raw = signed_packet_without_validation(self.key, packet)
                with self.assertRaisesRegex(
                    ProductionSignatureError, "unknown nested protobuf field"
                ):
                    verify_production_packet(raw, trust)

    def test_signer_rejects_retained_nested_unknown_field(self) -> None:
        packet = make_packet()
        nested = packet.signature_metadata
        nested.CopyFrom(type(nested).FromString(
            nested.SerializeToString() + wire_field(99, 2, b"unknown")
        ))

        with self.assertRaisesRegex(
            ProductionSignatureError, "unknown nested protobuf field"
        ):
            sign_production_packet(packet, self.key)


class TestProductionReplayRetention(unittest.TestCase):
    def setUp(self) -> None:
        self.key = ec.generate_private_key(ec.SECP256R1())

    def test_retention_extends_metadata_expiry_by_clock_skew(self) -> None:
        replay = RecordingReplayStore()
        skew = timedelta(seconds=30)
        trust = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
            clock_skew=skew,
        )
        agent = Agent(production_trust=trust, replay_store=replay)
        packet = make_packet()
        expiry = packet.signature_metadata.expires_at.ToDatetime(
            tzinfo=timezone.utc
        )

        self.assertIsNotNone(
            agent._decode_production_packet(sign_production_packet(packet, self.key))
        )
        self.assertEqual([expiry + skew], replay.expiries)

    def test_negative_clock_skew_fails_closed(self) -> None:
        raw = sign_production_packet(make_packet(), self.key)
        trust = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
            clock_skew=timedelta(microseconds=-1),
        )

        with self.assertRaises(ProductionSignatureError):
            verify_production_packet(raw, trust)


class TestProductionTimestampValidation(unittest.TestCase):
    def setUp(self) -> None:
        self.key = ec.generate_private_key(ec.SECP256R1())
        self.trust = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
        )

    def test_verifier_requires_complete_authoritative_trust(self) -> None:
        raw = sign_production_packet(make_packet(), self.key)
        authorities = (
            ("", "tenant-a", "scheduler-1"),
            ("worker-pool-a", "", "scheduler-1"),
            ("worker-pool-a", "tenant-a", ""),
        )
        for audience, tenant, sender in authorities:
            with self.subTest(audience=audience, tenant=tenant, sender=sender):
                trust = ProductionTrust(
                    audience=audience,
                    tenant=tenant,
                    sender=sender,
                    public_keys={"k1": self.key.public_key()},
                )
                with self.assertRaisesRegex(
                    ProductionSignatureError, "production trust authority required"
                ):
                    verify_production_packet(raw, trust)

    def test_invalid_expiry_components_are_normalized(self) -> None:
        invalid_components = (
            ("nanos", -1),
            ("nanos", 1_000_000_000),
            ("seconds", 253_402_300_800),
        )
        for field_name, value in invalid_components:
            with self.subTest(field_name=field_name, value=value):
                packet = make_packet()
                setattr(packet.signature_metadata.expires_at, field_name, value)
                raw = signed_packet_without_validation(self.key, packet)

                with self.assertRaises(ProductionSignatureError) as caught:
                    verify_production_packet(raw, self.trust)
                self.assertEqual("invalid signature expiry", str(caught.exception))

    def test_invalid_trust_clock_values_fail_with_static_error(self) -> None:
        invalid_values = (object(), datetime(2026, 1, 1), float("nan"))
        raw = sign_production_packet(make_packet(), self.key)
        for value in invalid_values:
            with self.subTest(value=repr(value)):
                trust = ProductionTrust(
                    audience="worker-pool-a",
                    tenant="tenant-a",
                    sender="scheduler-1",
                    public_keys={"k1": self.key.public_key()},
                    now=lambda value=value: value,
                )
                with self.assertRaises(ProductionSignatureError) as caught:
                    verify_production_packet(raw, trust)
                self.assertEqual(
                    "invalid production lifetime bounds", str(caught.exception)
                )

    def test_clock_skew_cannot_exceed_maximum_lifetime(self) -> None:
        raw = sign_production_packet(make_packet(), self.key)
        trust = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
            max_lifetime=timedelta(seconds=30),
            clock_skew=timedelta(seconds=31),
        )

        with self.assertRaises(ProductionSignatureError) as caught:
            verify_production_packet(raw, trust)
        self.assertEqual("invalid production lifetime bounds", str(caught.exception))

    def test_absolute_lifetime_and_clock_skew_caps(self) -> None:
        raw = sign_production_packet(make_packet(), self.key)
        invalid_bounds = (
            (timedelta(minutes=5, microseconds=1), timedelta(0)),
            (timedelta(minutes=5), timedelta(minutes=1, microseconds=1)),
            (timedelta(0), timedelta(0)),
            (timedelta(microseconds=-1), timedelta(0)),
        )
        for max_lifetime, clock_skew in invalid_bounds:
            with self.subTest(max_lifetime=max_lifetime, clock_skew=clock_skew):
                trust = ProductionTrust(
                    audience="worker-pool-a",
                    tenant="tenant-a",
                    sender="scheduler-1",
                    public_keys={"k1": self.key.public_key()},
                    max_lifetime=max_lifetime,
                    clock_skew=clock_skew,
                )
                with self.assertRaisesRegex(
                    ProductionSignatureError, "invalid production lifetime bounds"
                ):
                    verify_production_packet(raw, trust)

        boundary = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
            max_lifetime=timedelta(minutes=5),
            clock_skew=timedelta(minutes=1),
        )
        self.assertIsNotNone(verify_production_packet(raw, boundary))


if __name__ == "__main__":
    unittest.main()
