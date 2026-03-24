"""Opt-in validation helpers for CAP protobuf messages.

Each function accepts a protobuf message object and returns a list of
ValidationError instances.  An empty list means the message is valid.
"""

from dataclasses import dataclass
from typing import Any, List

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2

# Max known JobPriority enum value.
_JOB_PRIORITY_MAX = job_pb2.JOB_PRIORITY_CRITICAL  # 3

# Bounds limits for repeated fields and budget integers.
MAX_REPEATED_FIELD_SIZE = 1000
MAX_BUDGET_TOKENS = 10_000_000_000  # 10 billion


@dataclass(frozen=True)
class ValidationError:
    """A single validation failure."""

    field: str
    message: str


def validate_job_request(msg: Any) -> List[ValidationError]:
    """Validate semantic constraints on a JobRequest."""
    errors: List[ValidationError] = []
    if msg is None:
        errors.append(ValidationError("JobRequest", "must not be nil"))
        return errors
    if not msg.job_id:
        errors.append(ValidationError("job_id", "must not be empty"))
    if not msg.topic:
        errors.append(ValidationError("topic", "must not be empty"))
    if msg.priority < 0 or msg.priority > _JOB_PRIORITY_MAX:
        errors.append(
            ValidationError("priority", f"invalid value {msg.priority}")
        )
    if msg.step_index < 0:
        errors.append(ValidationError("step_index", "must not be negative"))
    # Repeated field bounds — prevent DoS via oversized messages.
    if msg.HasField("meta"):
        m = msg.meta
        if len(m.risk_tags) > MAX_REPEATED_FIELD_SIZE:
            errors.append(
                ValidationError("meta.risk_tags", f"exceeds max size ({MAX_REPEATED_FIELD_SIZE})")
            )
        if len(m.requires) > MAX_REPEATED_FIELD_SIZE:
            errors.append(
                ValidationError("meta.requires", f"exceeds max size ({MAX_REPEATED_FIELD_SIZE})")
            )
    if len(msg.labels) > MAX_REPEATED_FIELD_SIZE:
        errors.append(
            ValidationError("labels", f"exceeds max size ({MAX_REPEATED_FIELD_SIZE})")
        )

    if msg.HasField("budget"):
        b = msg.budget
        if b.max_input_tokens < 0:
            errors.append(
                ValidationError("budget.max_input_tokens", "must not be negative")
            )
        if b.max_input_tokens > MAX_BUDGET_TOKENS:
            errors.append(
                ValidationError("budget.max_input_tokens", f"exceeds max ({MAX_BUDGET_TOKENS})")
            )
        if b.max_output_tokens < 0:
            errors.append(
                ValidationError("budget.max_output_tokens", "must not be negative")
            )
        if b.max_output_tokens > MAX_BUDGET_TOKENS:
            errors.append(
                ValidationError("budget.max_output_tokens", f"exceeds max ({MAX_BUDGET_TOKENS})")
            )
        if b.max_total_tokens < 0:
            errors.append(
                ValidationError("budget.max_total_tokens", "must not be negative")
            )
        if b.max_total_tokens > MAX_BUDGET_TOKENS:
            errors.append(
                ValidationError("budget.max_total_tokens", f"exceeds max ({MAX_BUDGET_TOKENS})")
            )
        if b.deadline_ms < 0:
            errors.append(
                ValidationError("budget.deadline_ms", "must not be negative")
            )
    return errors


def validate_job_result(msg: Any) -> List[ValidationError]:
    """Validate semantic constraints on a JobResult."""
    errors: List[ValidationError] = []
    if msg is None:
        errors.append(ValidationError("JobResult", "must not be nil"))
        return errors
    if not msg.job_id:
        errors.append(ValidationError("job_id", "must not be empty"))
    if msg.status == job_pb2.JOB_STATUS_UNSPECIFIED:
        errors.append(ValidationError("status", "must not be UNSPECIFIED"))
    if not msg.worker_id:
        errors.append(ValidationError("worker_id", "must not be empty"))
    if msg.execution_ms < 0:
        errors.append(
            ValidationError("execution_ms", "must not be negative")
        )
    return errors


def validate_bus_packet(msg: Any) -> List[ValidationError]:
    """Validate semantic constraints on a BusPacket.

    If the packet carries a JobRequest or JobResult payload,
    it is also validated.
    """
    errors: List[ValidationError] = []
    if msg is None:
        errors.append(ValidationError("BusPacket", "must not be nil"))
        return errors
    if not msg.trace_id:
        errors.append(ValidationError("trace_id", "must not be empty"))
    if not msg.sender_id:
        errors.append(ValidationError("sender_id", "must not be empty"))
    if msg.protocol_version <= 0:
        errors.append(ValidationError("protocol_version", "must be > 0"))
    if not msg.HasField("created_at"):
        errors.append(ValidationError("created_at", "must not be nil"))
    payload_field = msg.WhichOneof("payload")
    if payload_field is None:
        errors.append(ValidationError("payload", "must not be nil"))
    elif payload_field == "job_request":
        errors.extend(validate_job_request(msg.job_request))
    elif payload_field == "job_result":
        errors.extend(validate_job_result(msg.job_result))
    return errors
