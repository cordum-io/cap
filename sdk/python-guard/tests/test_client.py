"""Tests for cordum_guard.client — HTTP mocking with respx."""

import logging
import ssl
from unittest.mock import patch

import certifi
import pytest
import httpx
import respx

from cordum_guard.client import CordumClient
from cordum_guard.exceptions import (
    CordumAuthError,
    CordumBlockedError,
    CordumConnectionError,
    CordumError,
    CordumTimeoutError,
)
from cordum_guard.types import Decision


GATEWAY = "http://localhost:8081"
API_KEY = "test-key-123"


@pytest.fixture()
def client():
    c = CordumClient(GATEWAY, api_key=API_KEY)
    yield c
    c.close()


class TestSubmitJob:
    @respx.mock
    def test_submit_job_success(self, client: CordumClient):
        respx.post(f"{GATEWAY}/api/v1/jobs").mock(
            return_value=httpx.Response(
                200, json={"job_id": "j-1", "trace_id": "t-1"}
            )
        )
        resp = client.submit_job("do something", topic="job.test")
        assert resp.job_id == "j-1"
        assert resp.trace_id == "t-1"

    @respx.mock
    def test_submit_job_with_options(self, client: CordumClient):
        route = respx.post(f"{GATEWAY}/api/v1/jobs").mock(
            return_value=httpx.Response(200, json={"job_id": "j-2"})
        )
        client.submit_job(
            "transfer",
            topic="job.finance",
            capability="bank_transfer",
            risk_tags=["write"],
            labels={"env": "prod"},
            priority="high",
        )
        body = route.calls[0].request.content
        import json

        parsed = json.loads(body)
        assert parsed["topic"] == "job.finance"
        assert parsed["capability"] == "bank_transfer"
        assert parsed["risk_tags"] == ["write"]
        assert parsed["labels"] == {"env": "prod"}
        assert parsed["priority"] == "high"


class TestGetJob:
    @respx.mock
    def test_get_job(self, client: CordumClient):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-1").mock(
            return_value=httpx.Response(
                200, json={"job_id": "j-1", "state": "succeeded", "topic": "job.test"}
            )
        )
        status = client.get_job("j-1")
        assert status.job_id == "j-1"
        assert status.state == "succeeded"

    @respx.mock
    def test_get_job_reads_approval_fields(self, client: CordumClient):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-1").mock(return_value=httpx.Response(200, json={
            "id": "j-1", "state": "DENIED", "approval_status": "rejected",
            "approval_decision": "reject", "approval_by": "bob", "approval_reason": "no",
        }))
        status = client.get_job("j-1")
        assert (status.approval_status, status.approval_decision) == ("rejected", "reject")
        assert (status.approval_by, status.approval_reason) == ("bob", "no")


class TestCancelJob:
    @respx.mock
    def test_cancel_job(self, client: CordumClient):
        respx.post(f"{GATEWAY}/api/v1/jobs/j-1/cancel").mock(
            return_value=httpx.Response(200, json={})
        )
        client.cancel_job("j-1")  # should not raise


