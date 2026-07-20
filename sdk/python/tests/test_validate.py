import os
import sys
import unittest

_repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
_sdk_root = os.path.join(_repo_root, "sdk", "python")

sys.path = [p for p in sys.path if not p.rstrip("/").endswith("python")]
sys.path.insert(0, _sdk_root)
sys.path.append(os.path.join(_sdk_root, "cap", "pb"))

from google.protobuf import timestamp_pb2

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.validate import (
    ValidationError,
    validate_bus_packet,
    validate_job_request,
    validate_job_result,
)


def _valid_job_request():
    return job_pb2.JobRequest(job_id="job-1", topic="job.tools")


def _valid_job_result():
    return job_pb2.JobResult(
        job_id="job-1",
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        worker_id="worker-1",
        execution_ms=100,
    )


def _valid_bus_packet(payload=None):
    req = payload or _valid_job_request()
    pkt = buspacket_pb2.BusPacket(
        trace_id="trace-1",
        sender_id="sender-1",
        protocol_version=1,
    )
    pkt.created_at.CopyFrom(timestamp_pb2.Timestamp(seconds=1000))
    pkt.job_request.CopyFrom(req)
    return pkt


class TestValidateJobRequest(unittest.TestCase):
    def test_valid_minimal(self):
        self.assertEqual(validate_job_request(_valid_job_request()), [])

    def test_valid_with_budget(self):
        req = _valid_job_request()
        req.budget.CopyFrom(
            job_pb2.Budget(
                max_input_tokens=1000,
                max_output_tokens=500,
                max_total_tokens=1500,
                deadline_ms=30000,
            )
        )
        self.assertEqual(validate_job_request(req), [])

    def test_valid_with_all_fields(self):
        req = _valid_job_request()
        req.priority = job_pb2.JOB_PRIORITY_CRITICAL
        req.context_ptr = "redis://ctx:1"
        req.adapter_id = "adapter-1"
        req.parent_job_id = "parent-1"
        req.workflow_id = "wf-1"
        req.step_index = 2
        req.memory_id = "mem-1"
        req.tenant_id = "tenant-1"
        req.principal_id = "user-1"
        self.assertEqual(validate_job_request(req), [])

    def test_none(self):
        errs = validate_job_request(None)
        self.assertEqual(len(errs), 1)
        self.assertEqual(errs[0].field, "JobRequest")

    def test_empty_job_id(self):
        req = _valid_job_request()
        req.job_id = ""
        errs = validate_job_request(req)
        self._assert_has_field_error(errs, "job_id")

    def test_empty_topic(self):
        req = _valid_job_request()
        req.topic = ""
        errs = validate_job_request(req)
        self._assert_has_field_error(errs, "topic")

    def test_negative_step_index(self):
        req = _valid_job_request()
        req.step_index = -1
        errs = validate_job_request(req)
        self._assert_has_field_error(errs, "step_index")

    def test_negative_budget_max_input_tokens(self):
        req = _valid_job_request()
        req.budget.CopyFrom(job_pb2.Budget(max_input_tokens=-1))
        errs = validate_job_request(req)
        self._assert_has_field_error(errs, "budget.max_input_tokens")

    def test_negative_budget_max_output_tokens(self):
        req = _valid_job_request()
        req.budget.CopyFrom(job_pb2.Budget(max_output_tokens=-1))
        errs = validate_job_request(req)
        self._assert_has_field_error(errs, "budget.max_output_tokens")

    def test_negative_budget_max_total_tokens(self):
        req = _valid_job_request()
        req.budget.CopyFrom(job_pb2.Budget(max_total_tokens=-1))
        errs = validate_job_request(req)
        self._assert_has_field_error(errs, "budget.max_total_tokens")

    def test_negative_budget_deadline_ms(self):
        req = _valid_job_request()
        req.budget.CopyFrom(job_pb2.Budget(deadline_ms=-1))
        errs = validate_job_request(req)
        self._assert_has_field_error(errs, "budget.deadline_ms")

    def test_zero_value_struct(self):
        errs = validate_job_request(job_pb2.JobRequest())
        self._assert_has_field_error(errs, "job_id")

    def _assert_has_field_error(self, errs, field):
        fields = [e.field for e in errs]
        self.assertIn(field, fields, f"Expected error for {field}, got: {errs}")


class TestValidateJobResult(unittest.TestCase):
    def test_valid_minimal(self):
        self.assertEqual(validate_job_result(_valid_job_result()), [])

    def test_valid_with_all_fields(self):
        res = _valid_job_result()
        res.result_ptr = "redis://res:1"
        res.error_code = "E_TEMP"
        res.error_message = "temporary failure"
        res.artifact_ptrs.extend(["redis://art:1", "redis://art:2"])
        self.assertEqual(validate_job_result(res), [])

    def test_none(self):
        errs = validate_job_result(None)
        self.assertEqual(len(errs), 1)
        self.assertEqual(errs[0].field, "JobResult")

    def test_empty_job_id(self):
        res = _valid_job_result()
        res.job_id = ""
        errs = validate_job_result(res)
        self._assert_has_field_error(errs, "job_id")

    def test_unspecified_status(self):
        res = _valid_job_result()
        res.status = job_pb2.JOB_STATUS_UNSPECIFIED
        errs = validate_job_result(res)
        self._assert_has_field_error(errs, "status")

    def test_empty_worker_id(self):
        res = _valid_job_result()
        res.worker_id = ""
        errs = validate_job_result(res)
        self._assert_has_field_error(errs, "worker_id")

    def test_negative_execution_ms(self):
        res = _valid_job_result()
        res.execution_ms = -1
        errs = validate_job_result(res)
        self._assert_has_field_error(errs, "execution_ms")

    def test_zero_value_struct(self):
        errs = validate_job_result(job_pb2.JobResult())
        self._assert_has_field_error(errs, "job_id")

    def _assert_has_field_error(self, errs, field):
        fields = [e.field for e in errs]
        self.assertIn(field, fields, f"Expected error for {field}, got: {errs}")


