import os
import sys
import unittest

_repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
_sdk_root = os.path.join(_repo_root, "sdk", "python")

# Avoid loading duplicate generated stubs from both /python and sdk/python/cap/pb.
sys.path = [p for p in sys.path if not p.rstrip("/").endswith("python")]
# Ensure the SDK package and generated modules are discoverable from repo root.
sys.path.insert(0, _sdk_root)
sys.path.append(os.path.join(_sdk_root, "cap", "pb"))

from cap.pb.cordum.agent.v1 import buspacket_pb2
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec


FIXTURE_DIR = os.path.join(_repo_root, "spec", "conformance", "fixtures")


def load_packet(name: str) -> buspacket_pb2.BusPacket:
    pkt = buspacket_pb2.BusPacket()
    with open(os.path.join(FIXTURE_DIR, name), "rb") as handle:
        pkt.ParseFromString(handle.read())
    return pkt


class TestConformanceFixtures(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with open(os.path.join(FIXTURE_DIR, "public_key.pem"), "rb") as handle:
            cls.public_key = serialization.load_pem_public_key(handle.read())

    def _verify_signature(self, pkt: buspacket_pb2.BusPacket):
        signature = pkt.signature
        pkt.ClearField("signature")
        unsigned = pkt.SerializeToString(deterministic=True)
        pkt.signature = signature
        self.public_key.verify(signature, unsigned, ec.ECDSA(hashes.SHA256()))

    def _assert_common(self, pkt: buspacket_pb2.BusPacket, trace_id: str, sender_id: str):
        self._verify_signature(pkt)
        self.assertEqual(pkt.trace_id, trace_id)
        self.assertEqual(pkt.sender_id, sender_id)
        self.assertEqual(pkt.protocol_version, 1)
        self.assertEqual(pkt.created_at.seconds, 1704164645)

    def test_job_request_fixture(self):
        pkt = load_packet("buspacket_job_request.bin")
        self._assert_common(pkt, "trace-job-request", "client-1")
        req = pkt.job_request
        self.assertEqual(req.job_id, "job-req-1")
        self.assertEqual(req.topic, "job.tools")
        self.assertEqual(req.priority, 1)
        self.assertEqual(req.context_ptr, "redis://ctx:job-req-1")
        self.assertEqual(req.env["region"], "us-east-1")
        self.assertEqual(req.env["sandbox"], "true")
        self.assertEqual(req.labels["env"], "prod")
        self.assertEqual(req.labels["team"], "platform")
        self.assertEqual(req.meta.idempotency_key, "idem-123")
        self.assertEqual(req.meta.labels["source"], "conformance")
        self.assertEqual(req.compensation.topic, "job.rollback")
        self.assertEqual(req.compensation.labels["rollback"], "true")

    def test_job_result_fixture(self):
        pkt = load_packet("buspacket_job_result.bin")
        self._assert_common(pkt, "trace-job-result", "worker-1")
        res = pkt.job_result
        self.assertEqual(res.job_id, "job-res-1")
        self.assertEqual(res.worker_id, "worker-1")
        self.assertEqual(res.status, 10)
        self.assertEqual(res.error_code, "E_TEMP")
        self.assertEqual(len(res.artifact_ptrs), 2)

    def test_heartbeat_fixture(self):
        pkt = load_packet("buspacket_heartbeat.bin")
        self._assert_common(pkt, "trace-heartbeat", "worker-1")
        hb = pkt.heartbeat
        self.assertEqual(hb.worker_id, "worker-1")
        self.assertEqual(hb.pool, "job.tools")
        self.assertEqual(hb.labels["zone"], "us-east-1a")
        self.assertEqual(hb.progress_pct, 60)

    def test_job_progress_fixture(self):
        pkt = load_packet("buspacket_job_progress.bin")
        self._assert_common(pkt, "trace-progress", "worker-1")
        progress = pkt.job_progress
        self.assertEqual(progress.job_id, "job-prog-1")
        self.assertEqual(progress.percent, 50)
        self.assertEqual(progress.status, 4)

    def test_job_cancel_fixture(self):
        pkt = load_packet("buspacket_job_cancel.bin")
        self._assert_common(pkt, "trace-cancel", "scheduler-1")
        cancel = pkt.job_cancel
        self.assertEqual(cancel.job_id, "job-cancel-1")
        self.assertEqual(cancel.requested_by, "user-7")

    def test_alert_fixture(self):
        pkt = load_packet("buspacket_alert.bin")
        self._assert_common(pkt, "trace-alert", "scheduler-1")
        alert = pkt.alert
        self.assertEqual(alert.level, "WARN")
        self.assertEqual(alert.component, "scheduler")


if __name__ == "__main__":
    unittest.main()
