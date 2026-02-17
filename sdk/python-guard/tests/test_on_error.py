"""Tests for on_error failure mode configuration."""

from unittest.mock import MagicMock, patch

import pytest

from cordum_guard import CordumClient, Decision
from cordum_guard.exceptions import CordumAuthError, CordumConnectionError
from cordum_guard.types import SafetyDecision


def _make_client(on_error="closed"):
    """Create a CordumClient with mocked HTTP transport."""
    client = CordumClient.__new__(CordumClient)
    client.base_url = "http://test:8081"
    client._http = MagicMock()
    client._on_error = on_error
    client._cache = None
    return client


def _mock_connection_error(client):
    """Configure the mock to raise CordumConnectionError."""
    import httpx
    client._http.request.side_effect = httpx.ConnectError("connection refused")


def _mock_success(client, decision="deny"):
    """Configure the mock to return a successful response."""
    mock_resp = MagicMock()
    mock_resp.status_code = 200
    mock_resp.content = b'{"decision": "' + decision.encode() + b'"}'
    mock_resp.json.return_value = {"decision": decision}
    client._http.request.return_value = mock_resp


def _mock_auth_error(client):
    """Configure the mock to return a 401 response."""
    mock_resp = MagicMock()
    mock_resp.status_code = 401
    mock_resp.text = "Unauthorized"
    client._http.request.return_value = mock_resp


class TestFailClosed:
    def test_default_raises_on_connection_error(self):
        client = _make_client(on_error="closed")
        _mock_connection_error(client)
        with pytest.raises(CordumConnectionError):
            client.evaluate_policy(topic="job.test")

    def test_explicit_closed_raises(self):
        client = _make_client(on_error="closed")
        _mock_connection_error(client)
        with pytest.raises(CordumConnectionError):
            client.evaluate_policy(topic="job.test")


class TestFailOpen:
    def test_returns_allow_on_connection_error(self):
        client = _make_client(on_error="open")
        _mock_connection_error(client)
        result = client.evaluate_policy(topic="job.test")
        assert result.decision == Decision.ALLOW
        assert "fail-open" in result.reason

    def test_normal_deny_not_affected(self):
        client = _make_client(on_error="open")
        _mock_success(client, decision="deny")
        result = client.evaluate_policy(topic="job.test")
        assert result.decision == Decision.DENY


class TestCallback:
    def test_callback_invoked_with_exception(self):
        callback = MagicMock(
            return_value=SafetyDecision(decision=Decision.DENY, reason="callback denied")
        )
        client = _make_client(on_error=callback)
        _mock_connection_error(client)
        result = client.evaluate_policy(topic="job.test")
        assert result.decision == Decision.DENY
        assert result.reason == "callback denied"
        callback.assert_called_once()
        assert isinstance(callback.call_args[0][0], CordumConnectionError)

    def test_callback_can_raise(self):
        def strict_callback(exc):
            raise exc

        client = _make_client(on_error=strict_callback)
        _mock_connection_error(client)
        with pytest.raises(CordumConnectionError):
            client.evaluate_policy(topic="job.test")

    def test_callback_can_allow(self):
        def lenient_callback(exc):
            return SafetyDecision(decision=Decision.ALLOW, reason="lenient")

        client = _make_client(on_error=lenient_callback)
        _mock_connection_error(client)
        result = client.evaluate_policy(topic="job.test")
        assert result.decision == Decision.ALLOW


class TestAuthErrorNotAffected:
    def test_auth_error_raises_with_fail_open(self):
        client = _make_client(on_error="open")
        _mock_auth_error(client)
        with pytest.raises(CordumAuthError):
            client.evaluate_policy(topic="job.test")

    def test_auth_error_raises_with_callback(self):
        callback = MagicMock(
            return_value=SafetyDecision(decision=Decision.ALLOW)
        )
        client = _make_client(on_error=callback)
        _mock_auth_error(client)
        with pytest.raises(CordumAuthError):
            client.evaluate_policy(topic="job.test")
        callback.assert_not_called()


class TestExplicitDenyNotAffected:
    def test_deny_returned_with_fail_open(self):
        client = _make_client(on_error="open")
        _mock_success(client, decision="deny")
        result = client.evaluate_policy(topic="job.test")
        assert result.decision == Decision.DENY

    def test_deny_returned_with_callback(self):
        callback = MagicMock()
        client = _make_client(on_error=callback)
        _mock_success(client, decision="deny")
        result = client.evaluate_policy(topic="job.test")
        assert result.decision == Decision.DENY
        callback.assert_not_called()
