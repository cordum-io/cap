"""Fail-closed CAP playground submitter for the direct local transport lab."""

from __future__ import annotations

import asyncio
import logging
import math
import os
import uuid
from dataclasses import dataclass
from typing import Awaitable, Callable, Optional, Protocol, cast

import cap
import cap.constants
import nats
from google.protobuf import timestamp_pb2
from google.protobuf.message import DecodeError

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from .monitoring import (
    _monitor_has_subscription,
    _wait_for_worker,
)


logging.basicConfig(level=logging.INFO, format="%(asctime)s [submit] %(message)s")
log = logging.getLogger("cap.playground.submit")

EXIT_SUCCESS = 0
EXIT_TRANSPORT = 2
EXIT_READINESS = 3
EXIT_RESULT_TIMEOUT = 4
EXIT_TERMINAL = 5
EXIT_PROTOCOL = 6
SUCCESS_TEXT = "CAP Playground Demo Complete!"

TRANSITIONAL_STATUSES = {
    job_pb2.JOB_STATUS_PENDING,
    job_pb2.JOB_STATUS_SCHEDULED,
    job_pb2.JOB_STATUS_DISPATCHED,
    job_pb2.JOB_STATUS_RUNNING,
}
TERMINAL_FAILURES = {
    job_pb2.JOB_STATUS_FAILED,
    job_pb2.JOB_STATUS_CANCELLED,
    job_pb2.JOB_STATUS_DENIED,
    job_pb2.JOB_STATUS_TIMEOUT,
    job_pb2.JOB_STATUS_FAILED_RETRYABLE,
    job_pb2.JOB_STATUS_FAILED_FATAL,
}


class MessageLike(Protocol):
    data: bytes


class SubscriptionLike(Protocol):
    async def next_msg(self, timeout: float) -> MessageLike: ...

    async def unsubscribe(self) -> None: ...


class ConnectionLike(Protocol):
    async def subscribe(self, subject: str) -> SubscriptionLike: ...

    async def flush(self) -> None: ...

    async def publish(self, subject: str, payload: bytes) -> None: ...

    async def drain(self) -> None: ...


@dataclass(frozen=True)
class SubmitConfig:
    job_id: str
    trace_id: str
    job_subject: str
    result_subject: str
    readiness_timeout: float
    result_timeout: float
    connect_timeout: float = 5.0
    cleanup_timeout: float = 2.0
    nats_url: str = "nats://127.0.0.1:4222"
    monitor_url: str = "http://127.0.0.1:8222"


ConnectFn = Callable[[SubmitConfig], Awaitable[ConnectionLike]]
ReadinessFn = Callable[[str, str, float], Awaitable[bool]]


