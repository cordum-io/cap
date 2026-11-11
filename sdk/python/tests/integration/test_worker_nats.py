import asyncio
import json
import os
import unittest
import uuid
from dataclasses import replace
from datetime import datetime, timedelta, timezone
from typing import Any, Awaitable, Callable, Optional

import nats
from nats.aio.client import Client as NATSClient
from nats.aio.msg import Msg
from nats.aio.subscription import Subscription

from cryptography.hazmat.primitives.asymmetric import ec

from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.packet_boundary import finalize_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.resource_registry import (
    InMemoryResourceBackend,
    ResourceBackendRegistration,
    ResourceRegistry,
    TrustedResourceContext,
)
from cap.runtime import Agent
from cap.subjects import SUBJECT_RESULT
from cap.worker import run_worker
from tests.worker_trust_runtime_support import trust_config


NATS_URL_ENV = "CAP_TEST_NATS_URL"
TIMEOUT_SECONDS = 5
FAILED_MESSAGE = "handler failed"


def _required_nats_url() -> str:
    value = os.environ.get(NATS_URL_ENV, "").strip()
    if not value:
        raise AssertionError(
            f"{NATS_URL_ENV} must name a real NATS server for this integration test"
        )
    return value


async def _connect(name: str, url: str) -> NATSClient:
    return await asyncio.wait_for(
        nats.connect(
            servers=url,
            name=name,
            allow_reconnect=False,
            connect_timeout=1,
            max_reconnect_attempts=0,
        ),
        timeout=TIMEOUT_SECONDS,
    )


class ObservedWorkerConnection:
    def __init__(self, client: NATSClient) -> None:
        self.client = client
        self.subscription_ready = asyncio.Event()
        self.drain_finished = asyncio.Event()
        self.drain_calls = 0
        self.flush_calls = 0

    async def publish(self, subject: str, data: bytes) -> None:
        await self.client.publish(subject, data)

    async def subscribe(
        self,
        subject: str,
        *,
        queue: str,
        cb: Callable[[Msg], Awaitable[None]],
    ) -> Subscription:
        subscription = await self.client.subscribe(subject, queue=queue, cb=cb)
        await self.client.flush(timeout=TIMEOUT_SECONDS)
        self.subscription_ready.set()
        return subscription

    async def drain(self) -> None:
        self.drain_calls += 1
        await self.client.flush(timeout=TIMEOUT_SECONDS)
        self.flush_calls += 1
        await self.client.drain()
        self.drain_finished.set()


class ObservedWorkerConnector:
    def __init__(self) -> None:
        self.connection: Optional[ObservedWorkerConnection] = None
        self.connected = asyncio.Event()

    async def connect(self, *, servers: str, name: str) -> ObservedWorkerConnection:
        client = await _connect(name, servers)
        self.connection = ObservedWorkerConnection(client)
        self.connected.set()
        return self.connection


class FailureThenSuccessHandler:
    def __init__(self, failed_id: str, secret: str) -> None:
        self.failed_id = failed_id
        self.secret = secret

    async def __call__(self, request: job_pb2.JobRequest) -> job_pb2.JobResult:
        if request.job_id == self.failed_id:
            raise RuntimeError(self.secret)
        return job_pb2.JobResult(
            status=job_pb2.JOB_STATUS_SUCCEEDED,
            result_ptr="memory://integration-success",
        )


def _job_packet(job_id: str, subject: str, trace_id: str) -> bytes:
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id,
        sender_id="cap-python-integration-submitter",
        protocol_version=DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(job_pb2.JobRequest(job_id=job_id, topic=subject))
    return packet.SerializeToString(deterministic=True)


async def _publish_job(
    client: NATSClient, subject: str, job_id: str, trace_id: str
) -> None:
    await client.publish(subject, _job_packet(job_id, subject, trace_id))
    await client.flush(timeout=TIMEOUT_SECONDS)


async def _next_result(
    subscription: Subscription, trace_id: str
) -> buspacket_pb2.BusPacket:
    loop = asyncio.get_running_loop()
    deadline = loop.time() + TIMEOUT_SECONDS
    while True:
        remaining = deadline - loop.time()
        if remaining <= 0:
            raise asyncio.TimeoutError("timed out waiting for matching CAP result")
        message = await subscription.next_msg(timeout=remaining)
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(message.data)
        if packet.trace_id == trace_id:
            return packet


async def _cancel_and_wait(task: "asyncio.Task[None]") -> None:
    task.cancel()
    try:
        await asyncio.wait_for(task, timeout=TIMEOUT_SECONDS)
    except asyncio.CancelledError:
        return
    raise AssertionError("cancelled worker task completed without CancelledError")


async def _cleanup_task(task: "asyncio.Task[None]") -> None:
    if not task.done():
        task.cancel()
    try:
        await asyncio.wait_for(task, timeout=TIMEOUT_SECONDS)
    except (asyncio.CancelledError, Exception):
        pass


def _start_worker(
    nats_url: str,
    subject: str,
    handler: FailureThenSuccessHandler,
    connector: ObservedWorkerConnector,
) -> "asyncio.Task[None]":
    return asyncio.create_task(
        run_worker(
            nats_url,
            subject,
            handler,
            sender_id="cap-python-integration-worker",
            connect_fn=connector.connect,
        )
    )


