<?php

declare(strict_types=1);

namespace Cordum\Cap;

use Cordum\Agent\V1\BusPacket;
use Google\Protobuf\Internal\Message;

/**
 * Deterministic protobuf serialization utilities.
 */
final class Codec
{
    /**
     * Serialize a protobuf message deterministically.
     * Google's PHP protobuf runtime produces deterministic output for proto3 messages.
     */
    public static function marshalDeterministic(Message $msg): string
    {
        return $msg->serializeToString();
    }

    /**
     * Serialize a BusPacket with the signature field cleared.
     * Used as the input to ECDSA signing/verification.
     */
    public static function marshalUnsignedForSignature(BusPacket $packet): string
    {
        $clone = clone $packet;
        $clone->setSignature('');
        return self::marshalDeterministic($clone);
    }
}
