"""Deterministic contract tests for the fail-closed playground submitter."""

import asyncio
from dataclasses import FrozenInstanceError, dataclass
from typing import List, Sequence

import pytest
from google.protobuf import timestamp_pb2

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from playground.submit import main as submit


JOB_ID = "playground-test-job"
TRACE_ID = "playground-test-trace"
SUCCESS_TEXT = "CAP Playground Demo Complete!"
EXIT_CONNECT = 2
EXIT_READINESS = 3
EXIT_TIMEOUT = 4
EXIT_TERMINAL = 5
EXIT_PROTOCOL = 6


@dataclass(frozen=True)
class FakeMessage:
    data: bytes


class FakeSubscription:
    def __init__(self, connection: "FakeConnection") -> None:
        self.connection = connection

    async def next_msg(self, timeout: float = 1.0) -> FakeMessage:
        self.connection.events.append("next_msg")
        if not self.connection.messages:
            raise asyncio.TimeoutError
        return self.connection.messages.pop(0)

    async def unsubscribe(self) -> None:
        self.connection.events.append("unsubscribe")
        if "unsubscribe" in self.connection.errors:
            raise RuntimeError("unsubscribe failed")
        if "unsubscribe-timeout" in self.connection.errors:
            await asyncio.sleep(60)


class FakeConnection:
    def __init__(self, messages: Sequence[bytes], errors: Sequence[str]) -> None:
        self.messages = [FakeMessage(data) for data in messages]
        self.events: List[str] = []
        self.errors = set(errors)
        self.published: List[bytes] = []

    async def subscribe(self, subject: str) -> FakeSubscription:
        self.events.append("subscribe:" + subject)
        if "subscribe" in self.errors:
            raise RuntimeError("subscribe failed")
        return FakeSubscription(self)

    async def flush(self) -> None:
        self.events.append("flush")

    async def publish(self, subject: str, payload: bytes) -> None:
        assert payload
        self.events.append("publish:" + subject)
        self.published.append(payload)
        if "publish" in self.errors:
            raise RuntimeError("publish failed")

    async def drain(self) -> None:
        self.events.append("drain")
        if "drain" in self.errors:
            raise RuntimeError("drain failed")
        if "drain-timeout" in self.errors:
            await asyncio.sleep(60)


class Harness:
    def __init__(self, messages: Sequence[bytes], *, ready: bool = True, errors: Sequence[str] = ()) -> None:
        self.connection = FakeConnection(messages, errors)
        self.ready = ready
        self.errors = set(errors)

    async def connect(self, *args: object, **kwargs: object) -> FakeConnection:
        del args, kwargs
        self.connection.events.append("connect")
        if "connect" in self.errors:
            raise ConnectionError("connect failed")
        if "connect-timeout" in self.errors:
            await asyncio.sleep(60)
        return self.connection

    async def readiness(self, *args: object, **kwargs: object) -> bool:
        del args, kwargs
        self.connection.events.append("ready")
        if "readiness" in self.errors:
            raise asyncio.TimeoutError
        return self.ready


def result_bytes(
    status: int,
    *,
    job_id: str = JOB_ID,
    trace_id: str = TRACE_ID,
    worker_id: str = "echo-worker",
) -> bytes:
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id, sender_id="echo-worker", protocol_version=1,
        created_at=timestamp_pb2.Timestamp(seconds=1),
    )
    packet.job_result.CopyFrom(
        job_pb2.JobResult(
            job_id=job_id,
            status=status,
            result_ptr="demo://result/playground-test-job",
            worker_id=worker_id,
            execution_ms=1,
        )
    )
    return packet.SerializeToString(deterministic=True)


def make_config():
    config_type = getattr(submit, "SubmitConfig", None)
    assert config_type is not None, "playground submitter must define SubmitConfig"
    return config_type(
        job_id=JOB_ID,
        trace_id=TRACE_ID,
        job_subject="job.echo",
        result_subject="sys.job.result",
        readiness_timeout=0.01,
        result_timeout=0.01,
        connect_timeout=0.01,
        cleanup_timeout=0.01,
    )


def run_harness(harness: Harness) -> int:
    run_submit = getattr(submit, "run_submit", None)
    assert callable(run_submit), "playground submitter must define run_submit"

    async def invoke() -> int:
        return await run_submit(
            make_config(),
            connect_fn=harness.connect,
            readiness_fn=harness.readiness,
        )

    return asyncio.run(invoke())


def test_matching_succeeded_returns_zero_and_prints_one_marker(capsys) -> None:
    harness = Harness([result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)])
    assert run_harness(harness) == 0
    assert capsys.readouterr().out.count(SUCCESS_TEXT) == 1


@pytest.mark.parametrize(
    "status,expected",
    [
        (job_pb2.JOB_STATUS_FAILED, EXIT_TERMINAL),
        (job_pb2.JOB_STATUS_CANCELLED, EXIT_TERMINAL),
        (999, EXIT_PROTOCOL),
    ],
    ids=["failed", "cancelled", "unknown"],
)
def test_non_success_status_returns_expected_code(
    status: int, expected: int, capsys
) -> None:
    harness = Harness([result_bytes(status)])
    assert run_harness(harness) == expected
    assert SUCCESS_TEXT not in capsys.readouterr().out


