import unittest

from cap.pb.cordum.agent.v1 import job_pb2
from cap.runtime import Context
from cap.testing import MockNATS, create_test_agent, run_handler


class TestTestingUtilities(unittest.IsolatedAsyncioTestCase):
    async def test_run_handler_echo(self):
        async def echo_handler(ctx: Context, data):
            return {"echo": data["prompt"]}

        result = await run_handler(
            echo_handler,
            {"prompt": "hello"},
            job_id="test-1",
            topic="job.echo",
        )

        self.assertEqual(result.status, job_pb2.JOB_STATUS_SUCCEEDED)
        self.assertEqual(result.job_id, "test-1")

    async def test_run_handler_failure(self):
        async def failing_handler(ctx: Context, data):
            raise RuntimeError("boom")

        result = await run_handler(
            failing_handler,
            {"prompt": "hello"},
            job_id="test-2",
            topic="job.fail",
        )

        self.assertEqual(result.status, job_pb2.JOB_STATUS_FAILED)
        self.assertIn("boom", result.error_message)

    async def test_create_test_agent(self):
        agent, mock, store = create_test_agent()
        self.assertIsNotNone(agent)
        self.assertIsInstance(mock, MockNATS)
        self.assertIsNotNone(store)


if __name__ == "__main__":
    unittest.main()
