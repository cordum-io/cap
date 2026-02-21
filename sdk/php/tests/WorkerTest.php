<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;
use Cordum\Agent\V1\JobStatus;
use Cordum\Cap\Context;
use Cordum\Cap\MiddlewareChain;
use Cordum\Cap\TestHelper;
use PHPUnit\Framework\TestCase;

class WorkerTest extends TestCase
{
    #[\PHPUnit\Framework\Attributes\Test]
    public function submitAndWaitReturnsResult(): void
    {
        $handler = function (Context $ctx, JobRequest $req): JobResult {
            $result = new JobResult();
            $result->setJobId($req->getJobId());
            $result->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
            $result->setWorkerId('test-worker');
            return $result;
        };

        $req = new JobRequest();
        $req->setJobId('j-1');
        $req->setTopic('test');

        $result = TestHelper::submitAndWait($handler, $req);
        $this->assertSame('j-1', $result->getJobId());
        $this->assertSame(JobStatus::JOB_STATUS_SUCCEEDED, $result->getStatus());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function submitAndWaitContextPopulated(): void
    {
        $captured = null;
        $handler = function (Context $ctx, JobRequest $req) use (&$captured): JobResult {
            $captured = $ctx;
            $result = new JobResult();
            $result->setJobId($req->getJobId());
            $result->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
            $result->setWorkerId('w');
            return $result;
        };

        $req = new JobRequest();
        $req->setJobId('j');
        $req->setTopic('t');

        TestHelper::submitAndWait($handler, $req);

        $this->assertSame('test-trace', $captured->traceId);
        $this->assertSame('test-sender', $captured->senderId);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function middlewareFIFOOrder(): void
    {
        $order = [];

        $makeMw = function (int $id) use (&$order): callable {
            return function (callable $next) use ($id, &$order): callable {
                return function (Context $ctx, JobRequest $req) use ($next, $id, &$order): JobResult {
                    $order[] = $id;
                    return $next($ctx, $req);
                };
            };
        };

        $baseHandler = function (Context $ctx, JobRequest $req): JobResult {
            $result = new JobResult();
            $result->setJobId($req->getJobId());
            $result->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
            $result->setWorkerId('w');
            return $result;
        };

        $wrapped = MiddlewareChain::apply($baseHandler, [$makeMw(1), $makeMw(2), $makeMw(3)]);

        $ctx = new Context(traceId: 't', senderId: 's');
        $req = new JobRequest();
        $req->setJobId('j');
        $req->setTopic('t');
        $wrapped($ctx, $req);

        $this->assertSame([1, 2, 3], $order);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function emptyMiddlewareReturnsBase(): void
    {
        $baseHandler = function (Context $ctx, JobRequest $req): JobResult {
            $result = new JobResult();
            $result->setJobId($req->getJobId());
            $result->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
            $result->setWorkerId('w');
            return $result;
        };

        $wrapped = MiddlewareChain::apply($baseHandler, []);
        $ctx = new Context(traceId: 't', senderId: 's');
        $req = new JobRequest();
        $req->setJobId('j-1');
        $req->setTopic('t');

        $result = $wrapped($ctx, $req);
        $this->assertSame('j-1', $result->getJobId());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function loggingMiddlewareDoesNotInterfere(): void
    {
        $baseHandler = function (Context $ctx, JobRequest $req): JobResult {
            $result = new JobResult();
            $result->setJobId($req->getJobId());
            $result->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
            $result->setWorkerId('w');
            return $result;
        };

        $wrapped = MiddlewareChain::apply($baseHandler, [MiddlewareChain::logging()]);
        $ctx = new Context(traceId: 't', senderId: 's');
        $req = new JobRequest();
        $req->setJobId('logged-job');
        $req->setTopic('t');

        $result = $wrapped($ctx, $req);
        $this->assertSame('logged-job', $result->getJobId());
    }
}
