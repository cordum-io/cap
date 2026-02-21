using Google.Protobuf.WellKnownTypes;
using NATS.Client.Core;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Progress and cancel payload constructors and emission helpers.
/// </summary>
public static class ProgressHelper
{
    /// <summary>
    /// Build a job progress payload.
    /// </summary>
    public static byte[] ProgressPayload(
        string senderId, string jobId, string stepId, int percent, string message)
    {
        var progress = new JobProgress
        {
            JobId = jobId,
            StepId = stepId,
            Percent = percent,
            Message = message,
        };

        var packet = new BusPacket
        {
            SenderId = senderId,
            ProtocolVersion = Subjects.DefaultProtocolVersion,
            CreatedAt = Timestamp.FromDateTimeOffset(DateTimeOffset.UtcNow),
            JobProgress = progress,
        };

        return Codec.MarshalDeterministic(packet);
    }

    /// <summary>
    /// Build a job cancel payload.
    /// </summary>
    public static byte[] CancelPayload(
        string senderId, string jobId, string reason, string requestedBy)
    {
        var cancel = new JobCancel
        {
            JobId = jobId,
            Reason = reason,
            RequestedBy = requestedBy,
        };

        var packet = new BusPacket
        {
            SenderId = senderId,
            ProtocolVersion = Subjects.DefaultProtocolVersion,
            CreatedAt = Timestamp.FromDateTimeOffset(DateTimeOffset.UtcNow),
            JobCancel = cancel,
        };

        return Codec.MarshalDeterministic(packet);
    }

    /// <summary>
    /// Publish a progress payload to the progress subject.
    /// </summary>
    public static async Task EmitProgress(NatsConnection nc, byte[] payload, CancellationToken ct = default)
    {
        await nc.PublishAsync(Subjects.Progress, payload, cancellationToken: ct);
    }

    /// <summary>
    /// Publish a cancel payload to the cancel subject.
    /// </summary>
    public static async Task EmitCancel(NatsConnection nc, byte[] payload, CancellationToken ct = default)
    {
        await nc.PublishAsync(Subjects.Cancel, payload, cancellationToken: ct);
    }
}
