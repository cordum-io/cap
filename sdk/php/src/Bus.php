<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Basis\Nats\Client;
use Basis\Nats\Configuration;

/**
 * NATS connection helper.
 */
final class Bus
{
    /**
     * Connect to NATS using the provided configuration.
     *
     * @param array{
     *   host?: string,
     *   port?: int,
     *   user?: string,
     *   pass?: string,
     *   token?: string,
     *   timeout?: float,
     * } $config Connection configuration
     * @return Client
     */
    public static function connectNats(array $config = []): Client
    {
        $natsConfig = new Configuration([
            'host' => $config['host'] ?? 'localhost',
            'port' => $config['port'] ?? 4222,
            'user' => $config['user'] ?? null,
            'pass' => $config['pass'] ?? null,
            'token' => $config['token'] ?? null,
            'timeout' => $config['timeout'] ?? 5.0,
        ]);

        return new Client($natsConfig);
    }
}
