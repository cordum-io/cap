using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class AgentTests
{
    [Fact]
    public void MockBus_TracksPublishes()
    {
        var bus = new MockBus();
        bus.Publish("sys.heartbeat", new byte[] { 1, 2, 3 });
        bus.Publish("sys.job.result", new byte[] { 4, 5 });

        Assert.Equal(2, bus.PublishedCount);
        Assert.Single(bus.PublishedTo("sys.heartbeat"));
        Assert.Single(bus.PublishedTo("sys.job.result"));
        Assert.Empty(bus.PublishedTo("sys.job.submit"));
    }

    [Fact]
    public void MockBus_Clear()
    {
        var bus = new MockBus();
        bus.Publish("sys.heartbeat", new byte[] { 1 });
        Assert.Equal(1, bus.PublishedCount);

        bus.Clear();
        Assert.Equal(0, bus.PublishedCount);
    }

    [Fact]
    public void RecordingMetrics_TracksEvents()
    {
        var m = new RecordingMetrics();
        m.OnJobReceived("j-1", "test");
        m.OnJobCompleted("j-1", 100, "SUCCEEDED");
        m.OnHeartbeatSent("w-1");
        m.OnJobFailed("j-2", "timeout");
        m.OnError("parse", "invalid proto");

        Assert.Single(m.ReceivedJobs);
        Assert.Single(m.CompletedJobs);
        Assert.Single(m.Heartbeats);
        Assert.Single(m.FailedJobs);
        Assert.Single(m.Errors);
        Assert.Contains("parse: invalid proto", m.Errors);
    }

    [Fact]
    public void RecordingMetrics_MultipleEvents()
    {
        var m = new RecordingMetrics();
        m.OnJobReceived("j-1", "t");
        m.OnJobReceived("j-2", "t");
        m.OnJobReceived("j-3", "t");

        Assert.Equal(3, m.ReceivedJobs.Count);
    }

    [Fact]
    public void SubmitAndWait_ErrorHandler_ThrowsPropagated()
    {
        HandlerDelegate handler = (ctx, req) => throw new CapException(CapErrorCode.JobTimeout, "timed out");

        Assert.Throws<CapException>(() =>
            TestHelper.SubmitAndWait(handler, new JobRequest { JobId = "j", Topic = "t" }));
    }
}
