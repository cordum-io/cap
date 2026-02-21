<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Basis\Nats\Client as NatsClient;
use OpenSSLAsymmetricKey;

/**
 * High-level CAP agent runtime. Manages handler registration, job dispatch,
 * and heartbeat emission.
 */
final class Agent
{
    private ?NatsClient $nc = null;
    private string $natsHost;
    private int $natsPort;
    private string $senderId;
    private string $pool;
    private int $maxParallel;
    private ?OpenSSLAsymmetricKey $privateKey;
    /** @var array<string, OpenSSLAsymmetricKey> */
    private array $publicKeys;
    private MetricsHook $metrics;
    /** @var callable[] */
    private array $middlewares = [];
    private int $heartbeatInterval;
    /** @var array<string, callable> */
    private array $handlers = [];
    private ?HeartbeatLoop $heartbeatLoop = null;

    /**
     * @param array{
     *   natsHost?: string,
     *   natsPort?: int,
     *   senderId: string,
     *   pool?: string,
     *   maxParallel?: int,
     *   privateKey?: OpenSSLAsymmetricKey|null,
     *   publicKeys?: array<string, OpenSSLAsymmetricKey>,
     *   metrics?: MetricsHook|null,
     *   heartbeatInterval?: int,
     * } $options
     */
    public function __construct(array $options)
    {
        $this->natsHost = $options['natsHost'] ?? 'localhost';
        $this->natsPort = $options['natsPort'] ?? 4222;
        $this->senderId = $options['senderId'];
        $this->pool = $options['pool'] ?? '';
        $this->maxParallel = $options['maxParallel'] ?? 1;
        $this->privateKey = $options['privateKey'] ?? null;
        $this->publicKeys = $options['publicKeys'] ?? [];
        $this->metrics = $options['metrics'] ?? new NoopMetrics();
        $this->heartbeatInterval = $options['heartbeatInterval'] ?? Subjects::DEFAULT_HEARTBEAT_INTERVAL;
    }

    /**
     * Register a handler for a subject.
     */
    public function register(string $subject, callable $handler): void
    {
        $this->handlers[$subject] = $handler;
    }

    /**
     * Add middleware(s) to the processing chain.
     */
    public function use(callable ...$middlewares): void
    {
        array_push($this->middlewares, ...$middlewares);
    }

    /**
     * Start all workers and the heartbeat loop.
     * Connects to NATS, subscribes to all registered subjects, and begins processing.
     */
    public function start(): void
    {
        $this->nc = Bus::connectNats([
            'host' => $this->natsHost,
            'port' => $this->natsPort,
        ]);

        // Apply middleware to all handlers
        $wrappedHandlers = [];
        foreach ($this->handlers as $subject => $handler) {
            $wrappedHandlers[$subject] = MiddlewareChain::apply($handler, $this->middlewares);
        }

        // Subscribe to each subject
        foreach ($wrappedHandlers as $subject => $wrapped) {
            $worker = new Worker(
                nc: $this->nc,
                subject: $subject,
                senderId: $this->senderId,
                handler: $wrapped,
                privateKey: $this->privateKey,
                publicKeys: $this->publicKeys,
                metrics: $this->metrics,
            );
            // In PHP's single-threaded model, we subscribe but don't start processing yet
            $this->nc->subscribe($subject, function (string $payload) use ($wrapped): void {
                $this->processMessage($payload, $wrapped);
            });
        }

        // Start heartbeat loop (will emit one tick per process cycle)
        $senderId = $this->senderId;
        $pool = $this->pool;
        $maxParallel = $this->maxParallel;

        $this->heartbeatLoop = new HeartbeatLoop(
            nc: $this->nc,
            payloadFn: fn () => HeartbeatHelper::heartbeatPayload(
                $senderId, $pool, 0, $maxParallel, 0.0
            ),
            workerId: $senderId,
            intervalSeconds: $this->heartbeatInterval,
            metrics: $this->metrics,
        );

        // Process messages (blocking)
        $this->nc->process();
    }

    /**
     * Stop the heartbeat loop and disconnect.
     */
    public function close(): void
    {
        if ($this->heartbeatLoop !== null) {
            $this->heartbeatLoop->stop();
            $this->heartbeatLoop = null;
        }
    }

    private function processMessage(string $payload, callable $handler): void
    {
        $packet = new \Cordum\Agent\V1\BusPacket();
        try {
            $packet->mergeFromString($payload);
        } catch (\Throwable $e) {
            $this->metrics->onError('parse', "failed: {$e->getMessage()}");
            return;
        }

        if (!empty($this->publicKeys)) {
            $sid = $packet->getSenderId();
            if (!isset($this->publicKeys[$sid])) {
                return;
            }
            if ($packet->getSignature() === '' || !Signing::verifyPacketSignature($packet, $this->publicKeys[$sid])) {
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
            $result = $handler($ctx, $req);

            if ($result->getJobId() === '') {
                $result->setJobId($req->getJobId());
            }
            if ($result->getWorkerId() === '') {
                $result->setWorkerId($this->senderId);
            }

            $elapsed = (int) ((hrtime(true) - $start) / 1_000_000);
            if ($result->getStatus() === \Cordum\Agent\V1\JobStatus::JOB_STATUS_FAILED) {
                $this->metrics->onJobFailed($result->getJobId(), $result->getErrorMessage());
            } else {
                $this->metrics->onJobCompleted($result->getJobId(), $elapsed, 'SUCCEEDED');
            }
        } catch (\Throwable $e) {
            $this->metrics->onJobFailed($req->getJobId(), $e->getMessage());
            $result = new \Cordum\Agent\V1\JobResult();
            $result->setJobId($req->getJobId());
            $result->setWorkerId($this->senderId);
            $result->setStatus(\Cordum\Agent\V1\JobStatus::JOB_STATUS_FAILED);
            $result->setErrorMessage($e->getMessage());
        }

        $now = microtime(true);
        $timestamp = new \Google\Protobuf\Timestamp();
        $timestamp->setSeconds((int) $now);
        $timestamp->setNanos((int) (($now - (int) $now) * 1_000_000_000));

        $outPacket = new \Cordum\Agent\V1\BusPacket();
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
    }
}
