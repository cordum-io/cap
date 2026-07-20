"""Adversarial compatibility checks for the low-level Python worker helper."""

import asyncio
import logging
import unittest

from cap.worker import run_worker

from worker_support import RecordingNATS


class TestLowLevelCompatibility(unittest.IsolatedAsyncioTestCase):
    async def test_static_session_token_warns_without_exposing_token(self) -> None:
        nats = RecordingNATS()

        async def handler(request):
            raise AssertionError("handler must not run")

        logger = logging.getLogger("cap.test.static-token")
        logger.setLevel(logging.WARNING)
        with self.assertLogs(logger, level="WARNING") as captured:
            task = asyncio.create_task(
                run_worker(
                    "nats://test",
                    "job.test",
                    handler,
                    connect_fn=nats.connect,
                    logger=logger,
                    session_token="do-not-log-this-token",
                )
            )
            await nats.subscription_ready.wait()
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await task

        output = " ".join(captured.output)
        self.assertIn("static compatibility", output)
        self.assertNotIn("do-not-log-this-token", output)


if __name__ == "__main__":
    unittest.main()
