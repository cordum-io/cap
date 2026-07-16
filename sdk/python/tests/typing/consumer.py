"""A downstream application compiled against CAP's public typed surface.

This file is a static fixture. It is type-checked but never executed.
"""

from dataclasses import dataclass
from typing import Optional

from cap import (
    Agent,
    BlobStore,
    CAPError,
    Context,
    InMemoryBlobStore,
    MetricsHook,
    cancel_payload,
    handshake_payload,
    heartbeat_payload_with_progress,
    progress_payload,
)


@dataclass(frozen=True)
class OrderRequest:
    """Validated application input for the registered CAP job."""

    order_id: str


@dataclass(frozen=True)
class OrderResult:
    """Typed application result serialized by the CAP runtime."""

    accepted: bool


class ApplicationMetrics:
    """A structurally typed metrics adapter owned by an SDK consumer."""

    def on_job_received(self, job_id: str, topic: str) -> None:
        pass

    def on_job_completed(
        self, job_id: str, duration_ms: int, status: str
    ) -> None:
        pass

    def on_job_failed(self, job_id: str, error_msg: str) -> None:
        pass

    def on_heartbeat_sent(self, worker_id: str) -> None:
        pass


def build_store() -> BlobStore:
    """Supply a concrete store through CAP's public protocol."""
    return InMemoryBlobStore()


def build_metrics() -> MetricsHook:
    """Supply an application adapter through CAP's public protocol."""
    return ApplicationMetrics()


def heartbeat() -> bytes:
    """Build a fully typed heartbeat callback for the runtime."""
    return heartbeat_payload_with_progress(
        worker_id="orders-worker",
        pool="orders",
        active_jobs=0,
        max_parallel=8,
        cpu_load=0.25,
        memory_load=0.5,
        progress_pct=0,
        agent_name="orders-agent",
    )


orders_agent = Agent(
    nats_url="nats://127.0.0.1:4222",
    store=build_store(),
    worker_id="orders-worker",
    pool="orders",
    max_parallel=8,
    heartbeat_payload_fn=heartbeat,
    metrics=build_metrics(),
)


@orders_agent.job(
    "job.orders.process",
    input_model=OrderRequest,
    output_model=OrderResult,
)
async def process_order(context: Context, request: OrderRequest) -> OrderResult:
    """Use the idiomatic decorator while preserving the handler signature."""
    return OrderResult(accepted=bool(context.job_id and request.order_id))


def build_agent() -> Agent:
    """Return the fully configured high-level runtime."""
    return orders_agent


async def manage_lifecycle(agent: Agent) -> None:
    """Exercise the public asynchronous lifecycle contract."""
    try:
        await agent.start()
    finally:
        await agent.close()


def context_identifiers(context: Context) -> tuple[str, str]:
    """Read stable, typed identifiers exposed to job handlers."""
    return context.job_id, context.trace_id


def protocol_payloads() -> tuple[bytes, bytes, bytes]:
    """Build handshake, progress, and cancellation envelopes."""
    handshake = handshake_payload(
        component_id="orders-worker",
        capabilities={"orders.process": True},
        ready_topics=("job.orders.process",),
    )
    progress = progress_payload(
        sender_id="orders-worker",
        job_id="job-123",
        step_id="validate",
        percent=50,
        message="validated",
    )
    cancellation = cancel_payload(
        sender_id="orders-api",
        job_id="job-123",
        reason="customer request",
        requested_by="user-456",
    )
    return handshake, progress, cancellation


def describe_error(error: CAPError) -> tuple[str, int, Optional[str]]:
    """Consume the typed CAP error taxonomy without stringly casting it."""
    message = str(error) or None
    return error.code, error.numeric_code, message
