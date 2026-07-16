"""Concurrency-limit and shutdown regressions for the Agent runtime."""

import asyncio
import logging
import unittest
from typing import Dict, List, cast

from cap.runtime import Agent, Context

from runtime_support import RecordingNATS, RecordingStore, make_job_message


TOPIC = "job.parallelism"
WAIT = 1.0


class TestRuntimeParallelism(unittest.IsolatedAsyncioTestCase):
    def test_max_parallel_rejects_non_integer_values(self) -> None:
        for value in (True, 1.5, float("nan"), "2"):
            with self.subTest(value=value), self.assertRaises(TypeError):
                Agent(max_parallel=cast(int, value))

    async def test_max_parallel_allows_exact_configured_concurrency(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        release, two_started = asyncio.Event(), asyncio.Event()
        active = peak = 0
        agent = Agent(store=store, connect_fn=nats.connect, max_parallel=2)

        @agent.job(TOPIC)
        async def handler(context: Context, data: object) -> Dict[str, bool]:
            nonlocal active, peak
            del context, data
            active += 1
            peak = max(peak, active)
            if active == 2:
                two_started.set()
            await release.wait()
            active -= 1
            return {"ok": True}

        for job_id in ("one", "two", "three"):
            await store.set("ctx:" + job_id, b"{}")
        await agent.start()
        deliveries = [asyncio.create_task(
            nats.deliver(TOPIC, make_job_message(TOPIC, job_id))
        ) for job_id in ("one", "two", "three")]
        try:
            await asyncio.wait_for(two_started.wait(), WAIT)
            await asyncio.sleep(0)
            self.assertEqual(2, sum(task.done() for task in deliveries))
        finally:
            release.set()
            await asyncio.gather(*deliveries, return_exceptions=True)
            await agent.close()
        self.assertEqual(2, peak)
        self.assertEqual(3, nats.result_count())

    async def test_close_flushes_dispatch_waiting_for_parallel_slot(self) -> None:
        events: List[str] = []
        nats = RecordingNATS(events)
        store = RecordingStore(events)
        started, release = asyncio.Event(), asyncio.Event()
        started_jobs: List[str] = []
        logger = logging.getLogger("cap.runtime.parallelism")
        logger.handlers = [logging.NullHandler()]
        logger.propagate = False
        agent = Agent(
            store=store,
            connect_fn=nats.connect,
            max_parallel=1,
            heartbeat_interval=60,
            shutdown_timeout=WAIT,
            logger=logger,
        )

        @agent.job(TOPIC)
        async def handler(context: Context, data: object) -> Dict[str, bool]:
            del data
            started_jobs.append(context.job_id)
            started.set()
            await release.wait()
            return {"ok": True}

        for job_id in ("first", "second"):
            await store.set("ctx:" + job_id, b"{}")
        await agent.start()
        await nats.deliver(TOPIC, make_job_message(TOPIC, "first"))
        await asyncio.wait_for(started.wait(), WAIT)
        second = asyncio.create_task(
            nats.deliver(TOPIC, make_job_message(TOPIC, "second"))
        )
        await asyncio.sleep(0)
        self.assertFalse(second.done())
        close = asyncio.create_task(agent.close())

        try:
            await asyncio.sleep(0.02)
            self.assertFalse(close.done())
        finally:
            release.set()
            await asyncio.wait_for(asyncio.gather(second, close), WAIT)

        self.assertEqual(["first", "second"], started_jobs)
        self.assertEqual(2, nats.result_count())
        self.assertEqual("closed", agent._lifecycle_state)


if __name__ == "__main__":
    unittest.main()
