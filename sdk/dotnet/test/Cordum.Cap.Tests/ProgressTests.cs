using Google.Protobuf;
using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class ProgressTests
{
    [Fact]
    public void ProgressPayload_Valid()
    {
        var bytes = ProgressHelper.ProgressPayload("s-1", "j-42", "step-3", 75, "processing");
        Assert.NotEmpty(bytes);

        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.Equal("s-1", packet.SenderId);
        Assert.Equal(1, packet.ProtocolVersion);

        var p = packet.JobProgress;
        Assert.NotNull(p);
        Assert.Equal("j-42", p.JobId);
        Assert.Equal("step-3", p.StepId);
        Assert.Equal(75, p.Percent);
        Assert.Equal("processing", p.Message);
    }

    [Fact]
    public void CancelPayload_Valid()
    {
        var bytes = ProgressHelper.CancelPayload("s-1", "j-99", "timeout", "admin");
        Assert.NotEmpty(bytes);

        var packet = BusPacket.Parser.ParseFrom(bytes);
        var c = packet.JobCancel;
        Assert.NotNull(c);
        Assert.Equal("j-99", c.JobId);
        Assert.Equal("timeout", c.Reason);
        Assert.Equal("admin", c.RequestedBy);
    }

    [Fact]
    public void ProgressPayload_ZeroPercent()
    {
        var bytes = ProgressHelper.ProgressPayload("s", "j", "st", 0, "");
        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.Equal(0, packet.JobProgress.Percent);
    }

    [Fact]
    public void ProgressPayload_100Percent()
    {
        var bytes = ProgressHelper.ProgressPayload("s", "j", "st", 100, "done");
        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.Equal(100, packet.JobProgress.Percent);
        Assert.Equal("done", packet.JobProgress.Message);
    }

    [Fact]
    public void ProgressPayload_HasTimestamp()
    {
        var bytes = ProgressHelper.ProgressPayload("s", "j", "st", 50, "half");
        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.NotNull(packet.CreatedAt);
        Assert.True(packet.CreatedAt.Seconds > 0);
    }

    [Fact]
    public void CancelPayload_HasTimestamp()
    {
        var bytes = ProgressHelper.CancelPayload("s", "j", "r", "u");
        var packet = BusPacket.Parser.ParseFrom(bytes);
        Assert.NotNull(packet.CreatedAt);
    }
}
