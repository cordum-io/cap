<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Basis\Nats\Client as NatsClient;
use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\JobRequest;
use Google\Protobuf\Timestamp;
use OpenSSLAsymmetricKey;

/**
 * CAP client for submitting jobs.
 */
final class CapClient
{
    /** @var array<string, OpenSSLAsymmetricKey> */
    private array $publicKeys;

    /**
     * @param NatsClient $nc NATS connection
     * @param array<string, OpenSSLAsymmetricKey> $publicKeys Public keys for signature verification
     */
    public function __construct(
        private readonly NatsClient $nc,
        array $publicKeys = [],
    ) {
        $this->publicKeys = $publicKeys;
    }

    /**
     * Submit a job request to the bus.
     */
    public function submit(
        JobRequest $req,
        string $traceId,
        string $senderId,
        ?OpenSSLAsymmetricKey $privateKey = null,
    ): void {
        $now = microtime(true);
        $timestamp = new Timestamp();
        $timestamp->setSeconds((int) $now);
        $timestamp->setNanos((int) (($now - (int) $now) * 1_000_000_000));

        $packet = new BusPacket();
        $packet->setTraceId($traceId);
        $packet->setSenderId($senderId);
        $packet->setProtocolVersion(Subjects::DEFAULT_PROTOCOL_VERSION);
        $packet->setCreatedAt($timestamp);
        $packet->setJobRequest($req);

        if ($privateKey !== null) {
            Signing::signPacket($packet, $privateKey);
        }

        $data = Codec::marshalDeterministic($packet);
        $this->nc->publish(Subjects::SUBMIT, $data);
    }

    /**
     * Submit a job request with a specific protocol version.
     */
    public function submitWithVersion(
        JobRequest $req,
        string $traceId,
        string $senderId,
        int $protocolVersion,
        ?OpenSSLAsymmetricKey $privateKey = null,
    ): void {
        $now = microtime(true);
        $timestamp = new Timestamp();
        $timestamp->setSeconds((int) $now);
        $timestamp->setNanos((int) (($now - (int) $now) * 1_000_000_000));

        $packet = new BusPacket();
        $packet->setTraceId($traceId);
        $packet->setSenderId($senderId);
        $packet->setProtocolVersion($protocolVersion);
        $packet->setCreatedAt($timestamp);
        $packet->setJobRequest($req);

        if ($privateKey !== null) {
            Signing::signPacket($packet, $privateKey);
        }

        $data = Codec::marshalDeterministic($packet);
        $this->nc->publish(Subjects::SUBMIT, $data);
    }
}
