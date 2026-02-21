using Google.Protobuf.WellKnownTypes;
using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class ValidateTests
{
    [Fact]
    public void ValidJobRequest_Passes()
    {
        var req = new JobRequest { JobId = "j-1", Topic = "test" };
        Validate.ValidateJobRequest(req); // Should not throw
    }

    [Fact]
    public void EmptyJobId_Rejected()
    {
        var req = new JobRequest { Topic = "test" };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateJobRequest(req));
        Assert.Equal("job_id", ex.Field);
    }

    [Fact]
    public void EmptyTopic_Rejected()
    {
        var req = new JobRequest { JobId = "j-1" };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateJobRequest(req));
        Assert.Equal("topic", ex.Field);
    }

    [Fact]
    public void NegativeStepIndex_Rejected()
    {
        var req = new JobRequest { JobId = "j-1", Topic = "t", StepIndex = -1 };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateJobRequest(req));
        Assert.Equal("step_index", ex.Field);
    }

    [Fact]
    public void NegativeBudget_Rejected()
    {
        var req = new JobRequest
        {
            JobId = "j-1",
            Topic = "t",
            Budget = new Budget { MaxInputTokens = -1 },
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateJobRequest(req));
        Assert.Contains("max_input_tokens", ex.Field);
    }

    [Fact]
    public void ValidJobResult_Passes()
    {
        var res = new JobResult
        {
            JobId = "j-1",
            WorkerId = "w-1",
            Status = JobStatus.Succeeded,
        };
        Validate.ValidateJobResult(res);
    }

    [Fact]
    public void UnspecifiedStatus_Rejected()
    {
        var res = new JobResult { JobId = "j-1", WorkerId = "w-1" };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateJobResult(res));
        Assert.Equal("status", ex.Field);
    }

    [Fact]
    public void EmptyWorkerId_Rejected()
    {
        var res = new JobResult { JobId = "j-1", Status = JobStatus.Succeeded };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateJobResult(res));
        Assert.Equal("worker_id", ex.Field);
    }

    [Fact]
    public void NegativeExecutionMs_Rejected()
    {
        var res = new JobResult
        {
            JobId = "j-1",
            WorkerId = "w-1",
            Status = JobStatus.Succeeded,
            ExecutionMs = -1,
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateJobResult(res));
        Assert.Equal("execution_ms", ex.Field);
    }

    [Fact]
    public void ValidBusPacket_Passes()
    {
        var pkt = new BusPacket
        {
            TraceId = "t",
            SenderId = "s",
            ProtocolVersion = 1,
            CreatedAt = new Timestamp { Seconds = 1 },
            JobRequest = new JobRequest { JobId = "j", Topic = "t" },
        };
        Validate.ValidateBusPacket(pkt);
    }

    [Fact]
    public void EmptyTraceId_Rejected()
    {
        var pkt = new BusPacket
        {
            SenderId = "s",
            ProtocolVersion = 1,
            CreatedAt = new Timestamp { Seconds = 1 },
            JobRequest = new JobRequest { JobId = "j", Topic = "t" },
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateBusPacket(pkt));
        Assert.Equal("trace_id", ex.Field);
    }

    [Fact]
    public void EmptySenderId_Rejected()
    {
        var pkt = new BusPacket
        {
            TraceId = "t",
            ProtocolVersion = 1,
            CreatedAt = new Timestamp { Seconds = 1 },
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateBusPacket(pkt));
        Assert.Equal("sender_id", ex.Field);
    }

    [Fact]
    public void ZeroProtocolVersion_Rejected()
    {
        var pkt = new BusPacket
        {
            TraceId = "t",
            SenderId = "s",
            CreatedAt = new Timestamp { Seconds = 1 },
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateBusPacket(pkt));
        Assert.Equal("protocol_version", ex.Field);
    }

    [Fact]
    public void MissingCreatedAt_Rejected()
    {
        var pkt = new BusPacket
        {
            TraceId = "t",
            SenderId = "s",
            ProtocolVersion = 1,
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateBusPacket(pkt));
        Assert.Equal("created_at", ex.Field);
    }

    [Fact]
    public void MissingPayload_Rejected()
    {
        var pkt = new BusPacket
        {
            TraceId = "t",
            SenderId = "s",
            ProtocolVersion = 1,
            CreatedAt = new Timestamp { Seconds = 1 },
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateBusPacket(pkt));
        Assert.Equal("payload", ex.Field);
    }

    [Fact]
    public void NestedValidation_JobRequestInPacket()
    {
        var pkt = new BusPacket
        {
            TraceId = "t",
            SenderId = "s",
            ProtocolVersion = 1,
            CreatedAt = new Timestamp { Seconds = 1 },
            JobRequest = new JobRequest { Topic = "t" }, // Missing JobId
        };
        var ex = Assert.Throws<ValidationException>(() => Validate.ValidateBusPacket(pkt));
        Assert.Equal("job_id", ex.Field);
    }
}
