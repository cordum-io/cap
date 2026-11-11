"""Fail-closed ResourceRef operations through operator-installed backends."""
from __future__ import annotations

import hashlib
import hmac
import re
from dataclasses import dataclass
from datetime import datetime, timezone
from types import MappingProxyType
from typing import Callable, Dict, Mapping, Optional, Protocol, Sequence, Tuple

from google.protobuf.timestamp_pb2 import Timestamp

from .pb.cordum.agent.v1.job_pb2 import ResourceRef
from .production_resource import (
    MAX_RESOURCE_IDENTIFIER_BYTES, MAX_RESOURCE_MEDIA_TYPE_BYTES,
    MAX_RESOURCE_PURPOSE_BYTES, MAX_RESOURCE_SIZE_BYTES, ResourceRefValidationError,
    _ID_PATTERN, _MEDIA_PATTERN, _PURPOSE_PATTERN, validate_resource_ref,
)

MAX_TRUSTED_TENANT_ID_BYTES = 128
MAX_TRUSTED_JOB_ID_BYTES = 256
_TENANT_PATTERN = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]*")
_JOB_PATTERN = re.compile(r"[A-Za-z0-9][A-Za-z0-9._:@\-\[\]]*")


class ResourceRegistryError(Exception):
    message = "cap: resource registry error"

    def __init__(self) -> None:
        super().__init__(self.message)


class InvalidResourceRegistryError(ResourceRegistryError):
    message = "cap: invalid resource registry"


class InvalidTrustedResourceContextError(ResourceRegistryError):
    message = "cap: invalid trusted resource context"


class InvalidResourceRefError(ResourceRegistryError):
    message = "cap: invalid production resource reference"


class ResourceResolverUnavailableError(ResourceRegistryError):
    message = "cap: resource resolver unavailable"


class ResourceExternalizerUnavailableError(ResourceRegistryError):
    message = "cap: resource externalizer unavailable"


class ResourceBackendUnavailableError(ResourceRegistryError):
    message = "cap: resource backend unavailable"


class ResourceExpiredError(ResourceRegistryError):
    message = "cap: resource expired"


class ResourceTooLargeError(ResourceRegistryError):
    message = "cap: resource exceeds installed limit"


class ResourceSizeMismatchError(ResourceRegistryError):
    message = "cap: resource size mismatch"


class ResourceMediaTypeMismatchError(ResourceRegistryError):
    message = "cap: resource media type mismatch"


class ResourceDigestMismatchError(ResourceRegistryError):
    message = "cap: resource digest mismatch"


class ResourceMetadataMismatchError(ResourceRegistryError):
    message = "cap: externalized resource metadata mismatch"


@dataclass(frozen=True)
class TrustedResourceContext:
    tenant_id: str
    job_id: str


@dataclass(frozen=True)
class ResourceResolveRequest:
    trusted_context: TrustedResourceContext
    uri: str
    media_type: str
    purpose: str
    declared_size_bytes: int


@dataclass(frozen=True)
class ResourceResolveResult:
    content: bytes
    media_type: str


@dataclass(frozen=True)
class ResolvedResource:
    content: bytes
    media_type: str


@dataclass(frozen=True)
class ResourceExternalizeRequest:
    trusted_context: TrustedResourceContext
    content: bytes
    media_type: str
    purpose: str
    expires_at: datetime
    size_bytes: int
    sha256: bytes


class ResourceResolverBackend(Protocol):
    async def resolve(self, request: ResourceResolveRequest) -> ResourceResolveResult: ...


class ResourceExternalizerBackend(Protocol):
    async def externalize(self, request: ResourceExternalizeRequest) -> ResourceRef: ...


@dataclass(frozen=True)
class ResourceBackendRegistration:
    id: str
    max_bytes: int
    resolver: ResourceResolverBackend
    externalizer: ResourceExternalizerBackend


def _matches(value: str, limit: int, pattern: re.Pattern[str]) -> bool:
    return isinstance(value, str) and len(value) <= limit and pattern.fullmatch(value) is not None


def _validate_trusted(trusted: TrustedResourceContext) -> None:
    if (not isinstance(trusted, TrustedResourceContext)
            or not _matches(trusted.tenant_id, MAX_TRUSTED_TENANT_ID_BYTES, _TENANT_PATTERN)
            or not _matches(trusted.job_id, MAX_TRUSTED_JOB_ID_BYTES, _JOB_PATTERN)):
        raise InvalidTrustedResourceContextError()


def _snapshot(ref: ResourceRef) -> ResourceRef:
    if not isinstance(ref, ResourceRef):
        raise InvalidResourceRefError()
    snapshot = ResourceRef()
    snapshot.CopyFrom(ref)
    original = snapshot.SerializeToString(deterministic=True)
    snapshot.DiscardUnknownFields()
    if original != snapshot.SerializeToString(deterministic=True):
        raise InvalidResourceRefError()
    return snapshot


