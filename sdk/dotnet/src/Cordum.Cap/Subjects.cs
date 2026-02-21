namespace Cordum.Cap;

/// <summary>
/// NATS subject constants and protocol defaults for the Cordum Agent Protocol.
/// </summary>
public static class Subjects
{
    public const string Submit = "sys.job.submit";
    public const string Result = "sys.job.result";
    public const string Heartbeat = "sys.heartbeat";
    public const string Alert = "sys.alert";
    public const string Progress = "sys.job.progress";
    public const string Cancel = "sys.job.cancel";
    public const string Dlq = "sys.job.dlq";
    public const string WorkflowEvent = "sys.workflow.event";
    public const string Handshake = "sys.handshake";

    public const int DefaultProtocolVersion = 1;
    public static readonly TimeSpan DefaultHeartbeatInterval = TimeSpan.FromSeconds(5);
}
