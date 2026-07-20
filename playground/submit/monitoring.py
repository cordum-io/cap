"""Bounded, proxy-free NATS monitoring readiness checks for the playground."""

from __future__ import annotations

import asyncio
import json
import logging
import threading
from typing import Callable, Optional, Protocol, cast
from urllib.request import ProxyHandler, build_opener


log = logging.getLogger("cap.playground.submit")


class HTTPResponseLike(Protocol):
    def read(self) -> bytes: ...

    def close(self) -> None: ...


class OpenFn(Protocol):
    def __call__(self, url: str, *, timeout: float) -> HTTPResponseLike: ...


ProbeFn = Callable[[str, str], bool]
DIRECT_OPEN = cast(OpenFn, build_opener(ProxyHandler({})).open)


def _finish_probe(
    future: "asyncio.Future[bool]",
    result: bool,
    error: Optional[Exception],
) -> None:
    if future.done():
        return
    if error is not None:
        future.set_exception(error)
    else:
        future.set_result(result)


async def _run_probe(
    probe_fn: ProbeFn,
    monitor_url: str,
    subject: str,
    timeout: float,
) -> bool:
    loop = asyncio.get_running_loop()
    future: asyncio.Future[bool] = loop.create_future()

    def execute() -> None:
        try:
            result, error = probe_fn(monitor_url, subject), None
        except Exception as exc:
            result, error = False, exc
        try:
            loop.call_soon_threadsafe(_finish_probe, future, result, error)
        except RuntimeError:
            pass

    threading.Thread(target=execute, name="cap-nats-monitor", daemon=True).start()
    return await asyncio.wait_for(future, timeout)


def _fetch_json(url: str, timeout: float, open_fn: OpenFn) -> object:
    response = open_fn(url, timeout=timeout)
    try:
        return cast(object, json.loads(response.read()))
    finally:
        response.close()


def _monitor_has_subscription(
    monitor_url: str,
    subject: str,
    *,
    open_fn: OpenFn = DIRECT_OPEN,
) -> bool:
    base_url = monitor_url.rstrip("/")
    health = _fetch_json(f"{base_url}/healthz", 1.0, open_fn)
    if not isinstance(health, dict) or health.get("status") != "ok":
        return False
    report = _fetch_json(f"{base_url}/subsz?subs=1", 1.0, open_fn)
    if not isinstance(report, dict):
        return False
    subscriptions = report.get("subscriptions_list")
    if not isinstance(subscriptions, list):
        return False
    return any(
        isinstance(entry, dict) and entry.get("subject") == subject
        for entry in subscriptions
    )


async def _wait_for_worker(
    monitor_url: str,
    subject: str,
    timeout: float,
    *,
    probe_fn: ProbeFn = _monitor_has_subscription,
    poll_interval: float = 0.25,
) -> bool:
    loop = asyncio.get_running_loop()
    deadline = loop.time() + timeout
    while loop.time() < deadline:
        remaining = deadline - loop.time()
        try:
            if await _run_probe(probe_fn, monitor_url, subject, remaining):
                return True
        except Exception as exc:
            log.debug("NATS monitoring not ready: %s", exc)
        remaining = deadline - loop.time()
        if remaining > 0:
            await asyncio.sleep(min(poll_interval, remaining))
    return False
