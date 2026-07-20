"""Adversarial lifecycle and compatibility tests for Python worker trust."""

import asyncio
import logging
import unittest
from datetime import timedelta
from types import SimpleNamespace
from unittest.mock import patch

import cap.worker_trust_async as trust_async
from cap.handshake import handshake_payload
from cap.pb.cordum.agent.v1 import buspacket_pb2
from cap.runtime import Agent, InMemoryBlobStore
from cap.subjects import SUBJECT_RESULT
from cap.trust_signing import sign_trust_packet
from cap.worker_trust import (
    WORKER_HANDSHAKE_MAX_SKEW,
    WorkerHandshakeExpiredError,
    WorkerHandshakePacketError,
)
from cap.worker_trust_runtime import (
    WorkerTrustLifecycle,
    WorkerTrustOperationalError,
    WorkerTrustRuntimeError,
)
from cap.worker_trust_runtime_config import RuntimeTrustSettings
from cap.worker_trust_verify import verify_challenge

from test_worker_trust import NOW, TrustFixture
from worker_trust_runtime_support import (
    TOPIC,
    WORKER_ID,
    RecordingNATS,
    session,
    trust_config,
    worker_capability,
)


def trusted_agent(nats, mode="enforce") -> Agent:
    return Agent(
        store=InMemoryBlobStore(),
        connect_fn=nats.connect,
        sender_id=WORKER_ID,
        heartbeat_interval=60,
        worker_trust_mode=mode,
        worker_trust=trust_config(),
        worker_trust_timeout=1.0,
        worker_trust_retries=1,
        logger=logging.getLogger("cap.test.worker-trust.adversarial"),
    )


def register(agent: Agent, calls=None) -> None:
    @agent.job(TOPIC)
    async def handler(context, data):
        del context, data
        if calls is not None:
            calls.append("handler")
        return {"ok": True}


class TestTrustAdmissionEdges(unittest.IsolatedAsyncioTestCase):
    async def test_disconnect_during_authentication_aborts_startup(self) -> None:
        nats = RecordingNATS()
        agent = trusted_agent(nats)
        register(agent)
        exchange_started, release_exchange = asyncio.Event(), asyncio.Event()

        async def delayed_exchange(*_args):
            exchange_started.set()
            await release_exchange.wait()
            return session("interrupted-session")

        with patch.object(
            WorkerTrustLifecycle,
            "_exchange_once",
            side_effect=delayed_exchange,
        ):
            start_task = asyncio.create_task(agent.start())
            await exchange_started.wait()
            self.assertIsNotNone(nats.disconnected_cb)
            await nats.disconnected_cb()
            release_exchange.set()
            try:
                with self.assertRaisesRegex(
                    WorkerTrustRuntimeError, "transport interrupted during startup"
                ):
                    await start_task
            finally:
                await agent.close()

        self.assertFalse(any(event[0] == "subscribe" for event in nats.events))
        self.assertFalse(any(event[0] == "publish" for event in nats.events))
        self.assertIn(("connection-drain",), nats.events)

    async def test_disconnect_closes_admission_before_subscription_replay(self) -> None:
        nats = RecordingNATS()
        agent = trusted_agent(nats)
        register(agent)
        with patch.object(
            WorkerTrustLifecycle,
            "_exchange_once",
            return_value=session("connected-session"),
        ):
            await agent.start()
        try:
            self.assertIsNotNone(nats.disconnected_cb)
            await nats.disconnected_cb()
            with self.assertRaises(WorkerTrustRuntimeError):
                agent._outbound_session_token()
            with patch("cap.runtime.decode_packet") as decoder:
                self.assertIsNone(agent._decode_admitted_packet(b"replayed"))
                decoder.assert_not_called()
        finally:
            await agent.close()

    async def test_warn_rejects_security_failure_before_subscribing(self) -> None:
        nats = RecordingNATS()
        agent = trusted_agent(nats, mode="warn")
        register(agent)
        with patch.object(
            WorkerTrustLifecycle,
            "_exchange_once",
            side_effect=WorkerHandshakePacketError("tampered proof"),
        ):
            with self.assertRaises(WorkerTrustRuntimeError):
                await agent.start()
        self.assertFalse(any(event[0] == "subscribe" for event in nats.events))
        self.assertIn(("connection-drain",), nats.events)

    async def test_close_retains_session_for_inflight_terminal_result(self) -> None:
        nats = RecordingNATS()
        agent = trusted_agent(nats)
        started, release = asyncio.Event(), asyncio.Event()

        @agent.job(TOPIC)
        async def handler(context, data):
            del context, data
            started.set()
            await release.wait()
            return {"ok": True}

        with patch.object(
            WorkerTrustLifecycle,
            "_exchange_once",
            return_value=session("closing-session"),
        ):
            await agent.start()
        packet = buspacket_pb2.BusPacket(trace_id="close-trace", sender_id="scheduler")
        packet.job_request.job_id = "job-close"
        packet.job_request.topic = TOPIC
        packet.job_request.context_ptr = "redis://ctx:job-close"
        await agent._store.set("ctx:job-close", b"{}")
        await agent._dispatch_handler(
            lambda: agent._on_msg(SimpleNamespace(data=b"job"), agent._handlers[TOPIC], packet)
        )
        await started.wait()
        close_task = asyncio.create_task(agent.close())
        await nats.wait_for_drains(2)
        release.set()
        await close_task

        result_data = next(
            data for subject, data in nats.published if subject == SUBJECT_RESULT
        )
        result = buspacket_pb2.BusPacket.FromString(result_data)
        self.assertEqual("closing-session", result.auth_token)


