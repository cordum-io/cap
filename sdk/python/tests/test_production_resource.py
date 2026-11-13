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


NOW_SECONDS = _timestamp(NOW).seconds
MIN_SECONDS = -62_135_596_800
MAX_SECONDS = 253_402_300_799
PRE_EPOCH_NOW = datetime(1969, 12, 31, 23, 59, 59, 500_000, tzinfo=timezone.utc)
OFFSET = timezone(timedelta(hours=5, minutes=30))


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


# Match Go validateResourceExpiry (production_resource.go:144-155) and its
# invalid/equal/now-1ns vectors (production_resource_test.go:40-42).
@pytest.mark.parametrize(
    "seconds,nanos,now",
    [
        pytest.param(NOW_SECONDS, 1, NOW, id="exact-second-plus-1ns"),
        pytest.param(NOW_SECONDS, 1_001, NOW + timedelta(microseconds=1), id="sub-microsecond-plus-1ns"),
        pytest.param(NOW_SECONDS, 1, NOW.astimezone(OFFSET), id="offset-now-plus-1ns"),
        pytest.param(-1, 500_000_001, PRE_EPOCH_NOW, id="pre-epoch-plus-1ns"),
        pytest.param(MAX_SECONDS, 999_999_999, NOW, id="max-valid-timestamp"),
        pytest.param(
            MIN_SECONDS, 0, datetime.min.replace(tzinfo=timezone(timedelta(hours=1))),
            id="min-valid-after-extreme-now",
        ),
    ],
)
def test_validate_resource_ref_accepts_expiry_after_now_at_nanosecond_precision(
    seconds: int, nanos: int, now: datetime,
) -> None:
    ref = _valid_ref()
    ref.expires_at.seconds, ref.expires_at.nanos = seconds, nanos
    validate_resource_ref(ref, ["redis"], now=now)


@pytest.mark.parametrize(
    "seconds,nanos,now",
    [
        pytest.param(NOW_SECONDS, 0, NOW, id="equal-exact-second"),
        pytest.param(NOW_SECONDS - 1, 999_999_999, NOW, id="1ns-before-exact-second"),
        pytest.param(NOW_SECONDS, 1_000, NOW + timedelta(microseconds=1), id="equal-sub-microsecond"),
        pytest.param(NOW_SECONDS, 999, NOW + timedelta(microseconds=1), id="1ns-before-sub-microsecond"),
        pytest.param(NOW_SECONDS, 0, NOW.astimezone(OFFSET), id="equal-offset-now"),
        pytest.param(-1, 500_000_000, PRE_EPOCH_NOW, id="equal-pre-epoch"),
        pytest.param(MIN_SECONDS, 0, NOW, id="min-valid-timestamp-in-past"),
        pytest.param(
            MAX_SECONDS, 999_999_999, datetime.max.replace(tzinfo=timezone(timedelta(hours=-1))),
            id="max-valid-before-extreme-now",
        ),
    ],
)
def test_validate_resource_ref_rejects_expiry_not_after_now(
    seconds: int, nanos: int, now: datetime,
) -> None:
    ref = _valid_ref()
    ref.expires_at.seconds, ref.expires_at.nanos = seconds, nanos
    with pytest.raises(ResourceRefValidationError, match=r"^resource expiry is not in the future$"):
        validate_resource_ref(ref, ["redis"], now=now)


@pytest.mark.parametrize(
    "seconds,nanos",
    [
        pytest.param(MAX_SECONDS + 1, 0, id="seconds-above-max"),
        pytest.param(MIN_SECONDS - 1, 999_999_999, id="seconds-below-min"),
        pytest.param(NOW_SECONDS + 3600, -1, id="negative-nanos"),
        pytest.param(NOW_SECONDS + 3600, 1_000_000_000, id="nanos-at-one-second"),
    ],
)
def test_validate_resource_ref_rejects_out_of_range_expiry(seconds: int, nanos: int) -> None:
    ref = _valid_ref()
    ref.expires_at.seconds, ref.expires_at.nanos = seconds, nanos
    with pytest.raises(ResourceRefValidationError, match=r"^resource expiry timestamp is invalid$"):
        validate_resource_ref(ref, ["redis"], now=NOW)


def test_canonical_legacy_redis_key_returns_exact_utf8_bytes() -> None:
    assert canonical_legacy_redis_key("redis://res:job-1[2]") == b"res:job-1[2]"
    assert canonical_legacy_redis_key("redis://res:run:loop[0]@2") == b"res:run:loop[0]@2"


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
