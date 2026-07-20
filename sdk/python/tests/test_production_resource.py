from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest
from google.protobuf.timestamp_pb2 import Timestamp

from cap.pb.cordum.agent.v1.job_pb2 import ResourceRef
from cap.production_resource import (
    MAX_LEGACY_REDIS_KEY_BYTES,
    MAX_RESOURCE_AUTHORITY_BYTES,
    MAX_RESOURCE_IDENTIFIER_BYTES,
    MAX_RESOURCE_SIZE_BYTES,
    MAX_RESOURCE_URI_BYTES,
    ResourceRefValidationError,
    canonical_legacy_redis_key,
    validate_resource_ref,
    validate_resource_ref_compatibility,
)


NOW = datetime(2026, 7, 20, 12, 0, tzinfo=timezone.utc)


def _timestamp(value: datetime) -> Timestamp:
    result = Timestamp()
    result.FromDatetime(value)
    return result


def _valid_ref() -> ResourceRef:
    return ResourceRef(
        resolver_id="redis",
        uri="redis://ctx:job-1",
        sha256=b"\x01" * 32,
        media_type="application/json",
        size_bytes=128,
        expires_at=_timestamp(NOW + timedelta(hours=1)),
        purpose="job-input",
    )


def test_validate_resource_ref_accepts_bounded_canonical_reference() -> None:
    validate_resource_ref(_valid_ref(), ["redis"], now=NOW)


@pytest.mark.parametrize(
    "field,value",
    [
        ("resolver_id", ""),
        ("resolver_id", " redis"),
        ("resolver_id", "redis/primary"),
        ("resolver_id", "r" * (MAX_RESOURCE_IDENTIFIER_BYTES + 1)),
        ("sha256", b"\x01" * 31),
        ("sha256", b"\x01" * 33),
        ("size_bytes", 0),
        ("size_bytes", MAX_RESOURCE_SIZE_BYTES + 1),
        ("media_type", ""),
        ("media_type", " application/json"),
        ("media_type", "Application/JSON"),
        ("media_type", "application/json; charset=utf-8"),
        ("purpose", ""),
        ("purpose", " input"),
        ("purpose", "job input"),
    ],
)
def test_validate_resource_ref_rejects_invalid_fields(field: str, value: object) -> None:
    ref = _valid_ref()
    setattr(ref, field, value)
    with pytest.raises(ResourceRefValidationError):
        validate_resource_ref(ref, ["redis"], now=NOW)


@pytest.mark.parametrize("installed", [[], ["Redis"], ["redis "], ["other"], ["redis", " invalid"]])
def test_validate_resource_ref_requires_exact_installed_resolver(
    installed: list[str],
) -> None:
    with pytest.raises(ResourceRefValidationError, match="resolver"):
        validate_resource_ref(_valid_ref(), installed, now=NOW)


def test_validate_resource_ref_rejects_string_resolver_configuration() -> None:
    with pytest.raises(ResourceRefValidationError, match="resolver configuration"):
        validate_resource_ref(_valid_ref(), "redis", now=NOW)


@pytest.mark.parametrize(
    "uri",
    [
        "",
        " redis://ctx:job-1",
        "redis://ctx:job-1 ",
        "redis:ctx:job-1",
        "Redis://ctx:job-1",
        "1redis://ctx:job-1",
        "redis://",
        "redis:///job",
        "redis://" + "a" * (MAX_RESOURCE_AUTHORITY_BYTES + 1),
        "redis://user:secret@ctx/job",
        "redis://ctx/job?token=secret",
        "redis://ctx/job#secret",
        "redis://ctx/../secret",
        "redis://ctx/%2e%2e/secret",
        "redis://ctx/%252e%252e/secret",
        "redis://ctx/a\\b",
        "redis://ctx/%00",
        "redis://ctx/",
        "redis://ctx//job",
        "redis://ctx/./job",
        "redis://ctx/%2Fjob",
        "redis://ctx/" + "a" * MAX_RESOURCE_URI_BYTES,
    ],
)
def test_validate_resource_ref_rejects_unsafe_uri(uri: str) -> None:
    ref = _valid_ref()
    ref.uri = uri
    with pytest.raises(ResourceRefValidationError, match="URI"):
        validate_resource_ref(ref, ["redis"], now=NOW)


def test_validate_resource_ref_requires_valid_future_expiry() -> None:
    missing = _valid_ref()
    missing.ClearField("expires_at")
    invalid = _valid_ref()
    invalid.expires_at.nanos = -1
    expired = _valid_ref()
    expired.expires_at.CopyFrom(_timestamp(NOW))
    for ref in (missing, invalid, expired):
        with pytest.raises(ResourceRefValidationError, match="expiry"):
            validate_resource_ref(ref, ["redis"], now=NOW)


def test_validate_resource_ref_rejects_naive_validation_time() -> None:
    with pytest.raises(ResourceRefValidationError, match="timezone-aware"):
        validate_resource_ref(_valid_ref(), ["redis"], now=NOW.replace(tzinfo=None))


def test_canonical_legacy_redis_key_returns_exact_utf8_bytes() -> None:
    assert canonical_legacy_redis_key("redis://res:job-1[2]") == b"res:job-1[2]"


@pytest.mark.parametrize(
    "pointer",
    [
        "",
        "Redis://res:job",
        "redis://",
        " redis://res:job",
        "redis://res:job ",
        "redis://res/job",
        "redis://res\\job",
        "redis://res..job",
        "redis://res%3Ajob",
        "redis://user@res:job",
        "redis://res:job?token=x",
        "redis://res:job#part",
        "redis://res:\x00job",
        "redis://" + "k" * (MAX_LEGACY_REDIS_KEY_BYTES + 1),
    ],
)
def test_canonical_legacy_redis_key_rejects_ambiguous_pointer(pointer: str) -> None:
    with pytest.raises(ResourceRefValidationError, match="legacy Redis"):
        canonical_legacy_redis_key(pointer)


def test_validate_resource_ref_compatibility_requires_same_redis_key() -> None:
    validate_resource_ref_compatibility("redis://ctx:job-1", _valid_ref())
    validate_resource_ref_compatibility("", _valid_ref())
    validate_resource_ref_compatibility("redis://ctx:job-1", None)


@pytest.mark.parametrize(
    "legacy,resolver,uri",
    [
        ("redis://ctx:other", "redis", "redis://ctx:job-1"),
        ("redis://ctx:job-1", "blob", "redis://ctx:job-1"),
        ("redis://ctx/../job-1", "redis", "redis://ctx:job-1"),
        ("redis://ctx:job-1", "redis", "redis://ctx:%6aob-1"),
    ],
)
def test_validate_resource_ref_compatibility_rejects_ambiguity(
    legacy: str, resolver: str, uri: str
) -> None:
    ref = _valid_ref()
    ref.resolver_id = resolver
    ref.uri = uri
    with pytest.raises(ResourceRefValidationError, match="legacy"):
        validate_resource_ref_compatibility(legacy, ref)
