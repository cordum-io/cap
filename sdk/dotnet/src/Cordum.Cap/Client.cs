using System.Security.Cryptography;
using Google.Protobuf;
using Google.Protobuf.WellKnownTypes;
using NATS.Client.Core;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// CAP client for submitting jobs.
/// </summary>
public sealed class Client
{
    private readonly NatsConnection _nc;
    private readonly Dictionary<string, ECDsa> _publicKeys;

    public Client(NatsConnection nc, Dictionary<string, ECDsa>? publicKeys = null)
    {
        _nc = nc;
        _publicKeys = publicKeys ?? new Dictionary<string, ECDsa>();
    }

    /// <summary>
    /// Submit a job request to the bus.
    /// </summary>
    public async Task SubmitAsync(
        JobRequest req,
        string traceId,
        string senderId,
        ECDsa? privateKey = null,
        CancellationToken ct = default)
    {
        var packet = new BusPacket
        {
            TraceId = traceId,
            SenderId = senderId,
            ProtocolVersion = Subjects.DefaultProtocolVersion,
            CreatedAt = Timestamp.FromDateTimeOffset(DateTimeOffset.UtcNow),
            JobRequest = req,
        };

        if (privateKey != null)
        {
            Signing.SignPacket(packet, privateKey);
        }

        var data = Codec.MarshalDeterministic(packet);
        await _nc.PublishAsync(Subjects.Submit, data, cancellationToken: ct);
    }

    /// <summary>
    /// Submit a job request with a specific protocol version.
    /// </summary>
    public async Task SubmitWithVersionAsync(
        JobRequest req,
        string traceId,
        string senderId,
        int protocolVersion,
        ECDsa? privateKey = null,
        CancellationToken ct = default)
    {
        var packet = new BusPacket
        {
            TraceId = traceId,
            SenderId = senderId,
            ProtocolVersion = protocolVersion,
            CreatedAt = Timestamp.FromDateTimeOffset(DateTimeOffset.UtcNow),
            JobRequest = req,
        };

        if (privateKey != null)
        {
            Signing.SignPacket(packet, privateKey);
        }

        var data = Codec.MarshalDeterministic(packet);
        await _nc.PublishAsync(Subjects.Submit, data, cancellationToken: ct);
    }
}
