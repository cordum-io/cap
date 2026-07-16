"""Security and deadline regressions for the high-level Agent runtime."""

import asyncio
import logging
import unittest
from typing import Dict, List, Optional, Tuple

from cap.metrics import MetricsHook
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.runtime import Agent, Context

from runtime_support import RecordingNATS, RecordingStore, make_job_message


TOPIC = "job.hardening"
WAIT = 1.0


class CaptureHandler(logging.Handler):
    def __init__(self) -> None:
        super().__init__()
        self.records: List[logging.LogRecord] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.records.append(record)


class ExplodingHandler(logging.Handler):
    def emit(self, record: logging.LogRecord) -> None:
        raise RuntimeError("logging backend failed")


class FailingMetrics(MetricsHook):
    def on_job_received(self, job_id: str, topic: str) -> None:
        raise RuntimeError("metrics-secret-received")

    def on_job_completed(self, job_id: str, duration_ms: int, status: str) -> None:
        raise RuntimeError("metrics-secret-completed")

    def on_job_failed(self, job_id: str, error_msg: str) -> None:
        raise RuntimeError("metrics-secret-failed")

    def on_heartbeat_sent(self, worker_id: str) -> None:
        raise RuntimeError("metrics-secret-heartbeat")


def decode_results(nats: RecordingNATS) -> List[job_pb2.JobResult]:
    results: List[job_pb2.JobResult] = []
    for subject, payload in nats.published:
        if subject != "sys.job.result":
            continue
        packet = buspacket_pb2.BusPacket.FromString(payload)
        result = job_pb2.JobResult()
        result.CopyFrom(packet.job_result)
        results.append(result)
    return results


def make_agent(
    nats: RecordingNATS,
    store: RecordingStore,
    logger: logging.Logger,
    *,
    timeout: float = 0.05,
    metrics: Optional[MetricsHook] = None,
) -> Agent:
    return Agent(
        store=store,
        connect_fn=nats.connect,
        sender_id="runtime-worker",
        heartbeat_interval=60,
        shutdown_timeout=timeout,
        logger=logger,
        metrics=metrics,
    )


