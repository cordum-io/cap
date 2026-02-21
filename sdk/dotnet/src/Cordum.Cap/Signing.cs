using System.Security.Cryptography;
using Google.Protobuf;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// ECDSA P-256 packet signing and verification.
/// Uses ASN.1/DER signature format for interoperability with Go/Python/Node/Java/Rust SDKs.
/// </summary>
public static class Signing
{
    /// <summary>
    /// Sign a BusPacket using ECDSA P-256 with SHA-256.
    /// Sets the Signature field on the packet in ASN.1/DER format.
    /// </summary>
    public static void SignPacket(BusPacket packet, ECDsa key)
    {
        var unsignedBytes = Codec.MarshalUnsignedForSignature(packet);
        using var sha256 = SHA256.Create();
        var hash = sha256.ComputeHash(unsignedBytes);
        var signature = key.SignHash(hash, DSASignatureFormat.Rfc3279DerSequence);
        packet.Signature = ByteString.CopyFrom(signature);
    }

    /// <summary>
    /// Verify a BusPacket's ECDSA P-256 signature.
    /// Returns true if the signature is valid.
    /// </summary>
    public static bool VerifyPacketSignature(BusPacket packet, ECDsa key)
    {
        if (packet.Signature.IsEmpty)
            return false;

        var unsignedBytes = Codec.MarshalUnsignedForSignature(packet);
        using var sha256 = SHA256.Create();
        var hash = sha256.ComputeHash(unsignedBytes);
        return key.VerifyHash(hash, packet.Signature.ToByteArray(), DSASignatureFormat.Rfc3279DerSequence);
    }

    /// <summary>
    /// Load an ECDSA private key from PEM-encoded PKCS#8 text.
    /// </summary>
    public static ECDsa LoadPrivateKey(string pem)
    {
        var key = ECDsa.Create();
        key.ImportFromPem(pem);
        return key;
    }

    /// <summary>
    /// Load an ECDSA public key from PEM-encoded SPKI text.
    /// </summary>
    public static ECDsa LoadPublicKey(string pem)
    {
        var key = ECDsa.Create();
        key.ImportFromPem(pem);
        return key;
    }
}
