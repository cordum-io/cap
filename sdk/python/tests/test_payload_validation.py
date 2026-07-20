"""Semantic validation matrix for progress, cancel, and alert payloads."""

import pytest
from google.protobuf import timestamp_pb2

from cap.pb.cordum.agent.v1 import alert_pb2, buspacket_pb2, job_pb2
from cap.progress import cancel_payload, progress_payload
from cap.validate import validate_bus_packet


def _decode(data: bytes) -> buspacket_pb2.BusPacket:
    return buspacket_pb2.BusPacket.FromString(data)


def _fields(packet: buspacket_pb2.BusPacket):
    return [error.field for error in validate_bus_packet(packet)]


def _envelope() -> buspacket_pb2.BusPacket:
    packet = buspacket_pb2.BusPacket(
        trace_id="trace-alert", sender_id="scheduler-1", protocol_version=1
    )
    packet.created_at.CopyFrom(timestamp_pb2.Timestamp(seconds=1000))
    return packet


def test_progress_builder_self_validates_and_rejects_semantic_mutations() -> None:
    packet = _decode(progress_payload("worker-1", "job-1", "step-1", 50, "half"))
    assert validate_bus_packet(packet) == []
    for field, value, expected in (
        ("job_id", "", "job_progress.job_id"),
        ("percent", -1, "job_progress.percent"),
        ("percent", 101, "job_progress.percent"),
    ):
        mutated = buspacket_pb2.BusPacket()
        mutated.CopyFrom(packet)
        setattr(mutated.job_progress, field, value)
        assert expected in _fields(mutated)


def test_cancel_builder_self_validates_and_rejects_semantic_mutations() -> None:
    packet = _decode(cancel_payload("scheduler-1", "job-1", "stop", "operator-1"))
    assert validate_bus_packet(packet) == []
    for field in ("job_id", "requested_by"):
        mutated = buspacket_pb2.BusPacket()
        mutated.CopyFrom(packet)
        setattr(mutated.job_cancel, field, "")
        assert "job_cancel.{}".format(field) in _fields(mutated)


def test_alert_accepts_legacy_or_complete_structured_shape() -> None:
    legacy = _envelope()
    legacy.alert.CopyFrom(
        alert_pb2.SystemAlert(
            level="WARN", message="queue high", component="scheduler", code="QUEUE"
        )
    )
    assert validate_bus_packet(legacy) == []
    structured = _envelope()
    structured.alert.CopyFrom(
        alert_pb2.SystemAlert(
            message="bad signature",
            severity=alert_pb2.ALERT_SEVERITY_CRITICAL,
            error_code_enum=job_pb2.ERROR_CODE_PROTOCOL_SIGNATURE_INVALID,
            source_component="scheduler-1",
        )
    )
    assert validate_bus_packet(structured) == []


@pytest.mark.parametrize(
    "field,value,expected",
    [
        ("message", "", "alert.message"),
        ("severity", 0, "alert.severity"),
        ("error_code_enum", 0, "alert.error_code_enum"),
        ("source_component", "attacker", "alert.source_component"),
    ],
)
def test_structured_alert_rejects_missing_or_forged_fields(
    field: str, value: object, expected: str
) -> None:
    packet = _envelope()
    packet.alert.CopyFrom(
        alert_pb2.SystemAlert(
            message="bad signature",
            severity=alert_pb2.ALERT_SEVERITY_ERROR,
            error_code_enum=job_pb2.ERROR_CODE_PROTOCOL_SIGNATURE_INVALID,
            source_component="scheduler-1",
        )
    )
    setattr(packet.alert, field, value)
    assert expected in _fields(packet)


def test_public_package_exports_full_payload_validators() -> None:
    import cap

    assert cap.validate_job_progress is not None
    assert cap.validate_job_cancel is not None
    assert cap.validate_alert is not None
