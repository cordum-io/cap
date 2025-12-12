import asyncio
import sys
import os
import unittest
from typing import Callable, Awaitable, Dict

# Avoid loading duplicate generated stubs from both /python and sdk/python/cap/pb.
sys.path = [p for p in sys.path if not p.rstrip("/").endswith("python")]
# Ensure generated modules under sdk/python/cap/pb are discoverable for `coretex.agent.v1.*`.
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "cap", "pb")))

from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes

from cap.pb.coretex.agent.v1 import job_pb2
from cap import client
from cap import worker


class MockNATS:
    def __init__(self):
        self.subscriptions = {}
        self.published = asyncio.Queue()

    async def publish(self, subject, data):
        await self.published.put((subject, data))

    async def subscribe(self, subject, queue, cb):
        self.subscriptions[subject] = cb

    async def connect(self, servers, name):
        return self

    async def drain(self):
        pass


class TestSDK(unittest.TestCase):
    def test_e2e(self):
        async def run_test():
            client_key = ec.generate_private_key(ec.SECP256R1())
            worker_key = ec.generate_private_key(ec.SECP256R1())

            mock_nats = MockNATS()

            async def handler(req: job_pb2.JobRequest) -> job_pb2.JobResult:
                return job_pb2.JobResult(status=job_pb2.JOB_STATUS_SUCCEEDED)

            worker_task = asyncio.create_task(
                worker.run_worker(
                    nats_url="",
                    subject="test.worker",
                    handler=handler,
                    public_keys={"test-client": client_key.public_key()},
                    private_key=worker_key,
                    sender_id="test-worker",
                    connect_fn=mock_nats.connect
                )
            )

            # Wait until the worker subscription is registered on the mock bus.
            for _ in range(20):
                if "test.worker" in mock_nats.subscriptions:
                    break
                await asyncio.sleep(0)
            self.assertIn("test.worker", mock_nats.subscriptions)

            job_request = job_pb2.JobRequest(job_id="test-job-1", topic="test.worker")
            await client.submit_job(
                mock_nats,
                job_request,
                "test-trace",
                "test-client",
                client_key
            )

            # Get the published job and send it to the worker
            subj, data = await mock_nats.published.get()
            self.assertEqual(subj, client.SUBJECT_SUBMIT)
            await mock_nats.subscriptions["test.worker"](type("obj", (object,), {"data": data})())

            # Get the result from the worker
            subj, data = await mock_nats.published.get()
            self.assertEqual(subj, worker.SUBJECT_RESULT)

            # Verify the signature of the result
            from cap.pb.coretex.agent.v1 import buspacket_pb2
            import hashlib
            result_packet = buspacket_pb2.BusPacket()
            result_packet.ParseFromString(data)
            signature = result_packet.signature
            result_packet.ClearField("signature")
            unsigned_data = result_packet.SerializeToString()
            digest = hashlib.sha256(unsigned_data).digest()
            worker_key.public_key().verify(signature, digest, ec.ECDSA(hashes.SHA256()))

            worker_task.cancel()

        asyncio.run(run_test())


if __name__ == "__main__":
    unittest.main()
