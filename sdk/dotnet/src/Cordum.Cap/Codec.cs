using Google.Protobuf;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Deterministic protobuf serialization utilities.
/// </summary>
public static class Codec
{
    /// <summary>
    /// Serialize a protobuf message deterministically.
    /// Map fields are serialized in sorted key order when deterministic mode is enabled.
    /// </summary>
    public static byte[] MarshalDeterministic(IMessage msg)
    {
        var size = msg.CalculateSize();
        var buffer = new byte[size];
        var output = new CodedOutputStream(buffer);
        output.Deterministic = true;
        msg.WriteTo(output);
        output.Flush();
        return buffer;
    }

    /// <summary>
    /// Serialize a BusPacket with the signature field cleared.
    /// Used as the input to ECDSA signing/verification.
    /// </summary>
    public static byte[] MarshalUnsignedForSignature(BusPacket packet)
    {
        var clone = packet.Clone();
        clone.Signature = ByteString.Empty;
        return MarshalDeterministic(clone);
    }
}
