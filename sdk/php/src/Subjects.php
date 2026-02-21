<?php

declare(strict_types=1);

namespace Cordum\Cap;

/**
 * NATS subject constants and protocol defaults for the Cordum Agent Protocol.
 */
final class Subjects
{
    public const SUBMIT = 'sys.job.submit';
    public const RESULT = 'sys.job.result';
    public const HEARTBEAT = 'sys.heartbeat';
    public const ALERT = 'sys.alert';
    public const PROGRESS = 'sys.job.progress';
    public const CANCEL = 'sys.job.cancel';
    public const DLQ = 'sys.job.dlq';
    public const WORKFLOW_EVENT = 'sys.workflow.event';
    public const HANDSHAKE = 'sys.handshake';

    public const DEFAULT_PROTOCOL_VERSION = 1;
    public const DEFAULT_HEARTBEAT_INTERVAL = 5; // seconds
}