def _positive_seconds(name: str, default: str) -> float:
    raw = os.environ.get(name, default)
    try:
        value = float(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be a positive finite number") from exc
    if not math.isfinite(value) or value <= 0:
        raise ValueError(f"{name} must be a positive finite number")
    return value


def _config_from_env() -> SubmitConfig:
    nats_url = os.environ.get("NATS_URL", "nats://127.0.0.1:4222").strip()
    monitor_url = os.environ.get("NATS_MONITOR_URL", "http://127.0.0.1:8222").strip()
    if not nats_url or not monitor_url:
        raise ValueError("NATS_URL and NATS_MONITOR_URL must not be blank")
    return SubmitConfig(
        job_id=f"playground-{uuid.uuid4().hex[:12]}",
        trace_id=f"trace-{uuid.uuid4().hex[:12]}",
        job_subject="job.echo",
        result_subject="sys.job.result",
        connect_timeout=_positive_seconds("CAP_NATS_CONNECT_TIMEOUT_SECONDS", "5"),
        readiness_timeout=_positive_seconds("CAP_WORKER_READY_TIMEOUT_SECONDS", "20"),
        result_timeout=_positive_seconds("CAP_RESULT_TIMEOUT_SECONDS", "15"),
        cleanup_timeout=_positive_seconds("CAP_CLEANUP_TIMEOUT_SECONDS", "2"),
        nats_url=nats_url,
        monitor_url=monitor_url,
    )


def _build_packet(config: SubmitConfig) -> buspacket_pb2.BusPacket:
    tokens = config.job_subject.split(".")
    invalid_subject = any(
        not token
        or any(character.isspace() for character in token)
        or "*" in token
        or ">" in token
        for token in tokens
    )
    if invalid_subject:
        raise ValueError("direct job subject must have nonempty literal tokens")
    request = job_pb2.JobRequest(
        job_id=config.job_id,
        topic=config.job_subject,
        priority=job_pb2.JOB_PRIORITY_INTERACTIVE,
        context_ptr=f"demo://context/{config.job_id}",
    )
    timestamp = timestamp_pb2.Timestamp()
    timestamp.GetCurrentTime()
    packet = buspacket_pb2.BusPacket(
        trace_id=config.trace_id,
        sender_id="playground-submitter",
        protocol_version=cap.constants.DEFAULT_PROTOCOL_VERSION,
        created_at=timestamp,
    )
    packet.job_request.CopyFrom(request)
    errors = [*cap.validate_job_request(request), *cap.validate_bus_packet(packet)]
    if errors:
        raise ValueError("invalid request packet: " + "; ".join(map(str, errors)))
    return packet


def _matching_verdict(data: bytes, config: SubmitConfig) -> Optional[int]:
    packet = buspacket_pb2.BusPacket()
    try:
        packet.ParseFromString(data)
    except DecodeError:
        return None
    if packet.WhichOneof("payload") != "job_result":
        return None
    result = packet.job_result
    if packet.trace_id != config.trace_id or result.job_id != config.job_id:
        return None
    errors = cap.validate_bus_packet(packet)
    if errors:
        log.error("Matching result failed validation: %s", errors)
        return EXIT_PROTOCOL
    if result.status == job_pb2.JOB_STATUS_SUCCEEDED:
        return EXIT_SUCCESS
    if result.status in TRANSITIONAL_STATUSES:
        return None
    if result.status in TERMINAL_FAILURES:
        log.error("Job ended with %s", job_pb2.JobStatus.Name(result.status))
        return EXIT_TERMINAL
    log.error("Job returned unknown status value %s", result.status)
    return EXIT_PROTOCOL


async def _await_result(subscription: SubscriptionLike, config: SubmitConfig) -> int:
    loop = asyncio.get_running_loop()
    deadline = loop.time() + config.result_timeout
    while True:
        remaining = deadline - loop.time()
        if remaining <= 0:
            log.error("Timed out waiting for a matching result")
            return EXIT_RESULT_TIMEOUT
        try:
            message = await asyncio.wait_for(subscription.next_msg(remaining), remaining)
        except asyncio.TimeoutError:
            log.error("Timed out waiting for a matching result")
            return EXIT_RESULT_TIMEOUT
        except Exception as exc:
            log.error("Result subscription failed: %s", exc)
            return EXIT_TRANSPORT
        verdict = _matching_verdict(message.data, config)
        if verdict is None:
            continue
        if verdict == EXIT_SUCCESS:
            print(f"{SUCCESS_TEXT} job_id={config.job_id} trace_id={config.trace_id}")
        return verdict


async def _connect(config: SubmitConfig) -> ConnectionLike:
    connection = await nats.connect(
        servers=[config.nats_url],
        allow_reconnect=False,
        max_reconnect_attempts=0,
        connect_timeout=config.connect_timeout,
        drain_timeout=config.cleanup_timeout,
        name="cap-playground-submitter",
    )
    return cast(ConnectionLike, connection)


async def _cleanup(
    connection: Optional[ConnectionLike],
    subscription: Optional[SubscriptionLike],
    timeout: float,
) -> None:
    if subscription is not None:
        try:
            await asyncio.wait_for(subscription.unsubscribe(), timeout)
        except Exception as exc:
            log.warning("Result unsubscribe failed: %s", exc)
    if connection is not None:
        try:
            await asyncio.wait_for(connection.drain(), timeout)
        except Exception as exc:
            log.warning("NATS drain failed: %s", exc)


async def run_submit(
    config: SubmitConfig,
    *,
    connect_fn: ConnectFn = _connect,
    readiness_fn: ReadinessFn = _wait_for_worker,
) -> int:
    connection: Optional[ConnectionLike] = None
    subscription: Optional[SubscriptionLike] = None
    try:
        try:
            connection = await asyncio.wait_for(connect_fn(config), config.connect_timeout)
            subscription = await asyncio.wait_for(
                connection.subscribe(config.result_subject), config.connect_timeout
            )
            await asyncio.wait_for(connection.flush(), config.connect_timeout)
        except Exception as exc:
            log.error("NATS setup failed: %s", exc)
            return EXIT_TRANSPORT
        try:
            ready = await readiness_fn(
                config.monitor_url, config.job_subject, config.readiness_timeout
            )
        except Exception as exc:
            log.error("Worker readiness failed: %s", exc)
            return EXIT_READINESS
        if not ready:
            log.error("Worker was not ready before the deadline")
            return EXIT_READINESS
        try:
            packet = _build_packet(config)
        except ValueError as exc:
            log.error("Request construction failed: %s", exc)
            return EXIT_PROTOCOL
        try:
            payload = packet.SerializeToString(deterministic=True)
            await asyncio.wait_for(
                connection.publish(config.job_subject, payload), config.connect_timeout
            )
            await asyncio.wait_for(connection.flush(), config.connect_timeout)
        except Exception as exc:
            log.error("NATS publish failed: %s", exc)
            return EXIT_TRANSPORT
        return await _await_result(subscription, config)
    finally:
        await _cleanup(connection, subscription, config.cleanup_timeout)


async def main() -> int:
    try:
        config = _config_from_env()
    except ValueError as exc:
        log.error("Invalid playground configuration: %s", exc)
        return EXIT_PROTOCOL
    log.info("Submitting direct development job %s", config.job_id)
    return await run_submit(config)


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
