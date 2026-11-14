"""Content-addressed JetStream KV ResourceRef backend ("nats-kv").

The URI is ``nats-kv://<bucket>/<tenant>/<sha256hex>``, with the trusted tenant
verbatim and a 64-character lowercase hexadecimal SHA-256 digest. The KV key is
``hex(tenant) + "." + sha256hex + "." + hex(media_type)``, in lowercase
hexadecimal of the bytes; the value is the raw content, with no header or
envelope. Media types have separate keys, so the same bytes stored with two
media types never overwrite each other. This is the storage format of the Go
backend ``internal/resourcekv``, byte for byte, and
``internal/resourcekv/testdata/golden-entry.json`` is the shared parity fixture.

Operators provision the bucket (history 1 suffices) with ``max_value_size``
between 1 byte and 1 GiB; it becomes the registration's ``max_bytes``, and an
unbounded bucket is refused. Set ``max_age`` to at least the longest reference
lifetime and keep the server's max_payload at or above ``max_value_size``.
Entries have no per-entry TTL. The connection, credentials and bucket come from
the operator; no ResourceRef chooses them. The registry still runs every
generic check before and after each call.
"""
from __future__ import annotations

import hashlib
import hmac
import re
from typing import TYPE_CHECKING

from nats.js.errors import KeyDeletedError, KeyNotFoundError

from .pb.cordum.agent.v1.job_pb2 import ResourceRef
from .production_resource import MAX_RESOURCE_AUTHORITY_BYTES, MAX_RESOURCE_SIZE_BYTES
from .resource_registry import (
    ResourceBackendRegistration,
    ResourceDigestMismatchError,
    ResourceExternalizeRequest,
    ResourceResolveRequest,
    ResourceResolveResult,
    ResourceSizeMismatchError,
    ResourceTooLargeError,
    _timestamp,
)

if TYPE_CHECKING:
    from nats.js.kv import KeyValue

NATS_KV_RESOLVER_ID = "nats-kv"
_URI_PREFIX = "nats-kv://"
_BUCKET_PATTERN = re.compile(r"[A-Za-z0-9_-]+")
_DIGEST_PATTERN = re.compile(r"[0-9a-f]{64}")


class NatsKvResourceError(Exception):
    message = "cap: nats-kv resource error"

    def __init__(self) -> None:
        super().__init__(self.message)


class InvalidNatsKvBucketError(NatsKvResourceError):
    message = "cap: invalid or unbounded nats-kv bucket"


class InvalidNatsKvURIError(NatsKvResourceError):
    message = "cap: invalid nats-kv URI"


class NatsKvTenantMismatchError(NatsKvResourceError):
    message = "cap: nats-kv tenant mismatch"


class NatsKvEntryNotFoundError(NatsKvResourceError):
    message = "cap: nats-kv entry not found"


class NatsKvStorageError(NatsKvResourceError):
    """A storage failure. The message never carries backend error text."""

    message = "cap: nats-kv storage unavailable"


def _entry_key(tenant: str, digest: str, media_type: str) -> str:
    return tenant.encode("utf-8").hex() + "." + digest + "." + media_type.encode("utf-8").hex()


