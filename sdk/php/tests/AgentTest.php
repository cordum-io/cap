<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;
use Cordum\Agent\V1\JobStatus;
use Cordum\Cap\CapException;
use Cordum\Cap\Context;
use Cordum\Cap\ErrorCode;
use Cordum\Cap\MockBus;
use Cordum\Cap\RecordingMetrics;
use Cordum\Cap\TestHelper;
use PHPUnit\Framework\TestCase;

class AgentTest extends TestCase
{
    #[\PHPUnit\Framework\Attributes\Test]
    public function mockBusTracksPublishes(): void
    {
        $bus = new MockBus();
        $bus->publish('sys.heartbeat', 'abc');
        $bus->publish('sys.job.result', 'def');

        $this->assertSame(2, $bus->publishedCount());
        $this->assertCount(1, $bus->publishedTo('sys.heartbeat'));
        $this->assertCount(1, $bus->publishedTo('sys.job.result'));
        $this->assertEmpty($bus->publishedTo('sys.job.submit'));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function mockBusClear(): void
    {
        $bus = new MockBus();
        $bus->publish('sys.heartbeat', 'x');
        $this->assertSame(1, $bus->publishedCount());

        $bus->clear();
        $this->assertSame(0, $bus->publishedCount());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function recordingMetricsTracksEvents(): void
    {
        $m = new RecordingMetrics();
        $m->onJobReceived('j-1', 'test');
        $m->onJobCompleted('j-1', 100, 'SUCCEEDED');
        $m->onHeartbeatSent('w-1');
        $m->onJobFailed('j-2', 'timeout');
        $m->onError('parse', 'invalid proto');

        $this->assertCount(1, $m->receivedJobs);
        $this->assertCount(1, $m->completedJobs);
        $this->assertCount(1, $m->heartbeats);
        $this->assertCount(1, $m->failedJobs);
        $this->assertCount(1, $m->errors);
        $this->assertSame('parse: invalid proto', $m->errors[0]);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function recordingMetricsMultipleEvents(): void
    {
        $m = new RecordingMetrics();
        $m->onJobReceived('j-1', 't');
        $m->onJobReceived('j-2', 't');
        $m->onJobReceived('j-3', 't');

        $this->assertCount(3, $m->receivedJobs);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function submitAndWaitErrorHandlerThrows(): void
    {
        $handler = function (Context $ctx, JobRequest $req): JobResult {
            throw CapException::jobTimeout('timed out');
        };

        $this->expectException(CapException::class);
        $req = new JobRequest();
        $req->setJobId('j');
        $req->setTopic('t');
        TestHelper::submitAndWait($handler, $req);
    }
}
