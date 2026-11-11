"""ResourceRegistry acceptance and Go parity, with all network access trapped."""
from __future__ import annotations

import asyncio
import hashlib
import inspect
import socket
import traceback
from dataclasses import is_dataclass, replace
from datetime import datetime, timedelta, timezone

import pytest
from google.protobuf.timestamp_pb2 import Timestamp

import cap
import cap.resource_registry as rr
from cap.pb.cordum.agent.v1.job_pb2 import ResourceRef
from cap.production_resource import MAX_RESOURCE_SIZE_BYTES, validate_resource_ref

NOW = datetime(2026, 7, 20, 12, tzinfo=timezone.utc)
EXPIRY = NOW + timedelta(hours=1)
TRUSTED = rr.TrustedResourceContext(tenant_id="tenant-a", job_id="job-7")
CONTENT = b"[1,2,3]"
CANARY = "private-backend-canary-do-not-leak"


@pytest.fixture(autouse=True)
def no_network(monkeypatch):
    def forbidden(*args, **kwargs):
        raise AssertionError("network forbidden")
    monkeypatch.setattr(socket.socket, "connect", forbidden)
    monkeypatch.setattr(socket.socket, "connect_ex", forbidden)


def _ref(content=CONTENT, **changes):
    stamp = Timestamp()
    stamp.FromDatetime(EXPIRY)
    fields = dict(resolver_id="memory", uri="memory://tenant-a/item",
                  sha256=hashlib.sha256(content).digest(), size_bytes=len(content),
                  media_type="application/json", purpose="job-input", expires_at=stamp)
    fields.update(changes)
    return ResourceRef(**fields)


class _Stub:
    def __init__(self):
        self.result = rr.ResourceResolveResult(CONTENT, "application/json")
        self.ref = _ref()
        self.calls = []
        self.error = None
        self.hook = lambda: None

    async def resolve(self, request):
        self.calls.append(request)
        self.hook()
        if self.error is not None:
            raise self.error
        return self.result

    async def externalize(self, request):
        self.calls.append(request)
        self.hook()
        if self.error is not None:
            raise self.error
        return self.ref


def _registration(backend, **changes):
    fields = dict(id="memory", max_bytes=64, resolver=backend, externalizer=backend)
    fields.update(changes)
    return rr.ResourceBackendRegistration(**fields)


def _registry(backend, max_bytes=64, now=lambda: NOW):
    return rr.ResourceRegistry([_registration(backend, max_bytes=max_bytes)], now=now)


def _externalize(registry, **changes):
    fields = dict(backend_id="memory", content=CONTENT, media_type="application/json",
                  purpose="job-input", expires_at=EXPIRY, trusted=TRUSTED)
    fields.update(changes)
    return registry.externalize(**fields)


def _operation(registry, operation, trusted=TRUSTED):
    if operation == "resolve":
        return registry.resolve(_ref(), trusted)
    return _externalize(registry, trusted=trusted)


def _rejected(coro_factory, error_cls):
    with pytest.raises(error_cls) as caught:
        result = coro_factory()
        if inspect.isawaitable(result):
            asyncio.run(result)
    err = caught.value
    assert type(err) is error_cls
    assert err.args == (error_cls.message,)
    assert err.__cause__ is None and err.__context__ is None
    assert CONTENT.decode() not in repr(err)
    return err


def test_in_memory_round_trip_returns_exact_bytes():
    registry = _registry(rr.InMemoryResourceBackend())
    expiry = (EXPIRY + timedelta(microseconds=7)).astimezone(timezone(timedelta(hours=2)))
    ref = asyncio.run(_externalize(registry, expires_at=expiry))
    expected = Timestamp()
    expected.FromDatetime(expiry)
    assert ref.resolver_id == "memory"
    assert ref.uri.startswith("memory://tenant-a/")
    assert ref.sha256 == hashlib.sha256(CONTENT).digest()
    assert ref.size_bytes == len(CONTENT)
    assert ref.media_type == "application/json" and ref.purpose == "job-input"
    assert (ref.expires_at.seconds, ref.expires_at.nanos) == (expected.seconds, expected.nanos)
    validate_resource_ref(ref, ["memory"], now=NOW)
    assert asyncio.run(registry.resolve(ref, TRUSTED)) == rr.ResolvedResource(CONTENT, "application/json")


