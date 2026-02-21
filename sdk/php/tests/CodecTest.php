<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\Heartbeat;
use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;
use Cordum\Agent\V1\JobStatus;
use Cordum\Cap\Codec;
use Google\Protobuf\Timestamp;
use PHPUnit\Framework\TestCase;

class CodecTest extends TestCase
{
    private function makeJobRequestPacket(): BusPacket
    {
        $req = new JobRequest();
        $req->setJobId('job-codec-1');
        $req->setTopic('codec.test');

        $ts = new Timestamp();
        $ts->setSeconds(1704164645);
        $ts->setNanos(0);

        $pkt = new BusPacket();
        $pkt->setTraceId('trace-codec');
        $pkt->setSenderId('sender-codec');
        $pkt->setProtocolVersion(1);
        $pkt->setCreatedAt($ts);
        $pkt->setJobRequest($req);

        return $pkt;
    }

    private function makeHeartbeatPacket(): BusPacket
    {
        $hb = new Heartbeat();
        $hb->setWorkerId('worker-hb');
        $hb->setPool('default');
        $hb->setActiveJobs(2);
        $hb->setMaxParallelJobs(8);
        $hb->setCpuLoad(45.5);

        $ts = new Timestamp();
        $ts->setSeconds(1704164645);

        $pkt = new BusPacket();
        $pkt->setTraceId('trace-hb');
        $pkt->setSenderId('worker-hb');
        $pkt->setProtocolVersion(1);
        $pkt->setCreatedAt($ts);
        $pkt->setHeartbeat($hb);

        return $pkt;
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function deterministic100Iterations(): void
    {
        $packet = $this->makeJobRequestPacket();
        $first = Codec::marshalDeterministic($packet);
        $this->assertNotEmpty($first);

        for ($i = 0; $i < 100; $i++) {
            $this->assertSame($first, Codec::marshalDeterministic($packet), "iteration {$i} differs");
        }
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function deterministicHeartbeat100Iterations(): void
    {
        $packet = $this->makeHeartbeatPacket();
        $first = Codec::marshalDeterministic($packet);

        for ($i = 0; $i < 100; $i++) {
            $this->assertSame($first, Codec::marshalDeterministic($packet));
        }
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function roundTripJobRequest(): void
    {
        $packet = $this->makeJobRequestPacket();
        $bytes = Codec::marshalDeterministic($packet);
        $decoded = new BusPacket();
        $decoded->mergeFromString($bytes);

        $this->assertSame('trace-codec', $decoded->getTraceId());
        $this->assertSame('sender-codec', $decoded->getSenderId());
        $this->assertSame(1, $decoded->getProtocolVersion());
        $this->assertSame('job-codec-1', $decoded->getJobRequest()->getJobId());
        $this->assertSame('codec.test', $decoded->getJobRequest()->getTopic());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function roundTripHeartbeat(): void
    {
        $packet = $this->makeHeartbeatPacket();
        $bytes = Codec::marshalDeterministic($packet);
        $decoded = new BusPacket();
        $decoded->mergeFromString($bytes);

        $hb = $decoded->getHeartbeat();
        $this->assertSame('worker-hb', $hb->getWorkerId());
        $this->assertSame('default', $hb->getPool());
        $this->assertSame(2, $hb->getActiveJobs());
        $this->assertSame(8, $hb->getMaxParallelJobs());
        $this->assertEqualsWithDelta(45.5, $hb->getCpuLoad(), 0.01);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function unsignedForSignatureStripsSignature(): void
    {
        $packet = $this->makeJobRequestPacket();
        $packet->setSignature('fake-sig');

        $unsigned = Codec::marshalUnsignedForSignature($packet);
        $decoded = new BusPacket();
        $decoded->mergeFromString($unsigned);

        $this->assertEmpty($decoded->getSignature());
        $this->assertSame('trace-codec', $decoded->getTraceId());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function unsignedMatchesCleanPacket(): void
    {
        $packet = $this->makeJobRequestPacket();
        $packet->setSignature('to-strip');

        $unsigned = Codec::marshalUnsignedForSignature($packet);

        $clean = clone $packet;
        $clean->setSignature('');
        $cleanBytes = Codec::marshalDeterministic($clean);

        $this->assertSame($cleanBytes, $unsigned);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function differentPacketsProduceDifferentBytes(): void
    {
        $bytes1 = Codec::marshalDeterministic($this->makeJobRequestPacket());
        $bytes2 = Codec::marshalDeterministic($this->makeHeartbeatPacket());
        $this->assertNotSame($bytes1, $bytes2);
    }
}