class TestRuntimeHardening(unittest.IsolatedAsyncioTestCase):
    def setUp(self) -> None:
        self.events: List[str] = []
        self.nats = RecordingNATS(self.events)
        self.store = RecordingStore(self.events)
        self.capture = CaptureHandler()
        self.logger = logging.getLogger("cap.runtime.hardening." + str(id(self)))
        self.logger.handlers = [self.capture, ExplodingHandler()]
        self.logger.propagate = False

    def tearDown(self) -> None:
        self.logger.handlers = []

    def test_nonpositive_shutdown_timeout_is_rejected(self) -> None:
        for timeout in (0.0, -1.0):
            with self.subTest(timeout=timeout), self.assertRaises(ValueError):
                make_agent(self.nats, self.store, self.logger, timeout=timeout)

    def test_constructor_does_not_dispatch_to_subclass_lifecycle_hook(self) -> None:
        class ExistingSubclass(Agent):
            def _initialize_lifecycle(self) -> None:
                raise AssertionError("base construction dispatched to subclass")

        agent = ExistingSubclass(store=self.store, connect_fn=self.nats.connect)
        self.assertEqual("idle", agent._lifecycle_state)

    async def test_handler_exception_is_safe_and_metrics_cannot_block_result(self) -> None:
        secret = "handler-secret-do-not-leak"
        bad_id = "bad\r\nforged" + ("x" * 300)
        calls: List[str] = []
        agent = make_agent(
            self.nats, self.store, self.logger, metrics=FailingMetrics()
        )

        @agent.job(TOPIC)
        async def handler(context: Context, data: Dict[str, bool]) -> Dict[str, bool]:
            calls.append(context.job_id)
            if data["fail"]:
                raise RuntimeError(secret)
            return {"ok": True}

        await self.store.set("ctx:" + bad_id, b'{"fail": true}')
        await self.store.set("ctx:good", b'{"fail": false}')
        await agent.start()
        try:
            await self.nats.deliver(TOPIC, make_job_message(TOPIC, bad_id))
            await self.nats.wait_for_result(WAIT)
            self.nats.result_published.clear()
            await self.nats.deliver(TOPIC, make_job_message(TOPIC, "good"))
            await self.nats.wait_for_result(WAIT)
        finally:
            await agent.close()

        results = decode_results(self.nats)
        self.assertEqual([bad_id, "good"], calls)
        self.assertEqual(2, len(results))
        self.assertEqual(job_pb2.JOB_STATUS_FAILED, results[0].status)
        self.assertEqual("handler failed", results[0].error_message)
        self.assertEqual(job_pb2.JOB_STATUS_SUCCEEDED, results[1].status)
        rendered = "\n".join(record.getMessage() for record in self.capture.records)
        self.assertNotIn(secret, rendered)
        handler_record = next(
            record for record in self.capture.records
            if record.getMessage() == "handler failed"
        )
        for field in (handler_record.job_id, handler_record.trace_id):
            self.assertLessEqual(len(field), 256)
            self.assertNotIn("\r", field)
            self.assertNotIn("\n", field)
            self.assertTrue(field.endswith("..."))

    async def test_close_deadline_survives_cancellation_resistant_handler(self) -> None:
        started, release = asyncio.Event(), asyncio.Event()
        agent = make_agent(self.nats, self.store, self.logger)

        @agent.job(TOPIC)
        async def handler(context: Context, data: object) -> Dict[str, bool]:
            del context, data
            started.set()
            try:
                await release.wait()
            except asyncio.CancelledError:
                await release.wait()
            return {"ok": True}

        await self.store.set("ctx:stuck", b"{}")
        await agent.start()
        await self.nats.deliver(TOPIC, make_job_message(TOPIC, "stuck"))
        await asyncio.wait_for(started.wait(), WAIT)
        handler_tasks = tuple(agent._handler_tasks)
        close_task = asyncio.create_task(agent.close())
        done, _ = await asyncio.wait({close_task}, timeout=0.25)
        if close_task not in done:
            release.set()
            await asyncio.gather(close_task, return_exceptions=True)
        self.assertIn(close_task, done, "close exceeded its configured deadline")
        with self.assertRaises(asyncio.TimeoutError):
            await close_task
        self.assertEqual(1, self.nats.connection_drain_count)
        self.assertEqual(1, self.store.close_count)
        release.set()
        await asyncio.gather(*handler_tasks, return_exceptions=True)

    async def _exercise_stuck_cleanup(self, stage: str) -> Tuple[bool, List[str]]:
        entered, release = asyncio.Event(), asyncio.Event()
        background: List[asyncio.Task[None]] = []
        agent = make_agent(self.nats, self.store, self.logger)

        @agent.job(TOPIC)
        async def handler(context: Context, data: object) -> Dict[str, bool]:
            del context, data
            return {"ok": True}

        async def stubborn() -> None:
            current = asyncio.current_task()
            if current is not None:
                background.append(current)
            entered.set()
            try:
                await release.wait()
            except asyncio.CancelledError:
                await release.wait()

        await agent.start()
        if stage == "subscription":
            agent._subscriptions[0].drain = stubborn
        elif stage == "heartbeat":
            assert agent._heartbeat_cancel_event is not None
            agent._heartbeat_cancel_event.set()
            assert agent._heartbeat_task is not None
            await agent._heartbeat_task
            agent._heartbeat_task = asyncio.create_task(stubborn())
        elif stage == "nats":
            self.nats.drain = stubborn
        else:
            self.store.close = stubborn

        close_task = asyncio.create_task(agent.close())
        await asyncio.wait_for(entered.wait(), WAIT)
        done, _ = await asyncio.wait({close_task}, timeout=0.25)
        completed_in_time = close_task in done
        release.set()
        outcome = await asyncio.gather(close_task, return_exceptions=True)
        await asyncio.gather(*background, return_exceptions=True)
        self.assertIsInstance(outcome[0], asyncio.TimeoutError)
        return completed_in_time, self.events

    async def test_each_cleanup_stage_has_a_deadline(self) -> None:
        for stage in ("subscription", "heartbeat", "nats", "store"):
            with self.subTest(stage=stage):
                self.setUp()
                completed, events = await self._exercise_stuck_cleanup(stage)
                self.assertTrue(completed, f"{stage} cleanup exceeded its deadline")
                if stage == "subscription":
                    self.assertEqual(1, self.nats.handles[1].drain_count)
                    second = "sub-drain:" + self.nats.handles[1].subject
                    self.assertLess(events.index(second), events.index("connection-drain"))
                if stage != "nats":
                    self.assertIn("connection-drain", events)
                if stage != "store":
                    self.assertIn("store-close", events)

    async def test_run_cancellation_survives_cleanup_failure(self) -> None:
        agent = make_agent(self.nats, self.store, self.logger)

        @agent.job(TOPIC)
        async def handler(context: Context, data: object) -> Dict[str, bool]:
            del context, data
            return {"ok": True}

        run_task = asyncio.create_task(agent.run())
        await self.nats.wait_for_subscription(TOPIC, WAIT)
        self.nats.connection_drain_error = RuntimeError("cleanup secret")
        run_task.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await run_task
        self.assertEqual(1, self.nats.connection_drain_count)
        self.assertEqual(1, self.store.close_count)


if __name__ == "__main__":
    unittest.main()
