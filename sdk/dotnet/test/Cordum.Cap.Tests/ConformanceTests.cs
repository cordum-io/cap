using System.Security.Cryptography;
using Google.Protobuf;
using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class ConformanceTests
{
    private static string FixturesDir()
    {
        // Navigate from sdk/dotnet/test/Cordum.Cap.Tests/ up to project root, then into spec/conformance/fixtures
        var dir = AppContext.BaseDirectory;
        var root = Path.GetFullPath(Path.Combine(dir, "..", "..", "..", "..", "..", ".."));
        return Path.Combine(root, "spec", "conformance", "fixtures");
    }

    private static BusPacket LoadFixture(string name)
    {
        var path = Path.Combine(FixturesDir(), name);
        var data = File.ReadAllBytes(path);
        return BusPacket.Parser.ParseFrom(data);
    }

    private static ECDsa LoadConformancePublicKey()
    {
        var path = Path.Combine(FixturesDir(), "public_key.pem");
        var pem = File.ReadAllText(path);
        return Signing.LoadPublicKey(pem);
    }

    private static void VerifyFixtureSignature(BusPacket pkt, ECDsa key)
    {
        Assert.False(pkt.Signature.IsEmpty, "fixture packet has no signature");
        Assert.True(Signing.VerifyPacketSignature(pkt, key), "signature verification failed");
    }

    private static void AssertTimestamp(BusPacket pkt)
    {
        Assert.NotNull(pkt.CreatedAt);
        // Expected: 2024-01-02T03:04:05Z = 1704164645 seconds
        Assert.Equal(1704164645, pkt.CreatedAt.Seconds);
        Assert.Equal(0, pkt.CreatedAt.Nanos);
        Assert.Equal(1, pkt.ProtocolVersion);
    }

    [Fact]
    public void Conformance_JobRequest()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_job_request.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var req = pkt.JobRequest;
        Assert.NotNull(req);
        Assert.Equal("job-req-1", req.JobId);
        Assert.Equal("job.tools", req.Topic);
        Assert.Equal(JobPriority.Interactive, req.Priority);
        Assert.Equal("redis://ctx:job-req-1", req.ContextPtr);
        Assert.Equal("us-east-1", req.Env["region"]);
        Assert.Equal("true", req.Env["sandbox"]);
        Assert.Equal("prod", req.Labels["env"]);
        Assert.Equal("platform", req.Labels["team"]);

        var meta = req.Meta;
        Assert.NotNull(meta);
        Assert.Equal("idem-123", meta.IdempotencyKey);
        Assert.Equal("repo.review", meta.Capability);
        Assert.Equal("conformance", meta.Labels["source"]);

        var comp = req.Compensation;
        Assert.NotNull(comp);
        Assert.Equal("job.rollback", comp.Topic);
        Assert.Equal("true", comp.Labels["rollback"]);
    }

    [Fact]
    public void Conformance_JobResult()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_job_result.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var res = pkt.JobResult;
        Assert.NotNull(res);
        Assert.Equal("job-res-1", res.JobId);
        Assert.Equal("worker-1", res.WorkerId);
        Assert.Equal(JobStatus.FailedRetryable, res.Status);
        Assert.Equal("E_TEMP", res.ErrorCode);
        Assert.Equal(2, res.ArtifactPtrs.Count);
    }

    [Fact]
    public void Conformance_Heartbeat()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_heartbeat.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var hb = pkt.Heartbeat;
        Assert.NotNull(hb);
        Assert.Equal("worker-1", hb.WorkerId);
        Assert.Equal("job.tools", hb.Pool);
        Assert.Equal("us-east-1a", hb.Labels["zone"]);
    }

    [Fact]
    public void Conformance_JobProgress()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_job_progress.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var progress = pkt.JobProgress;
        Assert.NotNull(progress);
        Assert.Equal("job-prog-1", progress.JobId);
        Assert.Equal(50, progress.Percent);
        Assert.Equal(JobStatus.Running, progress.Status);
    }

    [Fact]
    public void Conformance_JobCancel()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_job_cancel.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var cancel = pkt.JobCancel;
        Assert.NotNull(cancel);
        Assert.Equal("job-cancel-1", cancel.JobId);
        Assert.Equal("user-7", cancel.RequestedBy);
    }

    [Fact]
    public void Conformance_Alert()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_alert.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var alert = pkt.Alert;
        Assert.NotNull(alert);
        Assert.Equal("WARN", alert.Level);
        Assert.Equal("scheduler", alert.Component);
    }

    [Fact]
    public void Conformance_Handshake()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_handshake.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var hs = pkt.Handshake;
        Assert.NotNull(hs);
        Assert.Equal("worker-1", hs.ComponentId);
        Assert.Equal(ComponentRole.Worker, hs.Role);
        Assert.Equal(new[] { 1 }, hs.SupportedVersions);
        Assert.True(hs.Capabilities["signatures"]);
        Assert.True(hs.Capabilities["progress"]);
        Assert.True(hs.Capabilities["cancel"]);
        Assert.Equal("2.0.19", hs.SdkVersion);
    }

    [Fact]
    public void Conformance_AlertEnhanced()
    {
        var key = LoadConformancePublicKey();
        var pkt = LoadFixture("buspacket_alert_enhanced.bin");

        VerifyFixtureSignature(pkt, key);
        AssertTimestamp(pkt);

        var alert = pkt.Alert;
        Assert.NotNull(alert);
        Assert.Equal("CRITICAL", alert.Level);
        Assert.Equal("scheduler", alert.Component);
        Assert.Equal("SIGNATURE_INVALID", alert.Code);
        Assert.Equal(AlertSeverity.Critical, alert.Severity);
        Assert.Equal(ErrorCode.ProtocolSignatureInvalid, alert.ErrorCodeEnum);
        Assert.Equal("scheduler-1", alert.SourceComponent);
        Assert.Equal("worker-bad", alert.Details["sender"]);
        Assert.Equal("sys.job.result", alert.Details["subject"]);
        Assert.Equal("trace-offending-packet", alert.TraceId);
    }

    [Fact]
    public void Conformance_AllFixtures_SignatureVerification()
    {
        var key = LoadConformancePublicKey();
        var fixtures = new[]
        {
            "buspacket_job_request.bin",
            "buspacket_job_result.bin",
            "buspacket_heartbeat.bin",
            "buspacket_job_progress.bin",
            "buspacket_job_cancel.bin",
            "buspacket_alert.bin",
            "buspacket_handshake.bin",
            "buspacket_alert_enhanced.bin",
        };

        foreach (var fixture in fixtures)
        {
            var pkt = LoadFixture(fixture);
            Assert.False(pkt.Signature.IsEmpty, $"{fixture} has no signature");

            // Verify the full roundtrip: unsigned bytes -> SHA-256 -> verify
            var unsignedBytes = Codec.MarshalUnsignedForSignature(pkt);
            using var sha256 = SHA256.Create();
            var hash = sha256.ComputeHash(unsignedBytes);
            Assert.True(
                key.VerifyHash(hash, pkt.Signature.ToByteArray(), DSASignatureFormat.Rfc3279DerSequence),
                $"{fixture} signature verification failed");
        }
    }
}
