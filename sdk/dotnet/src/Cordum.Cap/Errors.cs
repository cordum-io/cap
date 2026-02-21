using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Error code categories matching the CAP protocol specification.
/// </summary>
public enum CapErrorCode
{
    Unspecified = 0,
    // Protocol errors (100-199)
    VersionMismatch = 100,
    MalformedPacket = 101,
    UnknownPayload = 102,
    SignatureInvalid = 103,
    SignatureMissing = 104,
    // Job errors (200-299)
    JobTimeout = 200,
    ResourceExhausted = 201,
    PermissionDenied = 202,
    InvalidInput = 203,
    JobNotFound = 204,
    DuplicateJob = 205,
    WorkerUnavailable = 206,
    // Safety errors (300-399)
    SafetyDenied = 300,
    PolicyViolation = 301,
    RiskTagBlocked = 302,
    // Transport errors (400-499)
    PublishFailed = 400,
    SubscribeFailed = 401,
    ConnectionLost = 402,
}

/// <summary>
/// Base exception for all CAP protocol errors.
/// </summary>
public class CapException : Exception
{
    public CapErrorCode Code { get; }

    public CapException(CapErrorCode code, string message) : base(message)
    {
        Code = code;
    }

    public CapException(CapErrorCode code, string message, Exception innerException)
        : base(message, innerException)
    {
        Code = code;
    }

    // Protocol errors
    public static CapException VersionMismatch(string msg) => new(CapErrorCode.VersionMismatch, msg);
    public static CapException MalformedPacket(string msg) => new(CapErrorCode.MalformedPacket, msg);
    public static CapException UnknownPayload(string msg) => new(CapErrorCode.UnknownPayload, msg);
    public static CapException SignatureInvalid(string msg) => new(CapErrorCode.SignatureInvalid, msg);
    public static CapException SignatureMissing(string msg) => new(CapErrorCode.SignatureMissing, msg);

    // Job errors
    public static CapException JobTimeout(string msg) => new(CapErrorCode.JobTimeout, msg);
    public static CapException ResourceExhausted(string msg) => new(CapErrorCode.ResourceExhausted, msg);
    public static CapException PermissionDenied(string msg) => new(CapErrorCode.PermissionDenied, msg);
    public static CapException InvalidInput(string msg) => new(CapErrorCode.InvalidInput, msg);
    public static CapException JobNotFound(string msg) => new(CapErrorCode.JobNotFound, msg);
    public static CapException DuplicateJob(string msg) => new(CapErrorCode.DuplicateJob, msg);
    public static CapException WorkerUnavailable(string msg) => new(CapErrorCode.WorkerUnavailable, msg);

    // Safety errors
    public static CapException SafetyDenied(string msg) => new(CapErrorCode.SafetyDenied, msg);
    public static CapException PolicyViolation(string msg) => new(CapErrorCode.PolicyViolation, msg);
    public static CapException RiskTagBlocked(string msg) => new(CapErrorCode.RiskTagBlocked, msg);

    // Transport errors
    public static CapException PublishFailed(string msg) => new(CapErrorCode.PublishFailed, msg);
    public static CapException SubscribeFailed(string msg) => new(CapErrorCode.SubscribeFailed, msg);
    public static CapException ConnectionLost(string msg) => new(CapErrorCode.ConnectionLost, msg);
}

/// <summary>
/// Validation-specific exception with field information.
/// </summary>
public class ValidationException : CapException
{
    public string Field { get; }

    public ValidationException(string field, string message)
        : base(CapErrorCode.InvalidInput, $"field={field}, {message}")
    {
        Field = field;
    }
}
