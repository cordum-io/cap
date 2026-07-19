"""Behavioral contract tests for Agent startup and graceful shutdown."""

import asyncio
import logging
import unittest
from dataclasses import dataclass
from typing import Dict, List, Tuple

from cap.runtime import Agent, Context

from runtime_support import RecordingNATS, RecordingStore, make_job_message


TOPIC = "job.lifecycle"
WORKER_ID = "lifecycle-worker"
TIMEOUT = 1.0


@dataclass
class BlockedJob:
    started: asyncio.Event
    release: asyncio.Event
    finished: asyncio.Event


def make_agent(
    nats: RecordingNATS,
    store: RecordingStore,
    shutdown_timeout: float = 1.0,
) -> Agent:
    logger = logging.getLogger("cap.runtime.lifecycle." + str(id(nats)))
    logger.handlers = [logging.NullHandler()]
    logger.propagate = False
    return Agent(
        store=store,
        connect_fn=nats.connect,
        sender_id=WORKER_ID,
        heartbeat_interval=60,
        shutdown_timeout=shutdown_timeout,
        logger=logger,
    )


def register_blocked_handler(agent: Agent, events: List[str]) -> BlockedJob:
    control = BlockedJob(asyncio.Event(), asyncio.Event(), asyncio.Event())

    @agent.job(TOPIC)
    async def handler(context: Context, data: object) -> Dict[str, bool]:
        del context, data
        events.append("handler-start")
        control.started.set()
        await control.release.wait()
        events.append("handler-finish")
        control.finished.set()
        return {"ok": True}

    return control


