/**
 * Typed error classes matching the CAP ErrorCode registry.
 * See spec/13-error-codes.md for the full taxonomy.
 */

/** Base class for all CAP protocol errors. */
export class CAPError extends Error {
  /** ErrorCode enum name (e.g. "ERROR_CODE_PROTOCOL_MALFORMED_PACKET"). */
  readonly code: string;
  /** Numeric ErrorCode value from the proto enum. */
  readonly numericCode: number;

  constructor(code: string, numericCode: number, message: string) {
    super(message);
    this.name = this.constructor.name;
    this.code = code;
    this.numericCode = numericCode;
  }
}

// Protocol errors (100-199)

export class VersionMismatchError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_PROTOCOL_VERSION_MISMATCH", 100, message);
  }
}

export class MalformedPacketError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_PROTOCOL_MALFORMED_PACKET", 101, message);
  }
}

export class UnknownPayloadError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD", 102, message);
  }
}

export class SignatureInvalidError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_PROTOCOL_SIGNATURE_INVALID", 103, message);
  }
}

export class SignatureMissingError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_PROTOCOL_SIGNATURE_MISSING", 104, message);
  }
}

// Job errors (200-299)

export class JobTimeoutError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_JOB_TIMEOUT", 200, message);
  }
}

export class ResourceExhaustedError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_JOB_RESOURCE_EXHAUSTED", 201, message);
  }
}

export class PermissionDeniedError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_JOB_PERMISSION_DENIED", 202, message);
  }
}

export class InvalidInputError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_JOB_INVALID_INPUT", 203, message);
  }
}

export class JobNotFoundError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_JOB_NOT_FOUND", 204, message);
  }
}

export class DuplicateJobError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_JOB_DUPLICATE", 205, message);
  }
}

export class WorkerUnavailableError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_JOB_WORKER_UNAVAILABLE", 206, message);
  }
}

// Safety errors (300-399)

export class SafetyDeniedError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_SAFETY_DENIED", 300, message);
  }
}

export class PolicyViolationError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_SAFETY_POLICY_VIOLATION", 301, message);
  }
}

export class RiskTagBlockedError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_SAFETY_RISK_TAG_BLOCKED", 302, message);
  }
}

// Transport errors (400-499)

export class PublishFailedError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_TRANSPORT_PUBLISH_FAILED", 400, message);
  }
}

export class SubscribeFailedError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED", 401, message);
  }
}

export class ConnectionLostError extends CAPError {
  constructor(message: string) {
    super("ERROR_CODE_TRANSPORT_CONNECTION_LOST", 402, message);
  }
}
