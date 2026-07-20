"""CAP-PRODUCTION atomic replay admission (in-memory reference store)."""

from __future__ import annotations

import threading
from dataclasses import dataclass
from datetime import datetime, timezone
from enum import Enum
from typing import Dict, Protocol


class ReplayOutcome(Enum):
    FIRST = "first"
    DUPLICATE = "duplicate"


class ReplayConflictError(ValueError):
    """Same (tenant, audience, sender, message_id) seen with a different digest."""


class ReplayStoreUnavailableError(ValueError):
    """The replay store rejected the request (invalid input or backend down).

    Callers MUST fail closed on this error, never treat it as first-seen.
    """


class ReplayStore(Protocol):
    def admit(
        self,
        tenant: str,
        audience: str,
        sender: str,
        message_id: bytes,
        digest: bytes,
        expires_at: datetime,
    ) -> ReplayOutcome: ...


@dataclass
class _Entry:
    digest: bytes
    expires_at: datetime


class InMemoryReplayStore:
    """Reference ReplayStore. Durable deployments (e.g. Cordum) supply a
    Redis- or other durable-backed implementation of the same interface."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._entries: Dict[tuple[str, str, str, bytes], _Entry] = {}

    def admit(
        self,
        tenant: str,
        audience: str,
        sender: str,
        message_id: bytes,
        digest: bytes,
        expires_at: datetime,
    ) -> ReplayOutcome:
        if not tenant or not audience or not sender or len(message_id) != 16 or not digest:
            raise ReplayStoreUnavailableError("invalid replay admission input")
        now = datetime.now(timezone.utc)
        if expires_at <= now:
            raise ReplayStoreUnavailableError("expires_at must be in the future")
        key = (tenant, audience, sender, bytes(message_id))
        with self._lock:
            expired = [key for key, entry in self._entries.items() if entry.expires_at <= now]
            for expired_key in expired:
                del self._entries[expired_key]
            existing = self._entries.get(key)
            if existing is not None and existing.expires_at > now:
                if existing.digest == digest:
                    return ReplayOutcome.DUPLICATE
                raise ReplayConflictError(f"replay digest conflict for message_id={message_id.hex()}")
            self._entries[key] = _Entry(digest=digest, expires_at=expires_at)
            return ReplayOutcome.FIRST
