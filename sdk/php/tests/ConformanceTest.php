<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\JobPriority;
use Cordum\Agent\V1\JobStatus;
use Cordum\Agent\V1\AlertSeverity;
use Cordum\Agent\V1\ComponentRole;
use Cordum\Cap\Codec;
use Cordum\Cap\Signing;
use OpenSSLAsymmetricKey;
use PHPUnit\Framework\TestCase;

class ConformanceTest extends TestCase
{
    private static function fixturesDir(): string
    {
        return dirname(__DIR__, 3) . '/spec/conformance/fixtures';
    }

    private static function loadFixture(string $name): BusPacket
    {
        $path = self::fixturesDir() . '/' . $name;
        $data = file_get_contents($path);
        if ($data === false) {
            throw new \RuntimeException("Failed to read fixture: {$name}");
        }
        $pkt = new BusPacket();
        $pkt->mergeFromString($data);
        return $pkt;
    }

    private static function loadConformancePublicKey(): OpenSSLAsymmetricKey
    {
        $path = self::fixturesDir() . '/public_key.pem';
        $pem = file_get_contents($path);
        return Signing::loadPublicKey($pem);
    }

    private function verifyFixtureSignature(BusPacket $pkt, OpenSSLAsymmetricKey $key): void
    {
        $this->assertNotEmpty($pkt->getSignature(), 'fixture packet has no signature');
        $this->assertTrue(Signing::verifyPacketSignature($pkt, $key), 'signature verification failed');
    }

    private function assertTimestamp(BusPacket $pkt): void
    {
        $ts = $pkt->getCreatedAt();
        $this->assertNotNull($ts);
        $this->assertSame(1704164645, $ts->getSeconds());
        $this->assertSame(0, $ts->getNanos());
        $this->assertSame(1, $pkt->getProtocolVersion());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceJobRequest(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_job_request.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $req = $pkt->getJobRequest();
        $this->assertNotNull($req);
        $this->assertSame('job-req-1', $req->getJobId());
        $this->assertSame('job.tools', $req->getTopic());
        $this->assertSame(JobPriority::JOB_PRIORITY_INTERACTIVE, $req->getPriority());
        $this->assertSame('redis://ctx:job-req-1', $req->getContextPtr());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceJobResult(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_job_result.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $res = $pkt->getJobResult();
        $this->assertNotNull($res);
        $this->assertSame('job-res-1', $res->getJobId());
        $this->assertSame('worker-1', $res->getWorkerId());
        $this->assertSame(JobStatus::JOB_STATUS_FAILED_RETRYABLE, $res->getStatus());
        $this->assertSame('E_TEMP', $res->getErrorCode());
        $this->assertCount(2, $res->getArtifactPtrs());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceHeartbeat(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_heartbeat.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $hb = $pkt->getHeartbeat();
        $this->assertNotNull($hb);
        $this->assertSame('worker-1', $hb->getWorkerId());
        $this->assertSame('job.tools', $hb->getPool());
        $this->assertSame('us-east-1a', $hb->getLabels()['zone']);
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceJobProgress(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_job_progress.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $progress = $pkt->getJobProgress();
        $this->assertNotNull($progress);
        $this->assertSame('job-prog-1', $progress->getJobId());
        $this->assertSame(50, $progress->getPercent());
        $this->assertSame(JobStatus::JOB_STATUS_RUNNING, $progress->getStatus());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceJobCancel(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_job_cancel.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $cancel = $pkt->getJobCancel();
        $this->assertNotNull($cancel);
        $this->assertSame('job-cancel-1', $cancel->getJobId());
        $this->assertSame('user-7', $cancel->getRequestedBy());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceAlert(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_alert.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $alert = $pkt->getAlert();
        $this->assertNotNull($alert);
        $this->assertSame('WARN', $alert->getLevel());
        $this->assertSame('scheduler', $alert->getComponent());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceHandshake(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_handshake.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $hs = $pkt->getHandshake();
        $this->assertNotNull($hs);
        $this->assertSame('worker-1', $hs->getComponentId());
        $this->assertSame(ComponentRole::COMPONENT_ROLE_WORKER, $hs->getRole());
        $this->assertSame('2.0.19', $hs->getSdkVersion());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceAlertEnhanced(): void
    {
        $key = self::loadConformancePublicKey();
        $pkt = self::loadFixture('buspacket_alert_enhanced.bin');

        $this->verifyFixtureSignature($pkt, $key);
        $this->assertTimestamp($pkt);

        $alert = $pkt->getAlert();
        $this->assertNotNull($alert);
        $this->assertSame('CRITICAL', $alert->getLevel());
        $this->assertSame('scheduler', $alert->getComponent());
        $this->assertSame('SIGNATURE_INVALID', $alert->getCode());
        $this->assertSame(AlertSeverity::ALERT_SEVERITY_CRITICAL, $alert->getSeverity());
        $this->assertSame('scheduler-1', $alert->getSourceComponent());
        $this->assertSame('worker-bad', $alert->getDetails()['sender']);
        $this->assertSame('sys.job.result', $alert->getDetails()['subject']);
        $this->assertSame('trace-offending-packet', $alert->getTraceId());
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function conformanceAllFixturesSignatureVerification(): void
    {
        $key = self::loadConformancePublicKey();
        $fixtures = [
            'buspacket_job_request.bin',
            'buspacket_job_result.bin',
            'buspacket_heartbeat.bin',
            'buspacket_job_progress.bin',
            'buspacket_job_cancel.bin',
            'buspacket_alert.bin',
            'buspacket_handshake.bin',
            'buspacket_alert_enhanced.bin',
        ];

        foreach ($fixtures as $fixture) {
            $pkt = self::loadFixture($fixture);
            $this->assertNotEmpty($pkt->getSignature(), "{$fixture} has no signature");

            $unsignedBytes = Codec::marshalUnsignedForSignature($pkt);
            $verified = openssl_verify($unsignedBytes, $pkt->getSignature(), $key, OPENSSL_ALGO_SHA256) === 1;
            $this->assertTrue($verified, "{$fixture} signature verification failed");
        }
    }
}
