"""Fail-closed CAP-PRODUCTION ResourceRef validation."""

from __future__ import annotations

from datetime import datetime, timezone
import re
from typing import Optional, Sequence

from .pb.cordum.agent.v1.job_pb2 import ResourceRef


MAX_RESOURCE_IDENTIFIER_BYTES = 128
MAX_RESOURCE_AUTHORITY_BYTES = 255
MAX_RESOURCE_URI_BYTES = 2048
MAX_RESOURCE_MEDIA_TYPE_BYTES = 127
MAX_RESOURCE_PURPOSE_BYTES = 128
MAX_RESOURCE_SIZE_BYTES = 1 << 30
MAX_LEGACY_REDIS_KEY_BYTES = 1024

_ID_PATTERN = re.compile(r"^[a-z0-9][a-z0-9._-]*$")
_PURPOSE_PATTERN = re.compile(r"^[a-z0-9][a-z0-9._:-]*$")
_MEDIA_PATTERN = re.compile(r"^[a-z0-9][a-z0-9!#$&^_.+-]*/[a-z0-9][a-z0-9!#$&^_.+-]*$")
_URI_PATTERN = re.compile(r"^([a-z][a-z0-9+.-]{0,31})://(.+)$")
_LEGACY_KEY_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9:._@\-\[\]]*$")


class ResourceRefValidationError(ValueError):
    """A structured or migration resource reference is unsafe."""


def validate_resource_ref(
    ref: ResourceRef,
    installed_resolvers: Sequence[str],
    *,
    now: Optional[datetime] = None,
) -> None:
    """Validate generic production bounds before resolver-specific checks."""
    if ref is None:
        raise ResourceRefValidationError("missing resource reference")
    _validate_installed_resolvers(installed_resolvers)
    _validate_identifiers(ref, installed_resolvers)
    if len(ref.sha256) != 32:
        raise ResourceRefValidationError("SHA-256 must contain exactly 32 bytes")
    if ref.size_bytes <= 0 or ref.size_bytes > MAX_RESOURCE_SIZE_BYTES:
        raise ResourceRefValidationError("declared size is outside production bounds")
    _validate_uri(ref.uri)
    _validate_expiry(ref, now or datetime.now(timezone.utc))


def _validate_installed_resolvers(installed: Sequence[str]) -> None:
    if isinstance(installed, (str, bytes)) or not installed:
        raise ResourceRefValidationError("resolver configuration is empty or noncanonical")
    if any(not _valid_identifier(value, MAX_RESOURCE_IDENTIFIER_BYTES, _ID_PATTERN) for value in installed):
        raise ResourceRefValidationError("resolver configuration is empty or noncanonical")


def _validate_identifiers(ref: ResourceRef, installed: Sequence[str]) -> None:
    if not _valid_identifier(ref.resolver_id, MAX_RESOURCE_IDENTIFIER_BYTES, _ID_PATTERN):
        raise ResourceRefValidationError("resolver ID is not canonical")
    if ref.resolver_id not in installed:
        raise ResourceRefValidationError("resolver is not installed")
    if not _valid_identifier(ref.media_type, MAX_RESOURCE_MEDIA_TYPE_BYTES, _MEDIA_PATTERN):
        raise ResourceRefValidationError("media type is not canonical")
    if not _valid_identifier(ref.purpose, MAX_RESOURCE_PURPOSE_BYTES, _PURPOSE_PATTERN):
        raise ResourceRefValidationError("purpose is not canonical")


def _valid_identifier(value: str, limit: int, pattern: re.Pattern[str]) -> bool:
    try:
        encoded = value.encode("ascii")
    except UnicodeEncodeError:
        return False
    return bool(value and len(encoded) <= limit and value.strip() == value and pattern.fullmatch(value))


