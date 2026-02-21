<?php

declare(strict_types=1);

namespace Cordum\Cap;

/**
 * Pluggable observability callbacks for CAP protocol events.
 */
interface MetricsHook
{
    public function onJobReceived(string $jobId, string $topic): void;

    public function onJobCompleted(string $jobId, int $durationMs, string $status): void;

    public function onJobFailed(string $jobId, string $errorMsg): void;

    public function onHeartbeatSent(string $workerId): void;

    public function onProgressEmitted(string $jobId, int $percent): void;

    public function onError(string $context, string $errorMsg): void;
}

/**
 * No-op metrics implementation. All callbacks are empty.
 */
class NoopMetrics implements MetricsHook
{
    public function onJobReceived(string $jobId, string $topic): void {}
    public function onJobCompleted(string $jobId, int $durationMs, string $status): void {}
    public function onJobFailed(string $jobId, string $errorMsg): void {}
    public function onHeartbeatSent(string $workerId): void {}
    public function onProgressEmitted(string $jobId, int $percent): void {}
    public function onError(string $context, string $errorMsg): void {}
}
