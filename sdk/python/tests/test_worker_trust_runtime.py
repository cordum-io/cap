"""Deterministic trust-lifecycle tests for the high-level Python runtime."""

import asyncio
import unittest
from datetime import datetime, timedelta, timezone
from types import SimpleNamespace
from unittest.mock import patch

from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2
from cap.worker_trust import (
    WorkerHandshakeSession,
    WorkerTrustConfig,
    WorkerTrustConfigError,
)
from cap.worker_trust_runtime import (
    RuntimeTrustSettings,
    WorkerTrustLifecycle,
    WorkerTrustRuntimeError,
)


WORKER_ID = "python-trust-worker"


def trust_config() -> WorkerTrustConfig:
    scheduler_key = ec.generate_private_key(ec.SECP256R1())
    return WorkerTrustConfig(
        worker_id=WORKER_ID,
        expected_agent_id="agent-python",
        tenant_id="tenant-python",
        audience="cordum-scheduler",
        proof_key_id="worker-key-1",
        proof_private_key=ec.generate_private_key(ec.SECP256R1()),
        expected_scheduler_id="scheduler-1",
        scheduler_public_keys={"scheduler-key-1": scheduler_key.public_key()},
        sdk_version="cap-python/v2",
    )


def capability() -> handshake_pb2.Handshake:
    return handshake_pb2.Handshake(
        component_id=WORKER_ID,
        role=handshake_pb2.COMPONENT_ROLE_WORKER,
        supported_versions=[1],
        capabilities={"job.python": True},
        sdk_version="cap-python/v2",
        ready_topics=["job.python"],
    )


class RecordingNATS:
    def __init__(self) -> None:
        self.events = []
        self.responses = [b"challenge-response", b"result-response"]

    async def request(self, subject, data, timeout):
        self.events.append(("request", subject, data, timeout))
        return SimpleNamespace(data=self.responses.pop(0))


def settings(mode: str = "enforce") -> RuntimeTrustSettings:
    return RuntimeTrustSettings.resolve(
        mode, trust_config(), WORKER_ID, timeout=1.0, retries=1
    )


def session(token: str, seconds: float = 60.0) -> WorkerHandshakeSession:
    now = datetime.now(timezone.utc)
    return WorkerHandshakeSession(
        token=token,
        issued_at=now,
        expires_at=now + timedelta(seconds=seconds),
    )


class TestRuntimeTrustSettings(unittest.TestCase):
    def test_blank_unconfigured_defaults_off_but_unsafe_modes_fail(self) -> None:
        self.assertEqual(
            "off", RuntimeTrustSettings.resolve(None, None, WORKER_ID).mode.value
        )
        for raw, config in (("optional", None), ("enforce", None)):
            with self.subTest(raw=raw):
                with self.assertRaises(ValueError):
                    RuntimeTrustSettings.resolve(raw, config, WORKER_ID)

    def test_off_is_explicit_and_rejects_conflicting_trust(self) -> None:
        resolved = RuntimeTrustSettings.resolve("off", None, WORKER_ID)
        self.assertEqual("off", resolved.mode.value)
        with self.assertRaises(WorkerTrustConfigError):
            RuntimeTrustSettings.resolve("off", trust_config(), WORKER_ID)

    def test_enabled_tuning_is_bounded_and_rejects_boolean_aliases(self) -> None:
        for tuning in (
            {"timeout": 0}, {"timeout": 61}, {"timeout": True},
            {"retries": 0}, {"retries": 11}, {"retries": True},
        ):
            with self.subTest(tuning=tuning):
                with self.assertRaises(WorkerTrustConfigError):
                    RuntimeTrustSettings.resolve(
                        "enforce", trust_config(), WORKER_ID, **tuning
                    )


