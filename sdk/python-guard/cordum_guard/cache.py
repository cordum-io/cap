"""TTL-based LRU policy cache for Cordum Guard SDK.

Uses only stdlib — no external dependencies.
"""

from __future__ import annotations

import threading
import time
from typing import Optional

from .types import Decision, SafetyDecision

CacheKey = tuple[str, str, tuple[str, ...]]


def make_cache_key(
    topic: str,
    capability: str,
    risk_tags: Optional[list[str]] = None,
) -> CacheKey:
    """Build a deterministic cache key from policy parameters."""
    return (topic, capability, tuple(sorted(risk_tags or [])))


class PolicyCache:
    """Thread-safe TTL-based LRU cache for safety decisions.

    REQUIRE_APPROVAL decisions are never stored.
    """

    def __init__(self, max_size: int = 1000, ttl_seconds: float = 60.0) -> None:
        self._max_size = max_size
        self._ttl = ttl_seconds
        # Insertion-ordered dict: {key: (SafetyDecision, expiry_monotonic)}
        self._store: dict[CacheKey, tuple[SafetyDecision, float]] = {}
        self._lock = threading.Lock()

    def get(self, key: CacheKey) -> Optional[SafetyDecision]:
        """Return a cached decision, or None if expired/missing."""
        with self._lock:
            entry = self._store.get(key)
            if entry is None:
                return None
            decision, expiry = entry
            if time.monotonic() > expiry:
                del self._store[key]
                return None
            # Move to end for LRU ordering.
            self._store.move_to_end(key)
            return decision

    def put(self, key: CacheKey, decision: SafetyDecision) -> None:
        """Store a decision. Skips REQUIRE_APPROVAL decisions."""
        if decision.decision == Decision.REQUIRE_APPROVAL:
            return
        with self._lock:
            expiry = time.monotonic() + self._ttl
            if key in self._store:
                self._store[key] = (decision, expiry)
                self._store.move_to_end(key)
            else:
                if len(self._store) >= self._max_size:
                    # Evict oldest (first) entry.
                    self._store.pop(next(iter(self._store)))
                self._store[key] = (decision, expiry)

    def clear(self) -> None:
        """Purge all cached entries."""
        with self._lock:
            self._store.clear()

    @property
    def size(self) -> int:
        """Current number of cached entries."""
        with self._lock:
            return len(self._store)
