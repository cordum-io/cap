"""Tests for policy caching."""

import time
from unittest.mock import MagicMock, patch

import pytest

from cordum_guard import CordumClient, Decision
from cordum_guard.cache import PolicyCache, make_cache_key
from cordum_guard.types import SafetyDecision


# ---- PolicyCache unit tests ------------------------------------------------


class TestCacheHitAndMiss:
    def test_cache_hit(self):
        cache = PolicyCache(ttl_seconds=60.0)
        key = make_cache_key("job.test", "cap-a")
        decision = SafetyDecision(decision=Decision.ALLOW)
        cache.put(key, decision)
        assert cache.get(key) is decision

    def test_cache_miss_different_key(self):
        cache = PolicyCache(ttl_seconds=60.0)
        key_a = make_cache_key("job.test", "cap-a")
        key_b = make_cache_key("job.test", "cap-b")
        cache.put(key_a, SafetyDecision(decision=Decision.ALLOW))
        assert cache.get(key_b) is None


class TestCacheExpiry:
    def test_expired_entry_returns_none(self):
        cache = PolicyCache(ttl_seconds=0.1)
        key = make_cache_key("job.test", "cap-a")
        cache.put(key, SafetyDecision(decision=Decision.ALLOW))
        assert cache.get(key) is not None
        time.sleep(0.15)
        assert cache.get(key) is None


class TestNoApprovalCache:
    def test_require_approval_not_cached(self):
        cache = PolicyCache(ttl_seconds=60.0)
        key = make_cache_key("job.test", "approval-cap")
        decision = SafetyDecision(decision=Decision.REQUIRE_APPROVAL)
        cache.put(key, decision)
        assert cache.get(key) is None
        assert cache.size == 0

    def test_allow_and_deny_are_cached(self):
        cache = PolicyCache(ttl_seconds=60.0)
        for d in (Decision.ALLOW, Decision.DENY, Decision.THROTTLE):
            key = make_cache_key("job.test", d.value)
            cache.put(key, SafetyDecision(decision=d))
        assert cache.size == 3


class TestMaxSizeEviction:
    def test_evicts_oldest_when_full(self):
        cache = PolicyCache(max_size=2, ttl_seconds=60.0)
        key_a = make_cache_key("job.test", "a")
        key_b = make_cache_key("job.test", "b")
        key_c = make_cache_key("job.test", "c")
        cache.put(key_a, SafetyDecision(decision=Decision.ALLOW))
        cache.put(key_b, SafetyDecision(decision=Decision.ALLOW))
        cache.put(key_c, SafetyDecision(decision=Decision.ALLOW))
        assert cache.size == 2
        assert cache.get(key_a) is None  # evicted
        assert cache.get(key_b) is not None
        assert cache.get(key_c) is not None


class TestClearCache:
    def test_clear_removes_all(self):
        cache = PolicyCache(ttl_seconds=60.0)
        for i in range(5):
            cache.put(make_cache_key("t", f"c{i}"), SafetyDecision(decision=Decision.ALLOW))
        assert cache.size == 5
        cache.clear()
        assert cache.size == 0


class TestCacheKey:
    def test_risk_tags_sorted(self):
        key_a = make_cache_key("t", "c", ["b", "a", "c"])
        key_b = make_cache_key("t", "c", ["a", "b", "c"])
        assert key_a == key_b

    def test_none_risk_tags(self):
        key = make_cache_key("t", "c", None)
        assert key == ("t", "c", ())


# ---- CordumClient integration tests ----------------------------------------


class TestClientCacheIntegration:
    """Test caching through the full CordumClient.evaluate_policy path."""

    def _make_client(self, cache_ttl: float = 30.0, cache_max_size: int = 100):
        """Create a CordumClient with mocked HTTP transport."""
        client = CordumClient.__new__(CordumClient)
        client.base_url = "http://test:8081"
        client._http = MagicMock()

        from cordum_guard.cache import PolicyCache
        client._cache = PolicyCache(max_size=cache_max_size, ttl_seconds=cache_ttl)
        return client

    def _mock_response(self, client, decision: str = "allow"):
        """Configure the mock to return a given decision."""
        mock_resp = MagicMock()
        mock_resp.status_code = 200
        mock_resp.content = b'{"decision": "' + decision.encode() + b'"}'
        mock_resp.json.return_value = {"decision": decision}
        client._http.request.return_value = mock_resp

    def test_cache_hit_avoids_http(self):
        client = self._make_client()
        self._mock_response(client, "allow")

        r1 = client.evaluate_policy(topic="job.t", capability="cap")
        r2 = client.evaluate_policy(topic="job.t", capability="cap")

        assert r1.decision == Decision.ALLOW
        assert r2.decision == Decision.ALLOW
        assert client._http.request.call_count == 1  # only 1 HTTP call

    def test_different_policies_both_hit_http(self):
        client = self._make_client()
        self._mock_response(client, "allow")

        client.evaluate_policy(topic="job.a", capability="a")
        client.evaluate_policy(topic="job.b", capability="b")

        assert client._http.request.call_count == 2

    def test_per_call_bypass(self):
        client = self._make_client()
        self._mock_response(client, "allow")

        client.evaluate_policy(topic="job.t", capability="cap")
        client.evaluate_policy(topic="job.t", capability="cap", cache=False)

        assert client._http.request.call_count == 2  # both hit HTTP

    def test_require_approval_not_cached(self):
        client = self._make_client()
        self._mock_response(client, "require_approval")

        client.evaluate_policy(topic="job.t", capability="cap")
        client.evaluate_policy(topic="job.t", capability="cap")

        assert client._http.request.call_count == 2  # not cached

    def test_clear_cache_invalidates(self):
        client = self._make_client()
        self._mock_response(client, "deny")

        client.evaluate_policy(topic="job.t", capability="cap")
        client.clear_cache()
        client.evaluate_policy(topic="job.t", capability="cap")

        assert client._http.request.call_count == 2

    def test_no_cache_when_ttl_zero(self):
        """Default client (cache_ttl=0) has no cache — no overhead."""
        client = CordumClient.__new__(CordumClient)
        client.base_url = "http://test:8081"
        client._http = MagicMock()
        client._cache = None

        mock_resp = MagicMock()
        mock_resp.status_code = 200
        mock_resp.content = b'{"decision": "allow"}'
        mock_resp.json.return_value = {"decision": "allow"}
        client._http.request.return_value = mock_resp

        client.evaluate_policy(topic="job.t", capability="cap")
        client.evaluate_policy(topic="job.t", capability="cap")

        assert client._http.request.call_count == 2  # no caching
