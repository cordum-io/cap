<?php

declare(strict_types=1);

namespace Cordum\Cap\Tests;

use Cordum\Agent\V1\BusPacket;
use Cordum\Agent\V1\Heartbeat;
use Cordum\Agent\V1\JobRequest;
use Cordum\Cap\Codec;
use Cordum\Cap\Signing;
use Google\Protobuf\Timestamp;
use OpenSSLAsymmetricKey;
use PHPUnit\Framework\TestCase;

class SigningTest extends TestCase
{
    private function makePacket(): BusPacket
    {
        $req = new JobRequest();
        $req->setJobId('job-sign-1');
        $req->setTopic('sign.test');

        $ts = new Timestamp();
        $ts->setSeconds(1704164645);

        $pkt = new BusPacket();
        $pkt->setTraceId('trace-sign');
        $pkt->setSenderId('sender-sign');
        $pkt->setProtocolVersion(1);
        $pkt->setCreatedAt($ts);
        $pkt->setJobRequest($req);

        return $pkt;
    }

    /** @return array{0: OpenSSLAsymmetricKey, 1: OpenSSLAsymmetricKey} */
    private function makeKeyPair(): array
    {
        $key = openssl_pkey_new([
            'curve_name' => 'prime256v1',
            'private_key_type' => OPENSSL_KEYTYPE_EC,
        ]);
        $details = openssl_pkey_get_details($key);
        openssl_pkey_export($key, $privatePem);
        $publicPem = $details['key'];

        $priv = openssl_pkey_get_private($privatePem);
        $pub = openssl_pkey_get_public($publicPem);

        return [$priv, $pub];
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function signAndVerifyRoundTrip(): void
    {
        [$priv, $pub] = $this->makeKeyPair();
        $packet = $this->makePacket();

        Signing::signPacket($packet, $priv);
        $this->assertNotEmpty($packet->getSignature());
        $this->assertTrue(Signing::verifyPacketSignature($packet, $pub));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function signAndVerifyHeartbeat(): void
    {
        [$priv, $pub] = $this->makeKeyPair();

        $hb = new Heartbeat();
        $hb->setWorkerId('worker-hb');
        $hb->setPool('gpu');
        $hb->setActiveJobs(3);

        $pkt = new BusPacket();
        $pkt->setTraceId('trace-hb');
        $pkt->setSenderId('worker-hb');
        $pkt->setProtocolVersion(1);
        $pkt->setHeartbeat($hb);

        Signing::signPacket($pkt, $priv);
        $this->assertTrue(Signing::verifyPacketSignature($pkt, $pub));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function tamperTraceIdDetected(): void
    {
        [$priv, $pub] = $this->makeKeyPair();
        $packet = $this->makePacket();
        Signing::signPacket($packet, $priv);

        $packet->setTraceId('tampered');
        $this->assertFalse(Signing::verifyPacketSignature($packet, $pub));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function tamperSenderIdDetected(): void
    {
        [$priv, $pub] = $this->makeKeyPair();
        $packet = $this->makePacket();
        Signing::signPacket($packet, $priv);

        $packet->setSenderId('tampered');
        $this->assertFalse(Signing::verifyPacketSignature($packet, $pub));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function tamperPayloadDetected(): void
    {
        [$priv, $pub] = $this->makeKeyPair();
        $packet = $this->makePacket();
        Signing::signPacket($packet, $priv);

        $tampered = new JobRequest();
        $tampered->setJobId('tampered');
        $tampered->setTopic('tampered');
        $packet->setJobRequest($tampered);
        $this->assertFalse(Signing::verifyPacketSignature($packet, $pub));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function emptySignatureRejected(): void
    {
        [, $pub] = $this->makeKeyPair();
        $packet = $this->makePacket();
        $this->assertFalse(Signing::verifyPacketSignature($packet, $pub));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function wrongKeyRejected(): void
    {
        [$priv1,] = $this->makeKeyPair();
        [, $pub2] = $this->makeKeyPair();

        $packet = $this->makePacket();
        Signing::signPacket($packet, $priv1);
        $this->assertFalse(Signing::verifyPacketSignature($packet, $pub2));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function signatureIsDerEncoded(): void
    {
        [$priv,] = $this->makeKeyPair();
        $packet = $this->makePacket();
        Signing::signPacket($packet, $priv);

        $sig = $packet->getSignature();
        // DER-encoded ECDSA signatures start with 0x30 (SEQUENCE tag)
        $this->assertSame(0x30, ord($sig[0]));
        // P-256 DER signatures are typically 68-73 bytes
        $this->assertGreaterThanOrEqual(68, strlen($sig));
        $this->assertLessThanOrEqual(73, strlen($sig));
    }

    #[\PHPUnit\Framework\Attributes\Test]
    public function signingDoesNotAlterContent(): void
    {
        [$priv,] = $this->makeKeyPair();
        $packet = $this->makePacket();
        $clean = clone $packet;
        $clean->setSignature('');
        $originalBytes = Codec::marshalDeterministic($clean);

        Signing::signPacket($packet, $priv);

        $after = clone $packet;
        $after->setSignature('');
        $afterBytes = Codec::marshalDeterministic($after);

        $this->assertSame($originalBytes, $afterBytes);
    }
}
