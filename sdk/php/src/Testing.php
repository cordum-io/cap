<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;

/**
 * Tracks published messages for test assertions.
 */
class MockBus
{
    /** @var array<array{subject: string, data: string}> */
    private array $published = [];

    public function publish(string $subject, string $data): void
    {
        $this->published[] = ['subject' => $subject, 'data' => $data];
    }

    public function publishedCount(): int
    {
        return count($this->published);
    }

    /**
     * @return string[]
     */
    public function publishedTo(string $subject): array
    {
        return array_values(array_map(
            fn (array $p) => $p['data'],
            array_filter($this->published, fn (array $p) => $p['subject'] === $subject),
        ));
    }

    public function clear(): void
    {
        $this->published = [];
    }
}

/**
 * Submit a job request to a handler and return the result.
 * Useful for quick unit tests without NATS infrastructure.
 */
final class TestHelper
{
    public static function submitAndWait(callable $handler, JobRequest $req): JobResult
    {
        $ctx = new Context(
            traceId: 'test-trace',
            senderId: 'test-sender',
        );
        return $handler($ctx, $req);
    }
}

/**
 * Recording implementation of MetricsHook for test assertions.
 */
class RecordingMetrics implements MetricsHook
{
    /** @var string[] */
    public array $receivedJobs = [];
    /** @var string[] */
    public array $completedJobs = [];
    /** @var string[] */
    public array $failedJobs = [];
    /** @var string[] */
    public array $heartbeats = [];
    /** @var string[] */
    public array $progressEvents = [];
    /** @var string[] */
    public array $errors = [];

    public function onJobReceived(string $jobId, string $topic): void
    {
        $this->receivedJobs[] = $jobId;
    }

    public function onJobCompleted(string $jobId, int $durationMs, string $status): void
    {
        $this->completedJobs[] = $jobId;
    }

    public function onJobFailed(string $jobId, string $errorMsg): void
    {
        $this->failedJobs[] = $jobId;
    }

    public function onHeartbeatSent(string $workerId): void
    {
        $this->heartbeats[] = $workerId;
    }

    public function onProgressEmitted(string $jobId, int $percent): void
    {
        $this->progressEvents[] = $jobId;
    }

    public function onError(string $context, string $errorMsg): void
    {
        $this->errors[] = "{$context}: {$errorMsg}";
    }
}
