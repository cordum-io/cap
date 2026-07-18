"""High-level Agent admission tests for authenticated worker trust."""

import asyncio
import logging
import unittest
from dataclasses import replace
from unittest.mock import patch

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.runtime import Agent, Context, InMemoryBlobStore
from cap.subjects import SUBJECT_HANDSHAKE, SUBJECT_RESULT
from cap.worker_trust_runtime import WorkerTrustLifecycle

from worker_trust_runtime_support import (
    TOPIC,
    WORKER_ID,
    RecordingNATS,
    session,
    trust_config,
    worker_capability,
)


def make_agent(nats, mode="enforce", config=None, **options):
    return Agent(
        store=InMemoryBlobStore(),
        connect_fn=nats.connect,
        sender_id=WORKER_ID,
        heartbeat_interval=60,
        worker_trust_mode=mode,
        worker_trust=trust_config() if config is None else config,
        worker_trust_timeout=1.0,
        worker_trust_retries=1,
        logger=logging.getLogger("cap.test.agent.trust"),
        **options,
    )


def register(agent, calls=None):
    @agent.job(TOPIC)
    async def handler(context, data):
        del context, data
        if calls is not None:
            calls.append("handler")
        return {"ok": True}


class TestAgentTrustStartup(unittest.IsolatedAsyncioTestCase):
    async def test_unconfigured_agent_defaults_to_legacy_off(self) -> None:
        nats = RecordingNATS()
        agent = Agent(
            store=InMemoryBlobStore(),
            connect_fn=nats.connect,
            sender_id=WORKER_ID,
            heartbeat_interval=60,
        )
        register(agent)

        await agent.start()
        try:
            self.assertEqual(1, nats.connect_count)
            self.assertEqual("", agent.session_token)
        finally:
            await agent.close()

    async def test_invalid_security_config_fails_before_connect(self) -> None:
        cases = (("optional", None), ("", "partial"), ("enforce", "partial"))
        for mode, kind in cases:
            with self.subTest(mode=mode, kind=kind):
                nats = RecordingNATS()
                config = None if kind is None else trust_config()
                if kind == "partial":
                    config = replace(config, tenant_id="")
                agent = Agent(
                    store=InMemoryBlobStore(),
                    connect_fn=nats.connect,
                    sender_id=WORKER_ID,
                    worker_trust_mode=mode,
                    worker_trust=config,
                )
                register(agent)
                with self.assertRaises((ValueError, RuntimeError)):
                    await agent.start()
                self.assertEqual(0, nats.connect_count)

    async def test_invalid_renew_interval_fails_before_connect(self) -> None:
        for interval in (0, True, float("nan"), 61):
            with self.subTest(interval=interval):
                nats = RecordingNATS()
                agent = make_agent(
                    nats, worker_trust_renew_min_interval=interval
                )
                register(agent)
                with self.assertRaises(ValueError):
                    await agent.start()
                self.assertEqual(0, nats.connect_count)

    async def test_trust_precedes_subscriptions_and_binds_generic_handshake(self) -> None:
        nats = RecordingNATS()
        agent = make_agent(nats)
        register(agent)

        async def exchange(self, purpose, current):
            del self, purpose, current
            nats.events.append(("trust",))
            return session("verified-session")

        with patch.object(WorkerTrustLifecycle, "_exchange_once", exchange):
            await agent.start()
        try:
            names = [event[0] for event in nats.events]
            self.assertLess(names.index("trust"), names.index("subscribe"))
            self.assertLess(names.index("subscribe"), names.index("publish"))
            payload = next(data for subject, data in nats.published
                           if subject == SUBJECT_HANDSHAKE)
            packet = buspacket_pb2.BusPacket.FromString(payload)
            self.assertEqual("verified-session", packet.auth_token)
            self.assertNotEqual("", packet.trace_id)
            self.assertEqual(worker_capability(), packet.handshake)
        finally:
            await agent.close()

    async def test_warn_operational_failure_admits_without_a_session(self) -> None:
        nats = RecordingNATS()
        agent = make_agent(nats, mode="warn")
        register(agent)
        with patch.object(
            WorkerTrustLifecycle, "_exchange_once", side_effect=OSError("offline")
        ):
            await agent.start()
        try:
            self.assertEqual("", agent.session_token)
            self.assertTrue(agent._trust_admitting)
            self.assertEqual(2, len(agent._subscriptions))
        finally:
            await agent.close()

    async def test_session_is_attached_to_heartbeat_and_result(self) -> None:
        nats = RecordingNATS()
        agent = make_agent(nats)
        register(agent)
        with patch.object(
            WorkerTrustLifecycle, "_exchange_once", return_value=session("attached")
        ):
            await agent.start()
        try:
            heartbeat = buspacket_pb2.BusPacket.FromString(
                agent._default_heartbeat_payload()
            )
            self.assertEqual("attached", heartbeat.auth_token)
            source = buspacket_pb2.BusPacket(trace_id="trace-job", sender_id="client")
            source.job_request.job_id = "job-1"
            context = Context(
                job=source.job_request,
                packet=source,
                logger=logging.LoggerAdapter(logging.getLogger(), {}),
            )
            await agent._publish_result(
                context,
                job_pb2.JobResult(
                    job_id="job-1", status=job_pb2.JOB_STATUS_SUCCEEDED,
                    worker_id=WORKER_ID,
                ),
            )
            result = buspacket_pb2.BusPacket.FromString(
                next(data for subject, data in nats.published if subject == SUBJECT_RESULT)
            )
            self.assertEqual("attached", result.auth_token)
        finally:
            await agent.close()

    async def test_cancelled_handshake_cleans_connection_without_subscribing(self) -> None:
        nats = RecordingNATS()
        agent = make_agent(nats)
        register(agent)
        entered = asyncio.Event()

        async def blocked_exchange(self, purpose, current):
            del self, purpose, current
            entered.set()
            await asyncio.Event().wait()

        with patch.object(WorkerTrustLifecycle, "_exchange_once", blocked_exchange):
            task = asyncio.create_task(agent.start())
            await asyncio.wait_for(entered.wait(), timeout=1)
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await task

        self.assertFalse(any(event[0] == "subscribe" for event in nats.events))
        self.assertIn(("connection-drain",), nats.events)


