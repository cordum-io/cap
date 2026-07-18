"""Pinned inbound and live-session boundaries for trusted Agent modes."""

import logging
import unittest
from dataclasses import replace
from types import SimpleNamespace
from unittest.mock import patch

from cryptography.hazmat.primitives.asymmetric import ec

from cap.packet_boundary import finalize_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.runtime import Agent, Context, InMemoryBlobStore
from cap.worker_trust_runtime import (
    WorkerTrustLifecycle,
    WorkerTrustRuntimeError,
)

from worker_trust_runtime_support import (
    TOPIC,
    WORKER_ID,
    RecordingNATS,
    session,
    trust_config,
)


def trust_material():
    scheduler_key = ec.generate_private_key(ec.SECP256R1())
    config = replace(
        trust_config(),
        scheduler_public_keys={"scheduler-pin": scheduler_key.public_key()},
    )
    return config, scheduler_key


def job_packet(sender: str, key=None) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket(
        trace_id="trace-trust-boundary",
        sender_id=sender,
        protocol_version=1,
    )
    packet.created_at.GetCurrentTime()
    packet.job_request.job_id = "job-trust-boundary"
    packet.job_request.topic = TOPIC
    packet.job_request.context_ptr = "redis://ctx:trust-boundary"
    return finalize_packet(packet, key)


def message(packet: buspacket_pb2.BusPacket):
    return SimpleNamespace(data=packet.SerializeToString(deterministic=True))


class TestAgentTrustBoundary(unittest.IsolatedAsyncioTestCase):
    async def _start(self, mode, config, public_keys=None):
        nats = RecordingNATS()
        store = InMemoryBlobStore()
        agent = Agent(
            store=store,
            connect_fn=nats.connect,
            sender_id=WORKER_ID,
            heartbeat_interval=60,
            worker_trust_mode=mode,
            worker_trust=config,
            worker_trust_timeout=1,
            worker_trust_retries=1,
            public_keys=public_keys,
            logger=logging.getLogger("cap.test.agent.trust.boundary"),
        )
        calls = []

        @agent.job(TOPIC)
        async def handler(context, data):
            del context, data
            calls.append("handler")
            return {"ok": True}

        with patch.object(
            WorkerTrustLifecycle, "_exchange_once", return_value=session("live")
        ):
            await agent.start()
        await store.set("ctx:trust-boundary", b"{}")
        return agent, nats, calls

    async def test_trusted_modes_ignore_legacy_keys_and_require_pins(self) -> None:
        for mode in ("warn", "enforce"):
            config, scheduler_key = trust_material()
            attacker = ec.generate_private_key(ec.SECP256R1())
            cases = (
                ("unsigned", job_packet(config.expected_scheduler_id), None, 0),
                ("wrong sender", job_packet("scheduler-attacker", scheduler_key), None, 0),
                ("legacy override", job_packet(config.expected_scheduler_id, attacker),
                 {config.expected_scheduler_id: attacker.public_key()}, 0),
                ("pinned scheduler", job_packet(config.expected_scheduler_id, scheduler_key),
                 None, 1),
            )
            for name, packet, legacy_keys, expected in cases:
                with self.subTest(mode=mode, case=name):
                    agent, _, calls = await self._start(mode, config, legacy_keys)
                    try:
                        await agent._on_msg(message(packet), agent._handlers[TOPIC])
                        self.assertEqual(expected, len(calls))
                    finally:
                        await agent.close()

    async def test_enforce_requires_live_session_but_warn_does_not(self) -> None:
        for mode, expected in (("warn", 1), ("enforce", 0)):
            config, scheduler_key = trust_material()
            agent, _, calls = await self._start(mode, config)
            try:
                agent._worker_trust._session = session("expired", seconds=-1)
                packet = job_packet(config.expected_scheduler_id, scheduler_key)
                await agent._on_msg(message(packet), agent._handlers[TOPIC])
                self.assertEqual(expected, len(calls))
            finally:
                await agent.close()

    async def test_direct_subject_uses_the_same_pinned_boundary(self) -> None:
        config, scheduler_key = trust_material()
        attacker = ec.generate_private_key(ec.SECP256R1())
        legacy = {config.expected_scheduler_id: attacker.public_key()}
        agent, _, calls = await self._start("enforce", config, legacy)
        try:
            await agent._on_direct_msg(
                message(job_packet(config.expected_scheduler_id, attacker))
            )
            self.assertEqual([], calls)
            await agent._on_direct_msg(
                message(job_packet(config.expected_scheduler_id, scheduler_key))
            )
            self.assertEqual(["handler"], calls)
        finally:
            await agent.close()

    async def test_enforce_does_not_build_tokenless_heartbeat(self) -> None:
        config, _ = trust_material()
        agent, _, _ = await self._start("enforce", config)
        try:
            agent._worker_trust._session = session("expired", seconds=-1)
            with self.assertRaises(WorkerTrustRuntimeError):
                agent._default_heartbeat_payload()
        finally:
            await agent.close()

    async def test_enforce_does_not_publish_tokenless_result(self) -> None:
        config, _ = trust_material()
        agent, nats, _ = await self._start("enforce", config)
        source = job_packet(config.expected_scheduler_id)
        context = Context(
            job=source.job_request,
            packet=source,
            logger=logging.LoggerAdapter(logging.getLogger(), {}),
        )
        before = len(nats.published)
        try:
            agent._worker_trust._session = session("expired", seconds=-1)
            with self.assertRaises(WorkerTrustRuntimeError):
                await agent._publish_result(
                    context,
                    job_pb2.JobResult(
                        job_id=source.job_request.job_id,
                        status=job_pb2.JOB_STATUS_SUCCEEDED,
                        worker_id=WORKER_ID,
                    ),
                )
            self.assertEqual(before, len(nats.published))
        finally:
            await agent.close()


if __name__ == "__main__":
    unittest.main()
