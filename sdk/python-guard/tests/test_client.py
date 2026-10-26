"""Tests for cordum_guard.client — HTTP mocking with respx."""

import pytest
import httpx
import respx

from cordum_guard.client import CordumClient
from cordum_guard.exceptions import (
    CordumAuthError,
    CordumConnectionError,
    CordumError,
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