class TestAgentTrustRenewal(unittest.IsolatedAsyncioTestCase):
    async def test_enforce_renew_failure_stops_admissions(self) -> None:
        nats = RecordingNATS()
        calls = []
        agent = make_agent(nats, worker_trust_renew_min_interval=0.01)
        register(agent, calls)
        exchanges = [session("short-session", 0.2), ValueError("renew denied")]

        async def exchange(self, purpose, current):
            del self, purpose, current
            value = exchanges.pop(0)
            if isinstance(value, BaseException):
                raise value
            return value

        with patch.object(WorkerTrustLifecycle, "_exchange_once", exchange):
            await agent.start()
            await nats.wait_for_drains(2)
        try:
            self.assertEqual("", agent.session_token)
            self.assertFalse(agent._trust_admitting)
            self.assertIsNone(agent._heartbeat_task)
        finally:
            await agent.close()

    async def test_reconnect_renews_before_resubscribe(self) -> None:
        nats = RecordingNATS()
        agent = make_agent(nats)
        register(agent)
        calls = []

        async def exchange(self, purpose, current):
            del self
            calls.append((purpose, current))
            return session("first" if not current else "second")

        with patch.object(WorkerTrustLifecycle, "_exchange_once", exchange):
            await agent.start()
            heartbeat = agent._heartbeat_task
            nats.events.clear()
            self.assertIsNotNone(nats.reconnected_cb)
            await nats.reconnected_cb()
        try:
            self.assertEqual("first", calls[1][1])
            names = [event[0] for event in nats.events]
            self.assertLess(names.index("sub-drain"), names.index("subscribe"))
            self.assertEqual("second", agent.session_token)
            self.assertIs(heartbeat, agent._heartbeat_task)
        finally:
            await agent.close()

    async def test_reconnect_after_failure_restarts_trusted_liveness(self) -> None:
        nats = RecordingNATS()
        agent = make_agent(nats, worker_trust_renew_min_interval=0.01)
        register(agent)
        exchanges = [
            session("short", 0.2),
            ValueError("renew denied"),
            session("recovered", 60),
        ]

        async def exchange(self, purpose, current):
            del self, purpose, current
            value = exchanges.pop(0)
            if isinstance(value, BaseException):
                raise value
            return value

        with patch.object(WorkerTrustLifecycle, "_exchange_once", exchange):
            await agent.start()
            await nats.wait_for_drains(2)
            while agent._worker_trust.renewal_running:
                await asyncio.sleep(0)
            self.assertIsNone(agent._heartbeat_task)
            await nats.reconnected_cb()
        try:
            self.assertEqual("recovered", agent.session_token)
            self.assertTrue(agent._trust_admitting)
            self.assertIsNotNone(agent._heartbeat_task)
            self.assertTrue(agent._worker_trust.renewal_running)
            self.assertEqual(2, len(agent._subscriptions))
        finally:
            await agent.close()


if __name__ == "__main__":
    unittest.main()
