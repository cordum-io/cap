"""Fakes shared by Python Agent trust-lifecycle tests."""

import asyncio
from datetime import datetime, timedelta, timezone
from typing import Any, Awaitable, Callable, Dict, List, Optional, Tuple

from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1 import handshake_pb2
from cap.worker_trust import WorkerHandshakeSession, WorkerTrustConfig


WORKER_ID = "python-agent-worker"
TOPIC = "job.python.agent"


def trust_config() -> WorkerTrustConfig:
    scheduler_key = ec.generate_private_key(ec.SECP256R1())
    return WorkerTrustConfig(
        worker_id=WORKER_ID,
        expected_agent_id="agent-python",
        tenant_id="tenant-python",
        audience="cordum-scheduler",
        proof_key_id="worker-key-1",
        proof_private_key=ec.generate_private_key(ec.SECP256R1()),
        expected_scheduler_id="scheduler-1",
        scheduler_public_keys={"scheduler-key-1": scheduler_key.public_key()},
        sdk_version="cap-python/v2",
    )


def session(token: str, seconds: float = 60.0) -> WorkerHandshakeSession:
    now = datetime.now(timezone.utc)
    return WorkerHandshakeSession(token, now, now + timedelta(seconds=seconds))


class Subscription:
    def __init__(self, subject: str, events: List[Tuple[Any, ...]]) -> None:
        self.subject = subject
        self.events = events
        self.drain_count = 0

    async def drain(self) -> None:
        self.drain_count += 1
        self.events.append(("sub-drain", self.subject))


class RecordingNATS:
    def __init__(self) -> None:
        self.events: List[Tuple[Any, ...]] = []
        self.callbacks: Dict[str, Callable[[Any], Awaitable[None]]] = {}
        self.handles: List[Subscription] = []
        self.published: List[Tuple[str, bytes]] = []
        self.connect_count = 0
        self.reconnected_cb: Optional[Callable[[], Awaitable[None]]] = None
        self.disconnected_cb: Optional[Callable[[], Awaitable[None]]] = None

    async def connect(
        self, servers, name, reconnected_cb=None, disconnected_cb=None
    ):
        del servers, name
        self.connect_count += 1
        self.reconnected_cb = reconnected_cb
        self.disconnected_cb = disconnected_cb
        self.events.append(("connect",))
        return self

    async def subscribe(self, subject, queue="", cb=None):
        del queue
        if cb is None:
            raise ValueError("callback required")
        self.events.append(("subscribe", subject))
        self.callbacks[subject] = cb
        handle = Subscription(subject, self.events)
        self.handles.append(handle)
        return handle

    async def publish(self, subject, data):
        self.events.append(("publish", subject))
        self.published.append((subject, data))

    async def request(self, subject, data, timeout):
        del subject, data, timeout
        raise AssertionError("test must patch the trust exchange")

    async def drain(self):
        self.events.append(("connection-drain",))

    async def wait_for_drains(self, minimum: int, timeout: float = 1.0) -> None:
        async def wait() -> None:
            while sum(handle.drain_count for handle in self.handles) < minimum:
                await asyncio.sleep(0)

        await asyncio.wait_for(wait(), timeout)


def worker_capability() -> handshake_pb2.Handshake:
    return handshake_pb2.Handshake(
        component_id=WORKER_ID,
        role=handshake_pb2.COMPONENT_ROLE_WORKER,
        supported_versions=[1],
        capabilities={TOPIC: True},
        sdk_version="cap-python/v2",
        ready_topics=[TOPIC],
    )
