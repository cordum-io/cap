"""NATS monitoring and environment contracts for the playground submitter."""

import asyncio
import json
import time
from pathlib import Path
from typing import Sequence

import pytest

from playground.submit import main as submit


class FakeResponse:
    def __init__(self, payload: object) -> None:
        self.payload = json.dumps(payload).encode("utf-8")

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *args: object) -> None:
        del args

    def read(self) -> bytes:
        return self.payload

    def close(self) -> None:
        return None


class FakeOpen:
    def __init__(self, payloads: Sequence[object]) -> None:
        self.payloads = list(payloads)
        self.calls: list[tuple[str, float]] = []

    def __call__(self, url: str, *, timeout: float) -> FakeResponse:
        self.calls.append((url, timeout))
        return FakeResponse(self.payloads.pop(0))


def _monitor_fn():
    function = getattr(submit, "_monitor_has_subscription", None)
    assert callable(function), "submitter must inspect NATS monitoring"
    return function


def _wait_fn():
    function = getattr(submit, "_wait_for_worker", None)
    assert callable(function), "submitter must bound worker readiness"
    return function


def test_monitor_requires_healthy_server_and_exact_object_subject() -> None:
    opener = FakeOpen(
        [
            {"status": "ok"},
            {"subscriptions_list": [{"subject": "job.echo"}]},
        ]
    )
    assert _monitor_fn()("http://nats:8222", "job.echo", open_fn=opener)
    assert [Path(url).name for url, _ in opener.calls] == ["healthz", "subsz?subs=1"]


@pytest.mark.parametrize(
    "health,subscriptions",
    [
        ({"status": "error"}, [{"subject": "job.echo"}]),
        ({"status": "ok"}, [{"subject": "job.echo.more"}]),
        ({"status": "ok"}, ["job.echo"]),
        ({"status": "ok"}, {"subject": "job.echo"}),
    ],
)
def test_monitor_rejects_unhealthy_or_noncanonical_payloads(
    health: object, subscriptions: object
) -> None:
    opener = FakeOpen([health, {"subscriptions_list": subscriptions}])
    assert not _monitor_fn()("http://nats:8222/", "job.echo", open_fn=opener)


def test_wait_for_worker_polls_until_exact_subscription() -> None:
    answers = iter([False, False, True])

    def probe(_url: str, _subject: str) -> bool:
        return next(answers)

    result = asyncio.run(
        _wait_fn()(
            "http://nats:8222", "job.echo", 0.2, probe_fn=probe, poll_interval=0.001
        )
    )
    assert result is True


def test_wait_for_worker_returns_false_at_one_deadline() -> None:
    result = asyncio.run(
        _wait_fn()(
            "http://nats:8222",
            "job.echo",
            0.003,
            probe_fn=lambda _url, _subject: False,
            poll_interval=0.001,
        )
    )
    assert result is False


def test_wait_for_worker_bounds_a_hung_monitor_probe() -> None:
    def hung_probe(_url: str, _subject: str) -> bool:
        time.sleep(0.5)
        return False

    async def invoke() -> bool:
        return await asyncio.wait_for(
            _wait_fn()(
                "http://nats:8222", "job.echo", 0.005, probe_fn=hung_probe
            ),
            0.1,
        )

    started = time.monotonic()
    assert asyncio.run(invoke()) is False
    assert time.monotonic() - started < 0.2


def test_config_reads_bounded_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("NATS_URL", "nats://broker:4222")
    monkeypatch.setenv("NATS_MONITOR_URL", "http://broker:8222")
    monkeypatch.setenv("CAP_NATS_CONNECT_TIMEOUT_SECONDS", "1.25")
    monkeypatch.setenv("CAP_WORKER_READY_TIMEOUT_SECONDS", "2.5")
    monkeypatch.setenv("CAP_RESULT_TIMEOUT_SECONDS", "3.75")
    config = submit._config_from_env()
    assert (config.nats_url, config.monitor_url) == (
        "nats://broker:4222",
        "http://broker:8222",
    )
    assert (config.connect_timeout, config.readiness_timeout, config.result_timeout) == (
        1.25,
        2.5,
        3.75,
    )


@pytest.mark.parametrize("value", ["0", "-1", "nan", "infinity", "bad"])
def test_config_rejects_unbounded_timeout(
    value: str, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setenv("CAP_RESULT_TIMEOUT_SECONDS", value)
    with pytest.raises(ValueError, match="CAP_RESULT_TIMEOUT_SECONDS"):
        submit._config_from_env()