def test_resolve_rejects_other_tenant():
    registry = _registry(rr.InMemoryResourceBackend())
    ref = asyncio.run(_externalize(registry))
    _rejected(lambda: registry.resolve(ref, replace(TRUSTED, tenant_id="tenant-b")),
              rr.ResourceBackendUnavailableError)


def test_resolve_rejects_digest_mismatch():
    stub = _Stub()
    stub.result = rr.ResourceResolveResult(b"[4,5,6]", "application/json")
    _rejected(lambda: _registry(stub).resolve(_ref(), TRUSTED), rr.ResourceDigestMismatchError)


def test_resolve_rejects_body_over_declared_size():
    stub = _Stub()
    stub.result = rr.ResourceResolveResult(CONTENT + b" ", "application/json")
    _rejected(lambda: _registry(stub).resolve(_ref(), TRUSTED), rr.ResourceTooLargeError)


def test_resolve_rejects_declared_size_over_backend_max():
    stub = _Stub()
    _rejected(lambda: _registry(stub).resolve(_ref(size_bytes=65), TRUSTED), rr.ResourceTooLargeError)
    assert stub.calls == []


def test_externalize_rejects_content_over_backend_max():
    stub = _Stub()
    _rejected(lambda: _externalize(_registry(stub), content=b"x" * 65), rr.ResourceTooLargeError)
    assert stub.calls == []


def test_resolve_rejects_media_type_mismatch():
    stub = _Stub()
    stub.result = rr.ResourceResolveResult(CONTENT, "text/plain")
    _rejected(lambda: _registry(stub).resolve(_ref(), TRUSTED), rr.ResourceMediaTypeMismatchError)


def test_resolve_rejects_expired_ref_before_backend():
    stub, ref = _Stub(), _ref()
    ref.expires_at.FromDatetime(NOW)
    _rejected(lambda: _registry(stub).resolve(ref, TRUSTED), rr.ResourceExpiredError)
    assert stub.calls == []


def test_resolve_rejects_expiry_during_fetch():
    stub, clock = _Stub(), [NOW]
    stub.hook = lambda: clock.__setitem__(0, NOW + timedelta(hours=2))
    _rejected(lambda: _registry(stub, now=lambda: clock[0]).resolve(_ref(), TRUSTED),
              rr.ResourceExpiredError)
    assert len(stub.calls) == 1


def test_resolve_rejects_unregistered_resolver():
    stub = _Stub()
    _rejected(lambda: _registry(stub).resolve(_ref(resolver_id="other"), TRUSTED),
              rr.ResourceResolverUnavailableError)
    assert stub.calls == []


@pytest.mark.parametrize("backend_id", ["other", "", "Memory", None, []])
def test_externalize_rejects_unregistered_backend(backend_id):
    stub = _Stub()
    _rejected(lambda: _externalize(_registry(stub), backend_id=backend_id),
              rr.ResourceExternalizerUnavailableError)
    assert stub.calls == []


@pytest.mark.parametrize("operation", ["resolve", "externalize"])
def test_backend_exception_text_never_surfaces(operation):
    stub = _Stub()
    stub.error = RuntimeError(CANARY)
    err = _rejected(lambda: _operation(_registry(stub), operation), rr.ResourceBackendUnavailableError)
    assert CANARY not in str(err) and CANARY not in repr(err)
    assert CANARY not in "".join(traceback.format_exception(type(err), err, err.__traceback__))


@pytest.mark.parametrize("change", [
    "empty", "not-registration", "duplicate",
    *[{"id": v} for v in ["", "Memory", " memory", "a/b", "x" * 129, "memory\n", None]],
    *[{"max_bytes": v} for v in [0, -1, MAX_RESOURCE_SIZE_BYTES + 1, True, 1.5]],
    *[{name: v} for name in ["resolver", "externalizer"] for v in [None, object()]],
])
def test_registry_rejects_unsafe_registrations(change):
    registration = _registration(_Stub())
    if isinstance(change, dict):
        entries = [replace(registration, **change)]
    else:
        entries = {"empty": [], "not-registration": [object()],
                   "duplicate": [registration, registration]}[change]
    _rejected(lambda: rr.ResourceRegistry(entries), rr.InvalidResourceRegistryError)


