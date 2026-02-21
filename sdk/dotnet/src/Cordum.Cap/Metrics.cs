namespace Cordum.Cap;

/// <summary>
/// Pluggable observability callbacks for CAP protocol events.
/// All methods have empty default implementations.
/// </summary>
public interface IMetricsHook
{
    void OnJobReceived(string jobId, string topic) { }
    void OnJobCompleted(string jobId, long durationMs, string status) { }
    void OnJobFailed(string jobId, string errorMsg) { }
    void OnHeartbeatSent(string workerId) { }
    void OnProgressEmitted(string jobId, int percent) { }
    void OnError(string context, string errorMsg) { }
}

/// <summary>
/// No-op metrics implementation. All callbacks are empty.
/// </summary>
public sealed class NoopMetrics : IMetricsHook { }
