"""MetricsHook for CAP SDK observability.

Implement the MetricsHook protocol to integrate with Prometheus,
StatsD, OpenTelemetry, or any other metrics system.
The default is NoopMetrics which has zero overhead.
"""

import logging
from typing import Callable, Protocol


def safe_metrics_call(logger: logging.Logger, callback: Callable[[], None]) -> None:
    """Run noncritical metrics code without letting it control job delivery."""
    try:
        callback()
    except Exception as exc:
        try:
            logger.warning(
                "metrics hook failed",
                extra={"exception_type": type(exc).__name__},
            )
        except Exception:
            pass


class MetricsHook(Protocol):
    """Receives lifecycle events for observability."""

    def on_job_received(self, job_id: str, topic: str) -> None: ...
    def on_job_completed(self, job_id: str, duration_ms: int, status: str) -> None: ...
    def on_job_failed(self, job_id: str, error_msg: str) -> None: ...
    def on_heartbeat_sent(self, worker_id: str) -> None: ...


class NoopMetrics:
    """No-op metrics implementation with zero overhead."""

    def on_job_received(self, job_id: str, topic: str) -> None:
        pass

    def on_job_completed(self, job_id: str, duration_ms: int, status: str) -> None:
        pass

    def on_job_failed(self, job_id: str, error_msg: str) -> None:
        pass

    def on_heartbeat_sent(self, worker_id: str) -> None:
        pass