def test_registry_snapshots_registrations():
    entries = [_registration(_Stub())]
    registry = rr.ResourceRegistry(entries, now=lambda: NOW)
    entries.clear()
    assert registry.backend_ids == ("memory",)
    assert asyncio.run(registry.resolve(_ref(), TRUSTED)).content == CONTENT


@pytest.mark.parametrize("operation", ["resolve", "externalize"])
@pytest.mark.parametrize("trusted", [
    *[replace(TRUSTED, tenant_id=v) for v in ["", "tenant/a", "x" * 129, "tenant-a\n", "tenänt", None]],
    *[replace(TRUSTED, job_id=v) for v in ["", "job 7", "x" * 257, "job-7\n", None]],
    None, {},
])
def test_invalid_trusted_context_rejected_before_backend(operation, trusted):
    stub = _Stub()
    _rejected(lambda: _operation(_registry(stub), operation, trusted), rr.InvalidTrustedResourceContextError)
    assert stub.calls == []


def _malformed_ref(case):
    ref = _ref()
    if case == "none":
        return None
    if case == "object":
        return object()
    if case in ("unknown", "nested-unknown"):
        target = ref if case == "unknown" else ref.expires_at
        target.MergeFromString(bytes([0xA0, 0x06, 0x01]))
    elif case == "missing-expiry":
        ref.ClearField("expires_at")
    elif case == "nanos":
        ref.expires_at.nanos = 10**9
    elif case == "negative-nanos":
        ref.expires_at.nanos = -1
    elif case == "year-10000":
        ref.expires_at.seconds = 253402300800
    else:
        setattr(ref, *case)
    return ref


@pytest.mark.parametrize("case", [
    "none", "object", "unknown", "nested-unknown", "missing-expiry", "nanos", "negative-nanos", "year-10000",
    *[("uri", v) for v in ["memory://u@h/x", "memory://h/x?y", "memory://h/%2e", "memory://h/a/../b"]],
    ("sha256", b"x" * 31), ("size_bytes", 0), ("media_type", "Application/JSON"),
])
def test_resolve_rejects_malformed_ref_before_backend(case):
    stub = _Stub()
    _rejected(lambda: _registry(stub).resolve(_malformed_ref(case), TRUSTED), rr.InvalidResourceRefError)
    assert stub.calls == []


def test_resolve_rejects_short_body():
    stub = _Stub()
    stub.result = rr.ResourceResolveResult(CONTENT[:-1], "application/json")
    _rejected(lambda: _registry(stub).resolve(_ref(), TRUSTED), rr.ResourceSizeMismatchError)


@pytest.mark.parametrize("result", [None, object(),
    rr.ResourceResolveResult(bytearray(CONTENT), "application/json"),
    rr.ResourceResolveResult(CONTENT.decode(), "application/json")])
def test_resolve_rejects_malformed_backend_result(result):
    stub = _Stub()
    stub.result = result
    _rejected(lambda: _registry(stub).resolve(_ref(), TRUSTED), rr.ResourceBackendUnavailableError)


def test_resolve_sends_only_trusted_identity_and_ref_fields():
    stub, ref = _Stub(), _ref()
    asyncio.run(_registry(stub).resolve(ref, TRUSTED))
    assert stub.calls == [rr.ResourceResolveRequest(TRUSTED, ref.uri, ref.media_type,
                                                  ref.purpose, ref.size_bytes)]


def test_resolve_verifies_snapshot_when_caller_mutates_ref():
    stub, ref = _Stub(), _ref()
    tampered = b"[9,8,7]"
    stub.result = rr.ResourceResolveResult(tampered, "application/json")
    def mutate():
        ref.sha256 = hashlib.sha256(tampered).digest()
        ref.size_bytes = len(tampered)
    stub.hook = mutate
    _rejected(lambda: _registry(stub).resolve(ref, TRUSTED), rr.ResourceDigestMismatchError)