class TestWorkerTrustLifecycle(unittest.IsolatedAsyncioTestCase):
    async def test_renew_interval_is_bounded(self) -> None:
        for value in (0, 61, True):
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    RuntimeTrustSettings.resolve(
                        "enforce",
                        trust_config(),
                        WORKER_ID,
                        renew_min_interval=value,
                    )

    async def test_issue_uses_two_bounded_requests_and_installs_verified_result(self) -> None:
        nc = RecordingNATS()
        life = WorkerTrustLifecycle(nc, settings(), capability())
        request = buspacket_pb2.BusPacket(trace_id="issue-trace")
        authenticate = buspacket_pb2.BusPacket(trace_id="issue-trace")
        challenge = buspacket_pb2.BusPacket(trace_id="issue-trace")
        result = buspacket_pb2.BusPacket(trace_id="issue-trace")
        verified = object()
        installed = session("session-one")

        with patch.multiple(
            "cap.worker_trust_runtime",
            build_challenge_request=lambda *_: request,
            verify_challenge=lambda *_: verified,
            build_authenticate=lambda *_: authenticate,
            verify_result=lambda *_: installed,
            marshal_worker_trust_packet=lambda packet: (
                b"challenge-request" if packet is request else b"authenticate"
            ),
            unmarshal_worker_trust_packet=lambda data: (
                challenge if data == b"challenge-response" else result
            ),
        ):
            self.assertTrue(await life.authenticate())

        self.assertEqual("session-one", life.session_token())
        self.assertEqual(2, len(nc.events))
        self.assertTrue(nc.events[0][1].endswith(".challenge"))
        self.assertTrue(nc.events[1][1].endswith(".authenticate"))

    async def test_warn_security_failure_clears_the_live_session(self) -> None:
        nc = RecordingNATS()
        life = WorkerTrustLifecycle(nc, settings("warn"), capability())
        with patch.object(
            life, "_exchange_once", return_value=session("live-session")
        ):
            self.assertTrue(await life.authenticate())

        with patch.object(
            life, "_exchange_once", side_effect=ValueError("tampered result")
        ):
            with self.assertRaises(WorkerTrustRuntimeError):
                await life.renew()

        self.assertEqual("", life.session_token())

    async def test_renew_proves_current_token_without_issue_fallback(self) -> None:
        life = WorkerTrustLifecycle(RecordingNATS(), settings(), capability())
        with patch.object(
            life, "_exchange_once", return_value=session("current-session")
        ):
            self.assertTrue(await life.authenticate())
        rotated = session("rotated-session")

        with patch.object(life, "_exchange_once", return_value=rotated) as exchange:
            self.assertTrue(await life.renew())

        self.assertEqual("rotated-session", life.session_token())
        args = exchange.call_args.args
        self.assertEqual(handshake_pb2.WORKER_HANDSHAKE_PURPOSE_RENEW, args[0])
        self.assertEqual("current-session", args[1])

    async def test_concurrent_renew_coalesces_superseded_token(self) -> None:
        life = WorkerTrustLifecycle(RecordingNATS(), settings(), capability())
        with patch.object(
            life, "_exchange_once", return_value=session("session-one")
        ):
            self.assertTrue(await life.authenticate())
        await life._exchange_lock.acquire()
        with patch.object(
            life,
            "_exchange_once",
            side_effect=(session("session-two"), ValueError("stale token")),
        ) as exchange:
            first = asyncio.create_task(life.renew())
            second = asyncio.create_task(life.renew())
            await asyncio.sleep(0)
            life._exchange_lock.release()
            results = await asyncio.gather(first, second, return_exceptions=True)

        self.assertEqual([True, True], results)
        self.assertEqual(1, exchange.call_count)
        self.assertEqual("session-two", life.session_token())

    async def test_reauthenticate_issues_when_prior_session_expired(self) -> None:
        life = WorkerTrustLifecycle(RecordingNATS(), settings(), capability())
        with patch.object(
            life, "_exchange_once", return_value=session("expired", seconds=-1)
        ):
            self.assertTrue(await life.authenticate())
        with patch.object(
            life, "_exchange_once", return_value=session("recovered")
        ) as exchange:
            self.assertTrue(await life.reauthenticate())

        self.assertEqual("recovered", life.session_token())
        self.assertEqual(
            handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE,
            exchange.call_args.args[0],
        )

    async def test_close_cancels_pending_renewal(self) -> None:
        life = WorkerTrustLifecycle(
            RecordingNATS(), settings(), capability()
        )
        with patch.object(life, "_exchange_once", return_value=session("session-one")):
            self.assertTrue(await life.authenticate())
        life.start_renewal(lambda _: asyncio.sleep(0))
        self.assertTrue(life.renewal_running)

        await life.close()

        self.assertFalse(life.renewal_running)
        self.assertEqual("", life.session_token())


if __name__ == "__main__":
    unittest.main()
