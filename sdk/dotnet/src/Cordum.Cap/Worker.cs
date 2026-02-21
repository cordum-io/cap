using System.Diagnostics;
using System.Security.Cryptography;
using Google.Protobuf;
using Google.Protobuf.WellKnownTypes;
using NATS.Client.Core;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// CAP worker that processes jobs from a NATS subject.
/// </summary>
public sealed class Worker
{
    private readonly NatsConnection _nc;
    private readonly string _subject;
    private readonly string _senderId;
    private readonly HandlerDelegate _baseHandler;
    private readonly ECDsa? _privateKey;
    private readonly Dictionary<string, ECDsa> _publicKeys;
    private readonly IMetricsHook _metrics;
    private readonly List<MiddlewareDelegate> _middlewares = new();

    public Worker(
        NatsConnection nc,
        string subject,
        string senderId,
        HandlerDelegate handler,
        ECDsa? privateKey = null,
        Dictionary<string, ECDsa>? publicKeys = null,
        IMetricsHook? metrics = null)
    {
        _nc = nc;
        _subject = subject;
        _senderId = senderId;
        _baseHandler = handler;
        _privateKey = privateKey;
        _publicKeys = publicKeys ?? new Dictionary<string, ECDsa>();
        _metrics = metrics ?? new NoopMetrics();
    }

    /// <summary>
    /// Add middleware(s) to the processing chain.
    /// </summary>
    public void Use(params MiddlewareDelegate[] mws)
    {
        _middlewares.AddRange(mws);
    }

    /// <summary>
    /// Start processing messages. Blocks until cancellation is requested.
    /// </summary>
    public async Task StartAsync(CancellationToken ct = default)
    {
        var handler = MiddlewareChain.ApplyMiddleware(_baseHandler, _middlewares);

        await foreach (var msg in _nc.SubscribeAsync<byte[]>(_subject, queueGroup: _subject, cancellationToken: ct))
        {
            if (msg.Data == null) continue;

            BusPacket packet;
            try
            {
                packet = BusPacket.Parser.ParseFrom(msg.Data);
            }
            catch (InvalidProtocolBufferException e)
            {
                _metrics.OnError("parse", $"failed: {e.Message}");
                continue;
            }

            if (_publicKeys.Count > 0)
            {
                if (!_publicKeys.TryGetValue(packet.SenderId, out var pubKey))
                    continue;
                if (packet.Signature.IsEmpty || !Signing.VerifyPacketSignature(packet, pubKey))
                    continue;
            }

            if (packet.PayloadCase != BusPacket.PayloadOneofCase.JobRequest)
                continue;

            var req = packet.JobRequest;
            _metrics.OnJobReceived(req.JobId, req.Topic);

            var ctx = new Context
            {
                TraceId = packet.TraceId,
                SenderId = packet.SenderId,
                Packet = packet,
            };

            var sw = Stopwatch.StartNew();
            JobResult result;
            try
            {
                result = handler(ctx, req);
                if (string.IsNullOrEmpty(result.JobId)) result.JobId = req.JobId;
                if (string.IsNullOrEmpty(result.WorkerId)) result.WorkerId = _senderId;

                sw.Stop();
                if (result.Status == JobStatus.Failed)
                {
                    _metrics.OnJobFailed(result.JobId, result.ErrorMessage);
                }
                else
                {
                    _metrics.OnJobCompleted(result.JobId, sw.ElapsedMilliseconds, "SUCCEEDED");
                }
            }
            catch (Exception ex)
            {
                sw.Stop();
                _metrics.OnJobFailed(req.JobId, ex.Message);
                result = new JobResult
                {
                    JobId = req.JobId,
                    WorkerId = _senderId,
                    Status = JobStatus.Failed,
                    ErrorMessage = ex.Message,
                };
            }

            var outPacket = new BusPacket
            {
                TraceId = packet.TraceId,
                SenderId = _senderId,
                ProtocolVersion = Subjects.DefaultProtocolVersion,
                CreatedAt = Timestamp.FromDateTimeOffset(DateTimeOffset.UtcNow),
                JobResult = result,
            };

            if (_privateKey != null)
            {
                Signing.SignPacket(outPacket, _privateKey);
            }

            var data = Codec.MarshalDeterministic(outPacket);
            await _nc.PublishAsync(Subjects.Result, data, cancellationToken: ct);
        }
    }
}
