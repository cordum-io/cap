import asyncio
import logging
import unittest
from typing import List

from cap.pb.cordum.agent.v1 import job_pb2
from cap.subjects import SUBJECT_RESULT
from cap.worker import run_worker

from worker_support import (
    RecordingMetrics,
    RecordingNATS,
    decode_result,
    job_packet,
)


class ListHandler(logging.Handler):
    def __init__(self):
        super().__init__()
        self.records: List[logging.LogRecord] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.records.append(record)


class TestWorkerFailureLifecycle(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.bus = RecordingNATS()
        self.metrics = RecordingMetrics()
        self.log_handler = ListHandler()
        self.logger = logging.getLogger("cap.worker.failure-tests")
        self.logger.handlers = [self.log_handler]
        self.logger.propagate = False
        self.tasks: List[asyncio.Task] = []

    async def asyncTearDown(self) -> None:
        for task in self.tasks:
            if not task.done():
                task.cancel()
        await asyncio.gather(*self.tasks, return_exceptions=True)
        self.logger.handlers = []

    async def start_worker(self, handler, middlewares=None):
        task = asyncio.create_task(
            run_worker(
                "nats://test",
                "job.test",
                handler,
                sender_id="worker-1",
                connect_fn=self.bus.connect,
                logger=self.logger,
                metrics=self.metrics,
                middlewares=middlewares,
            )
        )
        self.tasks.append(task)
        await asyncio.wait_for(self.bus.subscription_ready.wait(), 1)
        return task

    def assert_result_identity(self, packet, job_id, trace_id):
        self.assertEqual(packet.trace_id, trace_id)
        self.assertEqual(packet.job_result.job_id, job_id)
        self.assertEqual(packet.job_result.worker_id, "worker-1")

    async def test_handler_exception_publishes_safe_failure_and_continues(self):
        secret = "secret=do-not-leak"
        calls = []

        async def handler(request):
            calls.append(request.job_id)
            if request.job_id == "bad-job":
                raise RuntimeError(secret)
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        worker_task = await self.start_worker(handler)
        await self.bus.deliver("job.test", job_packet("bad-job", "job.test", "trace-bad"))

        self.assertEqual(len(self.bus.publications), 1)
        packet = decode_result(self.bus.publications[0])
        self.assert_result_identity(packet, "bad-job", "trace-bad")
        self.assertEqual(packet.job_result.status, job_pb2.JOB_STATUS_FAILED)
        self.assertEqual(packet.job_result.error_message, "handler failed")
        self.assertNotIn(secret, self.bus.publications[0].data.decode(errors="ignore"))
        self.assertEqual(self.metrics.failed, [("bad-job", "handler failed")])
        self.assertEqual(len(self.log_handler.records), 1)
        record = self.log_handler.records[0]
        self.assertEqual(record.getMessage(), "job handler failed")
        self.assertNotIn(secret, record.getMessage())
        self.assertFalse(record.exc_info)
        self.assertEqual(record.job_id, "bad-job")
        self.assertEqual(record.trace_id, "trace-bad")
        self.assertEqual(record.exception_type, "RuntimeError")

        await self.bus.deliver("job.test", job_packet("good-job", "job.test", "trace-good"))
        good = decode_result(self.bus.publications[1])
        self.assert_result_identity(good, "good-job", "trace-good")
        self.assertEqual(good.job_result.status, job_pb2.JOB_STATUS_SUCCEEDED)
        self.assertEqual(calls, ["bad-job", "good-job"])
        self.assertFalse(worker_task.done())

    async def test_none_result_publishes_one_failed_result(self):
        async def handler(_request):
            return None

        await self.start_worker(handler)
        await self.bus.deliver("job.test", job_packet("null-job", "job.test", "trace-null"))

        self.assertEqual(len(self.bus.publish_attempts), 1)
        packet = decode_result(self.bus.publications[0])
        self.assert_result_identity(packet, "null-job", "trace-null")
        self.assertEqual(packet.job_result.status, job_pb2.JOB_STATUS_FAILED)
        self.assertEqual(packet.job_result.error_message, "handler returned null")
        self.assertEqual(self.metrics.failed, [("null-job", "handler returned null")])
        self.assertEqual(self.metrics.completed, [])

    async def test_low_level_middlewares_keep_fifo_order(self):
        order = []

        async def handler(_request):
            order.append("handler")
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        def middleware(name):
            async def invoke(request, next_handler):
                order.append(name + "-before")
                result = await next_handler(request)
                order.append(name + "-after")
                return result

            return invoke

        await self.start_worker(handler, [middleware("a"), middleware("b")])
        await self.bus.deliver(
            "job.test", job_packet("mw-job", "job.test", "trace-mw")
        )
        self.assertEqual(
            order,
            ["a-before", "b-before", "handler", "b-after", "a-after"],
        )

    async def test_cancellation_propagates_and_drains_once(self):
        async def handler(_request):
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        worker_task = await self.start_worker(handler)
        worker_task.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await worker_task

        self.assertEqual(self.bus.connect_calls, [("nats://test", "worker-1")])
        self.assertEqual(self.bus.subscribe_calls, [("job.test", "job.test")])
        self.assertEqual(self.bus.drain_calls, 1)
        self.assertTrue(self.bus.drain_finished.is_set())
        self.assertEqual(self.bus.publish_attempts, [])

    async def test_handler_cancellation_propagates_without_terminal_result(self):
        async def handler(_request):
            raise asyncio.CancelledError()

        worker_task = await self.start_worker(handler)
        with self.assertRaises(asyncio.CancelledError):
            await self.bus.deliver(
                "job.test", job_packet("cancel-job", "job.test", "trace-cancel")
            )

        self.assertEqual(self.bus.publish_attempts, [])
        self.assertEqual(self.metrics.failed, [])
        self.assertEqual(self.metrics.completed, [])
        self.assertEqual(self.metrics.received, [("cancel-job", "job.test")])
        self.assertFalse(worker_task.done())

    async def test_cancellation_waits_for_in_flight_result(self):
        started = asyncio.Event()
        release = asyncio.Event()

        async def handler(_request):
            started.set()
            await release.wait()
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        worker_task = await self.start_worker(handler)
        delivery = asyncio.create_task(
            self.bus.deliver("job.test", job_packet("slow-job", "job.test", "trace-slow"))
        )
        self.tasks.append(delivery)
        await asyncio.wait_for(started.wait(), 1)
        worker_task.cancel()
        await asyncio.wait_for(self.bus.drain_started.wait(), 1)
        self.assertFalse(worker_task.done())
        self.assertFalse(self.bus.drain_finished.is_set())
        release.set()
        await asyncio.wait_for(delivery, 1)
        with self.assertRaises(asyncio.CancelledError):
            await worker_task
        self.assertEqual(len(self.bus.publications), 1)
        self.assertLess(self.bus.events.index("publish"), self.bus.events.index("drain-finish"))

    async def test_publish_failure_is_not_retried_as_failed_result(self):
        class PublishError(RuntimeError):
            pass

        error = PublishError("publish unavailable")
        self.bus.next_publish_error = error

        async def handler(_request):
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        worker_task = await self.start_worker(handler)
        with self.assertRaises(PublishError) as raised:
            await self.bus.deliver("job.test", job_packet("job-1", "job.test", "trace-1"))
        self.assertIs(raised.exception, error)
        self.assertEqual(len(self.bus.publish_attempts), 1)
        attempted = decode_result(self.bus.publish_attempts[0])
        self.assertEqual(attempted.job_result.status, job_pb2.JOB_STATUS_SUCCEEDED)
        self.assertEqual(self.metrics.failed, [])
        self.assertFalse(worker_task.done())

        await self.bus.deliver("job.test", job_packet("job-2", "job.test", "trace-2"))
        self.assertEqual(len(self.bus.publish_attempts), 2)
        self.assertEqual(len(self.bus.publications), 1)
        self.assertEqual(decode_result(self.bus.publications[0]).job_result.job_id, "job-2")

    async def test_subscribe_failure_cleans_connection_and_preserves_error(self):
        class SubscribeError(RuntimeError):
            pass

        error = SubscribeError("subscription unavailable")
        self.bus.subscribe_error = error

        async def handler(_request):
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        with self.assertRaises(SubscribeError) as raised:
            await asyncio.wait_for(
                run_worker(
                    "nats://test",
                    "job.test",
                    handler,
                    sender_id="worker-1",
                    connect_fn=self.bus.connect,
                ),
                1,
            )
        self.assertIs(raised.exception, error)
        self.assertEqual(self.bus.subscribe_calls, [("job.test", "job.test")])
        self.assertEqual(self.bus.drain_calls, 1)
        self.assertTrue(self.bus.drain_finished.is_set())
        self.assertEqual(self.bus.publish_attempts, [])

    async def test_cleanup_cancellation_does_not_mask_subscribe_error(self):
        class SubscribeError(RuntimeError):
            pass

        error = SubscribeError("subscription unavailable")
        self.bus.subscribe_error = error
        self.bus.drain_error = asyncio.CancelledError()

        async def handler(_request):
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        with self.assertRaises(SubscribeError) as raised:
            await run_worker(
                "nats://test",
                "job.test",
                handler,
                sender_id="worker-1",
                connect_fn=self.bus.connect,
                logger=self.logger,
            )
        self.assertIs(raised.exception, error)
        self.assertEqual(self.bus.drain_calls, 1)
        cleanup = self.log_handler.records[-1]
        self.assertEqual(cleanup.getMessage(), "worker connection drain failed")
        self.assertEqual(cleanup.exception_type, "CancelledError")
        self.assertFalse(cleanup.exc_info)


if __name__ == "__main__":
    unittest.main()
