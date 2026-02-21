using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class WorkerTests
{
    [Fact]
    public void SubmitAndWait_ReturnsResult()
    {
        HandlerDelegate handler = (ctx, req) => new JobResult
        {
            JobId = req.JobId,
            Status = JobStatus.Succeeded,
            WorkerId = "test-worker",
        };

        var req = new JobRequest { JobId = "j-1", Topic = "test" };
        var result = TestHelper.SubmitAndWait(handler, req);

        Assert.Equal("j-1", result.JobId);
        Assert.Equal(JobStatus.Succeeded, result.Status);
    }

    [Fact]
    public void SubmitAndWait_ContextPopulated()
    {
        string? capturedTraceId = null;
        string? capturedSenderId = null;

        HandlerDelegate handler = (ctx, req) =>
        {
            capturedTraceId = ctx.TraceId;
            capturedSenderId = ctx.SenderId;
            return new JobResult { JobId = req.JobId, Status = JobStatus.Succeeded, WorkerId = "w" };
        };

        TestHelper.SubmitAndWait(handler, new JobRequest { JobId = "j", Topic = "t" });

        Assert.Equal("test-trace", capturedTraceId);
        Assert.Equal("test-sender", capturedSenderId);
    }

    [Fact]
    public void Middleware_FIFOOrder()
    {
        var order = new List<int>();

        MiddlewareDelegate MakeMw(int id) => next => (ctx, req) =>
        {
            order.Add(id);
            return next(ctx, req);
        };

        HandlerDelegate baseHandler = (ctx, req) => new JobResult
        {
            JobId = req.JobId,
            Status = JobStatus.Succeeded,
            WorkerId = "w",
        };

        var chain = new List<MiddlewareDelegate> { MakeMw(1), MakeMw(2), MakeMw(3) };
        var wrapped = MiddlewareChain.ApplyMiddleware(baseHandler, chain);

        var ctx = new Context { TraceId = "t", SenderId = "s" };
        wrapped(ctx, new JobRequest { JobId = "j", Topic = "t" });

        Assert.Equal(new[] { 1, 2, 3 }, order);
    }

    [Fact]
    public void Middleware_EmptyChain_ReturnsBase()
    {
        HandlerDelegate baseHandler = (ctx, req) => new JobResult
        {
            JobId = req.JobId,
            Status = JobStatus.Succeeded,
            WorkerId = "w",
        };

        var wrapped = MiddlewareChain.ApplyMiddleware(baseHandler, new List<MiddlewareDelegate>());
        var ctx = new Context { TraceId = "t", SenderId = "s" };
        var result = wrapped(ctx, new JobRequest { JobId = "j-1", Topic = "t" });

        Assert.Equal("j-1", result.JobId);
    }

    [Fact]
    public void LoggingMiddleware_DoesNotInterfere()
    {
        HandlerDelegate baseHandler = (ctx, req) => new JobResult
        {
            JobId = req.JobId,
            Status = JobStatus.Succeeded,
            WorkerId = "w",
        };

        var chain = new List<MiddlewareDelegate> { MiddlewareChain.LoggingMiddleware() };
        var wrapped = MiddlewareChain.ApplyMiddleware(baseHandler, chain);

        var ctx = new Context { TraceId = "t", SenderId = "s" };
        var result = wrapped(ctx, new JobRequest { JobId = "logged-job", Topic = "t" });

        Assert.Equal("logged-job", result.JobId);
        Assert.Equal(JobStatus.Succeeded, result.Status);
    }
}