class TestTrustProtocolEdges(unittest.IsolatedAsyncioTestCase):
    async def test_warn_does_not_launder_non_transport_operational_types(self) -> None:
        for failure in (OSError("proof key failed"), asyncio.TimeoutError()):
            with self.subTest(failure=type(failure).__name__):
                lifecycle = WorkerTrustLifecycle(
                    RecordingNATS(),
                    RuntimeTrustSettings.resolve("warn", trust_config(), WORKER_ID),
                    worker_capability(),
                )
                with patch.object(
                    lifecycle, "_exchange_once", side_effect=failure
                ):
                    with self.assertRaises(WorkerTrustRuntimeError):
                        await lifecycle.authenticate()

    async def test_legacy_asyncio_timeout_is_operational_in_warn_mode(self) -> None:
        lifecycle = WorkerTrustLifecycle(
            RecordingNATS(),
            RuntimeTrustSettings.resolve("warn", trust_config(), WORKER_ID),
            worker_capability(),
        )
        with patch.object(trust_async, "_OPERATIONAL_ERRORS", ()):
            with patch.object(
                lifecycle,
                "_exchange_once",
                side_effect=WorkerTrustOperationalError(asyncio.TimeoutError()),
            ):
                self.assertFalse(await lifecycle.authenticate())

    async def test_request_timeout_is_tagged_at_transport_boundary(self) -> None:
        class TimeoutRequester:
            async def request(self, subject, data, timeout):
                del subject, data, timeout
                raise asyncio.TimeoutError()

        lifecycle = WorkerTrustLifecycle(
            TimeoutRequester(),
            RuntimeTrustSettings.resolve("warn", trust_config(), WORKER_ID),
            worker_capability(),
        )
        with patch(
            "cap.worker_trust_runtime.marshal_worker_trust_packet",
            return_value=b"request",
        ):
            with self.assertRaises(WorkerTrustOperationalError):
                await lifecycle._request(
                    "sys.worker.handshake.challenge", buspacket_pb2.BusPacket()
                )

    async def test_request_cancellation_survives_python39_wait_for_race(self) -> None:
        started = asyncio.Event()

        class CancellationSwallower:
            async def request(self, subject, data, timeout):
                del subject, data, timeout
                started.set()
                try:
                    await asyncio.Event().wait()
                except asyncio.CancelledError:
                    return SimpleNamespace(data=b"late-response")

        async def vulnerable_wait_for(awaitable, timeout):
            del timeout
            return await awaitable

        lifecycle = WorkerTrustLifecycle(
            CancellationSwallower(),
            RuntimeTrustSettings.resolve("enforce", trust_config(), WORKER_ID),
            worker_capability(),
        )
        with patch(
            "cap.worker_trust_runtime.asyncio.wait_for", vulnerable_wait_for
        ), patch(
            "cap.worker_trust_runtime.marshal_worker_trust_packet",
            return_value=b"request",
        ), patch(
            "cap.worker_trust_runtime.unmarshal_worker_trust_packet",
            return_value=buspacket_pb2.BusPacket(),
        ):
            task = asyncio.create_task(
                lifecycle._request("sys.worker.handshake.challenge", buspacket_pb2.BusPacket())
            )
            while not started.is_set():
                await asyncio.sleep(0)
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await task


class TestTrustCompatibility(unittest.TestCase):
    def test_request_created_at_must_be_within_protocol_skew(self) -> None:
        fixture = TrustFixture()
        request = fixture.request()
        request.created_at.FromDatetime(
            NOW - WORKER_HANDSHAKE_MAX_SKEW - timedelta(seconds=1)
        )
        request.ClearField("signature")
        sign_trust_packet(request, fixture.worker_key)

        with self.assertRaises(WorkerHandshakeExpiredError):
            verify_challenge(fixture.config, request, fixture.challenge(request), NOW)

    def test_legacy_handshake_only_advertises_exact_v1(self) -> None:
        for versions in ([], [2], [1, 2], [1, 1]):
            with self.subTest(versions=versions):
                with self.assertRaises(ValueError):
                    handshake_payload("worker", supported_versions=versions)

    def test_scheduler_response_key_id_is_pinned(self) -> None:
        fixture = TrustFixture()
        request = fixture.request()
        response = fixture.challenge(request)
        response.worker_handshake_challenge.server_key_id = "unconfigured-key"
        response.ClearField("signature")
        sign_trust_packet(response, fixture.scheduler_key)
        with self.assertRaises(WorkerHandshakePacketError):
            verify_challenge(fixture.config, request, response, NOW)


if __name__ == "__main__":
    unittest.main()
