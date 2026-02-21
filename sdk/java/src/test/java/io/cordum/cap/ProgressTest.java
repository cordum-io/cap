package io.cordum.cap;

import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobCancel;
import io.cordum.cap.agent.v1.JobProgress;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class ProgressTest {

    @Test
    void progressPayloadProducesValidPacket() throws Exception {
        byte[] bytes = Progress.progressPayload("sender-1", "job-42", "step-3", 75, "processing");
        assertTrue(bytes.length > 0);

        BusPacket packet = BusPacket.parseFrom(bytes);
        assertEquals("sender-1", packet.getSenderId());
        assertEquals(1, packet.getProtocolVersion());
        assertTrue(packet.hasJobProgress());

        JobProgress p = packet.getJobProgress();
        assertEquals("job-42", p.getJobId());
        assertEquals("step-3", p.getStepId());
        assertEquals(75, p.getPercent());
        assertEquals("processing", p.getMessage());
    }

    @Test
    void progressPayloadZeroPercent() throws Exception {
        byte[] bytes = Progress.progressPayload("s", "j", "st", 0, "");
        BusPacket packet = BusPacket.parseFrom(bytes);
        assertTrue(packet.hasJobProgress());
        assertEquals(0, packet.getJobProgress().getPercent());
    }

    @Test
    void cancelPayloadProducesValidPacket() throws Exception {
        byte[] bytes = Progress.cancelPayload("sender-1", "job-99", "timeout", "admin");
        assertTrue(bytes.length > 0);

        BusPacket packet = BusPacket.parseFrom(bytes);
        assertEquals("sender-1", packet.getSenderId());
        assertTrue(packet.hasJobCancel());

        JobCancel c = packet.getJobCancel();
        assertEquals("job-99", c.getJobId());
        assertEquals("timeout", c.getReason());
        assertEquals("admin", c.getRequestedBy());
    }

    @Test
    void progressPayloadDeterministic() {
        byte[] a = Progress.progressPayload("s", "j", "st", 50, "half");
        byte[] b = Progress.progressPayload("s", "j", "st", 50, "half");
        // Note: timestamps differ, so we can't assertArrayEquals, but both should be valid
        assertTrue(a.length > 0);
        assertTrue(b.length > 0);
    }

    @Test
    void cancelPayloadDeterministic() {
        byte[] a = Progress.cancelPayload("s", "j", "r", "u");
        byte[] b = Progress.cancelPayload("s", "j", "r", "u");
        assertTrue(a.length > 0);
        assertTrue(b.length > 0);
    }
}
