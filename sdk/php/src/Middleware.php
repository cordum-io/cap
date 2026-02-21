<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;

/**
 * Context passed through the middleware chain.
 */
class Context
{
    public function __construct(
        public readonly string $traceId,
        public readonly string $senderId,
        public readonly ?BusPacket $packet = null,
    ) {}
}

/**
 * Middleware composition utilities.
 *
 * Handler: callable(Context $ctx, JobRequest $req): JobResult
 * Middleware: callable(callable $next): callable
 */
final class MiddlewareChain
{
    /**
     * Compose a chain of middlewares around a base handler.
     * Middlewares execute in FIFO order (first added = outermost wrapper).
     *
     * @param callable $handler Base handler function
     * @param callable[] $middlewares Array of middleware functions
     * @return callable Wrapped handler
     */
    public static function apply(callable $handler, array $middlewares): callable
    {
        $result = $handler;
        for ($i = count($middlewares) - 1; $i >= 0; $i--) {
            $result = ($middlewares[$i])($result);
        }
        return $result;
    }

    /**
     * Middleware that logs job_id and duration for each request.
     *
     * @return callable Middleware function
     */
    public static function logging(): callable
    {
        return function (callable $next): callable {
            return function (Context $ctx, JobRequest $req) use ($next): JobResult {
                $start = hrtime(true);
                $result = $next($ctx, $req);
                $elapsed = (int) ((hrtime(true) - $start) / 1_000_000); // nanoseconds to ms
                error_log("[cap] job={$req->getJobId()} duration={$elapsed}ms");
                return $result;
            };
        };
    }
}
