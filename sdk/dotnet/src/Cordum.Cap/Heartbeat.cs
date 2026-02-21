using Google.Protobuf.WellKnownTypes;
using NATS.Client.Core;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Heartbeat payload constructors and emission helpers.
/// </summary>
public static class HeartbeatHelper
{
    /// <summary>
    /// Build a heartbeat payload with basic fields.
    /// </summary>
    public static byte[] HeartbeatPayload(
        string workerId, string pool, int activeJobs, int maxParallel, float cpuLoad)
    {
        return HeartbeatPayloadWithProgress(workerId, pool, activeJobs, maxParallel, cpuLoad, 0f, 0, "");
    }

    /// <summary>
    /// Build a heartbeat payload with memory load.
    /// </summary>
    public static byte[] HeartbeatPayloadWithMemory(
        string workerId, string pool, int activeJobs, int maxParallel, float cpuLoad, float memoryLoad)
    {
        return HeartbeatPayloadWithProgress(workerId, pool, activeJobs, maxParallel, cpuLoad, memoryLoad, 0, "");
    }

    /// <summary>
    /// Build a heartbeat payload with all fields including progress and memo.
    /// </summary>
    public static byte[] HeartbeatPayloadWithProgress(
        string workerId, string pool, int activeJobs, int maxParallel,
        float cpuLoad, float memoryLoad, int progressPct, string lastMemo)
    {
        var hb = new Heartbeat
        {
            WorkerId = workerId,
            Pool = pool,
            ActiveJobs = activeJobs,
            MaxParallelJobs = maxParallel,
            CpuLoad = cpuLoad,
            MemoryLoad = memoryLoad,
            ProgressPct = progressPct,
            LastMemo = lastMemo,
        };

        var packet = new BusPacket
        {
            SenderId = workerId,
            ProtocolVersion = Subjects.DefaultProtocolVersion,
            CreatedAt = Timestamp.FromDateTimeOffset(DateTimeOffset.UtcNow),
            Heartbeat = hb,
        };

        return Codec.MarshalDeterministic(packet);
    }

    /// <summary>
    /// Publish a heartbeat payload to the heartbeat subject.
    /// </summary>
    public static async Task EmitHeartbeat(NatsConnection nc, byte[] payload, CancellationToken ct = default)
    {
        await nc.PublishAsync(Subjects.Heartbeat, payload, cancellationToken: ct);
    }
}

/// <summary>
/// Periodically emits heartbeats using a PeriodicTimer.
/// </summary>
public sealed class HeartbeatLoop : IAsyncDisposable
{
    private readonly NatsConnection _nc;
    private readonly Func<byte[]> _payloadFn;
    private readonly string _workerId;
    private readonly TimeSpan _interval;
    private readonly IMetricsHook _metrics;
    private readonly CancellationTokenSource _cts = new();
    private Task? _loopTask;

    public HeartbeatLoop(
        NatsConnection nc,
        Func<byte[]> payloadFn,
        string workerId,
        TimeSpan interval,
        IMetricsHook? metrics = null)
    {
        _nc = nc;
        _payloadFn = payloadFn;
        _workerId = workerId;
        _interval = interval;
        _metrics = metrics ?? new NoopMetrics();
    }

    /// <summary>
    /// Start the heartbeat loop. Returns immediately; heartbeats run in the background.
    /// </summary>
    public void Start()
    {
        _loopTask = RunLoop(_cts.Token);
    }

    private async Task RunLoop(CancellationToken ct)
    {
        using var timer = new PeriodicTimer(_interval);
        try
        {
            while (await timer.WaitForNextTickAsync(ct))
            {
                try
                {
                    var payload = _payloadFn();
                    await HeartbeatHelper.EmitHeartbeat(_nc, payload, ct);
                    _metrics.OnHeartbeatSent(_workerId);
                }
                catch (OperationCanceledException)
                {
                    break;
                }
                catch
                {
                    // Swallow individual emit failures; loop continues
                }
            }
        }
        catch (OperationCanceledException)
        {
            // Normal shutdown
        }
    }

    /// <summary>
    /// Stop the heartbeat loop and wait for it to finish.
    /// </summary>
    public async Task StopAsync()
    {
        await _cts.CancelAsync();
        if (_loopTask != null)
        {
            await _loopTask;
        }
    }

    public async ValueTask DisposeAsync()
    {
        await StopAsync();
        _cts.Dispose();
    }
}
