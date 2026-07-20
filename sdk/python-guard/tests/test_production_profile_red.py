"""RED tests for CAP-PRODUCTION Guard behavior (task-a13f83fa)."""

from unittest.mock import MagicMock

import pytest

from cordum_guard import CordumClient, Decision


def _response(payload: dict[str, object]) -> MagicMock:
    response = MagicMock()
    response.status_code = 200
    response.content = b"{}"
    response.json.return_value = payload
    return response


def test_production_rejects_fail_open_configuration() -> None:
    with pytest.raises(ValueError, match="CAP-PRODUCTION.*fail closed"):
        CordumClient(
            "https://gateway.example",
            api_key="test-key",
            on_error="open",
            production_profile=True,
        )


@pytest.mark.parametrize("payload", [{}, {"decision": "unknown"}, {"decision": None}])
def test_production_missing_or_unknown_decision_is_deny(payload: dict[str, object]) -> None:
    client = CordumClient.__new__(CordumClient)
    client.base_url = "https://gateway.example"
    client._http = MagicMock()
    client._http.request.return_value = _response(payload)
    client._cache = None
    client._on_error = "closed"
    client._production_profile = True

    result = client.evaluate_policy(topic="job.production")

    assert result.decision == Decision.DENY


def test_production_does_not_cache_positive_allow_without_signed_lease() -> None:
    client = CordumClient(
        "https://gateway.example",
        api_key="test-key",
        cache_ttl=60,
        production_profile=True,
    )
    # _http must be mocked: a real httpx.Client exposes a bound method whose
    # return_value cannot be set (matches the sibling test's approach).
    client._http = MagicMock()
    client._http.request.return_value = _response({"decision": "allow"})

    client.evaluate_policy(topic="job.production", capability="read")
    client.evaluate_policy(topic="job.production", capability="read")

    assert client._http.request.call_count == 2
