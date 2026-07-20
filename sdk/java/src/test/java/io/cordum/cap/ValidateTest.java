package io.cordum.cap;

import com.google.protobuf.Timestamp;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.Budget;
import io.cordum.cap.agent.v1.JobPriority;
import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class ValidateTest {

    @Test
    void validJobRequest() {
        JobRequest req = JobRequest.newBuilder()
                .setJobId("job-1")
                .setTopic("test.topic")
                .build();
        assertDoesNotThrow(() -> Validate.validateJobRequest(req));
    }

    @Test
    void criticalJobPriorityIsAccepted() {
        JobRequest req = JobRequest.newBuilder()
                .setJobId("job-critical")
                .setTopic("test.topic")
                .setPriority(JobPriority.JOB_PRIORITY_CRITICAL)
                .build();
        assertDoesNotThrow(() -> Validate.validateJobRequest(req));
    }

    @Test
    void unknownJobPriorityIsRejected() {
        JobRequest req = JobRequest.newBuilder()
                .setJobId("job-unknown")
                .setTopic("test.topic")
                .setPriorityValue(JobPriority.JOB_PRIORITY_CRITICAL_VALUE + 1)
                .build();
        assertThrows(Errors.ValidationException.class,
                () -> Validate.validateJobRequest(req));
    }

    @Test
    void jobRequestNullRejects() {
        assertThrows(Errors.ValidationException.class,
                () -> Validate.validateJobRequest(null));
    }

    @Test
    void jobRequestEmptyJobIdRejects() {
        JobRequest req = JobRequest.newBuilder().setTopic("t").build();
        Errors.ValidationException ex = assertThrows(Errors.ValidationException.class,
                () -> Validate.validateJobRequest(req));
        assertEquals("job_id", ex.field());
    }

    @Test
    void jobRequestEmptyTopicRejects() {
        JobRequest req = JobRequest.newBuilder().setJobId("j").build();
        Errors.ValidationException ex = assertThrows(Errors.ValidationException.class,
                () -> Validate.validateJobRequest(req));
        assertEquals("topic", ex.field());
    }

    @Test
    void jobRequestNegativeBudgetRejects() {
        JobRequest req = JobRequest.newBuilder()
                .setJobId("j").setTopic("t")
                .setBudget(Budget.newBuilder().setMaxInputTokens(-1).build())
                .build();
        assertThrows(Errors.ValidationException.class,
                () -> Validate.validateJobRequest(req));
    }

    @Test
    void validJobResult() {
        JobResult res = JobResult.newBuilder()
                .setJobId("j-1")
                .setWorkerId("w-1")
                .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                .build();
        assertDoesNotThrow(() -> Validate.validateJobResult(res));
    }

    @Test
    void jobResultUnspecifiedStatusRejects() {
        JobResult res = JobResult.newBuilder()
                .setJobId("j").setWorkerId("w").build();
        assertThrows(Errors.ValidationException.class,
                () -> Validate.validateJobResult(res));
    }

    @Test
    void jobResultEmptyWorkerRejects() {
        JobResult res = JobResult.newBuilder()
                .setJobId("j")
                .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                .build();
        Errors.ValidationException ex = assertThrows(Errors.ValidationException.class,
                () -> Validate.validateJobResult(res));
        assertEquals("worker_id", ex.field());
    }

    @Test
    void validBusPacket() {
        BusPacket pkt = BusPacket.newBuilder()
                .setTraceId("t").setSenderId("s").setProtocolVersion(1)
                .setCreatedAt(Timestamp.newBuilder().setSeconds(1).build())
                .setJobRequest(JobRequest.newBuilder()
                        .setJobId("j").setTopic("t").build())
                .build();
        assertDoesNotThrow(() -> Validate.validateBusPacket(pkt));
    }

    @Test
    void busPacketEmptySenderRejects() {
        BusPacket pkt = BusPacket.newBuilder()
                .setTraceId("t").setProtocolVersion(1)
                .setCreatedAt(Timestamp.newBuilder().setSeconds(1).build())
                .setJobRequest(JobRequest.newBuilder()
                        .setJobId("j").setTopic("t").build())
                .build();
        Errors.ValidationException ex = assertThrows(Errors.ValidationException.class,
                () -> Validate.validateBusPacket(pkt));
        assertEquals("sender_id", ex.field());
    }

    @Test
    void busPacketNoPayloadRejects() {
        BusPacket pkt = BusPacket.newBuilder()
                .setTraceId("t").setSenderId("s").setProtocolVersion(1)
                .setCreatedAt(Timestamp.newBuilder().setSeconds(1).build())
                .build();
        Errors.ValidationException ex = assertThrows(Errors.ValidationException.class,
                () -> Validate.validateBusPacket(pkt));
        assertEquals("payload", ex.field());
    }
}
