using Google.Protobuf;
using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class HeartbeatTests
{
    [Fact]
    public void HeartbeatPayload_Valid()
    {
        var bytes = HeartbeatHelper.HeartbeatPayload("w-1", "pool-a", 3, 8, 42.5f);
        Assert.NotEmpty(bytes);

        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.Equal("w-1", packet.SenderId);
        Assert.Equal(1, packet.ProtocolVersion);

        var hb = packet.Heartbeat;
        Assert.NotNull(hb);
        Assert.Equal("w-1", hb.WorkerId);
        Assert.Equal("pool-a", hb.Pool);
        Assert.Equal(3, hb.ActiveJobs);
        Assert.Equal(8, hb.MaxParallelJobs);
        Assert.InRange(hb.CpuLoad, 42.4f, 42.6f);
        Assert.Equal(0f, hb.MemoryLoad);
        Assert.Equal(0, hb.ProgressPct);
    }

    [Fact]
    public void HeartbeatPayloadWithMemory_SetsMemoryLoad()
    {
        var bytes = HeartbeatHelper.HeartbeatPayloadWithMemory("w-1", "p", 1, 4, 10.0f, 55.0f);
        var packet = BusPacket.Parser.ParseFrom(bytes);
        var hb = packet.Heartbeat;

        Assert.NotNull(hb);
        Assert.InRange(hb.MemoryLoad, 54.9f, 55.1f);
        Assert.Equal(0, hb.ProgressPct);
    }

    [Fact]
    public void HeartbeatPayloadWithProgress_SetsAllFields()
    {
        var bytes = HeartbeatHelper.HeartbeatPayloadWithProgress(
            "w-2", "gpu", 5, 10, 80.0f, 60.0f, 75, "step-3");
        var packet = BusPacket.Parser.ParseFrom(bytes);
        var hb = packet.Heartbeat;

        Assert.NotNull(hb);
        Assert.Equal("w-2", hb.WorkerId);
        Assert.Equal("gpu", hb.Pool);
        Assert.Equal(5, hb.ActiveJobs);
        Assert.Equal(75, hb.ProgressPct);
        Assert.Equal("step-3", hb.LastMemo);
        Assert.InRange(hb.CpuLoad, 79.9f, 80.1f);
        Assert.InRange(hb.MemoryLoad, 59.9f, 60.1f);
    }

    [Fact]
    public void HeartbeatPayload_HasTimestamp()
    {
        var bytes = HeartbeatHelper.HeartbeatPayload("w", "p", 0, 1, 0f);
        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.NotNull(packet.CreatedAt);
        Assert.True(packet.CreatedAt.Seconds > 0);
    }

    [Fact]
    public void HeartbeatPayload_ZeroActiveJobs()
    {
        var bytes = HeartbeatHelper.HeartbeatPayload("w", "p", 0, 4, 0f);
        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.Equal(0, packet.Heartbeat.ActiveJobs);
    }
}
