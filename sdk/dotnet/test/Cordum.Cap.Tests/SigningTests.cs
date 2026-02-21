using System.Security.Cryptography;
using Google.Protobuf;
using Google.Protobuf.WellKnownTypes;
using Cordum.Agent.V1;
using Xunit;

namespace Cordum.Cap.Tests;

public class SigningTests
{
    private static BusPacket MakePacket() => new()
    {
        TraceId = "trace-sign",
        SenderId = "sender-sign",
        ProtocolVersion = 1,
        CreatedAt = new Timestamp { Seconds = 1704164645, Nanos = 0 },
        JobRequest = new JobRequest
        {
            JobId = "job-sign-1",
            Topic = "sign.test",
        },
    };

    private static (ECDsa Private, ECDsa Public) MakeKeyPair()
    {
        var key = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var pubOnly = ECDsa.Create(key.ExportParameters(false));
        return (key, pubOnly);
    }

    [Fact]
    public void SignAndVerify_RoundTrip()
    {
        var (priv, pub) = MakeKeyPair();
        var packet = MakePacket();

        Signing.SignPacket(packet, priv);
        Assert.False(packet.Signature.IsEmpty);
        Assert.True(Signing.VerifyPacketSignature(packet, pub));
    }

    [Fact]
    public void SignAndVerify_Heartbeat()
    {
        var (priv, pub) = MakeKeyPair();
        var packet = new BusPacket
        {
            TraceId = "trace-hb",
            SenderId = "worker-hb",
            ProtocolVersion = 1,
            Heartbeat = new Heartbeat
            {
                WorkerId = "worker-hb",
                Pool = "gpu",
                ActiveJobs = 3,
                MaxParallelJobs = 10,
                CpuLoad = 75.0f,
            },
        };

        Signing.SignPacket(packet, priv);
        Assert.True(Signing.VerifyPacketSignature(packet, pub));
    }

    [Fact]
    public void TamperTraceId_Detected()
    {
        var (priv, pub) = MakeKeyPair();
        var packet = MakePacket();
        Signing.SignPacket(packet, priv);

        packet.TraceId = "tampered";
        Assert.False(Signing.VerifyPacketSignature(packet, pub));
    }

    [Fact]
    public void TamperSenderId_Detected()
    {
        var (priv, pub) = MakeKeyPair();
        var packet = MakePacket();
        Signing.SignPacket(packet, priv);

        packet.SenderId = "tampered";
        Assert.False(Signing.VerifyPacketSignature(packet, pub));
    }

    [Fact]
    public void TamperPayload_Detected()
    {
        var (priv, pub) = MakeKeyPair();
        var packet = MakePacket();
        Signing.SignPacket(packet, priv);

        packet.JobRequest = new JobRequest { JobId = "tampered", Topic = "tampered" };
        Assert.False(Signing.VerifyPacketSignature(packet, pub));
    }

    [Fact]
    public void TamperProtocolVersion_Detected()
    {
        var (priv, pub) = MakeKeyPair();
        var packet = MakePacket();
        Signing.SignPacket(packet, priv);

        packet.ProtocolVersion = 99;
        Assert.False(Signing.VerifyPacketSignature(packet, pub));
    }

    [Fact]
    public void EmptySignature_Rejected()
    {
        var (_, pub) = MakeKeyPair();
        var packet = MakePacket();
        Assert.False(Signing.VerifyPacketSignature(packet, pub));
    }

    [Fact]
    public void WrongKey_Rejected()
    {
        var (priv1, _) = MakeKeyPair();
        var (_, pub2) = MakeKeyPair();
        var packet = MakePacket();

        Signing.SignPacket(packet, priv1);
        Assert.False(Signing.VerifyPacketSignature(packet, pub2));
    }

    [Fact]
    public void Signature_IsDerEncoded()
    {
        var (priv, _) = MakeKeyPair();
        var packet = MakePacket();
        Signing.SignPacket(packet, priv);

        var sig = packet.Signature.ToByteArray();
        // DER-encoded ECDSA signatures start with 0x30 (SEQUENCE tag)
        Assert.Equal(0x30, sig[0]);
        // P-256 DER signatures are typically 68-73 bytes
        Assert.InRange(sig.Length, 68, 73);
    }

    [Fact]
    public void Signing_DoesNotAlterContent()
    {
        var (priv, _) = MakeKeyPair();
        var packet = MakePacket();
        var clean = packet.Clone();
        clean.Signature = ByteString.Empty;
        var originalBytes = Codec.MarshalDeterministic(clean);

        Signing.SignPacket(packet, priv);

        var after = packet.Clone();
        after.Signature = ByteString.Empty;
        var afterBytes = Codec.MarshalDeterministic(after);

        Assert.Equal(originalBytes, afterBytes);
    }

    [Fact]
    public void DoubleSigning_ProducesValidSignature()
    {
        var (priv, pub) = MakeKeyPair();
        var packet = MakePacket();

        Signing.SignPacket(packet, priv);
        Assert.True(Signing.VerifyPacketSignature(packet, pub));

        Signing.SignPacket(packet, priv);
        Assert.True(Signing.VerifyPacketSignature(packet, pub));
    }
}
