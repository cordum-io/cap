package io.cordum.cap;

import com.google.protobuf.CodedOutputStream;
import com.google.protobuf.MessageLite;
import io.cordum.cap.agent.v1.BusPacket;

import java.io.ByteArrayOutputStream;
import java.io.IOException;

public final class Codec {

    private Codec() {}

    /**
     * Serialize a protobuf message with deterministic field ordering.
     * Produces byte-identical output to Go/Python/Node/C++ SDKs.
     */
    public static byte[] marshalDeterministic(MessageLite msg) {
        int size = msg.getSerializedSize();
        byte[] buf = new byte[size];
        try {
            CodedOutputStream cos = CodedOutputStream.newInstance(buf);
            cos.useDeterministicSerialization();
            msg.writeTo(cos);
            cos.checkNoSpaceLeft();
            return buf;
        } catch (IOException e) {
            throw new RuntimeException("deterministic serialization failed", e);
        }
    }

    /**
     * Serialize a BusPacket with the signature field cleared.
     * Used as the input to ECDSA signing/verification.
     */
    public static byte[] marshalUnsignedForSignature(BusPacket packet) {
        BusPacket unsigned = packet.toBuilder().clearSignature().build();
        return marshalDeterministic(unsigned);
    }
}