class TestValidateBusPacket(unittest.TestCase):
    def test_valid_with_job_request(self):
        self.assertEqual(validate_bus_packet(_valid_bus_packet()), [])

    def test_valid_with_job_result(self):
        pkt = buspacket_pb2.BusPacket(
            trace_id="trace-1",
            sender_id="worker-1",
            protocol_version=1,
        )
        pkt.created_at.CopyFrom(timestamp_pb2.Timestamp(seconds=1000))
        pkt.job_result.CopyFrom(_valid_job_result())
        self.assertEqual(validate_bus_packet(pkt), [])

    def test_valid_with_heartbeat(self):
        from cap.pb.cordum.agent.v1 import heartbeat_pb2

        pkt = buspacket_pb2.BusPacket(
            trace_id="trace-1",
            sender_id="w-1",
            protocol_version=1,
        )
        pkt.created_at.CopyFrom(timestamp_pb2.Timestamp(seconds=1000))
        pkt.heartbeat.CopyFrom(
            heartbeat_pb2.Heartbeat(worker_id="w-1", pool="p-1")
        )
        self.assertEqual(validate_bus_packet(pkt), [])

    def test_worker_payload_identity_must_equal_sender(self):
        from cap.pb.cordum.agent.v1 import heartbeat_pb2

        packets = []
        result = _valid_bus_packet()
        result.ClearField("job_request")
        result.job_result.CopyFrom(_valid_job_result())
        packets.append((result, "job_result.worker_id"))
        heartbeat = _valid_bus_packet()
        heartbeat.ClearField("job_request")
        heartbeat.heartbeat.CopyFrom(heartbeat_pb2.Heartbeat(worker_id="worker-1"))
        packets.append((heartbeat, "heartbeat.worker_id"))
        for packet, field in packets:
            self.assertIn(field, [error.field for error in validate_bus_packet(packet)])

    def test_none(self):
        errs = validate_bus_packet(None)
        self.assertEqual(len(errs), 1)
        self.assertEqual(errs[0].field, "BusPacket")

    def test_empty_trace_id(self):
        pkt = _valid_bus_packet()
        pkt.trace_id = ""
        errs = validate_bus_packet(pkt)
        self._assert_has_field_error(errs, "trace_id")

    def test_empty_sender_id(self):
        pkt = _valid_bus_packet()
        pkt.sender_id = ""
        errs = validate_bus_packet(pkt)
        self._assert_has_field_error(errs, "sender_id")

    def test_protocol_version_zero(self):
        pkt = _valid_bus_packet()
        pkt.protocol_version = 0
        errs = validate_bus_packet(pkt)
        self._assert_has_field_error(errs, "protocol_version")

    def test_nil_created_at(self):
        pkt = buspacket_pb2.BusPacket(
            trace_id="trace-1",
            sender_id="sender-1",
            protocol_version=1,
        )
        pkt.job_request.CopyFrom(_valid_job_request())
        errs = validate_bus_packet(pkt)
        self._assert_has_field_error(errs, "created_at")

    def test_nil_payload(self):
        pkt = buspacket_pb2.BusPacket(
            trace_id="trace-1",
            sender_id="sender-1",
            protocol_version=1,
        )
        pkt.created_at.CopyFrom(timestamp_pb2.Timestamp(seconds=1000))
        errs = validate_bus_packet(pkt)
        self._assert_has_field_error(errs, "payload")

    def test_delegates_to_job_request_validation(self):
        bad = job_pb2.JobRequest(job_id="", topic="t")
        pkt = _valid_bus_packet(bad)
        errs = validate_bus_packet(pkt)
        self._assert_has_field_error(errs, "job_id")

    def test_delegates_to_job_result_validation(self):
        bad = job_pb2.JobResult(
            job_id="", status=job_pb2.JOB_STATUS_SUCCEEDED, worker_id="w"
        )
        pkt = buspacket_pb2.BusPacket(
            trace_id="trace-1",
            sender_id="sender-1",
            protocol_version=1,
        )
        pkt.created_at.CopyFrom(timestamp_pb2.Timestamp(seconds=1000))
        pkt.job_result.CopyFrom(bad)
        errs = validate_bus_packet(pkt)
        self._assert_has_field_error(errs, "job_id")

    def test_zero_value_struct(self):
        errs = validate_bus_packet(buspacket_pb2.BusPacket())
        self._assert_has_field_error(errs, "trace_id")

    def _assert_has_field_error(self, errs, field):
        fields = [e.field for e in errs]
        self.assertIn(field, fields, f"Expected error for {field}, got: {errs}")


if __name__ == "__main__":
    unittest.main()
