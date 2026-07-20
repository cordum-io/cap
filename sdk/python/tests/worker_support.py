import asyncio
from dataclasses import dataclass
from typing import Awaitable, Callable, Dict, List, Optional, Tuple

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from google.protobuf import timestamp_pb2


@dataclass(frozen=True)
class Publication:
    subject: str
    data: bytes


class FakeMessage:
    def __init__(self, data: bytes):
        self.data = data


MessageCallback = Callable[[FakeMessage], Awaitable[None]]


class RecordingNATS:
    def __init__(self):
        self.callbacks: Dict[str, MessageCallback] = {}
        self.connect_calls: List[Tuple[str, str]] = []
        self.subscribe_calls: List[Tuple[str, str]] = []
        self.publish_attempts: List[Publication] = []
        self.publications: List[Publication] = []
        self.subscribe_error: Optional[Exception] = None
        self.next_publish_error: Optional[Exception] = None
        self.drain_error: Optional[BaseException] = None
        self.drain_calls = 0
        self.active_callbacks = 0
        self.subscription_ready = asyncio.Event()
        self.callbacks_idle = asyncio.Event()
        self.callbacks_idle.set()
        self.drain_started = asyncio.Event()
        self.drain_finished = asyncio.Event()
        self.events: List[str] = []

    async def connect(self, *, servers: str, name: str) -> "RecordingNATS":
        self.connect_calls.append((servers, name))
        return self

    async def subscribe(
        self, subject: str, *, queue: str, cb: MessageCallback
    ) -> None:
        self.subscribe_calls.append((subject, queue))
        if self.subscribe_error is not None:
            raise self.subscribe_error
        self.callbacks[subject] = cb
        self.subscription_ready.set()

    async def publish(self, subject: str, data: bytes) -> None:
        publication = Publication(subject, data)
        self.publish_attempts.append(publication)
        if self.next_publish_error is not None:
            error = self.next_publish_error
            self.next_publish_error = None
            raise error
        self.publications.append(publication)
        self.events.append("publish")

    async def deliver(self, subject: str, packet: buspacket_pb2.BusPacket) -> None:
        callback = self.callbacks[subject]
        self.active_callbacks += 1
        self.callbacks_idle.clear()
        try:
            await callback(FakeMessage(packet.SerializeToString(deterministic=True)))
        finally:
            self.active_callbacks -= 1
            if self.active_callbacks == 0:
                self.callbacks_idle.set()

    async def drain(self) -> None:
        self.drain_calls += 1
        self.events.append("drain-start")
        self.drain_started.set()
        await self.callbacks_idle.wait()
        if self.drain_error is not None:
            raise self.drain_error
        self.events.append("drain-finish")
        self.drain_finished.set()


class RecordingMetrics:
    def __init__(self):
        self.received: List[Tuple[str, str]] = []
        self.completed: List[Tuple[str, int, str]] = []
        self.failed: List[Tuple[str, str]] = []
        self.heartbeats: List[str] = []

    def on_job_received(self, job_id: str, topic: str) -> None:
        self.received.append((job_id, topic))

    def on_job_completed(self, job_id: str, duration_ms: int, status: str) -> None:
        self.completed.append((job_id, duration_ms, status))

    def on_job_failed(self, job_id: str, error_msg: str) -> None:
        self.failed.append((job_id, error_msg))

    def on_heartbeat_sent(self, worker_id: str) -> None:
        self.heartbeats.append(worker_id)


def job_packet(
    job_id: str, topic: str, trace_id: str
) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id,
        sender_id="test-client",
        protocol_version=1,
    )
    timestamp = timestamp_pb2.Timestamp()
    timestamp.GetCurrentTime()
    packet.created_at.CopyFrom(timestamp)
    packet.job_request.CopyFrom(job_pb2.JobRequest(job_id=job_id, topic=topic))
    return packet


def decode_result(publication: Publication) -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket()
    packet.ParseFromString(publication.data)
    return packet