def test_result_timeout_returns_timeout_code(capsys) -> None:
    assert run_harness(Harness([])) == EXIT_TIMEOUT
    assert SUCCESS_TEXT not in capsys.readouterr().out


@pytest.mark.parametrize(
    "messages",
    [
        [b"\xff-not-protobuf"],
        [
            result_bytes(job_pb2.JOB_STATUS_SUCCEEDED, trace_id="other-trace"),
            result_bytes(job_pb2.JOB_STATUS_SUCCEEDED, trace_id="other-trace"),
        ],
    ],
    ids=["wire-malformed", "duplicate-unrelated"],
)
def test_malformed_or_duplicate_unrelated_results_time_out(
    messages: Sequence[bytes], capsys
) -> None:
    assert run_harness(Harness(messages)) == EXIT_TIMEOUT
    assert SUCCESS_TEXT not in capsys.readouterr().out


def test_matching_semantically_invalid_result_is_protocol_error(capsys) -> None:
    malformed = result_bytes(job_pb2.JOB_STATUS_SUCCEEDED, worker_id="")
    assert run_harness(Harness([malformed])) == EXIT_PROTOCOL
    assert SUCCESS_TEXT not in capsys.readouterr().out


def test_trace_and_job_id_are_both_required(capsys) -> None:
    harness = Harness(
        [
            result_bytes(job_pb2.JOB_STATUS_SUCCEEDED, trace_id="other-trace"),
            result_bytes(job_pb2.JOB_STATUS_SUCCEEDED, job_id="other-job"),
        ]
    )
    assert run_harness(harness) == EXIT_TIMEOUT
    assert SUCCESS_TEXT not in capsys.readouterr().out


def test_noise_then_duplicate_matching_results_print_one_marker(capsys) -> None:
    success = result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)
    running = result_bytes(job_pb2.JOB_STATUS_RUNNING)
    harness = Harness([b"bad", running, success, success])
    assert run_harness(harness) == 0
    assert capsys.readouterr().out.count(SUCCESS_TEXT) == 1


@pytest.mark.parametrize("raises", [False, True], ids=["false", "raises"])
def test_readiness_timeout_prevents_publish_and_cleans_up(
    raises: bool, capsys
) -> None:
    errors = ["readiness"] if raises else []
    harness = Harness([], ready=False, errors=errors)
    assert run_harness(harness) == EXIT_READINESS
    assert not any(e.startswith("publish:") for e in harness.connection.events)
    assert harness.connection.events[-2:] == ["unsubscribe", "drain"]
    assert SUCCESS_TEXT not in capsys.readouterr().out


def test_success_order_flushes_subscription_and_publish() -> None:
    harness = Harness([result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)])
    assert run_harness(harness) == 0
    assert harness.connection.events[1:6] == [
        "subscribe:sys.job.result",
        "flush",
        "ready",
        "publish:job.echo",
        "flush",
    ]
    assert harness.connection.events[-2:] == ["unsubscribe", "drain"]


@pytest.mark.parametrize(
    "messages,ready",
    [
        ([result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)], True),
        ([result_bytes(job_pb2.JOB_STATUS_FAILED)], True),
        ([], True),
        ([b"malformed"], True),
        ([], False),
    ],
    ids=["success", "terminal", "timeout", "malformed", "readiness"],
)
def test_cleanup_runs_once_on_every_post_subscribe_path(
    messages: Sequence[bytes], ready: bool
) -> None:
    harness = Harness(messages, ready=ready)
    run_harness(harness)
    assert harness.connection.events.count("unsubscribe") == 1
    assert harness.connection.events.count("drain") == 1


@pytest.mark.parametrize(
    "messages,expected",
    [
        ([result_bytes(job_pb2.JOB_STATUS_SUCCEEDED)], 0),
        ([result_bytes(job_pb2.JOB_STATUS_FAILED)], EXIT_TERMINAL),
    ],
    ids=["success", "terminal"],
)
def test_cleanup_failures_do_not_mask_primary_verdict(
    messages: Sequence[bytes], expected: int
) -> None:
    harness = Harness(messages, errors=["unsubscribe", "drain"])
    assert run_harness(harness) == expected
    assert harness.connection.events[-2:] == ["unsubscribe", "drain"]


@pytest.mark.parametrize(
    "failure,unsubscribes,drains",
    [("connect", 0, 0), ("connect-timeout", 0, 0), ("subscribe", 0, 1), ("publish", 1, 1)],
)
def test_transport_failure_returns_connect_code_and_cleans_owned_resources(
    failure: str, unsubscribes: int, drains: int
) -> None:
    harness = Harness([], errors=[failure])
    assert run_harness(harness) == EXIT_CONNECT
    assert harness.connection.events.count("unsubscribe") == unsubscribes
    assert harness.connection.events.count("drain") == drains


def test_submit_config_is_frozen() -> None:
    with pytest.raises(FrozenInstanceError):
        setattr(make_config(), "job_id", "mutated")
