"""Mandatory real-JetStream test of the nats-kv backend through ResourceRegistry.

It starts its own JetStream server (the pinned image with -js) and fails, never
skips, when Docker or the server is unavailable.
"""
from __future__ import annotations

import asyncio
import base64
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

import nats
import pytest
from nats.js.api import KeyValueConfig

from cap import (
    InvalidNatsKvBucketError,
    NatsKvResourceBackend,
    NatsKvStorageError,
    ResourceBackendUnavailableError,
    ResourceDigestMismatchError,
    ResourceExpiredError,
    ResourceRegistry,
    ResourceTooLargeError,
    TrustedResourceContext,
)
from cap.pb.cordum.agent.v1.job_pb2 import ResourceRef
from tests.docker_nats_support import DockerNATSServer

GOLDEN_PATH = Path(__file__).resolve().parents[4] / "internal" / "resourcekv" / "testdata" / "golden-entry.json"
GOLDEN = json.loads(GOLDEN_PATH.read_text(encoding="utf-8"))
CONTENT = base64.b64decode(GOLDEN["value_base64"])
MEDIA = GOLDEN["ref"]["media_type"]
PURPOSE = GOLDEN["ref"]["purpose"]
EXPIRY = datetime(2030, 1, 1, tzinfo=timezone.utc)
NOW = datetime(2029, 12, 31, 23, tzinfo=timezone.utc)
TRUSTED = TrustedResourceContext(GOLDEN["tenant_id"], GOLDEN["job_id"])


@pytest.fixture(scope="module")
def jetstream_url():
    server = DockerNATSServer.start("-js")
    try:
        yield server.url
    finally:
        server.close()


def _golden_ref() -> ResourceRef:
    golden = GOLDEN["ref"]
    ref = ResourceRef(resolver_id=golden["resolver_id"], uri=golden["uri"],
                      sha256=bytes.fromhex(golden["sha256_hex"]), media_type=MEDIA,
                      size_bytes=golden["size_bytes"], purpose=PURPOSE)
    ref.expires_at.FromDatetime(EXPIRY)
    return ref


def _run(url, scenario):
    async def main():
        nc = await nats.connect(url, allow_reconnect=False, connect_timeout=2, max_reconnect_attempts=0)
        try:
            await scenario(nc.jetstream())
        finally:
            await nc.close()
    asyncio.run(main())


def test_round_trip_and_go_entry_parity(jetstream_url):
    async def scenario(js):
        kv = await js.create_key_value(KeyValueConfig(bucket=GOLDEN["bucket"], history=1, max_value_size=1024))
        backend = await NatsKvResourceBackend.open(kv)
        assert (backend.bucket, backend.max_bytes) == (GOLDEN["bucket"], 1024)
        registry = ResourceRegistry([backend.registration()], now=lambda: NOW)
        ref = await registry.externalize("nats-kv", CONTENT, media_type=MEDIA, purpose=PURPOSE,
                                         expires_at=EXPIRY, trusted=TRUSTED)
        assert ref == _golden_ref()
        assert (await kv.get(GOLDEN["key"])).value == CONTENT
        resolved = await registry.resolve(ref, TRUSTED)
        assert (resolved.content, resolved.media_type) == (CONTENT, MEDIA)
        # The entry the Go backend writes: the fixture's key and raw value bytes.
        await kv.purge(GOLDEN["key"])
        await kv.put(GOLDEN["key"], base64.b64decode(GOLDEN["value_base64"]))
        assert (await registry.resolve(_golden_ref(), TRUSTED)).content == CONTENT

    _run(jetstream_url, scenario)


def test_negatives_fail_closed_without_bytes(jetstream_url):
    async def scenario(js):
        kv = await js.create_key_value(KeyValueConfig(bucket="cap-negatives", history=1, max_value_size=len(CONTENT)))
        backend = await NatsKvResourceBackend.open(kv)
        registry = ResourceRegistry([backend.registration()], now=lambda: NOW)
        ref = await registry.externalize("nats-kv", CONTENT, media_type=MEDIA, purpose=PURPOSE,
                                         expires_at=EXPIRY, trusted=TRUSTED)
        key = GOLDEN["key"]

        with pytest.raises(ResourceBackendUnavailableError):  # URI tenant is not the trusted tenant
            await registry.resolve(ref, TrustedResourceContext("tenant-b", "job-1"))
        await kv.put(key, b"X" * len(CONTENT))
        with pytest.raises(ResourceDigestMismatchError):  # tampered stored value
            await registry.resolve(ref, TRUSTED)
        await kv.put(key, CONTENT)
        expired = ResourceRegistry([backend.registration()], now=lambda: EXPIRY + timedelta(seconds=1))
        with pytest.raises(ResourceExpiredError):
            await expired.resolve(ref, TRUSTED)
        with pytest.raises(ResourceTooLargeError):  # over the bucket's max_value_size
            await registry.externalize("nats-kv", CONTENT + b" ", media_type=MEDIA, purpose=PURPOSE,
                                       expires_at=EXPIRY, trusted=TRUSTED)
        oversize = ResourceRef()
        oversize.CopyFrom(ref)
        oversize.size_bytes = len(CONTENT) + 1
        with pytest.raises(ResourceTooLargeError):
            await registry.resolve(oversize, TRUSTED)

        await js.delete_key_value("cap-negatives")  # missing bucket
        with pytest.raises(ResourceBackendUnavailableError):
            await registry.resolve(ref, TRUSTED)
        with pytest.raises(ResourceBackendUnavailableError):
            await registry.externalize("nats-kv", CONTENT, media_type=MEDIA, purpose=PURPOSE,
                                       expires_at=EXPIRY, trusted=TRUSTED)
        with pytest.raises(NatsKvStorageError):
            await NatsKvResourceBackend.open(kv)

        unbounded = await js.create_key_value(KeyValueConfig(bucket="cap-unbounded", history=1))
        with pytest.raises(InvalidNatsKvBucketError):
            await NatsKvResourceBackend.open(unbounded)

    _run(jetstream_url, scenario)
