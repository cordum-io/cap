"""CAP (Cordum Agent Protocol) SDK for Python.

Provides helpers for submitting jobs, running workers, and building
high-level agents on the CAP bus.
"""

from .client import submit_job
from .worker import run_worker
from .bus import connect_nats
from .runtime import Agent, Context, BlobStore, RedisBlobStore, InMemoryBlobStore
from .middleware import Middleware, NextFn, logging_middleware
from .metrics import MetricsHook, NoopMetrics
from .heartbeat import (
    heartbeat_payload,
    heartbeat_payload_with_memory,
    heartbeat_payload_with_progress,
    emit_heartbeat,
    heartbeat_loop,
)
from .handshake import handshake_payload, publish_handshake
from .trust_signing import (
    TrustSigningError,
    sign_trust_packet,
    trust_packet_digest,
    verify_trust_packet,
)
from .progress import (
    progress_payload,
    cancel_payload,
    emit_progress,
    emit_cancel,
)
from .validate import (
    ValidationError,
    validate_job_request,
    validate_job_result,
    validate_bus_packet,
)
from .errors import (
    CAPError,
    VersionMismatchError,
    MalformedPacketError,
    UnknownPayloadError,
    SignatureInvalidError,
    SignatureMissingError,
    JobTimeoutError,
    ResourceExhaustedError,
    PermissionDeniedError,
    InvalidInputError,
    JobNotFoundError,
    DuplicateJobError,
    WorkerUnavailableError,
    SafetyDeniedError,
    PolicyViolationError,
    RiskTagBlockedError,
    PublishFailedError,
    SubscribeFailedError,
    ConnectionLostError,
)
from .subjects import (
    SUBJECT_SUBMIT,
    SUBJECT_RESULT,
    SUBJECT_HEARTBEAT,
    SUBJECT_ALERT,
    SUBJECT_PROGRESS,
    SUBJECT_CANCEL,
    SUBJECT_DLQ,
    SUBJECT_WORKFLOW_EVENT,
    SUBJECT_HANDSHAKE,
)

__all__ = [
    "submit_job",
    "run_worker",
    "connect_nats",
    "Agent",
    "Context",
    "BlobStore",
    "RedisBlobStore",
    "InMemoryBlobStore",
    "Middleware",
    "NextFn",
    "logging_middleware",
    "MetricsHook",
    "NoopMetrics",
    "heartbeat_payload",
    "heartbeat_payload_with_memory",
    "heartbeat_payload_with_progress",
    "emit_heartbeat",
    "heartbeat_loop",
    "handshake_payload",
    "publish_handshake",
    "TrustSigningError",
    "sign_trust_packet",
    "trust_packet_digest",
    "verify_trust_packet",
    "progress_payload",
    "cancel_payload",
    "emit_progress",
    "emit_cancel",
    "ValidationError",
    "validate_job_request",
    "validate_job_result",
    "validate_bus_packet",
    "SUBJECT_SUBMIT",
    "SUBJECT_RESULT",
    "SUBJECT_HEARTBEAT",
    "SUBJECT_ALERT",
    "SUBJECT_PROGRESS",
    "SUBJECT_CANCEL",
    "SUBJECT_DLQ",
    "SUBJECT_WORKFLOW_EVENT",
    "SUBJECT_HANDSHAKE",
    "CAPError",
    "VersionMismatchError",
    "MalformedPacketError",
    "UnknownPayloadError",
    "SignatureInvalidError",
    "SignatureMissingError",
    "JobTimeoutError",
    "ResourceExhaustedError",
    "PermissionDeniedError",
    "InvalidInputError",
    "JobNotFoundError",
    "DuplicateJobError",
    "WorkerUnavailableError",
    "SafetyDeniedError",
    "PolicyViolationError",
    "RiskTagBlockedError",
    "PublishFailedError",
    "SubscribeFailedError",
    "ConnectionLostError",
]
