using Google.Protobuf;
using Google.Protobuf.WellKnownTypes;
using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class CodecTests
{
    private static BusPacket MakeJobRequestPacket() => new()
    {
        TraceId = "trace-codec",
        SenderId = "sender-codec",
        ProtocolVersion = 1,
        CreatedAt = new Timestamp { Seconds = 1704164645, Nanos = 0 },
        JobRequest = new JobRequest
        {
            JobId = "job-codec-1",
            Topic = "codec.test",
        },
    };

    private static BusPacket MakeHeartbeatPacket() => new()
    {
        TraceId = "trace-hb",
        SenderId = "worker-hb",
        ProtocolVersion = 1,
        CreatedAt = new Timestamp { Seconds = 1704164645, Nanos = 0 },
        Heartbeat = new Heartbeat
        {
            WorkerId = "worker-hb",
            Pool = "default",
            ActiveJobs = 2,
            MaxParallelJobs = 8,
            CpuLoad = 45.5f,
        },
    };

    [Fact]
    public void Deterministic_100Iterations()
    {
        var packet = MakeJobRequestPacket();
        var first = Codec.MarshalDeterministic(packet);
        Assert.NotEmpty(first);

        for (var i = 0; i < 100; i++)
        {
            var next = Codec.MarshalDeterministic(packet);
            Assert.Equal(first, next);
        }
    }

    [Fact]
    public void Deterministic_Heartbeat_100Iterations()
    {
        var packet = MakeHeartbeatPacket();
        var first = Codec.MarshalDeterministic(packet);

        for (var i = 0; i < 100; i++)
        {
            Assert.Equal(first, Codec.MarshalDeterministic(packet));
        }
    }

    [Fact]
    public void RoundTrip_JobRequest()
    {
        var packet = MakeJobRequestPacket();
        var bytes = Codec.MarshalDeterministic(packet);
        var decoded = BusPacket.Parser.ParseFrom(bytes);

        Assert.Equal("trace-codec", decoded.TraceId);
        Assert.Equal("sender-codec", decoded.SenderId);
        Assert.Equal(1, decoded.ProtocolVersion);
        Assert.Equal("job-codec-1", decoded.JobRequest.JobId);
        Assert.Equal("codec.test", decoded.JobRequest.Topic);
    }

    [Fact]
    public void RoundTrip_Heartbeat()
    {
        var packet = MakeHeartbeatPacket();
        var bytes = Codec.MarshalDeterministic(packet);
        var decoded = BusPacket.Parser.ParseFrom(bytes);

        Assert.Equal("worker-hb", decoded.Heartbeat.WorkerId);
        Assert.Equal("default", decoded.Heartbeat.Pool);
        Assert.Equal(2, decoded.Heartbeat.ActiveJobs);
        Assert.Equal(8, decoded.Heartbeat.MaxParallelJobs);
        Assert.InRange(decoded.Heartbeat.CpuLoad, 45.4f, 45.6f);
    }

    [Fact]
    public void RoundTrip_JobResult()
    {
        var packet = new BusPacket
        {
            TraceId = "trace-res",
            SenderId = "worker-1",
            ProtocolVersion = 1,
            CreatedAt = new Timestamp { Seconds = 1704164645 },
            JobResult = new JobResult
            {
                JobId = "job-res-1",
                WorkerId = "worker-1",
                Status = JobStatus.Succeeded,
                ExecutionMs = 150,
            },
        };

        var bytes = Codec.MarshalDeterministic(packet);
        var decoded = BusPacket.Parser.ParseFrom(bytes);

        Assert.Equal("job-res-1", decoded.JobResult.JobId);
        Assert.Equal(JobStatus.Succeeded, decoded.JobResult.Status);
        Assert.Equal(150, decoded.JobResult.ExecutionMs);
    }

    [Fact]
    public void UnsignedForSignature_StripsSignature()
    {
        var packet = MakeJobRequestPacket();
        packet.Signature = ByteString.CopyFrom(new byte[] { 1, 2, 3, 4 });

        var unsigned = Codec.MarshalUnsignedForSignature(packet);
        var decoded = BusPacket.Parser.ParseFrom(unsigned);

        Assert.True(decoded.Signature.IsEmpty);
        Assert.Equal("trace-codec", decoded.TraceId);
    }

    [Fact]
    public void UnsignedForSignature_MatchesCleanPacket()
    {
        var packet = MakeJobRequestPacket();
        packet.Signature = ByteString.CopyFrom(new byte[] { 5, 6, 7 });

        var unsigned = Codec.MarshalUnsignedForSignature(packet);

        var clean = packet.Clone();
        clean.Signature = ByteString.Empty;
        var cleanBytes = Codec.MarshalDeterministic(clean);

        Assert.Equal(cleanBytes, unsigned);
    }

    [Fact]
    public void DifferentPackets_ProduceDifferentBytes()
    {
        var bytes1 = Codec.MarshalDeterministic(MakeJobRequestPacket());
        var bytes2 = Codec.MarshalDeterministic(MakeHeartbeatPacket());
        Assert.NotEqual(bytes1, bytes2);
    }
}
