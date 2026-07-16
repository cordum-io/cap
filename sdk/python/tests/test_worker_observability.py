"""Metrics hooks must not control low-level worker terminal results."""

import asyncio
import importlib
import logging
import unittest
from typing import List

from cryptography.hazmat.primitives.asymmetric import ec

from cap.metrics import MetricsHook
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.worker import _verify_packet, run_worker

from worker_support import RecordingNATS, decode_result, job_packet


heartbeat_module = importlib.import_module("cap.heartbeat")


class FailingMetrics(MetricsHook):
    def on_job_received(self, job_id: str, topic: str) -> None:
        raise RuntimeError("metrics received failed")

    def on_job_completed(self, job_id: str, duration_ms: int, status: str) -> None:
        raise RuntimeError("metrics completed failed")

    def on_job_failed(self, job_id: str, error_msg: str) -> None:
        raise RuntimeError("metrics failed failed")

    def on_heartbeat_sent(self, worker_id: str) -> None:
        raise RuntimeError("metrics heartbeat failed")


class ExplodingHandler(logging.Handler):
    def emit(self, record: logging.LogRecord) -> None:
        raise RuntimeError("logging backend failed")


class CaptureHandler(logging.Handler):
    def __init__(self) -> None:
        super().__init__()
        self.records: List[logging.LogRecord] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.records.append(record)


class TestWorkerObservabilityIsolation(unittest.IsolatedAsyncioTestCase):
    def test_signature_failure_log_bounds_untrusted_sender(self) -> None:
        private_key = ec.generate_private_key(ec.SECP256R1())
        sender = "sender\r\nforged" + ("x" * 300)
        packet = buspacket_pb2.BusPacket(sender_id=sender, signature=b"invalid")
        capture = CaptureHandler()
        logger = logging.getLogger("cap.worker.signature-log")
        logger.handlers = [capture]
        logger.propagate = False

        self.assertFalse(_verify_packet(packet, {sender: private_key.public_key()}, logger))
        rendered = logging.Formatter().format(capture.records[0])
        self.assertNotIn("\r", rendered)
        self.assertNotIn("\n", rendered)
        self.assertLessEqual(len(capture.records[0].sender_id), 256)

    async def test_metrics_exceptions_do_not_block_terminal_results(self) -> None:
        bus = RecordingNATS()
        calls: List[str] = []

        async def handler(request: job_pb2.JobRequest) -> job_pb2.JobResult:
            calls.append(request.job_id)
            if request.job_id == "bad":
                raise RuntimeError("handler secret")
            return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

        logger = logging.getLogger("cap.worker.metrics-isolation")
        logger.handlers = [ExplodingHandler()]
        logger.propagate = False
        worker = asyncio.create_task(
            run_worker(
                "nats://test",
                "job.test",
                handler,
                sender_id="worker-1",
                connect_fn=bus.connect,
                logger=logger,
                metrics=FailingMetrics(),
            )
        )
        try:
            await asyncio.wait_for(bus.subscription_ready.wait(), 1)
            await bus.deliver("job.test", job_packet("bad", "job.test", "trace-bad"))
            await bus.deliver("job.test", job_packet("good", "job.test", "trace-good"))
            self.assertEqual(["bad", "good"], calls)
            self.assertEqual(2, len(bus.publications))
            bad, good = (decode_result(item) for item in bus.publications)
            self.assertEqual(job_pb2.JOB_STATUS_FAILED, bad.job_result.status)
            self.assertEqual("handler failed", bad.job_result.error_message)
            self.assertEqual(job_pb2.JOB_STATUS_SUCCEEDED, good.job_result.status)
            self.assertFalse(worker.done())
        finally:
            worker.cancel()
            await asyncio.gather(worker, return_exceptions=True)

    async def test_heartbeat_metrics_failure_is_noncritical_and_nonleaking(self) -> None:
        bus = RecordingNATS()
        cancel = asyncio.Event()
        capture = CaptureHandler()
        logger = logging.getLogger("cap.heartbeat.metrics-isolation")
        logger.handlers = [capture]
        logger.propagate = False

        def payload() -> bytes:
            if len(bus.publications) >= 1:
                cancel.set()
            return heartbeat_module.heartbeat_payload(
                "heartbeat-worker", "default", 0, 1, 0.0
            )

        original = heartbeat_module._logger
        heartbeat_module._logger = logger
        try:
            await asyncio.wait_for(
                heartbeat_module.heartbeat_loop(
                    bus,
                    payload,
                    interval=0,
                    metrics=FailingMetrics(),
                    cancel_event=cancel,
                ),
                1,
            )
        finally:
            heartbeat_module._logger = original

        self.assertGreaterEqual(len(bus.publications), 1)
        self.assertTrue(capture.records)
        self.assertTrue(all(record.exc_info is None for record in capture.records))


if __name__ == "__main__":
    unittest.main()