class TestEvaluatePolicy:
    @respx.mock
    def test_allow(self, client: CordumClient):
        respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(
                200, json={"decision": "allow", "reason": ""}
            )
        )
        d = client.evaluate_policy(topic="job.guard", capability="read_data")
        assert d.decision == Decision.ALLOW

    @respx.mock
    def test_deny(self, client: CordumClient):
        respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(
                200,
                json={
                    "decision": "deny",
                    "reason": "blocked by policy",
                    "matched_rules": ["rule-1"],
                },
            )
        )
        d = client.evaluate_policy(topic="job.guard")
        assert d.decision == Decision.DENY
        assert d.reason == "blocked by policy"
        assert d.matched_rules == ["rule-1"]

    @respx.mock
    def test_throttle(self, client: CordumClient):
        respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(
                200,
                json={"decision": "throttle", "throttle_duration_seconds": 3.5},
            )
        )
        d = client.evaluate_policy(topic="job.guard")
        assert d.decision == Decision.THROTTLE
        assert d.throttle_duration_seconds == 3.5

    @respx.mock
    @pytest.mark.parametrize(
        "raw",
        ["unknown_value", "", "DECISION_TYPE_UNSPECIFIED", "DECISION_TYPE_QUARANTINE"],
    )
    def test_unknown_decision_fails_closed(self, client: CordumClient, raw: str):
        respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(200, json={"decision": raw})
        )
        d = client.evaluate_policy(topic="job.guard")
        assert d.decision == Decision.DENY
        assert "failing closed" in d.reason

    @respx.mock
    def test_empty_200_body_fails_closed(self, client: CordumClient):
        # e.g. a proxy answering 200 with no body: no verdict, no permission.
        respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(200)
        )
        d = client.evaluate_policy(topic="job.guard")
        assert d.decision == Decision.DENY
        assert "missing gateway decision" in d.reason

    @respx.mock
    @pytest.mark.parametrize(
        ("raw", "expected"),
        [
            # The gateway serialises the kernel's PolicyCheckResponse with
            # protojson, so verdicts arrive as DecisionType enum names.
            ("DECISION_TYPE_ALLOW", Decision.ALLOW),
            ("DECISION_TYPE_DENY", Decision.DENY),
            ("DECISION_TYPE_REQUIRE_HUMAN", Decision.REQUIRE_APPROVAL),
            ("DECISION_TYPE_THROTTLE", Decision.THROTTLE),
            ("DECISION_TYPE_ALLOW_WITH_CONSTRAINTS", Decision.ALLOW_WITH_CONSTRAINTS),
            ("require_human", Decision.REQUIRE_APPROVAL),
            ("require-approval", Decision.REQUIRE_APPROVAL),
            ("DENY", Decision.DENY),
        ],
    )
    def test_gateway_decision_names(self, client: CordumClient, raw, expected):
        respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(200, json={"decision": raw})
        )
        assert client.evaluate_policy(topic="job.guard").decision == expected

    @respx.mock
    def test_require_human_response_fields(self, client: CordumClient):
        # Exact shape of the gateway's protojson (camelCase) response.
        respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(
                200,
                json={
                    "decision": "DECISION_TYPE_REQUIRE_HUMAN",
                    "reason": "Transfers over $10,000 require human approval.",
                    "redactedContextPtr": "",
                    "policySnapshot": "snap-1",
                    "ruleId": "financial-approval",
                    "constraints": None,
                    "approvalRequired": True,
                    "approvalRef": "job-42",
                    "remediations": [],
                    "redactedContextRef": None,
                    "identity": None,
                },
            )
        )
        d = client.evaluate_policy(topic="job.guard", job_id="job-42")
        assert d.decision == Decision.REQUIRE_APPROVAL
        assert d.approval_required is True
        assert d.approval_ref == "job-42"
        assert d.rule_id == "financial-approval"
        assert d.policy_snapshot == "snap-1"
        assert d.constraints == {}

    @respx.mock
    def test_labels_sent_at_top_level(self, client: CordumClient):
        # The Safety Kernel matches rules on the request's top-level labels;
        # meta.labels is kept only for older consumers.
        route = respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(200, json={"decision": "allow"})
        )
        client.evaluate_policy(
            topic="job.guard", labels={"policy": "ops", "arg.amount": "15000"}
        )
        import json

        body = json.loads(route.calls[0].request.content)
        assert body["labels"] == {"policy": "ops", "arg.amount": "15000"}
        assert body["meta"]["labels"] == body["labels"]

    @respx.mock
    def test_request_body_includes_meta(self, client: CordumClient):
        route = respx.post(f"{GATEWAY}/api/v1/policy/evaluate").mock(
            return_value=httpx.Response(200, json={"decision": "allow"})
        )
        client.evaluate_policy(
            topic="job.guard",
            capability="write_db",
            risk_tags=["write"],
            labels={"policy": "ops"},
        )
        import json

        body = json.loads(route.calls[0].request.content)
        assert body["topic"] == "job.guard"
        assert body["meta"]["capability"] == "write_db"
        assert body["meta"]["risk_tags"] == ["write"]
        assert body["meta"]["labels"] == {"policy": "ops"}


class TestApprovals:
    @respx.mock
    def test_list_approvals(self, client: CordumClient):
        respx.get(f"{GATEWAY}/api/v1/approvals").mock(
            return_value=httpx.Response(
                200, json=[{"job_id": "j-1", "topic": "job.ops"}]
            )
        )
        items = client.list_approvals()
        assert len(items) == 1
        assert items[0].job_id == "j-1"

    @respx.mock
    def test_approve_job(self, client: CordumClient):
        respx.post(f"{GATEWAY}/api/v1/approvals/j-1/approve").mock(
            return_value=httpx.Response(200, json={})
        )
        client.approve_job("j-1")

    @respx.mock
    def test_reject_job(self, client: CordumClient):
        respx.post(f"{GATEWAY}/api/v1/approvals/j-1/reject").mock(
            return_value=httpx.Response(200, json={})
        )
        client.reject_job("j-1")