class NatsKvResourceBackend:
    """Resolver and externalizer over one operator-provisioned JetStream KV bucket.

    Build it with :meth:`open`. It is immutable afterwards, and the KV handle
    must be usable from the event loop that calls the registry.
    """

    __slots__ = ("_kv", "_bucket", "_max_bytes")

    def __init__(self, kv: KeyValue, bucket: str, max_bytes: int) -> None:
        if (kv is None or not isinstance(bucket, str)
                or len(bucket) > MAX_RESOURCE_AUTHORITY_BYTES
                or _BUCKET_PATTERN.fullmatch(bucket) is None
                or type(max_bytes) is not int
                or not 0 < max_bytes <= MAX_RESOURCE_SIZE_BYTES):
            raise InvalidNatsKvBucketError()
        self._kv = kv
        self._bucket = bucket
        self._max_bytes = max_bytes

    @classmethod
    async def open(cls, kv: KeyValue) -> NatsKvResourceBackend:
        """Validate the bucket and snapshot its ``max_value_size`` as the bound."""
        if kv is None:
            raise InvalidNatsKvBucketError()
        try:
            status = await kv.status()
        except Exception as exc:
            raise NatsKvStorageError() from exc
        bucket = getattr(status, "bucket", None)
        config = getattr(getattr(status, "stream_info", None), "config", None)
        bound = getattr(config, "max_msg_size", None)
        if not isinstance(bucket, str) or not isinstance(bound, int):
            raise InvalidNatsKvBucketError()
        return cls(kv, bucket, bound)

    @property
    def bucket(self) -> str:
        return self._bucket

    @property
    def max_bytes(self) -> int:
        return self._max_bytes

    def registration(self) -> ResourceBackendRegistration:
        """The paired resolver and externalizer for ``ResourceRegistry``."""
        return ResourceBackendRegistration(NATS_KV_RESOLVER_ID, self._max_bytes, self, self)

    async def resolve(self, request: ResourceResolveRequest) -> ResourceResolveResult:
        """Fetch only from this bucket and the trusted tenant's key."""
        tenant = request.trusted_context.tenant_id
        digest = self._uri_digest(request.uri, tenant)
        try:
            entry = await self._kv.get(_entry_key(tenant, digest, request.media_type))
        except (KeyNotFoundError, KeyDeletedError):
            raise NatsKvEntryNotFoundError() from None
        except Exception as exc:
            raise NatsKvStorageError() from exc
        value = getattr(entry, "value", None)
        if not isinstance(value, (bytes, bytearray)):
            raise NatsKvStorageError()
        return ResourceResolveResult(bytes(value), request.media_type)

    def _uri_digest(self, uri: str, tenant: str) -> str:
        if not isinstance(uri, str) or not uri.startswith(_URI_PREFIX):
            raise InvalidNatsKvURIError()
        parts = uri[len(_URI_PREFIX):].split("/")
        if len(parts) != 3 or parts[0] != self._bucket or not parts[1]:
            raise InvalidNatsKvURIError()
        if parts[1] != tenant:
            raise NatsKvTenantMismatchError()
        if _DIGEST_PATTERN.fullmatch(parts[2]) is None:
            raise InvalidNatsKvURIError()
        return parts[2]

    async def externalize(self, request: ResourceExternalizeRequest) -> ResourceRef:
        """Write content-addressed raw bytes and return the reference."""
        content = bytes(request.content)
        if len(content) != request.size_bytes:
            raise ResourceSizeMismatchError()
        if not hmac.compare_digest(hashlib.sha256(content).digest(), request.sha256):
            raise ResourceDigestMismatchError()
        if request.size_bytes > self._max_bytes:
            raise ResourceTooLargeError()
        tenant = request.trusted_context.tenant_id
        if ".." in tenant:
            raise InvalidNatsKvURIError()
        digest = request.sha256.hex()
        try:
            await self._kv.put(_entry_key(tenant, digest, request.media_type), content)
        except Exception as exc:
            raise NatsKvStorageError() from exc
        return ResourceRef(
            resolver_id=NATS_KV_RESOLVER_ID,
            uri=_URI_PREFIX + self._bucket + "/" + tenant + "/" + digest,
            sha256=request.sha256,
            media_type=request.media_type,
            size_bytes=request.size_bytes,
            expires_at=_timestamp(request.expires_at),
            purpose=request.purpose,
        )


__all__ = [
    "InvalidNatsKvBucketError",
    "InvalidNatsKvURIError",
    "NATS_KV_RESOLVER_ID",
    "NatsKvEntryNotFoundError",
    "NatsKvResourceBackend",
    "NatsKvResourceError",
    "NatsKvStorageError",
    "NatsKvTenantMismatchError",
]
