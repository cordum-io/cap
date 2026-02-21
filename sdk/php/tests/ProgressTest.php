<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\BusPacket;
use Cordum\Cap\ProgressHelper;
use PHPUnit\Framework\TestCase;

class ProgressTest extends TestCase
{
    #[\PHPUnit\Framework\Attributes\Test]
    public function progressPayloadValid(): void
    {
        $bytes = ProgressHelper::progressPayload('s-1', 'j-42', 'step-3', 75, 'processing');
        $this->assertNotEmpty($bytes);

        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertSame('s-1', $packet->getSenderId());

        $p = $packet->getJobProgress();
        $this->assertNotNull($p);
        $this->assertSame('j-42', $p->getJobId());
        $this->assertSame('step-3', $p->getStepId());
        $this->assertSame(75, $p->getPercent());
        $this->assertSame('processing', $p->getMessage());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function cancelPayloadValid(): void
    {
        $bytes = ProgressHelper::cancelPayload('s-1', 'j-99', 'timeout', 'admin');
        $this->assertNotEmpty($bytes);

        $packet = new BusPacket();
        $packet->mergeFromString($bytes);

        $c = $packet->getJobCancel();
        $this->assertNotNull($c);
        $this->assertSame('j-99', $c->getJobId());
        $this->assertSame('timeout', $c->getReason());
        $this->assertSame('admin', $c->getRequestedBy());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function progressPayloadZeroPercent(): void
    {
        $bytes = ProgressHelper::progressPayload('s', 'j', 'st', 0, '');
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertSame(0, $packet->getJobProgress()->getPercent());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function progressPayload100Percent(): void
    {
        $bytes = ProgressHelper::progressPayload('s', 'j', 'st', 100, 'done');
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertSame(100, $packet->getJobProgress()->getPercent());
        $this->assertSame('done', $packet->getJobProgress()->getMessage());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function progressPayloadHasTimestamp(): void
    {
        $bytes = ProgressHelper::progressPayload('s', 'j', 'st', 50, 'half');
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertNotNull($packet->getCreatedAt());
        $this->assertGreaterThan(0, $packet->getCreatedAt()->getSeconds());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function cancelPayloadHasTimestamp(): void
    {
        $bytes = ProgressHelper::cancelPayload('s', 'j', 'r', 'u');
        $packet = new BusPacket();
        $packet->mergeFromString($bytes);
        $this->assertNotNull($packet->getCreatedAt());
    }
}
