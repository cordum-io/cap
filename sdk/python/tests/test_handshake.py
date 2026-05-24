import asyncio
import unittest

from cap.runtime import Agent, InMemoryBlobStore
from cap.pb.cordum.agent.v1 import buspacket_pb2, handshake_pb2


class MockNATS:
    def __init__(self):
        self.subscriptions = {}
        self.published = asyncio.Queue()

    async def publish(self, subject, data):
        await self.published.put((subject, data))

    async def subscribe(self, subject, queue="", cb=None):
        self.subscriptions[subject] = cb

    async def connect(self, servers, name):
        return self

    async def drain(self):
        return None


class TestHandshake(unittest.IsolatedAsyncioTestCase):
    async def test_agent_start_publishes_handshake(self):
        store = InMemoryBlobStore()
        mock = MockNATS()
        agent = Agent(store=store, connect_fn=mock.connect, sender_id="worker-handshake")

        @agent.job("job.handshake")
        async def handler(_ctx, _data):
            return {}

        @agent.job("job.handshake.secondary")
        async def secondary_handler(_ctx, _data):
            return {}

        await agent.start()

        subject, payload = await asyncio.wait_for(mock.published.get(), timeout=1)
        self.assertEqual(subject, "sys.handshake")

        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)
        self.assertEqual(packet.sender_id, "worker-handshake")
        self.assertEqual(packet.handshake.component_id, "worker-handshake")
        self.assertEqual(packet.handshake.role, handshake_pb2.COMPONENT_ROLE_WORKER)
        self.assertEqual(list(packet.handshake.supported_versions), [1])
        self.assertTrue(packet.handshake.capabilities["job.handshake"])
        self.assertTrue(packet.handshake.capabilities["job.handshake.secondary"])
        self.assertEqual(
            list(packet.handshake.ready_topics),
            ["job.handshake", "job.handshake.secondary"],
        )

        await agent.close()


class TestHandshakePayloadAgentName(unittest.TestCase):
    def test_handshake_payload_with_agent_name(self):
        from cap.handshake import handshake_payload

        payload = handshake_payload(
            component_id="worker-named",
            capabilities={"job.x": True},
            agent_name="  Claude Code\n— Billing  ",
        )
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)
        # Sanitized display label round-trips.
        self.assertEqual(packet.handshake.agent_name, "Claude Code — Billing")

    def test_handshake_payload_agent_name_optional_for_old_clients(self):
        from cap.handshake import handshake_payload

        payload = handshake_payload(component_id="worker-x")
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(payload)
        self.assertEqual(packet.handshake.agent_name, "")


if __name__ == "__main__":
    unittest.main()
