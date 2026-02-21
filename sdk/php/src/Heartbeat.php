<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Basis\Nats\Client as NatsClient;
use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\Heartbeat as HeartbeatMessage;
use Google\Protobuf\Timestamp;

/**
 * Heartbeat payload constructors and emission helpers.
 */
final class HeartbeatHelper
{
    /**
     * Build a heartbeat payload with basic fields.
     */
    public static function heartbeatPayload(
        string $workerId,
        string $pool,
        int $activeJobs,
        int $maxParallel,
        float $cpuLoad,
    ): string {
        return self::heartbeatPayloadWithProgress($workerId, $pool, $activeJobs, $maxParallel, $cpuLoad, 0.0, 0, '');
    }

    /**
     * Build a heartbeat payload with memory load.
     */
    public static function heartbeatPayloadWithMemory(
        string $workerId,
        string $pool,
        int $activeJobs,
        int $maxParallel,
        float $cpuLoad,
        float $memoryLoad,
    ): string {
        return self::heartbeatPayloadWithProgress($workerId, $pool, $activeJobs, $maxParallel, $cpuLoad, $memoryLoad, 0, '');
    }

    /**
     * Build a heartbeat payload with all fields including progress and memo.
     */
    public static function heartbeatPayloadWithProgress(
        string $workerId,
        string $pool,
        int $activeJobs,
        int $maxParallel,
        float $cpuLoad,
        float $memoryLoad,
        int $progressPct,
        string $lastMemo,
    ): string {
        $hb = new HeartbeatMessage();
        $hb->setWorkerId($workerId);
        $hb->setPool($pool);
        $hb->setActiveJobs($activeJobs);
        $hb->setMaxParallelJobs($maxParallel);
        $hb->setCpuLoad($cpuLoad);
        $hb->setMemoryLoad($memoryLoad);
        $hb->setProgressPct($progressPct);
        $hb->setLastMemo($lastMemo);

        $now = microtime(true);
        $timestamp = new Timestamp();
        $timestamp->setSeconds((int) $now);
        $timestamp->setNanos((int) (($now - (int) $now) * 1_000_000_000));

        $packet = new BusPacket();
        $packet->setSenderId($workerId);
        $packet->setProtocolVersion(Subjects::DEFAULT_PROTOCOL_VERSION);
        $packet->setCreatedAt($timestamp);
        $packet->setHeartbeat($hb);

        return Codec::marshalDeterministic($packet);
    }

    /**
     * Publish a heartbeat payload to the heartbeat subject.
     */
    public static function emitHeartbeat(NatsClient $nc, string $payload): void
    {
        $nc->publish(Subjects::HEARTBEAT, $payload);
    }
}

/**
 * Periodically emits heartbeats using a timer loop.
 * Uses sleep-based loop since PHP is single-threaded.
 */
class HeartbeatLoop
{
    private bool $running = false;

    /**
     * @param NatsClient $nc NATS connection
     * @param callable $payloadFn Function returning heartbeat payload bytes
     * @param string $workerId Worker ID for metrics
     * @param int $intervalSeconds Heartbeat interval in seconds
     * @param MetricsHook $metrics Metrics hook
     */
    public function __construct(
        private readonly NatsClient $nc,
        private readonly mixed $payloadFn,
        private readonly string $workerId,
        private readonly int $intervalSeconds = Subjects::DEFAULT_HEARTBEAT_INTERVAL,
        private readonly MetricsHook $metrics = new NoopMetrics(),
    ) {}

    /**
     * Run the heartbeat loop. Blocks until stop() is called.
     * Best used with pcntl_signal for clean shutdown.
     */
    public function start(): void
    {
        $this->running = true;
        while ($this->running) {
            try {
                $payload = ($this->payloadFn)();
                HeartbeatHelper::emitHeartbeat($this->nc, $payload);
                $this->metrics->onHeartbeatSent($this->workerId);
            } catch (\Throwable) {
                // Swallow individual emit failures
            }
            sleep($this->intervalSeconds);
        }
    }

    /**
     * Emit a single heartbeat tick. Useful for integration with external event loops.
     */
    public function tick(): void
    {
        try {
            $payload = ($this->payloadFn)();
            HeartbeatHelper::emitHeartbeat($this->nc, $payload);
            $this->metrics->onHeartbeatSent($this->workerId);
        } catch (\Throwable) {
            // Swallow
        }
    }

    /**
     * Stop the heartbeat loop.
     */
    public function stop(): void
    {
        $this->running = false;
    }
}
