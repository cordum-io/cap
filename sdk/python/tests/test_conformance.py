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

from cap.pb.cordum.agent.v1 import (
    alert_pb2,
    buspacket_pb2,
    handshake_pb2,
    job_pb2,
    policy_pb2,
    safety_pb2,
)
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
        self.assertEqual(hb.auth_token, "attest-worker-1")

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

    def test_handshake_fixture(self):
        pkt = load_packet("buspacket_handshake.bin")
        self._assert_common(pkt, "trace-handshake", "worker-1")
        hs = pkt.handshake
        self.assertEqual(hs.component_id, "worker-1")
        self.assertEqual(hs.role, handshake_pb2.COMPONENT_ROLE_WORKER)
        self.assertEqual(list(hs.supported_versions), [1])
        self.assertTrue(hs.capabilities["signatures"])
        self.assertTrue(hs.capabilities["progress"])
        self.assertTrue(hs.capabilities["cancel"])
        self.assertFalse(hs.capabilities["compensation"])
        self.assertEqual(hs.sdk_version, "2.0.19")
        self.assertEqual(list(hs.ready_topics), ["job.tools", "job.tools.bulk"])

    def test_alert_enhanced_fixture(self):
        pkt = load_packet("buspacket_alert_enhanced.bin")
        self._assert_common(pkt, "trace-alert-enhanced", "scheduler-1")
        alert = pkt.alert
        # Legacy fields
        self.assertEqual(alert.level, "CRITICAL")
        self.assertEqual(alert.component, "scheduler")
        self.assertEqual(alert.code, "SIGNATURE_INVALID")
        # Enhanced fields
        self.assertEqual(alert.severity, alert_pb2.ALERT_SEVERITY_CRITICAL)
        self.assertEqual(alert.error_code_enum, job_pb2.ERROR_CODE_PROTOCOL_SIGNATURE_INVALID)
        self.assertEqual(alert.source_component, "scheduler-1")
        self.assertEqual(alert.details["sender"], "worker-bad")
        self.assertEqual(alert.details["subject"], "sys.job.result")
        self.assertEqual(alert.trace_id, "trace-offending-packet")


class TestPolicyShapesConformance(unittest.TestCase):
    """Cross-SDK decode parity for the Policy Studio shapes (epic-d9a6c0a1).

    These fixtures are NOT BusPacket-wrapped — they are standalone proto-marshalled
    bytes, since Rule/Decision/Bundle live in higher-layer APIs (HTTP/gRPC unary,
    audit-chain), not on the cap bus. Mirrors sdk/go/conformance_test.go's
    TestPolicyShapesConformance.
    """

    @staticmethod
    def _load(name: str) -> bytes:
        with open(os.path.join(FIXTURE_DIR, name), "rb") as handle:
            return handle.read()

    def test_policy_rule_fixture(self):
        rule = policy_pb2.Rule()
        rule.ParseFromString(self._load("policy_rule.bin"))
        self.assertEqual(rule.id, "rule-input-secrets")
        self.assertEqual(rule.name, "Block secret leaks in input")
        self.assertEqual(rule.type, policy_pb2.RULE_TYPE_INPUT)
        self.assertEqual(rule.scope.kind, policy_pb2.RULE_SCOPE_KIND_TENANT)
        self.assertEqual(rule.scope.value, "acme")
        self.assertEqual(rule.status, policy_pb2.RULE_STATUS_PUBLISHED)
        self.assertEqual(rule.version, "v1")
        self.assertEqual(rule.audit.created_by, "yaron@cordum.io")
        self.assertEqual(rule.description, "Conformance fixture for RuleType=INPUT.")
        # Struct payloads round-trip lossless.
        self.assertEqual(rule.decide.fields["decision"].string_value, "deny")
        self.assertEqual(rule.decide.fields["reason"].string_value, "secret_leak")

    def test_policy_decision_fixture(self):
        decision = policy_pb2.Decision()
        decision.ParseFromString(self._load("policy_decision.bin"))
        self.assertEqual(decision.source, policy_pb2.DECISION_SOURCE_JOB)
        self.assertEqual(decision.rule_id, "rule-input-secrets")
        self.assertEqual(decision.bundle_id, "bundle-acme-default")
        self.assertEqual(decision.bundle_version, "v12")
        # Exercises the appended DecisionType wire value 6 — proves the
        # safety_pb2 descriptor extension decodes correctly via Python.
        self.assertEqual(decision.type, safety_pb2.DECISION_TYPE_QUARANTINE)
        self.assertEqual(decision.type, 6)
        self.assertEqual(len(decision.trace), 1)
        self.assertEqual(decision.trace[0].rule_id, "rule-input-secrets")
        self.assertEqual(decision.trace[0].decision_type, safety_pb2.DECISION_TYPE_QUARANTINE)
        self.assertEqual(decision.trace[0].reason, "secret_leak")
        self.assertEqual(decision.input_ref, "blob://acme/input/r1")
        self.assertEqual(decision.output_ref, "blob://acme/output/r1")
        self.assertEqual(decision.audit_hash, "0xabcdef")

    def test_policy_bundle_fixture(self):
        bundle = policy_pb2.Bundle()
        bundle.ParseFromString(self._load("policy_bundle.bin"))
        self.assertEqual(bundle.id, "bundle-edge-prod")
        self.assertEqual(bundle.name, "Production edge bundle")
        self.assertEqual(list(bundle.rule_ids), ["rule-edge-fs-write-deny"])
        self.assertEqual(bundle.scope_binding.kind, policy_pb2.RULE_SCOPE_KIND_EDGE_FLEET)
        self.assertEqual(bundle.scope_binding.value, "fleet-prod")
        self.assertEqual(len(bundle.versions), 1)
        self.assertEqual(bundle.versions[0].version, "v3")
        self.assertEqual(bundle.versions[0].audit_hash, "0xdeadbeef")
        # Exercises the new EdgeMode enum.
        self.assertEqual(bundle.metadata.edge_mode, policy_pb2.EDGE_MODE_ENTERPRISE_STRICT)


if __name__ == "__main__":
    unittest.main()