@pytest.mark.parametrize("operation", ["resolve", "externalize"])
def test_cancellation_propagates_unchanged(operation):
    stub = _Stub()
    stub.error = asyncio.CancelledError()
    with pytest.raises(asyncio.CancelledError) as caught:
        asyncio.run(_operation(_registry(stub), operation))
    assert caught.value is stub.error


@pytest.mark.parametrize("case", [
    ("resolver_id", "other"), ("media_type", "text/plain"), ("purpose", "result"),
    ("size_bytes", 1), ("sha256", b"z" * 32), "one-nanosecond", "missing-expiry", "none", "object",
])
def test_externalize_rejects_inconsistent_backend_ref(case):
    stub = _Stub()
    if case == "one-nanosecond":
        stub.ref.expires_at.nanos += 1
    else:
        stub.ref = _malformed_ref(case)
    _rejected(lambda: _externalize(_registry(stub)), rr.ResourceMetadataMismatchError)


@pytest.mark.parametrize("case", [("uri", "memory://u@h/x"), "unknown", "nested-unknown"])
def test_externalize_rejects_unsafe_backend_ref(case):
    stub = _Stub()
    stub.ref = _malformed_ref(case)
    _rejected(lambda: _externalize(_registry(stub)), rr.InvalidResourceRefError)


@pytest.mark.parametrize("changes,error_cls", [
    ({"media_type": "Application/JSON"}, rr.InvalidResourceRefError),
    ({"media_type": "application/json\n"}, rr.InvalidResourceRefError),
    ({"purpose": ""}, rr.InvalidResourceRefError),
    ({"expires_at": EXPIRY.replace(tzinfo=None)}, rr.InvalidResourceRefError),
    ({"expires_at": None}, rr.InvalidResourceRefError),
    ({"expires_at": NOW}, rr.ResourceExpiredError),
    ({"content": b""}, rr.ResourceSizeMismatchError),
    ({"content": bytearray(b"x")}, rr.ResourceBackendUnavailableError),
])
def test_externalize_rejects_invalid_input_before_backend(changes, error_cls):
    stub = _Stub()
    _rejected(lambda: _externalize(_registry(stub), **changes), error_cls)
    assert stub.calls == []


def test_externalize_rejects_expiry_during_backend_call():
    stub, clock = _Stub(), [NOW]
    stub.hook = lambda: clock.__setitem__(0, EXPIRY)
    _rejected(lambda: _externalize(_registry(stub, now=lambda: clock[0])), rr.ResourceExpiredError)
    assert len(stub.calls) == 1


def test_externalize_returns_private_copy():
    stub = _Stub()
    ref = asyncio.run(_externalize(_registry(stub)))
    assert ref == stub.ref and ref is not stub.ref
    stub.ref.Clear()
    assert ref == _ref()


def test_externalize_sends_bounded_content_and_trusted_context():
    stub = _Stub()
    asyncio.run(_externalize(_registry(stub)))
    assert stub.calls == [rr.ResourceExternalizeRequest(TRUSTED, CONTENT, "application/json",
                                                       "job-input", EXPIRY, len(CONTENT),
                                                       hashlib.sha256(CONTENT).digest())]


@pytest.mark.parametrize("operation", ["resolve", "externalize"])
@pytest.mark.parametrize("now", [datetime(2026, 7, 20, 12), None, "not-a-clock"])
def test_clock_must_be_timezone_aware(operation, now):
    stub = _Stub()
    _rejected(lambda: _operation(_registry(stub, now=lambda: now), operation), rr.InvalidResourceRegistryError)
    assert stub.calls == []


def test_public_api_is_exported():
    classes = {name for name, value in vars(rr).items() if isinstance(value, type)
               and value.__module__ == rr.__name__ and not name.startswith("_")}
    constants = {"MAX_TRUSTED_TENANT_ID_BYTES", "MAX_TRUSTED_JOB_ID_BYTES"}
    assert len(classes) == 23
    assert rr.__all__ == sorted(classes | constants)
    for name in classes:
        value = getattr(rr, name)
        assert (issubclass(value, rr.ResourceRegistryError) or is_dataclass(value)
                or getattr(value, "_is_protocol", False)
                or name in {"ResourceRegistry", "InMemoryResourceBackend"})
        assert name in cap.__all__ and getattr(cap, name) is value
