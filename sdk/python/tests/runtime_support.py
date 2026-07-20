"""Deterministic lifecycle fakes for the CAP Python runtime tests."""

import asyncio
from dataclasses import dataclass
from typing import Awaitable, Callable, Dict, List, Optional, Tuple

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.runtime import InMemoryBlobStore
from cap.subjects import SUBJECT_RESULT


@dataclass
class Message:
    data: bytes


MessageCallback = Callable[[Message], Awaitable[None]]


class RecordingSubscription:
    def __init__(self, subject: str, events: List[str]) -> None:
        self.subject = subject
        self.events = events
        self.drain_count = 0
        self.drain_error: Optional[BaseException] = None

    async def drain(self) -> None:
        self.drain_count += 1
        self.events.append("sub-drain:" + self.subject)
        if self.drain_error is not None:
            raise self.drain_error


class RecordingStore(InMemoryBlobStore):
    def __init__(self, events: List[str]) -> None:
        super().__init__()
        self.events = events
        self.close_count = 0
        self.close_error: Optional[BaseException] = None

    async def close(self) -> None:
        self.close_count += 1
        self.events.append("store-close")
        if self.close_error is not None:
            raise self.close_error


class RecordingNATS:
    def __init__(
        self,
        events: List[str],
        fail_subscribe_at: Optional[int] = None,
        connect_gate: Optional[asyncio.Event] = None,
    ) -> None:
        self.events = events
        self.fail_subscribe_at = fail_subscribe_at
        self.connect_gate = connect_gate
        self.connect_entered = asyncio.Event()
        self.result_published = asyncio.Event()
        self.subscribe_error = RuntimeError("subscribe failed")
        self.subscriptions: Dict[str, MessageCallback] = {}
        self.handles: List[RecordingSubscription] = []
        self.published: List[Tuple[str, bytes]] = []
        self.connect_count = 0
        self.subscribe_count = 0
        self.connection_drain_count = 0
        self.connection_drain_error: Optional[BaseException] = None

    async def connect(self, servers: str, name: str) -> "RecordingNATS":
        del servers, name
        self.connect_count += 1
        self.connect_entered.set()
        if self.connect_gate is not None:
            await self.connect_gate.wait()
        return self

    async def subscribe(
        self,
        subject: str,
        queue: str = "",
        cb: Optional[MessageCallback] = None,
    ) -> RecordingSubscription:
        del queue
        self.subscribe_count += 1
        if self.subscribe_count == self.fail_subscribe_at:
            raise self.subscribe_error
        if cb is None:
            raise ValueError("message callback is required")
        self.subscriptions[subject] = cb
        handle = RecordingSubscription(subject, self.events)
        self.handles.append(handle)
        return handle

    async def publish(self, subject: str, data: bytes) -> None:
        self.published.append((subject, data))
        if subject == SUBJECT_RESULT:
            self.events.append("result-publish")
            self.result_published.set()

    async def drain(self) -> None:
        self.connection_drain_count += 1
        self.events.append("connection-drain")
        if self.connection_drain_error is not None:
            raise self.connection_drain_error

    async def deliver(self, subject: str, message: Message) -> None:
        await self.subscriptions[subject](message)
        await asyncio.sleep(0)

    async def wait_for_subscription(self, subject: str, timeout: float) -> None:
        async def _wait() -> None:
            while subject not in self.subscriptions:
                await asyncio.sleep(0)

        await asyncio.wait_for(_wait(), timeout)

    async def wait_for_drains(self, minimum: int, timeout: float) -> None:
        async def _wait() -> None:
            while sum(handle.drain_count for handle in self.handles) < minimum:
                await asyncio.sleep(0)

        await asyncio.wait_for(_wait(), timeout)

    async def wait_for_result(self, timeout: float) -> None:
        await asyncio.wait_for(self.result_published.wait(), timeout)

    def result_count(self) -> int:
        return sum(subject == SUBJECT_RESULT for subject, _ in self.published)


def make_job_message(topic: str, job_id: str) -> Message:
    request = job_pb2.JobRequest(
        job_id=job_id,
        topic=topic,
        context_ptr="redis://ctx:" + job_id,
    )
    packet = buspacket_pb2.BusPacket(
        trace_id="trace-" + job_id,
        sender_id="test-client",
        protocol_version=1,
    )
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(request)
    return Message(packet.SerializeToString(deterministic=True))