class TestRuntimeLifecycle(unittest.IsolatedAsyncioTestCase):
    async def _blocked_fixture(
        self,
    ) -> Tuple[Agent, RecordingNATS, RecordingStore, BlockedJob, List[str]]:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        control = register_blocked_handler(agent, events)
        await agent.start()
        events.clear()
        return agent, nats, store, control, events

    async def _exercise_inflight_close(self, delivery_subject: str) -> None:
        agent, nats, store, control, events = await self._blocked_fixture()
        job_id = "job-" + delivery_subject.replace(".", "-")
        await store.set("ctx:" + job_id, b"{}")
        await nats.deliver(delivery_subject, make_job_message(TOPIC, job_id))
        await asyncio.wait_for(control.started.wait(), TIMEOUT)
        close_task = asyncio.create_task(agent.close())

        try:
            await nats.wait_for_drains(2, TIMEOUT)
            self.assertFalse(close_task.done())
            self.assertEqual(0, nats.connection_drain_count)
            self.assertEqual(0, store.close_count)
        finally:
            control.release.set()
            await asyncio.wait_for(control.finished.wait(), TIMEOUT)
            await nats.wait_for_result(TIMEOUT)
            await asyncio.wait_for(close_task, TIMEOUT)

        self.assertEqual(2, len(nats.handles))
        self.assertTrue(all(handle.drain_count == 1 for handle in nats.handles))
        handler_finish = events.index("handler-finish")
        self.assertTrue(all(events.index("sub-drain:" + handle.subject) < handler_finish
                            for handle in nats.handles))
        terminal_order = [event for event in events if event in {
            "handler-finish", "result-publish", "connection-drain", "store-close"
        }]
        self.assertEqual([
            "handler-finish", "result-publish", "connection-drain", "store-close"
        ], terminal_order)

    async def test_close_waits_for_topic_and_direct_inflight_jobs(self) -> None:
        for subject in (TOPIC, "worker." + WORKER_ID + ".jobs"):
            with self.subTest(subject=subject):
                await self._exercise_inflight_close(subject)

    async def test_close_is_concurrent_and_repeat_idempotent(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        register_blocked_handler(agent, events)
        await agent.start()

        await asyncio.wait_for(asyncio.gather(agent.close(), agent.close()), TIMEOUT)
        await asyncio.wait_for(agent.close(), TIMEOUT)

        self.assertEqual(2, len(nats.handles))
        self.assertTrue(all(handle.drain_count == 1 for handle in nats.handles))
        self.assertEqual(1, nats.connection_drain_count)
        self.assertEqual(1, store.close_count)

    async def test_duplicate_start_rejected_before_second_connect(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        register_blocked_handler(agent, events)
        await agent.start()

        try:
            with self.assertRaises(RuntimeError):
                await agent.start()
            self.assertEqual(1, nats.connect_count)
        finally:
            await agent.close()

    async def test_concurrent_start_has_exactly_one_winner(self) -> None:
        events: List[str] = []
        gate = asyncio.Event()
        nats = RecordingNATS(events, connect_gate=gate)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        register_blocked_handler(agent, events)
        first = asyncio.create_task(agent.start())
        await asyncio.wait_for(nats.connect_entered.wait(), TIMEOUT)
        second = asyncio.create_task(agent.start())
        await asyncio.sleep(0)
        gate.set()

        results = await asyncio.wait_for(
            asyncio.gather(first, second, return_exceptions=True), TIMEOUT
        )
        try:
            errors = [result for result in results if isinstance(result, BaseException)]
            self.assertEqual(1, sum(result is None for result in results))
            self.assertEqual(1, len(errors))
            self.assertIsInstance(errors[0], RuntimeError)
            self.assertEqual(1, nats.connect_count)
        finally:
            await agent.close()

    async def test_close_while_starting_is_rejected_without_resource_race(self) -> None:
        events: List[str] = []
        gate = asyncio.Event()
        nats = RecordingNATS(events, connect_gate=gate)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        register_blocked_handler(agent, events)
        start_task = asyncio.create_task(agent.start())
        await asyncio.wait_for(nats.connect_entered.wait(), TIMEOUT)

        try:
            with self.assertRaises(RuntimeError):
                await agent.close()
            self.assertEqual(0, nats.connection_drain_count)
            self.assertEqual(0, store.close_count)
        finally:
            gate.set()
            await asyncio.wait_for(start_task, TIMEOUT)
            await agent.close()

        self.assertEqual(1, nats.connection_drain_count)
        self.assertEqual(1, store.close_count)

    async def test_partial_start_failure_cleans_acquired_resources(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events, fail_subscribe_at=2)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        register_blocked_handler(agent, events)

        try:
            with self.assertRaises(RuntimeError) as raised:
                await agent.start()
            self.assertIs(raised.exception, nats.subscribe_error)
            self.assertEqual([1], [handle.drain_count for handle in nats.handles])
            self.assertEqual(1, nats.connection_drain_count)
            self.assertEqual(1, store.close_count)
            await agent.close()
            self.assertEqual(1, nats.connection_drain_count)
            self.assertEqual(1, store.close_count)
        finally:
            if nats.connection_drain_count == 0 or store.close_count == 0:
                await agent.close()

    async def test_close_preserves_first_error_and_continues_all_stages(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        register_blocked_handler(agent, events)
        await agent.start()
        first_error = RuntimeError("subscription drain failed")
        nats.handles[0].drain_error = first_error
        nats.connection_drain_error = RuntimeError("connection drain failed")
        store.close_error = RuntimeError("store close failed")

        for _ in range(2):
            with self.assertRaises(RuntimeError) as raised:
                await agent.close()
            self.assertIs(raised.exception, first_error)

        self.assertTrue(all(handle.drain_count == 1 for handle in nats.handles))
        self.assertEqual(1, nats.connection_drain_count)
        self.assertEqual(1, store.close_count)

    async def test_close_timeout_cancels_stuck_handler_and_closes_resources(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        agent = make_agent(nats, store, shutdown_timeout=0.05)
        control = register_blocked_handler(agent, events)
        job_id = "job-timeout"
        await store.set("ctx:" + job_id, b"{}")
        await agent.start()
        await nats.deliver(TOPIC, make_job_message(TOPIC, job_id))
        await asyncio.wait_for(control.started.wait(), TIMEOUT)

        with self.assertRaises(asyncio.TimeoutError):
            await asyncio.wait_for(agent.close(), TIMEOUT)

        self.assertEqual(1, nats.connection_drain_count)
        self.assertEqual(1, store.close_count)
        self.assertEqual(0, nats.result_count())

    async def test_run_cancellation_waits_for_terminal_result(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        agent = make_agent(nats, store)
        control = register_blocked_handler(agent, events)
        job_id = "job-run-cancel"
        await store.set("ctx:" + job_id, b"{}")
        run_task = asyncio.create_task(agent.run())
        await nats.wait_for_subscription(TOPIC, TIMEOUT)
        events.clear()
        await nats.deliver(TOPIC, make_job_message(TOPIC, job_id))
        await asyncio.wait_for(control.started.wait(), TIMEOUT)
        run_task.cancel()

        try:
            await asyncio.sleep(0.05)
            self.assertFalse(run_task.done())
            self.assertEqual(0, nats.connection_drain_count)
            self.assertEqual(0, store.close_count)
        finally:
            control.release.set()
            await asyncio.wait_for(control.finished.wait(), TIMEOUT)
            await nats.wait_for_result(TIMEOUT)
            with self.assertRaises(asyncio.CancelledError):
                await asyncio.wait_for(run_task, TIMEOUT)

        self.assertEqual(1, nats.result_count())
        self.assertEqual(2, len(nats.handles))
        self.assertTrue(all(handle.drain_count == 1 for handle in nats.handles))
        self.assertEqual(1, nats.connection_drain_count)
        self.assertEqual(1, store.close_count)
        await agent.close()
        self.assertEqual(1, nats.connection_drain_count)
        self.assertEqual(1, store.close_count)


if __name__ == "__main__":
    unittest.main()
