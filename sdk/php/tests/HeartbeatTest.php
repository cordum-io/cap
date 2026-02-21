<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\BusPacket;
use Cordum\Cap\HeartbeatHelper;
use PHPUnit\Framework\TestCase;

class HeartbeatTest extends TestCase
{
    #[\PHPUnit\Framework\Attributes\Test]
    public function heartbeatPayloadValid(): void
    {
        $bytes = HeartbeatHelper::heartbeatPayload('w-1', 'pool-a', 3, 8, 42.5);
        $this->assertNotEmpty($bytes);

        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertSame('w-1', $packet->getSenderId());
        $this->assertSame(1, $packet->getProtocolVersion());

        $hb = $packet->getHeartbeat();
        $this->assertNotNull($hb);
        $this->assertSame('w-1', $hb->getWorkerId());
        $this->assertSame('pool-a', $hb->getPool());
        $this->assertSame(3, $hb->getActiveJobs());
        $this->assertSame(8, $hb->getMaxParallelJobs());
        $this->assertEqualsWithDelta(42.5, $hb->getCpuLoad(), 0.01);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function heartbeatPayloadWithMemorySetsMemoryLoad(): void
    {
        $bytes = HeartbeatHelper::heartbeatPayloadWithMemory('w-1', 'p', 1, 4, 10.0, 55.0);
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);

        $hb = $packet->getHeartbeat();
        $this->assertEqualsWithDelta(55.0, $hb->getMemoryLoad(), 0.01);
        $this->assertSame(0, $hb->getProgressPct());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function heartbeatPayloadWithProgressSetsAllFields(): void
    {
        $bytes = HeartbeatHelper::heartbeatPayloadWithProgress('w-2', 'gpu', 5, 10, 80.0, 60.0, 75, 'step-3');
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);

        $hb = $packet->getHeartbeat();
        $this->assertSame('w-2', $hb->getWorkerId());
        $this->assertSame('gpu', $hb->getPool());
        $this->assertSame(5, $hb->getActiveJobs());
        $this->assertSame(75, $hb->getProgressPct());
        $this->assertSame('step-3', $hb->getLastMemo());
        $this->assertEqualsWithDelta(80.0, $hb->getCpuLoad(), 0.01);
        $this->assertEqualsWithDelta(60.0, $hb->getMemoryLoad(), 0.01);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function heartbeatPayloadHasTimestamp(): void
    {
        $bytes = HeartbeatHelper::heartbeatPayload('w', 'p', 0, 1, 0.0);
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertNotNull($packet->getCreatedAt());
        $this->assertGreaterThan(0, $packet->getCreatedAt()->getSeconds());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function heartbeatPayloadZeroActiveJobs(): void
    {
        $bytes = HeartbeatHelper::heartbeatPayload('w', 'p', 0, 4, 0.0);
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertSame(0, $packet->getHeartbeat()->getActiveJobs());
    }
}
