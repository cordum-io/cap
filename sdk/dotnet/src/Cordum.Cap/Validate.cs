using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Input validation matching the CAP guard SDK rules.
/// </summary>
public static class Validate
{
    /// <summary>
    /// Validate a JobRequest. Throws ValidationException on invalid input.
    /// </summary>
    public static void ValidateJobRequest(JobRequest req)
    {
        if (string.IsNullOrEmpty(req.JobId))
            throw new ValidationException("job_id", "must not be empty");
        if (string.IsNullOrEmpty(req.Topic))
            throw new ValidationException("topic", "must not be empty");
        if (req.StepIndex < 0)
            throw new ValidationException("step_index", "must be >= 0");
        if (req.Budget != null)
        {
            if (req.Budget.MaxInputTokens < 0)
                throw new ValidationException("budget.max_input_tokens", "must be >= 0");
            if (req.Budget.MaxOutputTokens < 0)
                throw new ValidationException("budget.max_output_tokens", "must be >= 0");
            if (req.Budget.MaxTotalTokens < 0)
                throw new ValidationException("budget.max_total_tokens", "must be >= 0");
            if (req.Budget.DeadlineMs < 0)
                throw new ValidationException("budget.deadline_ms", "must be >= 0");
        }
    }

    /// <summary>
    /// Validate a JobResult. Throws ValidationException on invalid input.
    /// </summary>
    public static void ValidateJobResult(JobResult res)
    {
        if (string.IsNullOrEmpty(res.JobId))
            throw new ValidationException("job_id", "must not be empty");
        if (res.Status == JobStatus.Unspecified)
            throw new ValidationException("status", "must not be UNSPECIFIED");
        if (string.IsNullOrEmpty(res.WorkerId))
            throw new ValidationException("worker_id", "must not be empty");
        if (res.ExecutionMs < 0)
            throw new ValidationException("execution_ms", "must be >= 0");
    }

    /// <summary>
    /// Validate a BusPacket envelope. Throws ValidationException on invalid input.
    /// </summary>
    public static void ValidateBusPacket(BusPacket pkt)
    {
        if (string.IsNullOrEmpty(pkt.TraceId))
            throw new ValidationException("trace_id", "must not be empty");
        if (string.IsNullOrEmpty(pkt.SenderId))
            throw new ValidationException("sender_id", "must not be empty");
        if (pkt.ProtocolVersion <= 0)
            throw new ValidationException("protocol_version", "must be > 0");
        if (pkt.CreatedAt == null)
            throw new ValidationException("created_at", "must not be null");
        if (pkt.PayloadCase == BusPacket.PayloadOneofCase.None)
            throw new ValidationException("payload", "must not be empty");

        switch (pkt.PayloadCase)
        {
            case BusPacket.PayloadOneofCase.JobRequest:
                ValidateJobRequest(pkt.JobRequest);
                break;
            case BusPacket.PayloadOneofCase.JobResult:
                ValidateJobResult(pkt.JobResult);
                break;
        }
    }
}
