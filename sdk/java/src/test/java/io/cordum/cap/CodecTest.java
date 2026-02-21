package io.cordum.cap;

import com.google.protobuf.ByteString;
import com.google.protobuf.Timestamp;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class CodecTest {

    private BusPacket makePacket() {
        return BusPacket.newBuilder()
                .setTraceId("test-trace")
                .setSenderId("test-sender")
                .setProtocolVersion(1)
                .setJobRequest(JobRequest.newBuilder()
                        .setJobId("job-1")
                        .setTopic("test.topic")
                        .build())
                .build();
    }

    @Test
    void deterministicSerializationIsStable() {
        BusPacket packet = makePacket();
        byte[] first = Codec.marshalDeterministic(packet);
        assertNotNull(first);
        assertTrue(first.length > 0);

        for (int i = 0; i < 100; i++) {
            byte[] next = Codec.marshalDeterministic(packet);
            assertArrayEquals(first, next, "iteration " + i);
        }
    }

    @Test
    void deterministicHeartbeatIsStable() {
        io.cordum.cap.agent.v1.Heartbeat hb = io.cordum.cap.agent.v1.Heartbeat.newBuilder()
                .setWorkerId("worker-1")
                .setPool("pool-a")
                .setActiveJobs(3)
                .setMaxParallelJobs(8)
                .setCpuLoad(42.5f)
                .setMemoryLoad(55.0f)
                .setProgressPct(67)
                .setLastMemo("checkpoint")
                .build();

        BusPacket packet = BusPacket.newBuilder()
                .setSenderId("worker-1")
                .setProtocolVersion(1)
                .setHeartbeat(hb)
                .build();

        byte[] first = Codec.marshalDeterministic(packet);
        for (int i = 0; i < 50; i++) {
            assertArrayEquals(first, Codec.marshalDeterministic(packet));
        }
    }

    @Test
    void marshalUnsignedStripsSignature() {
        BusPacket packet = makePacket().toBuilder()
                .setSignature(ByteString.copyFromUtf8("fake-sig-data"))
                .build();

        byte[] bytes = Codec.marshalUnsignedForSignature(packet);
        assertTrue(bytes.length > 0);

        BusPacket decoded = assertDoesNotThrow(() -> BusPacket.parseFrom(bytes));
        assertTrue(decoded.getSignature().isEmpty());
        assertEquals("test-sender", decoded.getSenderId());
        assertEquals("test-trace", decoded.getTraceId());
        assertEquals(1, decoded.getProtocolVersion());
        assertEquals("job-1", decoded.getJobRequest().getJobId());
    }

    @Test
    void marshalUnsignedPreservesAllOtherFields() {
        BusPacket packet = makePacket().toBuilder()
                .setSignature(ByteString.copyFromUtf8("to-be-stripped"))
                .build();

        byte[] withoutSig = Codec.marshalUnsignedForSignature(packet);
        byte[] clean = Codec.marshalDeterministic(
                packet.toBuilder().clearSignature().build());

        assertArrayEquals(clean, withoutSig);
    }

    @Test
    void emptyMessage() {
        BusPacket empty = BusPacket.getDefaultInstance();
        byte[] bytes = Codec.marshalDeterministic(empty);
        assertEquals(0, bytes.length);
    }

    @Test
    void roundTrip() {
        BusPacket packet = makePacket();
        byte[] bytes = Codec.marshalDeterministic(packet);
        assertTrue(bytes.length > 0);

        BusPacket decoded = assertDoesNotThrow(() -> BusPacket.parseFrom(bytes));
        assertEquals(packet.getTraceId(), decoded.getTraceId());
        assertEquals(packet.getSenderId(), decoded.getSenderId());
        assertEquals(packet.getProtocolVersion(), decoded.getProtocolVersion());
        assertEquals(packet.getJobRequest().getJobId(), decoded.getJobRequest().getJobId());
        assertEquals(packet.getJobRequest().getTopic(), decoded.getJobRequest().getTopic());
    }
}