class TestWorkerRealNATS(unittest.IsolatedAsyncioTestCase):
    async def test_handler_failure_is_safe_and_worker_remains_live(self) -> None:
        nats_url = _required_nats_url()
        suffix = uuid.uuid4().hex
        subject = f"test.cap.python.worker.{suffix}"
        trace_id = f"trace-{suffix}"
        failed_id = f"failed-{suffix}"
        success_id = f"success-{suffix}"
        secret = "credential=must-not-leak"
        connector = ObservedWorkerConnector()
        submitter = await _connect("cap-python-integration-submitter", nats_url)
        results = await submitter.subscribe(SUBJECT_RESULT)
        await submitter.flush(timeout=TIMEOUT_SECONDS)
        handler = FailureThenSuccessHandler(failed_id, secret)
        worker_task = _start_worker(nats_url, subject, handler, connector)
        try:
            await asyncio.wait_for(connector.connected.wait(), TIMEOUT_SECONDS)
            worker_connection = connector.connection
            self.assertIsNotNone(worker_connection)
            assert worker_connection is not None
            self.assertIsNot(worker_connection.client, submitter)
            await asyncio.wait_for(
                worker_connection.subscription_ready.wait(), TIMEOUT_SECONDS
            )
            await _publish_job(submitter, subject, failed_id, trace_id)
            failed = await _next_result(results, trace_id)
            self.assertEqual(failed_id, failed.job_result.job_id)
            self.assertEqual(job_pb2.JOB_STATUS_FAILED, failed.job_result.status)
            self.assertEqual(FAILED_MESSAGE, failed.job_result.error_message)
            self.assertNotIn(secret.encode(), failed.SerializeToString())
            self.assertFalse(worker_task.done())

            await _publish_job(submitter, subject, success_id, trace_id)
            succeeded = await _next_result(results, trace_id)
            self.assertEqual(success_id, succeeded.job_result.job_id)
            self.assertEqual(job_pb2.JOB_STATUS_SUCCEEDED, succeeded.job_result.status)
            self.assertEqual(
                "memory://integration-success", succeeded.job_result.result_ptr
            )

            await _cancel_and_wait(worker_task)
            await asyncio.wait_for(
                worker_connection.drain_finished.wait(), TIMEOUT_SECONDS
            )
            self.assertEqual(1, worker_connection.drain_calls)
            self.assertEqual(1, worker_connection.flush_calls)
            self.assertTrue(worker_connection.client.is_closed)
        finally:
            await _cleanup_task(worker_task)
            if not submitter.is_closed:
                await asyncio.wait_for(submitter.drain(), TIMEOUT_SECONDS)

    async def test_context_ref_round_trip_over_real_nats(self) -> None:
        nats_url = _required_nats_url()
        suffix = uuid.uuid4().hex
        topic = f"test.cap.python.agent.{suffix}"
        job_id = f"job-{suffix}"
        trace_id = f"trace-{suffix}"
        scheduler_key = ec.generate_private_key(ec.SECP256R1())
        config = replace(
            trust_config(),
            scheduler_public_keys={"scheduler-pin": scheduler_key.public_key()},
        )
        backend = InMemoryResourceBackend()
        registry = ResourceRegistry(
            [ResourceBackendRegistration("memory", 1 << 20, backend, backend)]
        )
        trusted = TrustedResourceContext(config.tenant_id, job_id)
        context_ref = await registry.externalize(
            "memory", json.dumps({"x": 1}).encode("utf-8"),
            media_type="application/json", purpose="job-context",
            expires_at=datetime.now(timezone.utc) + timedelta(minutes=5),
            trusted=trusted,
        )
        # Warn mode: no scheduler answers the trust exchange, so the worker runs
        # without a session but still pins the scheduler key and the tenant.
        agent = Agent(
            nats_url=nats_url, sender_id=config.worker_id, heartbeat_interval=60,
            worker_trust_mode="warn", worker_trust=config,
            worker_trust_timeout=0.5, worker_trust_retries=1,
            resource_registry=registry, result_backend_id="memory",
        )

        @agent.job(topic)
        async def handler(ctx: Any, data: Any) -> Any:
            return {"y": data["x"] + 1}

        submitter = await _connect("cap-python-integration-submitter", nats_url)
        results = await submitter.subscribe(SUBJECT_RESULT)
        await submitter.flush(timeout=TIMEOUT_SECONDS)
        try:
            await asyncio.wait_for(agent.start(), TIMEOUT_SECONDS)
            await agent._nc.flush(timeout=TIMEOUT_SECONDS)
            packet = buspacket_pb2.BusPacket(
                trace_id=trace_id,
                sender_id=config.expected_scheduler_id,
                protocol_version=DEFAULT_PROTOCOL_VERSION,
            )
            packet.created_at.GetCurrentTime()
            packet.job_request.CopyFrom(
                job_pb2.JobRequest(job_id=job_id, topic=topic, context_ref=context_ref)
            )
            signed = finalize_packet(packet, scheduler_key)
            await submitter.publish(topic, signed.SerializeToString(deterministic=True))
            await submitter.flush(timeout=TIMEOUT_SECONDS)

            result = (await _next_result(results, trace_id)).job_result
            self.assertEqual(job_id, result.job_id)
            self.assertEqual(job_pb2.JOB_STATUS_SUCCEEDED, result.status)
            self.assertEqual("", result.result_ptr)
            self.assertTrue(result.HasField("result_ref"))
            self.assertEqual("job-result", result.result_ref.purpose)
            resolved = await registry.resolve(result.result_ref, trusted)
            self.assertEqual(json.dumps({"y": 2}).encode("utf-8"), resolved.content)
            self.assertEqual("application/json", resolved.media_type)
        finally:
            await asyncio.wait_for(agent.close(), TIMEOUT_SECONDS)
            if not submitter.is_closed:
                await asyncio.wait_for(submitter.drain(), TIMEOUT_SECONDS)


if __name__ == "__main__":
    unittest.main()
