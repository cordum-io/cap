import asyncio
import math
import os
import sys
from typing import Optional, Sequence
from uuid import uuid4

import cap
import cap.constants
import nats
from google.protobuf import timestamp_pb2
from google.protobuf.message import DecodeError
from nats.aio.client import Client as NATS
from nats.aio.subscription import Subscription
from nats.errors import TimeoutError as NATSTimeoutError

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2


WORKER_SUBJECT = "job.echo"
CLIENT_ID = "client-echo-py"
EXIT_TRANSPORT = 2
EXIT_TIMEOUT = 4
EXIT_TERMINAL = 5
EXIT_PROTOCOL = 6
CONNECT_TIMEOUT_SECONDS = 5.0


def _result_timeout_seconds() -> float:
    raw = os.getenv("CAP_RESULT_TIMEOUT_SECONDS", "10")
    try:
        timeout = float(raw)
    except ValueError as exc:
        raise ValueError("CAP_RESULT_TIMEOUT_SECONDS must be a number") from exc
    if not math.isfinite(timeout) or timeout <= 0:
        raise ValueError(
            "CAP_RESULT_TIMEOUT_SECONDS must be finite and greater than zero"
        )
    return timeout


def _validation_summary(errors: Sequence[cap.ValidationError]) -> str:
    return "; ".join(f"{error.field}: {error.message}" for error in errors)


def _error_summary(error: Exception) -> str:
    return str(error).strip() or type(error).__name__


async def _report_nats_error(error: Exception) -> None:
    print(f"NATS client warning: {_error_summary(error)}", file=sys.stderr)


def _build_request(job_id: str) -> job_pb2.JobRequest:
    request = job_pb2.JobRequest(
        job_id=job_id,
        topic=WORKER_SUBJECT,
        context_ptr=f"demo://context/{job_id}",
    )
    errors = cap.validate_job_request(request)
    if errors:
        raise ValueError(f"invalid JobRequest: {_validation_summary(errors)}")
    has_whitespace = any(char.isspace() for char in request.topic)
    has_empty_token = any(not token for token in request.topic.split("."))
    has_wildcard = "*" in request.topic or ">" in request.topic
    if has_whitespace or has_empty_token or has_wildcard:
        raise ValueError(
            "direct worker subject requires nonempty tokens and no "
            "whitespace or wildcards"
        )
    return request


def _build_packet(
    request: job_pb2.JobRequest, trace_id: str
) -> buspacket_pb2.BusPacket:
    created_at = timestamp_pb2.Timestamp()
    created_at.GetCurrentTime()
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id,
        sender_id=CLIENT_ID,
        protocol_version=cap.constants.DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.CopyFrom(created_at)
    packet.job_request.CopyFrom(request)
    errors = cap.validate_bus_packet(packet)
    if errors:
        raise ValueError(f"invalid BusPacket: {_validation_summary(errors)}")
    return packet


def _status_name(status: int) -> str:
    try:
        return str(job_pb2.JobStatus.Name(status))
    except ValueError:
        return f"UNKNOWN({status})"


def _matching_verdict(
    packet: buspacket_pb2.BusPacket, job_id: str, trace_id: str
) -> Optional[int]:
    if (
        packet.trace_id != trace_id
        or packet.WhichOneof("payload") != "job_result"
    ):
        return None
    if packet.job_result.job_id != job_id:
        return None
    errors = cap.validate_bus_packet(packet)
    if errors:
        print(
            f"matching result is invalid: {_validation_summary(errors)}",
            file=sys.stderr,
        )
        return EXIT_PROTOCOL
    status = packet.job_result.status
    if status == job_pb2.JOB_STATUS_SUCCEEDED:
        print(f"CAP_SIMPLE_ECHO_SUCCESS job_id={job_id}")
        return 0
    if status in (
        job_pb2.JOB_STATUS_FAILED,
        job_pb2.JOB_STATUS_CANCELLED,
        job_pb2.JOB_STATUS_DENIED,
        job_pb2.JOB_STATUS_TIMEOUT,
        job_pb2.JOB_STATUS_FAILED_RETRYABLE,
        job_pb2.JOB_STATUS_FAILED_FATAL,
    ):
        print(f"job ended with {_status_name(status)}", file=sys.stderr)
        return EXIT_TERMINAL
    if status in (
        job_pb2.JOB_STATUS_PENDING,
        job_pb2.JOB_STATUS_SCHEDULED,
        job_pb2.JOB_STATUS_DISPATCHED,
        job_pb2.JOB_STATUS_RUNNING,
    ):
        return None
    print(
        f"matching result has unsupported status {_status_name(status)}",
        file=sys.stderr,
    )
    return EXIT_PROTOCOL


async def _wait_for_result(
    subscription: Subscription, deadline: float, job_id: str, trace_id: str
) -> int:
    loop = asyncio.get_running_loop()
    while True:
        remaining = deadline - loop.time()
        if remaining <= 0:
            print(
                "timed out waiting for a matching successful JobResult",
                file=sys.stderr,
            )
            return EXIT_TIMEOUT
        try:
            message = await subscription.next_msg(timeout=remaining)
        except (asyncio.TimeoutError, NATSTimeoutError):
            print(
                "timed out waiting for a matching successful JobResult",
                file=sys.stderr,
            )
            return EXIT_TIMEOUT
        packet = buspacket_pb2.BusPacket()
        try:
            packet.ParseFromString(message.data)
        except DecodeError:
            continue
        verdict = _matching_verdict(packet, job_id, trace_id)
        if verdict is not None:
            return verdict


async def _cleanup(
    connection: Optional[NATS], subscription: Optional[Subscription]
) -> None:
    if subscription is not None:
        try:
            await subscription.unsubscribe()
        except Exception as exc:
            print(f"cleanup warning: unsubscribe failed: {exc}", file=sys.stderr)
    if connection is not None:
        try:
            await connection.drain()
        except Exception as exc:
            print(f"cleanup warning: NATS drain failed: {exc}", file=sys.stderr)


async def main() -> int:
    job_id = f"job-echo-{uuid4().hex}"
    trace_id = f"trace-echo-{uuid4().hex}"
    try:
        request = _build_request(job_id)
        packet = _build_packet(request, trace_id)
        timeout = _result_timeout_seconds()
    except ValueError as exc:
        print(exc, file=sys.stderr)
        return EXIT_PROTOCOL

    connection: Optional[NATS] = None
    subscription: Optional[Subscription] = None
    try:
        nats_url = os.getenv("CAP_NATS_URL", "nats://127.0.0.1:4222")
        connection = await asyncio.wait_for(
            nats.connect(
                nats_url,
                name=CLIENT_ID,
                error_cb=_report_nats_error,
                connect_timeout=2,
                max_reconnect_attempts=1,
            ),
            timeout=CONNECT_TIMEOUT_SECONDS,
        )
        subscription = await connection.subscribe(cap.SUBJECT_RESULT)
        await connection.flush()
        deadline = asyncio.get_running_loop().time() + timeout
        print(
            "DEVELOPMENT ONLY: direct NATS publish bypasses Gateway, "
            "Scheduler, and Safety Kernel.",
            file=sys.stderr,
        )
        payload = packet.SerializeToString(deterministic=True)
        await connection.publish(request.topic, payload)
        await connection.flush()
        return await _wait_for_result(subscription, deadline, job_id, trace_id)
    except Exception as exc:
        print(f"NATS transport failed: {_error_summary(exc)}", file=sys.stderr)
        return EXIT_TRANSPORT
    finally:
        await _cleanup(connection, subscription)


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
