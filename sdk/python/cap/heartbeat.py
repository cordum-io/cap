"""Heartbeat helpers for CAP Python SDK.

These helpers build and publish heartbeat BusPacket envelopes.
"""

import asyncio
import logging
from typing import Callable, Optional

from cryptography.hazmat.primitives.asymmetric import ec
from google.protobuf import timestamp_pb2

from cap.client import DEFAULT_PROTOCOL_VERSION
from cap.metrics import MetricsHook, safe_metrics_call
from cap.pb.cordum.agent.v1 import buspacket_pb2, heartbeat_pb2
from cap.packet_boundary import Publisher, finalize_packet, parse_packet
from cap.subjects import SUBJECT_HEARTBEAT

_logger = logging.getLogger("cap.heartbeat")

#: Maximum length (in characters) of a human-facing agent display label.
AGENT_NAME_MAX_LEN = 128


def sanitize_agent_name(name: str) -> str:
    """Trim, collapse internal whitespace, and bound a human-facing agent
    display label for safe transport on ``Heartbeat``/``Handshake``.

    The result is a DISPLAY label only: consumers MUST prefer authenticated
    identity records over this self-reported value and MUST NOT treat it as
    proof of identity.
    """
    return " ".join(name.split())[:AGENT_NAME_MAX_LEN]


def heartbeat_payload(
    worker_id: str,
    pool: str,
    active_jobs: int,
    max_parallel: int,
    cpu_load: float,
    auth_token: str = "",
    agent_name: str = "",
    *,
    session_token: Optional[str] = None,
) -> bytes:
    """Build a heartbeat payload with CPU utilization only."""
    return heartbeat_payload_with_progress(
        worker_id=worker_id,
        pool=pool,
        active_jobs=active_jobs,
        max_parallel=max_parallel,
        cpu_load=cpu_load,
        auth_token=auth_token,
        agent_name=agent_name,
        session_token=session_token,
    )


def heartbeat_payload_with_memory(
    worker_id: str,
    pool: str,
    active_jobs: int,
    max_parallel: int,
    cpu_load: float,
    memory_load: float,
    auth_token: str = "",
    agent_name: str = "",
    *,
    session_token: Optional[str] = None,
) -> bytes:
    """Build a heartbeat payload including memory utilization."""
    return heartbeat_payload_with_progress(
        worker_id=worker_id,
        pool=pool,
        active_jobs=active_jobs,
        max_parallel=max_parallel,
        cpu_load=cpu_load,
        memory_load=memory_load,
        auth_token=auth_token,
        agent_name=agent_name,
        session_token=session_token,
    )


def heartbeat_payload_with_progress(
    worker_id: str,
    pool: str,
    active_jobs: int,
    max_parallel: int,
    cpu_load: float,
    memory_load: float = 0.0,
    progress_pct: int = 0,
    last_memo: str = "",
    auth_token: str = "",
    agent_name: str = "",
    *,
    session_token: Optional[str] = None,
) -> bytes:
    """Build a heartbeat payload including optional progress fields."""
    ts = timestamp_pb2.Timestamp()
    ts.GetCurrentTime()
    auth_token = auth_token.strip()

    packet = buspacket_pb2.BusPacket()
    packet.trace_id = worker_id
    packet.sender_id = worker_id
    packet.protocol_version = DEFAULT_PROTOCOL_VERSION
    packet.created_at.CopyFrom(ts)
    packet.heartbeat.CopyFrom(
        heartbeat_pb2.Heartbeat(
            worker_id=worker_id,
            pool=pool,
            active_jobs=active_jobs,
            max_parallel_jobs=max_parallel,
            cpu_load=cpu_load,
            memory_load=memory_load,
            progress_pct=progress_pct,
            last_memo=last_memo,
            auth_token=auth_token,
            agent_name=sanitize_agent_name(agent_name),
        )
    )
    outgoing = finalize_packet(packet, session_token=session_token)
    return outgoing.SerializeToString(deterministic=True)


async def emit_heartbeat(
    nc: Publisher,
    payload: bytes,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    *,
    session_token: Optional[str] = None,
) -> None:
    """Publish one heartbeat packet to the heartbeat subject."""
    packet = finalize_packet(
        parse_packet(payload), private_key, session_token=session_token
    )
    data = packet.SerializeToString(deterministic=True)
    await nc.publish(SUBJECT_HEARTBEAT, data)


async def _wait_interval(
    interval: float,
    cancel_event: Optional[asyncio.Event],
) -> bool:
    if cancel_event is None:
        await asyncio.sleep(interval)
        return False
    sleep_task = asyncio.create_task(asyncio.sleep(interval))
    cancel_task = asyncio.create_task(cancel_event.wait())
    done, pending = await asyncio.wait(
        {sleep_task, cancel_task},
        return_when=asyncio.FIRST_COMPLETED,
    )
    for task in pending:
        task.cancel()
    if pending:
        await asyncio.gather(*pending, return_exceptions=True)
    return cancel_task in done and cancel_event.is_set()


async def heartbeat_loop(
    nc: Publisher,
    payload_fn: Callable[[], bytes],
    interval: float = 5.0,
    private_key: Optional[ec.EllipticCurvePrivateKey] = None,
    metrics: Optional[MetricsHook] = None,
    cancel_event: Optional[asyncio.Event] = None,
    *,
    session_token: Optional[str] = None,
) -> None:
    """Emit heartbeat packets periodically until cancelled."""
    sleep_interval = max(0.0, interval)

    while True:
        if cancel_event is not None and cancel_event.is_set():
            return
        if await _wait_interval(sleep_interval, cancel_event):
            return

        try:
            payload = payload_fn()
            await emit_heartbeat(
                nc=nc,
                payload=payload,
                private_key=private_key,
                session_token=session_token,
            )
            if metrics is not None:
                packet = buspacket_pb2.BusPacket()
                packet.ParseFromString(payload)
                worker_id = packet.heartbeat.worker_id or packet.sender_id
                safe_metrics_call(
                    _logger, lambda: metrics.on_heartbeat_sent(worker_id)
                )
        except asyncio.CancelledError:
            raise
        except Exception:
            _logger.exception("heartbeat emission failed")
