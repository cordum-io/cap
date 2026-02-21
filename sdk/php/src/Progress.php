<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Basis\Nats\Client as NatsClient;
use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\JobCancel;
use Cordum\Agent\V1\JobProgress;
use Google\Protobuf\Timestamp;

/**
 * Progress and cancel payload constructors and emission helpers.
 */
final class ProgressHelper
{
    /**
     * Build a job progress payload.
     */
    public static function progressPayload(
        string $senderId,
        string $jobId,
        string $stepId,
        int $percent,
        string $message,
    ): string {
        $progress = new JobProgress();
        $progress->setJobId($jobId);
        $progress->setStepId($stepId);
        $progress->setPercent($percent);
        $progress->setMessage($message);

        $now = microtime(true);
        $timestamp = new Timestamp();
        $timestamp->setSeconds((int) $now);
        $timestamp->setNanos((int) (($now - (int) $now) * 1_000_000_000));

        $packet = new BusPacket();
        $packet->setSenderId($senderId);
        $packet->setProtocolVersion(Subjects::DEFAULT_PROTOCOL_VERSION);
        $packet->setCreatedAt($timestamp);
        $packet->setJobProgress($progress);

        return Codec::marshalDeterministic($packet);
    }

    /**
     * Build a job cancel payload.
     */
    public static function cancelPayload(
        string $senderId,
        string $jobId,
        string $reason,
        string $requestedBy,
    ): string {
        $cancel = new JobCancel();
        $cancel->setJobId($jobId);
        $cancel->setReason($reason);
        $cancel->setRequestedBy($requestedBy);

        $now = microtime(true);
        $timestamp = new Timestamp();
        $timestamp->setSeconds((int) $now);
        $timestamp->setNanos((int) (($now - (int) $now) * 1_000_000_000));

        $packet = new BusPacket();
        $packet->setSenderId($senderId);
        $packet->setProtocolVersion(Subjects::DEFAULT_PROTOCOL_VERSION);
        $packet->setCreatedAt($timestamp);
        $packet->setJobCancel($cancel);

        return Codec::marshalDeterministic($packet);
    }

    /**
     * Publish a progress payload to the progress subject.
     */
    public static function emitProgress(NatsClient $nc, string $payload): void
    {
        $nc->publish(Subjects::PROGRESS, $payload);
    }

    /**
     * Publish a cancel payload to the cancel subject.
     */
    public static function emitCancel(NatsClient $nc, string $payload): void
    {
        $nc->publish(Subjects::CANCEL, $payload);
    }
}