def _validate_uri(uri: str) -> None:
    try:
        encoded = uri.encode("ascii")
    except UnicodeEncodeError as exc:
        raise ResourceRefValidationError("resource URI must be ASCII") from exc
    if not uri or len(encoded) > MAX_RESOURCE_URI_BYTES or uri.strip() != uri:
        raise ResourceRefValidationError("resource URI is empty, untrimmed, or too long")
    if any(byte < 0x21 or byte > 0x7E for byte in encoded):
        raise ResourceRefValidationError("resource URI contains a control character")
    match = _URI_PATTERN.fullmatch(uri)
    if match is None:
        raise ResourceRefValidationError("resource URI scheme is not canonical")
    authority_and_path = match.group(2)
    if any(character in authority_and_path for character in "@?#\\%"):
        raise ResourceRefValidationError("resource URI contains credentials, metadata, or escapes")
    _validate_path(authority_and_path)


def _validate_path(authority_and_path: str) -> None:
    authority, separator, path = authority_and_path.partition("/")
    if not authority or len(authority.encode("ascii")) > MAX_RESOURCE_AUTHORITY_BYTES:
        raise ResourceRefValidationError("resource URI authority is empty or too long")
    if not separator:
        return
    segments = path.split("/")
    if any(not segment or segment in {".", ".."} or ".." in segment for segment in segments):
        raise ResourceRefValidationError("resource URI path is not normalized")


def _validate_expiry(ref: ResourceRef, now: datetime) -> None:
    if now.tzinfo is None or now.utcoffset() is None:
        raise ResourceRefValidationError("validation time must be timezone-aware")
    if not ref.HasField("expires_at"):
        raise ResourceRefValidationError("resource expiry is required")
    if ref.expires_at.nanos < 0 or ref.expires_at.nanos >= 1_000_000_000:
        raise ResourceRefValidationError("resource expiry timestamp is invalid")
    try:
        expiry = ref.expires_at.ToDatetime(tzinfo=timezone.utc)
    except (OverflowError, ValueError) as exc:
        raise ResourceRefValidationError("resource expiry timestamp is invalid") from exc
    if expiry <= now.astimezone(timezone.utc):
        raise ResourceRefValidationError("resource expiry is not in the future")


def canonical_legacy_redis_key(pointer: str) -> bytes:
    """Return exact Redis key bytes without URL-parser normalization."""
    if pointer.strip() != pointer or len(pointer.encode("utf-8")) > len("redis://") + MAX_LEGACY_REDIS_KEY_BYTES:
        raise ResourceRefValidationError("legacy Redis pointer is untrimmed or too long")
    if not pointer.startswith("redis://"):
        raise ResourceRefValidationError("legacy Redis pointer has the wrong scheme")
    key = pointer[len("redis://") :]
    at, colon = key.find("@"), key.find(":")
    has_userinfo = at >= 0 and (colon < 0 or at < colon)
    if _LEGACY_KEY_PATTERN.fullmatch(key) is None or ".." in key or has_userinfo:
        raise ResourceRefValidationError("legacy Redis key is empty or ambiguous")
    return key.encode("ascii")


def validate_resource_ref_compatibility(
    legacy_pointer: str, ref: Optional[ResourceRef]
) -> None:
    """Reject differing dual legacy/structured Redis references."""
    if not legacy_pointer or ref is None:
        return
    if ref.resolver_id != "redis":
        raise ResourceRefValidationError("legacy Redis pointer uses a different resolver")
    legacy_key = canonical_legacy_redis_key(legacy_pointer)
    try:
        structured_key = canonical_legacy_redis_key(ref.uri)
    except ResourceRefValidationError as exc:
        raise ResourceRefValidationError("legacy structured URI is ambiguous") from exc
    if legacy_key != structured_key:
        raise ResourceRefValidationError("legacy and structured references differ")


__all__ = [
    "MAX_LEGACY_REDIS_KEY_BYTES",
    "MAX_RESOURCE_AUTHORITY_BYTES",
    "MAX_RESOURCE_IDENTIFIER_BYTES",
    "MAX_RESOURCE_MEDIA_TYPE_BYTES",
    "MAX_RESOURCE_PURPOSE_BYTES",
    "MAX_RESOURCE_SIZE_BYTES",
    "MAX_RESOURCE_URI_BYTES",
    "ResourceRefValidationError",
    "canonical_legacy_redis_key",
    "validate_resource_ref",
    "validate_resource_ref_compatibility",
]
