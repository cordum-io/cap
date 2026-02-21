package io.cordum.cap;

import com.google.protobuf.ByteString;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.spec.ECGenParameterSpec;

import static org.junit.jupiter.api.Assertions.*;

class SigningTest {

    private static KeyPair keyPair;

    @BeforeAll
    static void generateKeys() throws Exception {
        KeyPairGenerator gen = KeyPairGenerator.getInstance("EC");
        gen.initialize(new ECGenParameterSpec("secp256r1"));
        keyPair = gen.generateKeyPair();
    }

    private BusPacket makePacket() {
        return BusPacket.newBuilder()
                .setTraceId("trace-1")
                .setSenderId("sender-1")
                .setProtocolVersion(1)
                .setJobRequest(JobRequest.newBuilder()
                        .setJobId("job-1")
                        .setTopic("test.topic")
                        .build())
                .build();
    }

    @Test
    void signAndVerifyRoundTrip() throws Exception {
        BusPacket packet = makePacket();
        BusPacket signed = Signing.signPacket(packet, keyPair.getPrivate());

        assertFalse(signed.getSignature().isEmpty());
        assertTrue(Signing.verifyPacketSignature(signed, keyPair.getPublic()));
    }

    @Test
    void tamperDetection() throws Exception {
        BusPacket signed = Signing.signPacket(makePacket(), keyPair.getPrivate());

        BusPacket tampered = signed.toBuilder()
                .setSenderId("tampered-sender")
                .build();

        assertFalse(Signing.verifyPacketSignature(tampered, keyPair.getPublic()));
    }

    @Test
    void emptySignatureRejectsFalse() throws Exception {
        BusPacket packet = makePacket();
        assertFalse(Signing.verifyPacketSignature(packet, keyPair.getPublic()));
    }

    @Test
    void invalidSignatureRejected() throws Exception {
        BusPacket packet = makePacket().toBuilder()
                .setSignature(ByteString.copyFromUtf8("not-a-valid-signature"))
                .build();

        assertFalse(Signing.verifyPacketSignature(packet, keyPair.getPublic()));
    }

    @Test
    void wrongKeyRejected() throws Exception {
        KeyPairGenerator gen = KeyPairGenerator.getInstance("EC");
        gen.initialize(new ECGenParameterSpec("secp256r1"));
        KeyPair other = gen.generateKeyPair();

        BusPacket signed = Signing.signPacket(makePacket(), keyPair.getPrivate());
        assertFalse(Signing.verifyPacketSignature(signed, other.getPublic()));
    }

    @Test
    void signedPacketPreservesFields() throws Exception {
        BusPacket packet = makePacket();
        BusPacket signed = Signing.signPacket(packet, keyPair.getPrivate());

        assertEquals(packet.getTraceId(), signed.getTraceId());
        assertEquals(packet.getSenderId(), signed.getSenderId());
        assertEquals(packet.getProtocolVersion(), signed.getProtocolVersion());
        assertEquals(packet.getJobRequest().getJobId(), signed.getJobRequest().getJobId());
    }

    @Test
    void deterministicSignatureInput() throws Exception {
        BusPacket packet = makePacket();

        byte[] unsigned1 = Codec.marshalUnsignedForSignature(packet);
        byte[] unsigned2 = Codec.marshalUnsignedForSignature(packet);
        assertArrayEquals(unsigned1, unsigned2);
    }
}
