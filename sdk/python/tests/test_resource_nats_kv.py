"""nats-kv backend: Go fixture parity and fail-closed checks, with all network access trapped."""
from __future__ import annotations

import asyncio
import base64
import hashlib
import json
import socket
from datetime import datetime, timedelta, timezone
from pathlib import Path
from types import SimpleNamespace

import pytest
from nats.js.errors import KeyDeletedError, KeyNotFoundError

import cap
import cap.resource_nats_kv as kvmod
from cap.pb.cordum.agent.v1.job_pb2 import ResourceRef
from cap.resource_registry import (
    ResourceBackendUnavailableError, ResourceDigestMismatchError, ResourceExpiredError,
    ResourceExternalizeRequest, ResourceRegistry, ResourceResolveRequest,
    ResourceSizeMismatchError, ResourceTooLargeError, TrustedResourceContext,
)

GOLDEN_PATH = Path(__file__).resolve().parents[3] / "internal" / "resourcekv" / "testdata" / "golden-entry.json"
GOLDEN = json.loads(GOLDEN_PATH.read_text(encoding="utf-8"))
GOLDEN_BYTES = base64.b64decode(GOLDEN["value_base64"])
GOLDEN_EXPIRY = datetime(2030, 1, 1, tzinfo=timezone.utc)
NOW = datetime(2029, 12, 31, 23, tzinfo=timezone.utc)
TRUSTED = TrustedResourceContext(GOLDEN["tenant_id"], GOLDEN["job_id"])
CANARY = "private-backend-canary-do-not-leak"


@pytest.fixture(autouse=True)
def no_network(monkeypatch):
    connect, connect_ex, socketpair = socket.socket.connect, socket.socket.connect_ex, socket.socketpair

    def forbidden(*args, **kwargs):
        raise AssertionError("network forbidden")

    def wakeup_pair(*args, **kwargs):
        # Windows implements asyncio's private wakeup pipe with loopback sockets.
        with monkeypatch.context() as local:
            local.setattr(socket.socket, "connect", connect)
            local.setattr(socket.socket, "connect_ex", connect_ex)
            return socketpair(*args, **kwargs)

    monkeypatch.setattr(socket, "socketpair", wakeup_pair)
    monkeypatch.setattr(socket.socket, "connect", forbidden)
    monkeypatch.setattr(socket.socket, "connect_ex", forbidden)


class FakeKV:
    """The three nats-py KeyValue calls the backend uses, over a dict."""

    def __init__(self, bucket=GOLDEN["bucket"], bound=1024):
        self.status_value = SimpleNamespace(
            bucket=bucket, stream_info=SimpleNamespace(config=SimpleNamespace(max_msg_size=bound)))
        self.entries = {}
        self.gets = []
        self.puts = []
        self.fail = None

    async def status(self):
        if self.fail is not None:
            raise self.fail
        return self.status_value

    async def get(self, key):
        self.gets.append(key)
        if self.fail is not None:
            raise self.fail
        if key not in self.entries:
            raise KeyNotFoundError()
        return SimpleNamespace(value=self.entries[key])

    async def put(self, key, value):
        self.puts.append((key, value))
        if self.fail is not None:
            raise self.fail
        self.entries[key] = value
        return len(self.puts)


def _backend(kv=None):
    return asyncio.run(kvmod.NatsKvResourceBackend.open(kv if kv is not None else FakeKV()))


def _registry(backend, now=NOW):
    return ResourceRegistry([backend.registration()], now=lambda: now)


def _golden_ref(**changes):
    golden = GOLDEN["ref"]
    fields = dict(resolver_id=golden["resolver_id"], uri=golden["uri"],
                  sha256=bytes.fromhex(golden["sha256_hex"]), media_type=golden["media_type"],
                  size_bytes=golden["size_bytes"], purpose=golden["purpose"])
    fields.update(changes)
    ref = ResourceRef(**fields)
    if "expires_at" not in changes:
        ref.expires_at.FromDatetime(GOLDEN_EXPIRY)
    return ref


def _externalize(registry, content=GOLDEN_BYTES, trusted=TRUSTED, media_type=None):
    return asyncio.run(registry.externalize(
        kvmod.NATS_KV_RESOLVER_ID, content, media_type=media_type or GOLDEN["ref"]["media_type"],
        purpose=GOLDEN["ref"]["purpose"], expires_at=GOLDEN_EXPIRY, trusted=trusted))


def _resolve_request(uri, tenant=GOLDEN["tenant_id"]):
    return ResourceResolveRequest(TrustedResourceContext(tenant, "job-1"), uri,
                                  GOLDEN["ref"]["media_type"], GOLDEN["ref"]["purpose"], 16)


