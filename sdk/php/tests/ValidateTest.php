<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;
use Cordum\Agent\V1\JobStatus;
use Cordum\Agent\V1\Budget;
use Cordum\Cap\Validate;
use Cordum\Cap\ValidationException;
use Google\Protobuf\Timestamp;
use PHPUnit\Framework\TestCase;

class ValidateTest extends TestCase
{
    #[\PHPUnit\Framework\Attributes\Test]
    public function validJobRequestPasses(): void
    {
        $req = new JobRequest();
        $req->setJobId('j-1');
        $req->setTopic('test');
        Validate::validateJobRequest($req);
        $this->assertTrue(true); // No exception
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function emptyJobIdRejected(): void
    {
        $req = new JobRequest();
        $req->setTopic('test');
        $this->expectException(ValidationException::class);
        Validate::validateJobRequest($req);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function emptyTopicRejected(): void
    {
        $req = new JobRequest();
        $req->setJobId('j-1');
        $this->expectException(ValidationException::class);
        Validate::validateJobRequest($req);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function validJobResultPasses(): void
    {
        $res = new JobResult();
        $res->setJobId('j-1');
        $res->setWorkerId('w-1');
        $res->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
        Validate::validateJobResult($res);
        $this->assertTrue(true);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function unspecifiedStatusRejected(): void
    {
        $res = new JobResult();
        $res->setJobId('j-1');
        $res->setWorkerId('w-1');
        $this->expectException(ValidationException::class);
        Validate::validateJobResult($res);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function emptyWorkerIdRejected(): void
    {
        $res = new JobResult();
        $res->setJobId('j-1');
        $res->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
        $this->expectException(ValidationException::class);
        Validate::validateJobResult($res);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function validBusPacketPasses(): void
    {
        $req = new JobRequest();
        $req->setJobId('j');
        $req->setTopic('t');

        $ts = new Timestamp();
        $ts->setSeconds(1);

        $pkt = new BusPacket();
        $pkt->setTraceId('t');
        $pkt->setSenderId('s');
        $pkt->setProtocolVersion(1);
        $pkt->setCreatedAt($ts);
        $pkt->setJobRequest($req);

        Validate::validateBusPacket($pkt);
        $this->assertTrue(true);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function emptySenderIdRejected(): void
    {
        $pkt = new BusPacket();
        $pkt->setTraceId('t');
        $pkt->setProtocolVersion(1);
        $this->expectException(ValidationException::class);
        Validate::validateBusPacket($pkt);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function emptyTraceIdRejected(): void
    {
        $pkt = new BusPacket();
        $pkt->setSenderId('s');
        $pkt->setProtocolVersion(1);
        $this->expectException(ValidationException::class);
        Validate::validateBusPacket($pkt);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function zeroProtocolVersionRejected(): void
    {
        $pkt = new BusPacket();
        $pkt->setTraceId('t');
        $pkt->setSenderId('s');
        $ts = new Timestamp();
        $ts->setSeconds(1);
        $pkt->setCreatedAt($ts);
        $this->expectException(ValidationException::class);
        Validate::validateBusPacket($pkt);
    }
}
