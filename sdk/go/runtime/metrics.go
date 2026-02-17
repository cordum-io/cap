package runtime

// MetricsHook receives lifecycle events for observability.
// Implement this interface to integrate with Prometheus, StatsD, OpenTelemetry, etc.
// The default is NoopMetrics which has zero overhead.
type MetricsHook interface {
	OnJobReceived(jobID, topic string)
	OnJobCompleted(jobID string, durationMs int64, status string)
	OnJobFailed(jobID string, errorMsg string)
	OnHeartbeatSent(workerID string)
}

// NoopMetrics is a no-op implementation of MetricsHook.
var NoopMetrics MetricsHook = noopMetrics{}

type noopMetrics struct{}

func (noopMetrics) OnJobReceived(string, string)        {}
func (noopMetrics) OnJobCompleted(string, int64, string) {}
func (noopMetrics) OnJobFailed(string, string)           {}
func (noopMetrics) OnHeartbeatSent(string)               {}