def _job(**fields):
    return httpx.Response(200, json={"id": "j-w", "topic": "job.guard", **fields})


PENDING = {"state": "APPROVAL_REQUIRED", "approval_status": "pending"}
APPROVED = {
    "state": "PENDING", "approval_status": "approved",
    "approval_decision": "approve", "approval_by": "alice",
}


class TestWaitForApproval:
    """Bodies follow GET /api/v1/jobs/{id} for an approval job (cordum gateway
    handleGetJob)."""

    @respx.mock
    def test_returns_once_approved(self, client: CordumClient):
        route = respx.get(f"{GATEWAY}/api/v1/jobs/j-w").mock(
            side_effect=[_job(**PENDING), _job(**APPROVED)]
        )
        status = client.wait_for_approval("j-w", timeout=5, poll_interval=0)
        assert status.approval_by == "alice"
        assert route.call_count == 2

    @respx.mock
    def test_approval_survives_the_job_being_requeued(self, client: CordumClient):
        # An approved job whose dispatch fails is re-checked and put back in
        # the approval queue: approval_status says pending again, but
        # approval_decision still records the human's approve.
        respx.get(f"{GATEWAY}/api/v1/jobs/j-w").mock(return_value=_job(
            state="APPROVAL_REQUIRED", approval_status="pending",
            approval_decision="approve", approval_by="alice",
        ))
        assert client.wait_for_approval("j-w", timeout=0).approval_decision == "approve"

    @respx.mock
    def test_approved_job_that_finished_counts(self, client: CordumClient):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-w").mock(
            return_value=_job(**{**APPROVED, "state": "SUCCEEDED"})
        )
        assert client.wait_for_approval("j-w", timeout=0).state == "SUCCEEDED"

    @pytest.mark.parametrize(
        "body, reason",
        [
            ({"state": "DENIED", "approval_status": "rejected", "approval_decision": "reject",
              "approval_by": "bob", "approval_reason": "too large"}, "approval rejected: too large"),
            ({"state": "TIMEOUT", "approval_status": "pending"}, "approval expired"),
            ({"state": "DENIED", "approval_status": "invalidated",
              "approval_decision": "invalidate"}, "approval invalidated"),
            ({**APPROVED, "state": "DENIED"}, "approval job ended DENIED"),
            ({"state": "CANCELLED", "approval_status": "pending"}, "approval job ended CANCELLED"),
            ({"state": "FAILED"}, "approval job ended FAILED"),
            ({"state": "SUCCEEDED"}, "succeeded without an approval"),
        ],
        ids=["rejected", "expired", "invalidated", "denied-after-approval", "cancelled",
             "failed", "succeeded-unapproved"],
    )
    @respx.mock
    def test_blocks_unless_approved(self, client: CordumClient, body, reason):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-w").mock(return_value=_job(**body))
        with pytest.raises(CordumBlockedError, match=reason) as exc_info:
            client.wait_for_approval("j-w", timeout=5, poll_interval=0)
        assert exc_info.value.decision.decision == Decision.DENY

    @pytest.mark.parametrize(
        "body",
        [PENDING, {**APPROVED, "approval_by": ""}],
        ids=["pending", "approved-without-approver"],
    )
    @respx.mock
    def test_times_out_while_unresolved(self, client: CordumClient, body):
        # The gateway derives "approved" from the job state for legacy
        # records; without an approver it is not trusted.
        respx.get(f"{GATEWAY}/api/v1/jobs/j-w").mock(return_value=_job(**body))
        with pytest.raises(CordumTimeoutError):
            client.wait_for_approval("j-w", timeout=0)


