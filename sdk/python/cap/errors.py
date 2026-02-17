"""Typed error classes matching the CAP ErrorCode registry.

See spec/13-error-codes.md for the full taxonomy.
"""


class CAPError(Exception):
    """Base class for all CAP protocol errors."""

    code: str = "ERROR_CODE_UNSPECIFIED"
    numeric_code: int = 0

    def __init__(self, message: str) -> None:
        super().__init__(message)


# Protocol errors (100-199)


class VersionMismatchError(CAPError):
    code = "ERROR_CODE_PROTOCOL_VERSION_MISMATCH"
    numeric_code = 100


class MalformedPacketError(CAPError):
    code = "ERROR_CODE_PROTOCOL_MALFORMED_PACKET"
    numeric_code = 101


class UnknownPayloadError(CAPError):
    code = "ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD"
    numeric_code = 102


class SignatureInvalidError(CAPError):
    code = "ERROR_CODE_PROTOCOL_SIGNATURE_INVALID"
    numeric_code = 103


class SignatureMissingError(CAPError):
    code = "ERROR_CODE_PROTOCOL_SIGNATURE_MISSING"
    numeric_code = 104


# Job errors (200-299)


class JobTimeoutError(CAPError):
    code = "ERROR_CODE_JOB_TIMEOUT"
    numeric_code = 200


class ResourceExhaustedError(CAPError):
    code = "ERROR_CODE_JOB_RESOURCE_EXHAUSTED"
    numeric_code = 201


class PermissionDeniedError(CAPError):
    code = "ERROR_CODE_JOB_PERMISSION_DENIED"
    numeric_code = 202


class InvalidInputError(CAPError):
    code = "ERROR_CODE_JOB_INVALID_INPUT"
    numeric_code = 203


class JobNotFoundError(CAPError):
    code = "ERROR_CODE_JOB_NOT_FOUND"
    numeric_code = 204


class DuplicateJobError(CAPError):
    code = "ERROR_CODE_JOB_DUPLICATE"
    numeric_code = 205


class WorkerUnavailableError(CAPError):
    code = "ERROR_CODE_JOB_WORKER_UNAVAILABLE"
    numeric_code = 206


# Safety errors (300-399)


class SafetyDeniedError(CAPError):
    code = "ERROR_CODE_SAFETY_DENIED"
    numeric_code = 300


class PolicyViolationError(CAPError):
    code = "ERROR_CODE_SAFETY_POLICY_VIOLATION"
    numeric_code = 301


class RiskTagBlockedError(CAPError):
    code = "ERROR_CODE_SAFETY_RISK_TAG_BLOCKED"
    numeric_code = 302


# Transport errors (400-499)


class PublishFailedError(CAPError):
    code = "ERROR_CODE_TRANSPORT_PUBLISH_FAILED"
    numeric_code = 400


class SubscribeFailedError(CAPError):
    code = "ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED"
    numeric_code = 401


class ConnectionLostError(CAPError):
    code = "ERROR_CODE_TRANSPORT_CONNECTION_LOST"
    numeric_code = 402