def test_golden_fixture_is_the_go_entry():
    assert GOLDEN["ref"]["sha256_hex"] == hashlib.sha256(GOLDEN_BYTES).hexdigest()
    assert GOLDEN["ref"]["size_bytes"] == len(GOLDEN_BYTES)
    assert GOLDEN["ref"]["expires_at"] == "2030-01-01T00:00:00Z"


def test_externalize_writes_the_go_key_and_value_bytes():
    kv = FakeKV()
    ref = _externalize(_registry(_backend(kv)))
    assert kv.puts == [(GOLDEN["key"], GOLDEN_BYTES)]
    assert ref == _golden_ref()
    assert ref.uri == GOLDEN["ref"]["uri"]
    assert ref.expires_at.ToDatetime(tzinfo=timezone.utc) == GOLDEN_EXPIRY


def test_resolve_reads_the_go_written_entry():
    kv = FakeKV()
    kv.entries[GOLDEN["key"]] = GOLDEN_BYTES
    resolved = asyncio.run(_registry(_backend(kv)).resolve(_golden_ref(), TRUSTED))
    assert resolved.content == GOLDEN_BYTES
    assert resolved.media_type == GOLDEN["ref"]["media_type"]
    assert kv.gets == [GOLDEN["key"]]


def test_media_types_have_separate_keys():
    kv = FakeKV()
    registry = _registry(_backend(kv))
    json_ref = _externalize(registry)
    text_ref = _externalize(registry, media_type="text/plain")
    assert json_ref.uri == text_ref.uri
    assert [key for key, _ in kv.puts] == [GOLDEN["key"], "74656e616e742d61." + GOLDEN["ref"]["sha256_hex"] + ".746578742f706c61696e"]
    assert asyncio.run(registry.resolve(text_ref, TRUSTED)).media_type == "text/plain"


@pytest.mark.parametrize("bucket,bound", [
    (GOLDEN["bucket"], None), (GOLDEN["bucket"], -1), (GOLDEN["bucket"], 0),
    (GOLDEN["bucket"], (1 << 30) + 1), (GOLDEN["bucket"], True), (GOLDEN["bucket"], "1024"),
    ("", 1024), ("a.b", 1024), ("a b", 1024), ("a/b", 1024), ("x" * 256, 1024), (None, 1024),
])
def test_open_refuses_an_invalid_or_unbounded_bucket(bucket, bound):
    with pytest.raises(kvmod.InvalidNatsKvBucketError):
        _backend(FakeKV(bucket=bucket, bound=bound))


def test_open_accepts_the_bounds_edges():
    assert _backend(FakeKV(bucket="x" * 255, bound=1 << 30)).max_bytes == 1 << 30
    assert _backend(FakeKV(bound=1)).registration().max_bytes == 1


def test_open_refuses_a_missing_handle_and_hides_status_errors():
    with pytest.raises(kvmod.InvalidNatsKvBucketError):
        asyncio.run(kvmod.NatsKvResourceBackend.open(None))
    kv = FakeKV()
    kv.fail = RuntimeError(CANARY)
    with pytest.raises(kvmod.NatsKvStorageError) as caught:
        _backend(kv)
    assert CANARY not in str(caught.value)


@pytest.mark.parametrize("uri", [
    "nats://cap-resources/tenant-a/" + "a" * 64,
    "nats-kv://other/tenant-a/" + "a" * 64,
    "nats-kv://cap-resources/" + "a" * 64,
    "nats-kv://cap-resources/tenant-a/" + "a" * 64 + "/x",
    "nats-kv://cap-resources//" + "a" * 64,
    "nats-kv://cap-resources/tenant-a/" + "A" * 64,
    "nats-kv://cap-resources/tenant-a/" + "a" * 63,
    "nats-kv://cap-resources/tenant-a/" + "g" * 64,
])
def test_resolve_refuses_a_malformed_uri_before_fetching(uri):
    kv = FakeKV()
    with pytest.raises(kvmod.InvalidNatsKvURIError):
        asyncio.run(_backend(kv).resolve(_resolve_request(uri)))
    assert kv.gets == []


def test_resolve_refuses_another_tenants_uri_before_fetching():
    kv = FakeKV()
    with pytest.raises(kvmod.NatsKvTenantMismatchError):
        asyncio.run(_backend(kv).resolve(_resolve_request(GOLDEN["ref"]["uri"], tenant="tenant-b")))
    assert kv.gets == []
    with pytest.raises(ResourceBackendUnavailableError):
        asyncio.run(_registry(_backend(kv)).resolve(_golden_ref(), TrustedResourceContext("tenant-b", "job-1")))
    assert kv.gets == []


