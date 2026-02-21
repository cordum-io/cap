<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Cordum\Agent\V1\BusPacket;
use OpenSSLAsymmetricKey;

/**
 * ECDSA P-256 packet signing and verification.
 * Uses ASN.1/DER signature format for interoperability with Go/Python/Node/Java/Rust/C# SDKs.
 */
final class Signing
{
    /**
     * Sign a BusPacket using ECDSA P-256 with SHA-256.
     * Sets the signature field on the packet in ASN.1/DER format.
     *
     * @param BusPacket $packet The packet to sign (modified in-place)
     * @param OpenSSLAsymmetricKey $privateKey ECDSA private key
     * @throws CapException on signing failure
     */
    public static function signPacket(BusPacket $packet, OpenSSLAsymmetricKey $privateKey): void
    {
        $unsignedBytes = Codec::marshalUnsignedForSignature($packet);

        $signature = '';
        if (!openssl_sign($unsignedBytes, $signature, $privateKey, OPENSSL_ALGO_SHA256)) {
            throw CapException::signatureInvalid('failed to sign packet: ' . openssl_error_string());
        }

        $packet->setSignature($signature);
    }

    /**
     * Verify a BusPacket's ECDSA P-256 signature.
     *
     * @param BusPacket $packet The packet to verify
     * @param OpenSSLAsymmetricKey $publicKey ECDSA public key
     * @return bool true if the signature is valid
     */
    public static function verifyPacketSignature(BusPacket $packet, OpenSSLAsymmetricKey $publicKey): bool
    {
        $sig = $packet->getSignature();
        if ($sig === '' || $sig === null) {
            return false;
        }

        $unsignedBytes = Codec::marshalUnsignedForSignature($packet);

        return openssl_verify($unsignedBytes, $sig, $publicKey, OPENSSL_ALGO_SHA256) === 1;
    }

    /**
     * Load an ECDSA private key from PEM-encoded text.
     *
     * @param string $pem PEM-encoded PKCS#8 EC private key
     * @return OpenSSLAsymmetricKey
     * @throws CapException on parse failure
     */
    public static function loadPrivateKey(string $pem): OpenSSLAsymmetricKey
    {
        $key = openssl_pkey_get_private($pem);
        if ($key === false) {
            throw CapException::signatureInvalid('failed to load private key: ' . openssl_error_string());
        }
        return $key;
    }

    /**
     * Load an ECDSA public key from PEM-encoded text.
     *
     * @param string $pem PEM-encoded SPKI EC public key
     * @return OpenSSLAsymmetricKey
     * @throws CapException on parse failure
     */
    public static function loadPublicKey(string $pem): OpenSSLAsymmetricKey
    {
        $key = openssl_pkey_get_public($pem);
        if ($key === false) {
            throw CapException::signatureInvalid('failed to load public key: ' . openssl_error_string());
        }
        return $key;
    }
}
