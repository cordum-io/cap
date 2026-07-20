"""Additional packet and bounded-cleanup contracts for the submitter."""

import asyncio
import time
from dataclasses import replace
from typing import Sequence

import cap
import pytest

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from playground.submit import main as submit
from playground.tests.test_submit import (
    FakeConnection,
    FakeMessage,
    FakeSubscription,
    Harness,
    make_config,
    result_bytes,
    run_harness,
)


class SlowConnection(FakeConnection):
    def __init__(self, stage: str) -> None:
        super().__init__([result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)], [])
        self.stage = stage

    async def subscribe(self, subject: str) -> FakeSubscription:
        if self.stage == "subscribe":
            await asyncio.sleep(0.5)
        return await super().subscribe(subject)

    async def flush(self) -> None:
        if self.stage == "flush":
            await asyncio.sleep(0.5)
        await super().flush()

    async def publish(self, subject: str, payload: bytes) -> None:
        if self.stage == "publish":
            await asyncio.sleep(0.5)
        await super().publish(subject, payload)


class SlowHarness(Harness):
    def __init__(self, stage: str) -> None:
        super().__init__([])
        self.connection = SlowConnection(stage)


class DeadlineSubscription(FakeSubscription):
    async def next_msg(self, timeout: float = 1.0) -> FakeMessage:
        if timeout <= 1.0:
            raise asyncio.TimeoutError
        return await super().next_msg(timeout)


class DeadlineConnection(FakeConnection):
    async def subscribe(self, subject: str) -> DeadlineSubscription:
        await super().subscribe(subject)
        return DeadlineSubscription(self)


def test_published_packet_is_valid_and_matches_direct_subject() -> None:
    harness = Harness([result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)])
    assert run_harness(harness) == 0
    assert len(harness.connection.published) == 1
    packet = buspacket_pb2.BusPacket.FromString(harness.connection.published[0])
    assert packet.trace_id == "playground-test-trace"
    assert packet.job_request.job_id == "playground-test-job"
    assert packet.job_request.topic == "job.echo"
    assert packet.job_request.context_ptr.startswith("demo://context/")
    assert cap.validate_bus_packet(packet) == []
    assert cap.validate_job_request(packet.job_request) == []


@pytest.mark.parametrize(
    "subject",
    ["job..echo", ".job", "job.", "job.ech*o", "job.>x"],
)
def test_packet_rejects_nonliteral_subject_tokens(subject: str) -> None:
    with pytest.raises(ValueError, match="nonempty literal tokens"):
        submit._build_packet(replace(make_config(), job_subject=subject))


@pytest.mark.parametrize(
    "messages,expected,errors",
    [
        ([result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)], 0, ["unsubscribe-timeout"]),
        ([result_bytes(job_pb2.JOB_STATUS_FAILED)], 5, ["drain-timeout"]),
        ([], 4, ["unsubscribe-timeout", "drain-timeout"]),
    ],
    ids=["success", "terminal", "result-timeout"],
)
def test_hung_cleanup_is_bounded_without_masking_verdict(
    messages: Sequence[bytes], expected: int, errors: Sequence[str]
) -> None:
    harness = Harness(messages, errors=errors)
    started = time.monotonic()
    assert run_harness(harness) == expected
    assert time.monotonic() - started < 0.2
    assert harness.connection.events[-2:] == ["unsubscribe", "drain"]


@pytest.mark.parametrize("stage", ["subscribe", "flush", "publish"])
def test_hung_transport_stage_is_bounded(stage: str) -> None:
    harness = SlowHarness(stage)
    started = time.monotonic()
    assert run_harness(harness) == 2
    assert time.monotonic() - started < 0.2


def test_configured_result_deadline_overrides_nats_one_second_default() -> None:
    harness = Harness([])
    harness.connection = DeadlineConnection(
        [result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)], []
    )

    async def invoke() -> int:
        return await submit.run_submit(
            replace(make_config(), result_timeout=2.5),
            connect_fn=harness.connect,
            readiness_fn=harness.readiness,
        )

    assert asyncio.run(invoke()) == 0