@pytest.mark.parametrize("error", [None, KeyNotFoundError(), KeyDeletedError()])
def test_resolve_reports_a_missing_or_deleted_key_as_not_found(error):
    kv = FakeKV()
    backend = _backend(kv)
    kv.fail = error
    with pytest.raises(kvmod.NatsKvEntryNotFoundError):
        asyncio.run(backend.resolve(_resolve_request(GOLDEN["ref"]["uri"])))
    with pytest.raises(ResourceBackendUnavailableError):
        asyncio.run(_registry(backend).resolve(_golden_ref(), TRUSTED))
    assert kv.gets == [GOLDEN["key"], GOLDEN["key"]]


def test_storage_errors_never_carry_backend_text():
    kv = FakeKV()
    backend = _backend(kv)
    kv.fail = RuntimeError(CANARY)
    with pytest.raises(kvmod.NatsKvStorageError) as on_get:
        asyncio.run(backend.resolve(_resolve_request(GOLDEN["ref"]["uri"])))
    request = ResourceExternalizeRequest(TRUSTED, GOLDEN_BYTES, GOLDEN["ref"]["media_type"], "job-input",
                                         GOLDEN_EXPIRY, len(GOLDEN_BYTES), hashlib.sha256(GOLDEN_BYTES).digest())
    with pytest.raises(kvmod.NatsKvStorageError) as on_put:
        asyncio.run(backend.externalize(request))
    for caught in (on_get, on_put):
        assert str(caught.value) == "cap: nats-kv storage unavailable"
        assert CANARY not in str(caught.value)


def test_registry_refuses_tampered_oversize_and_expired_entries_without_returning_bytes():
    kv = FakeKV(bound=16)
    backend = _backend(kv)
    kv.entries[GOLDEN["key"]] = b"X" * len(GOLDEN_BYTES)
    with pytest.raises(ResourceDigestMismatchError):
        asyncio.run(_registry(backend).resolve(_golden_ref(), TRUSTED))
    kv.entries[GOLDEN["key"]] = GOLDEN_BYTES + b" "
    with pytest.raises(ResourceTooLargeError):
        asyncio.run(_registry(backend).resolve(_golden_ref(), TRUSTED))
    kv.gets.clear()
    with pytest.raises(ResourceTooLargeError):
        asyncio.run(_registry(backend).resolve(_golden_ref(size_bytes=17), TRUSTED))
    with pytest.raises(ResourceExpiredError):
        asyncio.run(_registry(backend, now=GOLDEN_EXPIRY + timedelta(seconds=1)).resolve(_golden_ref(), TRUSTED))
    assert kv.gets == []
    with pytest.raises(ResourceTooLargeError):
        _externalize(_registry(backend), content=GOLDEN_BYTES + b" ")
    assert kv.puts == []


def test_externalize_rechecks_the_request_and_refuses_dotdot_tenants():
    kv = FakeKV(bound=16)
    backend = _backend(kv)
    digest = hashlib.sha256(GOLDEN_BYTES).digest()

    def request(content=GOLDEN_BYTES, size=len(GOLDEN_BYTES), sha=digest, tenant="tenant-a"):
        return ResourceExternalizeRequest(TrustedResourceContext(tenant, "job-1"), content,
                                          "application/json", "job-input", GOLDEN_EXPIRY, size, sha)

    for bad, error in [(request(size=15), ResourceSizeMismatchError),
                       (request(sha=b"\x00" * 32), ResourceDigestMismatchError),
                       (request(content=b"y" * 17, size=17, sha=hashlib.sha256(b"y" * 17).digest()),
                        ResourceTooLargeError),
                       (request(tenant="tenant..a"), kvmod.InvalidNatsKvURIError)]:
        with pytest.raises(error):
            asyncio.run(backend.externalize(bad))
    with pytest.raises(ResourceBackendUnavailableError):
        _externalize(_registry(backend), trusted=TrustedResourceContext("tenant..a", "job-1"))
    assert kv.puts == []


def test_public_names():
    assert kvmod.__all__ == sorted(kvmod.__all__)
    for name in kvmod.__all__:
        assert name in cap.__all__ and getattr(cap, name) is getattr(kvmod, name)
    assert kvmod.NATS_KV_RESOLVER_ID == GOLDEN["ref"]["resolver_id"] == "nats-kv"
    for name in kvmod.__all__:
        value = getattr(kvmod, name)
        if isinstance(value, type) and issubclass(value, Exception):
            assert issubclass(value, kvmod.NatsKvResourceError)
