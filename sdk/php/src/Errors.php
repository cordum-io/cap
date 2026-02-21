<?php

declare(strict_types=1);

namespace Cordum\Cap;

/**
 * Error code categories matching the CAP protocol specification.
 */
final class ErrorCode
{
    public const UNSPECIFIED = 0;
    // Protocol errors (100-199)
    public const VERSION_MISMATCH = 100;
    public const MALFORMED_PACKET = 101;
    public const UNKNOWN_PAYLOAD = 102;
    public const SIGNATURE_INVALID = 103;
    public const SIGNATURE_MISSING = 104;
    // Job errors (200-299)
    public const JOB_TIMEOUT = 200;
    public const RESOURCE_EXHAUSTED = 201;
    public const PERMISSION_DENIED = 202;
    public const INVALID_INPUT = 203;
    public const JOB_NOT_FOUND = 204;
    public const DUPLICATE_JOB = 205;
    public const WORKER_UNAVAILABLE = 206;
    // Safety errors (300-399)
    public const SAFETY_DENIED = 300;
    public const POLICY_VIOLATION = 301;
    public const RISK_TAG_BLOCKED = 302;
    // Transport errors (400-499)
    public const PUBLISH_FAILED = 400;
    public const SUBSCRIBE_FAILED = 401;
    public const CONNECTION_LOST = 402;
}

/**
 * Base exception for all CAP protocol errors.
 */
class CapException extends \RuntimeException
{
    public function __construct(
        public readonly int $errorCode,
        string $message,
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, $errorCode, $previous);
    }

    // Protocol errors
    public static function versionMismatch(string $msg): self
    {
        return new self(ErrorCode::VERSION_MISMATCH, $msg);
    }

    public static function malformedPacket(string $msg): self
    {
        return new self(ErrorCode::MALFORMED_PACKET, $msg);
    }

    public static function unknownPayload(string $msg): self
    {
        return new self(ErrorCode::UNKNOWN_PAYLOAD, $msg);
    }

    public static function signatureInvalid(string $msg): self
    {
        return new self(ErrorCode::SIGNATURE_INVALID, $msg);
    }

    public static function signatureMissing(string $msg): self
    {
        return new self(ErrorCode::SIGNATURE_MISSING, $msg);
    }

    // Job errors
    public static function jobTimeout(string $msg): self
    {
        return new self(ErrorCode::JOB_TIMEOUT, $msg);
    }

    public static function resourceExhausted(string $msg): self
    {
        return new self(ErrorCode::RESOURCE_EXHAUSTED, $msg);
    }

    public static function permissionDenied(string $msg): self
    {
        return new self(ErrorCode::PERMISSION_DENIED, $msg);
    }

    public static function invalidInput(string $msg): self
    {
        return new self(ErrorCode::INVALID_INPUT, $msg);
    }

    public static function jobNotFound(string $msg): self
    {
        return new self(ErrorCode::JOB_NOT_FOUND, $msg);
    }

    public static function duplicateJob(string $msg): self
    {
        return new self(ErrorCode::DUPLICATE_JOB, $msg);
    }

    public static function workerUnavailable(string $msg): self
    {
        return new self(ErrorCode::WORKER_UNAVAILABLE, $msg);
    }

    // Safety errors
    public static function safetyDenied(string $msg): self
    {
        return new self(ErrorCode::SAFETY_DENIED, $msg);
    }

    public static function policyViolation(string $msg): self
    {
        return new self(ErrorCode::POLICY_VIOLATION, $msg);
    }

    public static function riskTagBlocked(string $msg): self
    {
        return new self(ErrorCode::RISK_TAG_BLOCKED, $msg);
    }

    // Transport errors
    public static function publishFailed(string $msg): self
    {
        return new self(ErrorCode::PUBLISH_FAILED, "publish failed: {$msg}");
    }

    public static function subscribeFailed(string $msg): self
    {
        return new self(ErrorCode::SUBSCRIBE_FAILED, "subscribe failed: {$msg}");
    }

    public static function connectionLost(string $msg): self
    {
        return new self(ErrorCode::CONNECTION_LOST, "connection lost: {$msg}");
    }
}

/**
 * Validation-specific exception with field information.
 */
class ValidationException extends CapException
{
    public function __construct(
        public readonly string $field,
        string $message,
    ) {
        parent::__construct(ErrorCode::INVALID_INPUT, "field={$field}, {$message}");
    }
}
