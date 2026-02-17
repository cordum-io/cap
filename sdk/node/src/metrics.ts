/**
 * MetricsHook receives lifecycle events for observability.
 * Implement this interface to integrate with Prometheus, StatsD, OpenTelemetry, etc.
 * The default is noopMetrics which has zero overhead.
 */
export interface MetricsHook {
  onJobReceived(jobId: string, topic: string): void;
  onJobCompleted(jobId: string, durationMs: number, status: string): void;
  onJobFailed(jobId: string, errorMsg: string): void;
  onHeartbeatSent(workerId: string): void;
}

/** No-op metrics implementation with zero overhead. */
export const noopMetrics: MetricsHook = {
  onJobReceived() {},
  onJobCompleted() {},
  onJobFailed() {},
  onHeartbeatSent() {},
};
