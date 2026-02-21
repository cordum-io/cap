using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Tracks published messages for test assertions.
/// </summary>
public sealed class MockBus
{
    private readonly List<(string Subject, byte[] Data)> _published = new();
    private readonly object _lock = new();

    public void Publish(string subject, byte[] data)
    {
        lock (_lock)
        {
            _published.Add((subject, data));
        }
    }

    public int PublishedCount
    {
        get { lock (_lock) { return _published.Count; } }
    }

    public List<byte[]> PublishedTo(string subject)
    {
        lock (_lock)
        {
            return _published
                .Where(p => p.Subject == subject)
                .Select(p => p.Data)
                .ToList();
        }
    }

    public void Clear()
    {
        lock (_lock)
        {
            _published.Clear();
        }
    }
}

/// <summary>
/// Submit a job request to a handler and return the result.
/// Useful for quick unit tests without NATS infrastructure.
/// </summary>
public static class TestHelper
{
    public static JobResult SubmitAndWait(HandlerDelegate handler, JobRequest req)
    {
        var ctx = new Context
        {
            TraceId = "test-trace",
            SenderId = "test-sender",
            Packet = null,
        };
        return handler(ctx, req);
    }
}

/// <summary>
/// Recording implementation of IMetricsHook for test assertions.
/// </summary>
public sealed class RecordingMetrics : IMetricsHook
{
    private readonly object _lock = new();

    public List<string> ReceivedJobs { get; } = new();
    public List<string> CompletedJobs { get; } = new();
    public List<string> FailedJobs { get; } = new();
    public List<string> Heartbeats { get; } = new();
    public List<string> ProgressEvents { get; } = new();
    public List<string> Errors { get; } = new();

    public void OnJobReceived(string jobId, string topic)
    {
        lock (_lock) { ReceivedJobs.Add(jobId); }
    }

    public void OnJobCompleted(string jobId, long durationMs, string status)
    {
        lock (_lock) { CompletedJobs.Add(jobId); }
    }

    public void OnJobFailed(string jobId, string errorMsg)
    {
        lock (_lock) { FailedJobs.Add(jobId); }
    }

    public void OnHeartbeatSent(string workerId)
    {
        lock (_lock) { Heartbeats.Add(workerId); }
    }

    public void OnProgressEmitted(string jobId, int percent)
    {
        lock (_lock) { ProgressEvents.Add(jobId); }
    }

    public void OnError(string context, string errorMsg)
    {
        lock (_lock) { Errors.Add($"{context}: {errorMsg}"); }
    }
}
