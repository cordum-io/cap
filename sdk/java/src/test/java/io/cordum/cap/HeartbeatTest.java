package io.cordum.cap;

import com.google.protobuf.util.Timestamps;
import io.cordum.cap.agent.v1.BusPacket;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class HeartbeatTest {

    @Test
    void heartbeatPayloadProducesValidPacket() throws Exception {
        byte[] bytes = Heartbeat.heartbeatPayload("worker-1", "pool-a", 3, 8, 42.5f);
        assertTrue(bytes.length > 0);

        BusPacket packet = BusPacket.parseFrom(bytes);
        assertEquals("worker-1", packet.getSenderId());
        assertEquals(1, packet.getProtocolVersion());
        assertTrue(packet.hasHeartbeat());

        io.cordum.cap.agent.v1.Heartbeat hb = packet.getHeartbeat();
        assertEquals("worker-1", hb.getWorkerId());
        assertEquals("pool-a", hb.getPool());
        assertEquals(3, hb.getActiveJobs());
        assertEquals(8, hb.getMaxParallelJobs());
        assertEquals(42.5f, hb.getCpuLoad(), 0.01f);
    }

    @Test
    void withMemorySetsMemoryLoad() throws Exception {
        byte[] bytes = Heartbeat.heartbeatPayloadWithMemory("w-1", "pool", 1, 4, 10f, 55f);
        BusPacket packet = BusPacket.parseFrom(bytes);

        assertEquals(55f, packet.getHeartbeat().getMemoryLoad(), 0.01f);
        assertEquals(0, packet.getHeartbeat().getProgressPct());
        assertTrue(packet.getHeartbeat().getLastMemo().isEmpty());
    }

    @Test
    void withProgressSetsAllFields() throws Exception {
        byte[] bytes = Heartbeat.heartbeatPayloadWithProgress(
                "w-2", "gpu-pool", 5, 10, 80f, 60f, 75, "step-3-done");
        BusPacket packet = BusPacket.parseFrom(bytes);

        io.cordum.cap.agent.v1.Heartbeat hb = packet.getHeartbeat();
        assertEquals("w-2", hb.getWorkerId());
        assertEquals("gpu-pool", hb.getPool());
        assertEquals(5, hb.getActiveJobs());
        assertEquals(10, hb.getMaxParallelJobs());
        assertEquals(80f, hb.getCpuLoad(), 0.01f);
        assertEquals(60f, hb.getMemoryLoad(), 0.01f);
        assertEquals(75, hb.getProgressPct());
        assertEquals("step-3-done", hb.getLastMemo());
    }

    @Test
    void heartbeatPayloadsHaveValidTimeAndStableSemantics() throws Exception {
        byte[] payload1 = Heartbeat.heartbeatPayload("w-1", "p", 0, 1, 0f);
        byte[] payload2 = Heartbeat.heartbeatPayload("w-1", "p", 0, 1, 0f);

        BusPacket packet1 = BusPacket.parseFrom(payload1);
        BusPacket packet2 = BusPacket.parseFrom(payload2);
        assertTrue(Timestamps.isValid(packet1.getCreatedAt()));
        assertTrue(Timestamps.isValid(packet2.getCreatedAt()));
        assertTrue(packet1.getCreatedAt().getSeconds() > 0);
        assertTrue(packet2.getCreatedAt().getSeconds() > 0);

        // Creation time is expected to vary; all other payload semantics are stable.
        assertEquals(
                packet1.toBuilder().clearCreatedAt().build(),
                packet2.toBuilder().clearCreatedAt().build());
    }
}
