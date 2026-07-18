"""Opt-in semantic validation helpers for CAP protobuf messages."""

from dataclasses import dataclass
from typing import Any, List

from cap.constants import DEFAULT_PROTOCOL_VERSION
from cap.pb.cordum.agent.v1 import alert_pb2, buspacket_pb2, job_pb2


_JOB_PRIORITY_MAX = job_pb2.JOB_PRIORITY_CRITICAL
MAX_REPEATED_FIELD_SIZE = 1000
MAX_BUDGET_TOKENS = 10_000_000_000


@dataclass(frozen=True)
class ValidationError:
    """A single validation failure."""

    field: str
    message: str


def validate_job_request(msg: Any) -> List[ValidationError]:
    """Validate semantic constraints on a JobRequest."""
    errors: List[ValidationError] = []
    if msg is None:
        return [ValidationError("JobRequest", "must not be nil")]
    if not msg.job_id:
        errors.append(ValidationError("job_id", "must not be empty"))
    if not msg.topic:
        errors.append(ValidationError("topic", "must not be empty"))
    if msg.priority < 0 or msg.priority > _JOB_PRIORITY_MAX:
        errors.append(ValidationError("priority", f"invalid value {msg.priority}"))
    if msg.step_index < 0:
        errors.append(ValidationError("step_index", "must not be negative"))
    errors.extend(_validate_job_collections(msg))
    if msg.HasField("budget"):
        errors.extend(_validate_budget(msg.budget))
    return errors


def _validate_job_collections(msg: Any) -> List[ValidationError]:
    errors: List[ValidationError] = []
    if msg.HasField("meta"):
        if len(msg.meta.risk_tags) > MAX_REPEATED_FIELD_SIZE:
            errors.append(ValidationError("meta.risk_tags", "exceeds max size (1000)"))
        if len(msg.meta.requires) > MAX_REPEATED_FIELD_SIZE:
            errors.append(ValidationError("meta.requires", "exceeds max size (1000)"))
    if len(msg.labels) > MAX_REPEATED_FIELD_SIZE:
        errors.append(ValidationError("labels", "exceeds max size (1000)"))
    return errors


def _validate_budget(budget: Any) -> List[ValidationError]:
    errors: List[ValidationError] = []
    for field in ("max_input_tokens", "max_output_tokens", "max_total_tokens"):
        value = getattr(budget, field)
        if value < 0:
            errors.append(ValidationError("budget." + field, "must not be negative"))
        if value > MAX_BUDGET_TOKENS:
            errors.append(ValidationError("budget." + field, "exceeds max (10000000000)"))
    if budget.deadline_ms < 0:
        errors.append(ValidationError("budget.deadline_ms", "must not be negative"))
    return errors


def validate_job_result(msg: Any) -> List[ValidationError]:
    """Validate semantic constraints on a JobResult."""
    errors: List[ValidationError] = []
    if msg is None:
        return [ValidationError("JobResult", "must not be nil")]
    if not msg.job_id:
        errors.append(ValidationError("job_id", "must not be empty"))
    if msg.status == job_pb2.JOB_STATUS_UNSPECIFIED:
        errors.append(ValidationError("status", "must not be UNSPECIFIED"))
    if not msg.worker_id:
        errors.append(ValidationError("worker_id", "must not be empty"))
    if msg.execution_ms < 0:
        errors.append(ValidationError("execution_ms", "must not be negative"))
    return errors


def validate_job_progress(msg: Any) -> List[ValidationError]:
    """Validate the required identity and 0..100 progress bound."""
    errors: List[ValidationError] = []
    if not msg.job_id:
        errors.append(ValidationError("job_progress.job_id", "must not be empty"))
    if msg.percent < 0 or msg.percent > 100:
        errors.append(ValidationError("job_progress.percent", "must be 0..100"))
    if msg.status not in job_pb2.JobStatus.values():
        errors.append(ValidationError("job_progress.status", "is invalid"))
    return errors


def validate_job_cancel(msg: Any) -> List[ValidationError]:
    """Validate cancellation target and requester identity."""
    errors: List[ValidationError] = []
    if not msg.job_id:
        errors.append(ValidationError("job_cancel.job_id", "must not be empty"))
    if not msg.requested_by:
        errors.append(ValidationError("job_cancel.requested_by", "must not be empty"))
    return errors


def validate_alert(msg: Any, sender_id: str) -> List[ValidationError]:
    """Accept legacy alerts or require a complete bound structured shape."""
    errors: List[ValidationError] = []
    if not msg.message:
        errors.append(ValidationError("alert.message", "must not be empty"))
    structured = bool(msg.severity or msg.error_code_enum or msg.source_component)
    if not structured:
        for field in ("level", "component", "code"):
            if not getattr(msg, field):
                errors.append(ValidationError("alert." + field, "must not be empty"))
        return errors
    if msg.severity not in alert_pb2.AlertSeverity.values()[1:]:
        errors.append(ValidationError("alert.severity", "must be specified"))
    if msg.error_code_enum not in job_pb2.ErrorCode.values()[1:]:
        errors.append(ValidationError("alert.error_code_enum", "must be specified"))
    if msg.source_component != sender_id:
        errors.append(ValidationError("alert.source_component", "must equal sender_id"))
    return errors


def validate_bus_packet(msg: Any) -> List[ValidationError]:
    """Validate the exact-v1 envelope and its selected payload."""
    if msg is None:
        return [ValidationError("BusPacket", "must not be nil")]
    errors: List[ValidationError] = []
    if not msg.trace_id:
        errors.append(ValidationError("trace_id", "must not be empty"))
    if not msg.sender_id:
        errors.append(ValidationError("sender_id", "must not be empty"))
    if msg.protocol_version != DEFAULT_PROTOCOL_VERSION:
        errors.append(ValidationError("protocol_version", "must equal 1"))
    if not msg.HasField("created_at"):
        errors.append(ValidationError("created_at", "must not be nil"))
    payload = msg.WhichOneof("payload")
    if payload is None:
        errors.append(ValidationError("payload", "must not be nil"))
    else:
        errors.extend(_validate_payload(msg, payload))
    return errors


def _validate_payload(msg: Any, payload: str) -> List[ValidationError]:
    if payload == "job_request":
        return validate_job_request(msg.job_request)
    if payload == "job_result":
        errors = validate_job_result(msg.job_result)
        if msg.job_result.worker_id != msg.sender_id:
            errors.append(ValidationError("job_result.worker_id", "must equal sender_id"))
        return errors
    if payload == "heartbeat" and msg.heartbeat.worker_id != msg.sender_id:
        return [ValidationError("heartbeat.worker_id", "must equal sender_id")]
    if payload == "alert":
        return validate_alert(msg.alert, msg.sender_id)
    if payload == "job_progress":
        return validate_job_progress(msg.job_progress)
    if payload == "job_cancel":
        return validate_job_cancel(msg.job_cancel)
    if payload == "handshake" and msg.handshake.component_id != msg.sender_id:
        return [ValidationError("handshake.component_id", "must equal sender_id")]
    if payload.startswith("worker_handshake_"):
        return _validate_worker_trust(msg)
    return []


def _validate_worker_trust(msg: Any) -> List[ValidationError]:
    from cap.worker_trust import WorkerHandshakePacketError
    from cap.worker_trust_validate import validate_worker_trust_packet

    try:
        validate_worker_trust_packet(msg)
    except WorkerHandshakePacketError as exc:
        field, _, message = str(exc).partition(": ")
        return [ValidationError(field, message or "is invalid")]
    return []
