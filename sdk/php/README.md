# CAP PHP SDK

PHP SDK for the Cordum Agent Protocol (CAP). Provides NATS-based worker/client abstractions, ECDSA packet signing, deterministic serialization, heartbeat/progress helpers, middleware, and metrics hooks.

## Quick Start

### Prerequisites
- PHP 8.1+
- ext-openssl (for ECDSA signing)
- Composer

### Install
```bash
cd sdk/php
composer install
```

### Run Tests
```bash
vendor/bin/phpunit
```

### Generate Protobuf Types
Proto compilation outputs to `proto/`. For standalone generation:
```bash
CAP_RUN_PHP=1 ./tools/make_protos.sh
```

## Usage

### Agent (High-Level API)

```php
use Cordum\Cap\Agent;
use Cordum\Cap\Middleware;
use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobResult;
use Cordum\Agent\V1\JobStatus;

$agent = new Agent([
    'natsUrl' => 'nats://localhost:4222',
    'senderId' => 'worker-php-1',
    'pool' => 'job.tools',
    'maxParallel' => 4,
]);

$agent->register('job.summarize', function (Middleware\Context $ctx, JobRequest $req): JobResult {
    $result = new JobResult();
    $result->setJobId($req->getJobId());
    $result->setWorkerId('worker-php-1');
    $result->setStatus(JobStatus::JOB_STATUS_SUCCEEDED);
    $result->setResultPtr('redis://result/' . $req->getJobId());
    return $result;
});

$agent->start();
```

### Worker (Low-Level API)

```php
use Cordum\Cap\Bus;
use Cordum\Cap\Worker;

$nc = Bus::connectNats(['url' => 'nats://localhost:4222']);

$worker = new Worker($nc, 'job.echo', 'worker-echo-1', function ($ctx, $req) {
    $result = new \Cordum\Agent\V1\JobResult();
    $result->setJobId($req->getJobId());
    $result->setWorkerId('worker-echo-1');
    $result->setStatus(\Cordum\Agent\V1\JobStatus::JOB_STATUS_SUCCEEDED);
    return $result;
});

$worker->start();
```

### Client (Job Submission)

```php
use Cordum\Cap\Bus;
use Cordum\Cap\CapClient;
use Cordum\Agent\V1\JobRequest;

$nc = Bus::connectNats(['url' => 'nats://localhost:4222']);
$client = new CapClient($nc);

$req = new JobRequest();
$req->setJobId('job-1');
$req->setTopic('job.echo');
$req->setContextPtr('redis://ctx/job-1');

$client->submit($req, 'trace-1', 'client-php', $privateKey);
```

### Heartbeats

```php
use Cordum\Cap\HeartbeatHelper;

// One-shot heartbeat
$payload = HeartbeatHelper::heartbeatPayload('worker-1', 'job.tools', 2, 8, 45.0);
HeartbeatHelper::emitHeartbeat($nc, $payload);

// Heartbeat loop (blocking, with tick for event loops)
$loop = new HeartbeatHelper\HeartbeatLoop($nc, function () {
    return HeartbeatHelper::heartbeatPayload('worker-1', 'job.tools', 0, 8, 0.0);
}, 'worker-1', 5, $metrics);

$loop->start(); // blocking
// or use $loop->tick() in your own event loop
```

### Progress and Cancel

```php
use Cordum\Cap\ProgressHelper;

$payload = ProgressHelper::progressPayload('worker-1', 'job-1', 'step-3', 75, 'processing');
ProgressHelper::emitProgress($nc, $payload);

$cancel = ProgressHelper::cancelPayload('client-1', 'job-1', 'timeout', 'admin');
ProgressHelper::emitCancel($nc, $cancel);
```

## Middleware

Add cross-cutting concerns without modifying handlers:

```php
use Cordum\Cap\Middleware\MiddlewareChain;

// Built-in logging middleware
$agent->use(MiddlewareChain::logging());

// Custom middleware
$agent->use(function (callable $next) {
    return function ($ctx, $req) use ($next) {
        $start = hrtime(true);
        $result = $next($ctx, $req);
        $elapsed = (hrtime(true) - $start) / 1e6;
        echo "Job {$req->getJobId()} took {$elapsed}ms\n";
        return $result;
    };
});
```

Middleware executes in registration order (FIFO).

## Signing

ECDSA P-256 with SHA-256 (ASN.1/DER format), interoperable with Go/Python/Node/Java/Rust/C# SDKs:

```php
use Cordum\Cap\Signing;

// Load keys from PEM
$privateKey = Signing::loadPrivateKey($pemString);
$publicKey = Signing::loadPublicKey($pemString);

// Sign a packet
Signing::signPacket($packet, $privateKey);

// Verify a packet
$valid = Signing::verifyPacketSignature($packet, $publicKey);
```

## Metrics

Implement `MetricsHook` to integrate with Prometheus, OpenTelemetry, or any observability system:

```php
use Cordum\Cap\MetricsHook;

interface MetricsHook
{
    public function onJobReceived(string $jobId, string $topic): void;
    public function onJobCompleted(string $jobId, int $durationMs, string $status): void;
    public function onJobFailed(string $jobId, string $errorMsg): void;
    public function onHeartbeatSent(string $workerId): void;
    public function onProgressEmitted(string $jobId, int $percent): void;
    public function onError(string $category, string $message): void;
}
```

Default is `NoopMetrics` (zero overhead).

## Testing

Test handlers without a real NATS server:

```php
use Cordum\Cap\Testing\MockBus;
use Cordum\Cap\Testing\TestHelper;
use Cordum\Agent\V1\JobRequest;
use Cordum\Agent\V1\JobStatus;

$req = new JobRequest();
$req->setJobId('test-1');
$req->setTopic('job.echo');

$result = TestHelper::submitAndWait($myHandler, $req);
$this->assertSame(JobStatus::JOB_STATUS_SUCCEEDED, $result->getStatus());
```

## Structure

- `Cordum\Cap` — SDK classes (Codec, Signing, Validate, Worker, CapClient, Agent, etc.)
- `Cordum\Agent\V1` — Generated protobuf types (BusPacket, JobRequest, JobResult, etc.)
- Proto source: `proto/cordum/agent/v1/*.proto`

## API Reference

| Class | Description |
|-------|-------------|
| `Agent` | High-level runtime with handler registration, middleware, heartbeats |
| `Worker` | Low-level NATS subscriber with job dispatch |
| `CapClient` | Job submission helper |
| `Bus` | NATS connection factory |
| `Codec` | Deterministic protobuf serialization |
| `Signing` | ECDSA P-256 packet signing/verification |
| `Validate` | Input validation (JobRequest, JobResult, BusPacket) |
| `HeartbeatHelper` | Heartbeat payload builders and emission loop |
| `ProgressHelper` | Progress/cancel payload builders and emission |
| `MiddlewareChain` | Composable interceptor chain |
| `MetricsHook` | Pluggable observability callbacks |
| `CapException` | Typed error codes and exception classes |
| `MockBus` / `TestHelper` | Testing utilities and recording metrics |
