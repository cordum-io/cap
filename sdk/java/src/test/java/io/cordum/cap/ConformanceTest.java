package io.cordum.cap;

import com.google.protobuf.ByteString;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;
import org.junit.jupiter.api.Test;

import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.spec.ECGenParameterSpec;

import static org.junit.jupiter.api.Assertions.*;

class ConformanceTest {

    @Test
    void busPacketJobRequestRoundTrip() throws Exception {
        BusPacket original = BusPacket.newBuilder()
                .setTraceId("trace-42")
                .setSenderId("client-1")
                .setProtocolVersion(1)
                .setJobRequest(JobRequest.newBuilder()
                        .setJobId("job-100")
                        .setTopic("test.conformance")
                        .build())
                .build();

        byte[] bytes = Codec.marshalDeterministic(original);
        BusPacket parsed = BusPacket.parseFrom(bytes);

        assertEquals("trace-42", parsed.getTraceId());
        assertEquals("client-1", parsed.getSenderId());
        assertEquals(1, parsed.getProtocolVersion());
        assertEquals(BusPacket.PayloadCase.JOB_REQUEST, parsed.getPayloadCase());
        assertEquals("job-100", parsed.getJobRequest().getJobId());
        assertEquals("test.conformance", parsed.getJobRequest().getTopic());
    }

    @Test
    void busPacketJobResultRoundTrip() throws Exception {
        BusPacket original = BusPacket.newBuilder()
                .setTraceId("trace-43")
                .setSenderId("worker-1")
                .setProtocolVersion(1)
                .setJobResult(JobResult.newBuilder()
                        .setJobId("job-100")
                        .setWorkerId("worker-1")
                        .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                        .setExecutionMs(150)
                        .build())
                .build();

        byte[] bytes = Codec.marshalDeterministic(original);
        BusPacket parsed = BusPacket.parseFrom(bytes);

        assertEquals(BusPacket.PayloadCase.JOB_RESULT, parsed.getPayloadCase());
        assertEquals("job-100", parsed.getJobResult().getJobId());
        assertEquals(JobStatus.JOB_STATUS_SUCCEEDED, parsed.getJobResult().getStatus());
        assertEquals(150, parsed.getJobResult().getExecutionMs());
    }

    @Test
    void busPacketHeartbeatRoundTrip() throws Exception {
        io.cordum.cap.agent.v1.Heartbeat hb = io.cordum.cap.agent.v1.Heartbeat.newBuilder()
                .setWorkerId("worker-1")
                .setPool("pool-a")
                .setActiveJobs(3)
                .setMaxParallelJobs(8)
                .setCpuLoad(42.5f)
                .setMemoryLoad(55.0f)
                .build();

        BusPacket original = BusPacket.newBuilder()
                .setSenderId("worker-1")
                .setProtocolVersion(1)
                .setHeartbeat(hb)
                .build();

        byte[] bytes = Codec.marshalDeterministic(original);
        BusPacket parsed = BusPacket.parseFrom(bytes);

        assertTrue(parsed.hasHeartbeat());
        assertEquals("worker-1", parsed.getHeartbeat().getWorkerId());
        assertEquals(42.5f, parsed.getHeartbeat().getCpuLoad(), 0.01f);
    }

    @Test
    void signedPacketConformance() throws Exception {
        KeyPairGenerator gen = KeyPairGenerator.getInstance("EC");
        gen.initialize(new ECGenParameterSpec("secp256r1"));
        KeyPair kp = gen.generateKeyPair();

        BusPacket packet = BusPacket.newBuilder()
                .setTraceId("trace-signed")
                .setSenderId("signer-1")
                .setProtocolVersion(1)
                .setJobRequest(JobRequest.newBuilder()
                        .setJobId("job-signed")
                        .setTopic("conformance.sign")
                        .build())
                .build();

        BusPacket signed = Signing.signPacket(packet, kp.getPrivate());
        assertFalse(signed.getSignature().isEmpty());

        // Serialize and re-parse (simulating wire transfer)
        byte[] wire = Codec.marshalDeterministic(signed);
        BusPacket received = BusPacket.parseFrom(wire);

        assertTrue(Signing.verifyPacketSignature(received, kp.getPublic()));
        assertEquals("trace-signed", received.getTraceId());
        assertEquals("job-signed", received.getJobRequest().getJobId());
    }

    @Test
    void unsignedBytesAreStable() {
        BusPacket packet = BusPacket.newBuilder()
                .setTraceId("trace-stable")
                .setSenderId("sender-stable")
                .setProtocolVersion(1)
                .setSignature(ByteString.copyFromUtf8("some-sig"))
                .setJobRequest(JobRequest.newBuilder()
                        .setJobId("j-stable")
                        .setTopic("t-stable")
                        .build())
                .build();

        byte[] a = Codec.marshalUnsignedForSignature(packet);
        byte[] b = Codec.marshalUnsignedForSignature(packet);

        assertArrayEquals(a, b);
        // Verify signature was stripped
        BusPacket decoded = assertDoesNotThrow(() -> BusPacket.parseFrom(a));
        assertTrue(decoded.getSignature().isEmpty());
    }
}
