<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;
use Cordum\Agent\V1\JobStatus;

/**
 * Input validation matching the CAP guard SDK rules.
 */
final class Validate
{
    /**
     * Validate a JobRequest. Throws ValidationException on invalid input.
     */
    public static function validateJobRequest(JobRequest $req): void
    {
        if ($req->getJobId() === '') {
            throw new ValidationException('job_id', 'must not be empty');
        }
        if ($req->getTopic() === '') {
            throw new ValidationException('topic', 'must not be empty');
        }
        if ($req->getStepIndex() < 0) {
            throw new ValidationException('step_index', 'must be >= 0');
        }
        $budget = $req->getBudget();
        if ($budget !== null) {
            if ($budget->getMaxInputTokens() < 0) {
                throw new ValidationException('budget.max_input_tokens', 'must be >= 0');
            }
            if ($budget->getMaxOutputTokens() < 0) {
                throw new ValidationException('budget.max_output_tokens', 'must be >= 0');
            }
            if ($budget->getMaxTotalTokens() < 0) {
                throw new ValidationException('budget.max_total_tokens', 'must be >= 0');
            }
            if ($budget->getDeadlineMs() < 0) {
                throw new ValidationException('budget.deadline_ms', 'must be >= 0');
            }
        }
    }

    /**
     * Validate a JobResult. Throws ValidationException on invalid input.
     */
    public static function validateJobResult(JobResult $res): void
    {
        if ($res->getJobId() === '') {
            throw new ValidationException('job_id', 'must not be empty');
        }
        if ($res->getStatus() === JobStatus::JOB_STATUS_UNSPECIFIED) {
            throw new ValidationException('status', 'must not be UNSPECIFIED');
        }
        if ($res->getWorkerId() === '') {
            throw new ValidationException('worker_id', 'must not be empty');
        }
        if ($res->getExecutionMs() < 0) {
            throw new ValidationException('execution_ms', 'must be >= 0');
        }
    }

    /**
     * Validate a BusPacket envelope. Throws ValidationException on invalid input.
     */
    public static function validateBusPacket(BusPacket $pkt): void
    {
        if ($pkt->getTraceId() === '') {
            throw new ValidationException('trace_id', 'must not be empty');
        }
        if ($pkt->getSenderId() === '') {
            throw new ValidationException('sender_id', 'must not be empty');
        }
        if ($pkt->getProtocolVersion() <= 0) {
            throw new ValidationException('protocol_version', 'must be > 0');
        }
        if ($pkt->getCreatedAt() === null) {
            throw new ValidationException('created_at', 'must not be null');
        }
        if ($pkt->getPayload() === '') {
            throw new ValidationException('payload', 'must not be empty');
        }

        switch ($pkt->getPayload()) {
            case 'job_request':
                self::validateJobRequest($pkt->getJobRequest());
                break;
            case 'job_result':
                self::validateJobResult($pkt->getJobResult());
                break;
        }
    }
}
