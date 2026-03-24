"""Testing utilities for CAP Python SDK.

Allows testing handlers without running NATS or Redis.
"""

import asyncio
import json
from typing import Any, Awaitable, Callable, Dict, List, Optional, Tuple, Type, Union

from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.runtime import Agent, Context, InMemoryBlobStore


class MockNATS:
    """In-memory mock that replaces a real NATS connection for testing."""

    def __init__(self) -> None:
        self.subscriptions: Dict[str, Any] = {}
        self.published: "asyncio.Queue[Tuple[str, bytes]]" = asyncio.Queue()

    async def publish(self, subject: str, data: bytes) -> None:
        await self.published.put((subject, data))

    async def subscribe(self, subject: str, queue: str, cb: Any) -> None:
        self.subscriptions[subject] = cb

    async def connect(self, servers: str, name: str) -> "MockNATS":
        return self

    async def drain(self) -> None:
        return None


def create_test_agent(
    *,
    sender_id: str = "test-worker",
    retries: int = 0,
    **kwargs: Any,
) -> Tuple[Agent, MockNATS, InMemoryBlobStore]:
    """Create an Agent pre-configured with MockNATS + InMemoryBlobStore.

    Returns (agent, mock_nats, store). No NATS or Redis required.
    """
    mock = MockNATS()
    store = InMemoryBlobStore()
    agent = Agent(
        store=store,
        connect_fn=mock.connect,
        sender_id=sender_id,
        retries=retries,
        **kwargs,
    )
    return agent, mock, store


async def run_handler(
    handler: Callable[..., Union[Awaitable[Any], Any]],
    input_data: Any,
    *,
    input_model: Optional[Type[Any]] = None,
    output_model: Optional[Type[Any]] = None,
    job_id: str = "test-job-1",
    topic: str = "job.test",
    sender_id: str = "test-worker",
    retries: int = 0,
) -> job_pb2.JobResult:
    """Run a single handler invocation and return the JobResult.

    No NATS or Redis required.

    Args:
        handler: The handler function to test.
        input_data: The input data (will be JSON-serialized into the blob store).
        input_model: Optional Pydantic model for input validation.
        output_model: Optional Pydantic model for output validation.
        job_id: Job ID for the test request.
        topic: Topic for the test request.
        sender_id: Worker sender ID.
        retries: Number of retries on handler failure.

    Returns:
        The JobResult produced by the handler.
    """
    agent, mock, store = create_test_agent(sender_id=sender_id, retries=retries)

    @agent.job(topic, input_model=input_model, output_model=output_model)
    async def _wrapper(ctx: Context, data: Any) -> Any:
        result = handler(ctx, data)
        if asyncio.iscoroutine(result):
            return await result
        return result

    await agent.start()

    ctx_key = f"ctx:{job_id}"
    await store.set(ctx_key, json.dumps(input_data).encode("utf-8"))

    req = job_pb2.JobRequest(
        job_id=job_id,
        topic=topic,
        context_ptr=f"redis://{ctx_key}",
    )
    packet = buspacket_pb2.BusPacket()
    packet.trace_id = "test-trace"
    packet.sender_id = "test-client"
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.job_request.CopyFrom(req)

    cb = mock.subscriptions.get(topic)
    if cb is None:
        raise RuntimeError(f"no subscription found for topic {topic}")

    msg = type("Msg", (object,), {"data": packet.SerializeToString(deterministic=True)})()
    await cb(msg)

    subject, payload = await asyncio.wait_for(mock.published.get(), timeout=5.0)

    result_packet = buspacket_pb2.BusPacket()
    result_packet.ParseFromString(payload)

    await agent.close()

    return result_packet.job_result
