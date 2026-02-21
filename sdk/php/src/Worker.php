<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Basis\Nats\Client as NatsClient;
use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\JobResult;
use Cordum\Agent\V1\JobStatus;
use Google\Protobuf\Timestamp;
use OpenSSLAsymmetricKey;

/**
 * CAP worker that processes jobs from a NATS subject.
 */
final class Worker
{
    /** @var callable[] */
    private array $middlewares = [];

    /**
     * @param NatsClient $nc NATS connection
     * @param string $subject Subject to subscribe to
     * @param string $senderId Worker identifier
     * @param callable $handler Base handler: fn(Context, JobRequest): JobResult
     * @param OpenSSLAsymmetricKey|null $privateKey For signing outgoing packets
     * @param array<string, OpenSSLAsymmetricKey> $publicKeys For verifying incoming packets
     * @param MetricsHook|null $metrics Metrics callback hook
     */
    public function __construct(
        private readonly NatsClient $nc,
        private readonly string $subject,
        private readonly string $senderId,
        private readonly mixed $handler,
        private readonly ?OpenSSLAsymmetricKey $privateKey = null,
        private readonly array $publicKeys = [],
        private readonly MetricsHook $metrics = new NoopMetrics(),
    ) {}

    /**
     * Add middleware(s) to the processing chain.
     */
    public function use(callable ...$middlewares): void
    {
        array_push($this->middlewares, ...$middlewares);
    }

    /**
     * Start processing messages. Blocks until interrupted.
     */
    public function start(): void
    {
        $handler = MiddlewareChain::apply($this->handler, $this->middlewares);

        $this->nc->subscribe($this->subject, function (string $payload) use ($handler): void {
            $packet = new BusPacket();
            try {
                $packet->mergeFromString($payload);
            } catch (\Throwable $e) {
                $this->metrics->onError('parse', "failed: {$e->getMessage()}");
                return;
            }

            // Verify signature if public keys are configured
            if (!empty($this->publicKeys)) {
                $senderId = $packet->getSenderId();
                if (!isset($this->publicKeys[$senderId])) {
                    return;
                }
                if ($packet->getSignature() === '' || !Signing::verifyPacketSignature($packet, $this->publicKeys[$senderId])) {
                    return;
                }
            }

            if ($packet->getPayload() !== 'job_request') {
                return;
            }

            $req = $packet->getJobRequest();
            $this->metrics->onJobReceived($req->getJobId(), $req->getTopic());

            $ctx = new Context(
                traceId: $packet->getTraceId(),
                senderId: $packet->getSenderId(),
                packet: $packet,
            );

            $start = hrtime(true);
            try {
                /** @var JobResult $result */
                $result = $handler($ctx, $req);

                if ($result->getJobId() === '') {
                    $result->setJobId($req->getJobId());
                }
                if ($result->getWorkerId() === '') {
                    $result->setWorkerId($this->senderId);
                }

                $elapsed = (int) ((hrtime(true) - $start) / 1_000_000);

                if ($result->getStatus() === JobStatus::JOB_STATUS_FAILED) {
                    $this->metrics->onJobFailed($result->getJobId(), $result->getErrorMessage());
                } else {
                    $this->metrics->onJobCompleted($result->getJobId(), $elapsed, 'SUCCEEDED');
                }
            } catch (\Throwable $e) {
                $this->metrics->onJobFailed($req->getJobId(), $e->getMessage());
                $result = new JobResult();
                $result->setJobId($req->getJobId());
                $result->setWorkerId($this->senderId);
                $result->setStatus(JobStatus::JOB_STATUS_FAILED);
                $result->setErrorMessage($e->getMessage());
            }

            $now = microtime(true);
            $timestamp = new Timestamp();
            $timestamp->setSeconds((int) $now);
            $timestamp->setNanos((int) (($now - (int) $now) * 1_000_000_000));

            $outPacket = new BusPacket();
            $outPacket->setTraceId($packet->getTraceId());
            $outPacket->setSenderId($this->senderId);
            $outPacket->setProtocolVersion(Subjects::DEFAULT_PROTOCOL_VERSION);
            $outPacket->setCreatedAt($timestamp);
            $outPacket->setJobResult($result);

            if ($this->privateKey !== null) {
                Signing::signPacket($outPacket, $this->privateKey);
            }

            $data = Codec::marshalDeterministic($outPacket);
            $this->nc->publish(Subjects::RESULT, $data);
        });

        // Process messages (blocking)
        $this->nc->process();
    }
}