class TestErrorHandling:
    @respx.mock
    def test_401_raises_auth_error(self, client: CordumClient):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-1").mock(
            return_value=httpx.Response(401, text="Unauthorized")
        )
        with pytest.raises(CordumAuthError):
            client.get_job("j-1")

    @respx.mock
    def test_403_raises_auth_error(self, client: CordumClient):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-1").mock(
            return_value=httpx.Response(403, text="Forbidden")
        )
        with pytest.raises(CordumAuthError):
            client.get_job("j-1")

    @respx.mock
    def test_500_raises_cordum_error(self, client: CordumClient):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-1").mock(
            return_value=httpx.Response(500, text="Internal Server Error")
        )
        with pytest.raises(CordumError):
            client.get_job("j-1")

    def test_connection_error(self):
        c = CordumClient("http://127.0.0.1:1", api_key="k", timeout=0.5)
        with pytest.raises(CordumConnectionError):
            c.get_job("j-1")
        c.close()


class TestHeaders:
    @respx.mock
    def test_api_key_and_tenant_headers(self):
        route = respx.get(f"{GATEWAY}/api/v1/jobs/j-1").mock(
            return_value=httpx.Response(
                200, json={"job_id": "j-1", "state": "running"}
            )
        )
        c = CordumClient(GATEWAY, api_key="my-key", tenant_id="acme")
        c.get_job("j-1")
        headers = route.calls[0].request.headers
        assert headers["x-api-key"] == "my-key"
        assert headers["x-tenant-id"] == "acme"
        c.close()


def _one_ca_bundle(tmp_path) -> str:
    """A PEM bundle holding exactly one CA certificate."""
    pem = open(certifi.where(), encoding="ascii").read()
    end = "-----END CERTIFICATE-----"
    first = pem[pem.index("-----BEGIN CERTIFICATE-----"):pem.index(end) + len(end)]
    path = tmp_path / "ca.pem"
    path.write_text(first + "\n", encoding="ascii")
    return str(path)


class TestTLS:
    @pytest.fixture(autouse=True)
    def _no_env_ca(self, monkeypatch):
        monkeypatch.delenv("CORDUM_CA_CERT", raising=False)

    def _verify_arg(self, **kwargs):
        with patch("cordum_guard.client.httpx.Client") as http_client:
            CordumClient(GATEWAY, api_key=API_KEY, **kwargs)
        return http_client.call_args.kwargs["verify"]

    def test_verifies_by_default(self):
        assert self._verify_arg() is True

    def test_ca_cert_path_is_passed_through(self, tmp_path):
        ctx = self._verify_arg(ca_cert=_one_ca_bundle(tmp_path))
        assert isinstance(ctx, ssl.SSLContext)
        assert ctx.verify_mode == ssl.CERT_REQUIRED and ctx.check_hostname
        # Only the bundle is trusted, not the default CAs.
        assert len(ctx.get_ca_certs()) == 1

    def test_env_default_applies(self, tmp_path, monkeypatch):
        monkeypatch.setenv("CORDUM_CA_CERT", _one_ca_bundle(tmp_path))
        ctx = self._verify_arg()
        assert isinstance(ctx, ssl.SSLContext)
        assert len(ctx.get_ca_certs()) == 1

    def test_missing_bundle_fails_at_construction(self, tmp_path):
        with pytest.raises(FileNotFoundError):
            CordumClient(GATEWAY, api_key=API_KEY, ca_cert=str(tmp_path / "missing.pem"))

    def test_ssl_context_is_passed_as_is(self, tmp_path, monkeypatch):
        monkeypatch.setenv("CORDUM_CA_CERT", str(tmp_path / "ignored.pem"))
        ctx = ssl.create_default_context()
        assert self._verify_arg(verify=ctx) is ctx

    def test_verify_false_disables_and_warns(self, caplog, tmp_path, monkeypatch):
        monkeypatch.setenv("CORDUM_CA_CERT", str(tmp_path / "ignored.pem"))
        with caplog.at_level(logging.WARNING, logger="cordum_guard"):
            assert self._verify_arg(verify=False) is False
        assert "TLS certificate verification is disabled" in caplog.text

    def test_verify_true_does_not_warn(self, caplog):
        with caplog.at_level(logging.WARNING, logger="cordum_guard"):
            self._verify_arg(verify=True)
        assert caplog.text == ""


class TestContextManager:
    @respx.mock
    def test_context_manager(self):
        respx.get(f"{GATEWAY}/api/v1/jobs/j-1").mock(
            return_value=httpx.Response(
                200, json={"job_id": "j-1", "state": "running"}
            )
        )
        with CordumClient(GATEWAY, api_key=API_KEY) as c:
            s = c.get_job("j-1")
            assert s.state == "running"
