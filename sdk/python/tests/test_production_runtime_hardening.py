"""Fail-closed runtime configuration and replay logging regressions."""

import unittest
from dataclasses import replace

from cryptography.hazmat.primitives.asymmetric import ec

from cap.production_replay import (
    ReplayConflictError,
    ReplayStoreUnavailableError,
)
from cap.production_signing import (
    ProductionSignatureError,
    ProductionTrust,
    sign_production_packet,
    verify_production_packet,
)
from tests.production_admission_support import (
    make_packet,
    production_agent,
)


class _FailingReplayStore:
    def __init__(self, result=None, error: Exception | None = None) -> None:
        self._result = result
        self._error = error

    def admit(self, *_args):
        if self._error is not None:
            raise self._error
        return self._result


class TestProductionRuntimeHardening(unittest.TestCase):
    def setUp(self) -> None:
        self.key = ec.generate_private_key(ec.SECP256R1())
        self.raw = sign_production_packet(make_packet(), self.key)

    def test_agent_snapshots_caller_owned_key_mapping(self) -> None:
        public_keys = {"k1": self.key.public_key()}
        agent = production_agent(public_keys)
        public_keys.clear()

        self.assertIsNotNone(agent._decode_production_packet(self.raw))

    def test_padded_trust_authorities_are_rejected_without_normalizing(self) -> None:
        base = ProductionTrust(
            audience="worker-pool-a",
            tenant="tenant-a",
            sender="scheduler-1",
            public_keys={"k1": self.key.public_key()},
        )
        for field in ("audience", "tenant", "sender"):
            with self.subTest(field=field):
                trust = replace(base, **{field: f" {getattr(base, field)}"})
                with self.assertRaisesRegex(
                    ProductionSignatureError, "production trust authority required"
                ):
                    verify_production_packet(self.raw, trust)

    def test_replay_failures_log_only_static_categories(self) -> None:
        secret = "redis://user:password@private-host"
        cases = (
            (ReplayConflictError(secret), "replay conflict"),
            (ReplayStoreUnavailableError(secret), "replay store unavailable"),
            (RuntimeError(secret), "replay store unavailable"),
        )
        for error, category in cases:
            with self.subTest(error=type(error).__name__):
                store = _FailingReplayStore(error=error)
                agent = production_agent({"k1": self.key.public_key()}, store)
                with self.assertLogs("cap.runtime", level="WARNING") as logs:
                    self.assertIsNone(agent._decode_production_packet(self.raw))
                output = " ".join(logs.output)
                self.assertIn(category, output)
                self.assertNotIn(secret, output)

    def test_invalid_replay_outcome_uses_static_log(self) -> None:
        secret = "redis://user:password@private-host"
        store = _FailingReplayStore(result=secret)
        agent = production_agent({"k1": self.key.public_key()}, store)

        with self.assertLogs("cap.runtime", level="WARNING") as logs:
            self.assertIsNone(agent._decode_production_packet(self.raw))
        output = " ".join(logs.output)
        self.assertIn("invalid outcome", output)
        self.assertNotIn(secret, output)


if __name__ == "__main__":
    unittest.main()