def _timestamp(value: datetime) -> Timestamp:
    result = Timestamp()
    result.FromDatetime(value.astimezone(timezone.utc))
    return result


def _expired(ref: ResourceRef, now: datetime) -> bool:
    if not ref.HasField("expires_at") or not 0 <= ref.expires_at.nanos < 10**9:
        return False
    try:
        ref.expires_at.ToDatetime(tzinfo=timezone.utc)
    except (OverflowError, ValueError):
        return False
    stamp = _timestamp(now)
    return (ref.expires_at.seconds, ref.expires_at.nanos) <= (stamp.seconds, stamp.nanos)


def _generic_valid(ref: ResourceRef, ids: Sequence[str], now: datetime) -> bool:
    try:
        validate_resource_ref(ref, ids, now=now)
    except ResourceRefValidationError:
        return False
    return True


class ResourceRegistry:
    """Snapshot of installed backends; backend reads must honor declared_size_bytes."""
    __slots__ = ("_backends", "_now")

    def __init__(self, registrations: Sequence[ResourceBackendRegistration], *,
                 now: Optional[Callable[[], datetime]] = None) -> None:
        backends: Dict[str, ResourceBackendRegistration] = {}
        for item in registrations:
            if (not isinstance(item, ResourceBackendRegistration)
                    or not _matches(item.id, MAX_RESOURCE_IDENTIFIER_BYTES, _ID_PATTERN)
                    or type(item.max_bytes) is not int
                    or not 0 < item.max_bytes <= MAX_RESOURCE_SIZE_BYTES
                    or not callable(getattr(item.resolver, "resolve", None))
                    or not callable(getattr(item.externalizer, "externalize", None))
                    or item.id in backends):
                raise InvalidResourceRegistryError()
            backends[item.id] = item
        if not backends:
            raise InvalidResourceRegistryError()
        self._backends: Mapping[str, ResourceBackendRegistration] = MappingProxyType(backends)
        self._now = now if now is not None else lambda: datetime.now(timezone.utc)

    @property
    def backend_ids(self) -> Tuple[str, ...]:
        return tuple(self._backends)

    def _current_time(self) -> datetime:
        now = self._now()
        if not isinstance(now, datetime) or now.utcoffset() is None:
            raise InvalidResourceRegistryError()
        return now.astimezone(timezone.utc)

    async def resolve(self, ref: ResourceRef, trusted: TrustedResourceContext) -> ResolvedResource:
        _validate_trusted(trusted)
        snapshot = _snapshot(ref)
        now = self._current_time()
        if not _generic_valid(snapshot, [snapshot.resolver_id], now):
            if _expired(snapshot, now):
                raise ResourceExpiredError()
            raise InvalidResourceRefError()
        backend = self._backends.get(snapshot.resolver_id)
        if backend is None:
            raise ResourceResolverUnavailableError()
        if snapshot.size_bytes > backend.max_bytes:
            raise ResourceTooLargeError()
        request = ResourceResolveRequest(trusted, snapshot.uri, snapshot.media_type,
                                         snapshot.purpose, snapshot.size_bytes)
        result = None
        try:
            result = await backend.resolver.resolve(request)
        except Exception:
            pass
        if not isinstance(result, ResourceResolveResult) or type(result.content) is not bytes:
            raise ResourceBackendUnavailableError()
        if len(result.content) > snapshot.size_bytes:
            raise ResourceTooLargeError()
        if _expired(snapshot, self._current_time()):
            raise ResourceExpiredError()
        if result.media_type != snapshot.media_type:
            raise ResourceMediaTypeMismatchError()
        if len(result.content) != snapshot.size_bytes:
            raise ResourceSizeMismatchError()
        if not hmac.compare_digest(hashlib.sha256(result.content).digest(), snapshot.sha256):
            raise ResourceDigestMismatchError()
        return ResolvedResource(result.content, result.media_type)


class InMemoryResourceBackend:
    """Test-only, non-durable process storage. Not thread-safe or for production."""

    def __init__(self, resolver_id: str = "memory") -> None:
        self.resolver_id = resolver_id
        self._entries: Dict[Tuple[str, str], Tuple[bytes, str]] = {}

    async def resolve(self, request: ResourceResolveRequest) -> ResourceResolveResult:
        content, media_type = self._entries[(request.trusted_context.tenant_id, request.uri)]
        return ResourceResolveResult(content, media_type)

    async def externalize(self, request: ResourceExternalizeRequest) -> ResourceRef:
        tenant = request.trusted_context.tenant_id
        uri = f"memory://{tenant}/{request.sha256.hex()}"
        self._entries[(tenant, uri)] = (request.content, request.media_type)
        return ResourceRef(resolver_id=self.resolver_id, uri=uri, sha256=request.sha256,
                           size_bytes=request.size_bytes, media_type=request.media_type,
                           purpose=request.purpose, expires_at=_timestamp(request.expires_at))
