package io.cordum.cap;

import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;
import io.cordum.cap.agent.v1.JobPriority;
import io.cordum.cap.agent.v1.Budget;

public final class Validate {

    private Validate() {}

    public static void validateJobRequest(JobRequest req) {
        if (req == null) {
            throw new Errors.ValidationException("job_request", "must not be null");
        }
        if (req.getJobId().isEmpty()) {
            throw new Errors.ValidationException("job_id", "must not be empty");
        }
        if (req.getTopic().isEmpty()) {
            throw new Errors.ValidationException("topic", "must not be empty");
        }
        if (req.getPriorityValue() < 0
                || req.getPriorityValue() > JobPriority.JOB_PRIORITY_CRITICAL_VALUE) {
            throw new Errors.ValidationException("priority", "out of valid range");
        }
        if (req.getStepIndex() < 0) {
            throw new Errors.ValidationException("step_index", "must be >= 0");
        }
        if (req.hasBudget()) {
            Budget b = req.getBudget();
            if (b.getMaxInputTokens() < 0) {
                throw new Errors.ValidationException("budget.max_input_tokens", "must be >= 0");
            }
            if (b.getMaxOutputTokens() < 0) {
                throw new Errors.ValidationException("budget.max_output_tokens", "must be >= 0");
            }
            if (b.getMaxTotalTokens() < 0) {
                throw new Errors.ValidationException("budget.max_total_tokens", "must be >= 0");
            }
            if (b.getDeadlineMs() < 0) {
                throw new Errors.ValidationException("budget.deadline_ms", "must be >= 0");
            }
        }
    }

    public static void validateJobResult(JobResult res) {
        if (res == null) {
            throw new Errors.ValidationException("job_result", "must not be null");
        }
        if (res.getJobId().isEmpty()) {
            throw new Errors.ValidationException("job_id", "must not be empty");
        }
        if (res.getStatus() == JobStatus.JOB_STATUS_UNSPECIFIED) {
            throw new Errors.ValidationException("status", "must not be UNSPECIFIED");
        }
        if (res.getWorkerId().isEmpty()) {
            throw new Errors.ValidationException("worker_id", "must not be empty");
        }
        if (res.getExecutionMs() < 0) {
            throw new Errors.ValidationException("execution_ms", "must be >= 0");
        }
    }

    public static void validateBusPacket(BusPacket pkt) {
        if (pkt == null) {
            throw new Errors.ValidationException("bus_packet", "must not be null");
        }
        if (pkt.getTraceId().isEmpty()) {
            throw new Errors.ValidationException("trace_id", "must not be empty");
        }
        if (pkt.getSenderId().isEmpty()) {
            throw new Errors.ValidationException("sender_id", "must not be empty");
        }
        if (pkt.getProtocolVersion() <= 0) {
            throw new Errors.ValidationException("protocol_version", "must be > 0");
        }
        if (!pkt.hasCreatedAt()) {
            throw new Errors.ValidationException("created_at", "must not be null");
        }
        if (pkt.getPayloadCase() == BusPacket.PayloadCase.PAYLOAD_NOT_SET) {
            throw new Errors.ValidationException("payload", "must not be empty");
        }
        if (pkt.getPayloadCase() == BusPacket.PayloadCase.JOB_REQUEST) {
            validateJobRequest(pkt.getJobRequest());
        }
        if (pkt.getPayloadCase() == BusPacket.PayloadCase.JOB_RESULT) {
            validateJobResult(pkt.getJobResult());
        }
    }
}
