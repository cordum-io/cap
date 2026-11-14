"""Dispatch decision table and exact-redelivery outbox contract (F1-F10)."""
from __future__ import annotations

import asyncio
from types import SimpleNamespace
from unittest.mock import patch

import pytest
from cryptography.hazmat.primitives.asymmetric import ec

from cap.packet_boundary import finalize_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.production_events import seal_production_event
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionTrust, verify_production_packet
from cap.runtime import Agent, InMemoryBlobStore
from cap.subjects import SUBJECT_RESULT
from production_admission_support import activate_production_session, make_packet, production_agent
from runtime_support import RecordingNATS
from test_outcome import readme_lines_with
from test_runtime_resources import RecordingMetrics

D1, D2 = ("d1", 1, "worker"), ("d2", 2, "worker")
DIGEST, RECORDED = b"digest", ("trace-recorded", b"result")
TOPIC = "job.test"


def fence(**options):
    from cap.dispatch_fence import DispatchFence
    return DispatchFence(**options)


def decision(store, dispatch=D1, digest=DIGEST, job="job", **options):
    value, reason, replay = store.admit(job, dispatch, digest, "worker", **options)
    return value.name, reason, replay


@pytest.mark.parametrize("state,dispatch,digest,expected", [
    ("unknown", D1, DIGEST, ("RUN", "new dispatch", None)),
    ("recorded", D1, DIGEST, ("REPLAY", "recorded result", RECORDED)),
    ("finished", D1, DIGEST, ("RUN", "new dispatch", None)),
    ("running", D1, DIGEST, ("DROP", "in flight", None)),
    ("recorded", D1, b"changed", ("DROP", "conflicting dispatch", None)),
    ("running", D2, DIGEST, ("RUN", "new dispatch", None)),
    ("running", ("old", 1, "worker"), DIGEST, ("DROP", "stale attempt", None)),
    ("running", ("future", 3, "worker"), DIGEST, ("DROP", "future attempt", None)),
    ("recorded", D2, DIGEST, ("RUN", "new dispatch", None)),
    ("recorded", ("gap", 4, "worker"), DIGEST, ("RUN", "new dispatch", None)),
    ("recorded", ("other", 1, "worker"), DIGEST, ("DROP", "stale attempt", None)),
], ids=["unknown", "replay", "interrupted", "inflight", "conflict", "next-running",
        "stale-running", "future-running", "next-done", "gap-done", "stale-done"])
def test_decision_table(state, dispatch, digest, expected):
    store = fence()
    if state != "unknown":
        assert decision(store)[0] == "RUN"
    if state == "recorded":
        assert store.record("job", D1, *RECORDED) is True
    if state == "finished":
        store.finish("job", D1)
    assert decision(store, dispatch, digest) == expected


@pytest.mark.parametrize("job,dispatch,reason", [
    ("job", ("", 1, "worker"), "malformed dispatch"),
    ("job", ("d", 0, "worker"), "malformed dispatch"),
    ("job", ("x" * 129, 1, "worker"), "malformed dispatch"),
    ("job", ("é" * 65, 1, "worker"), "malformed dispatch"),
    ("x" * 257, D1, "malformed dispatch"),
    ("job", ("d", 1, "other"), "wrong worker"),
])
def test_invalid_dispatch(job, dispatch, reason):
    assert decision(fence(), dispatch, job=job) == ("DROP", reason, None)


def test_record_and_finish_cannot_replace_another_attempt():
    store = fence()
    assert store.record("job", D1, *RECORDED) is False
    decision(store)
    assert decision(store, D2)[0] == "RUN"
    assert store.record("job", D1, *RECORDED) is False
    store.finish("job", D1)
    assert decision(store, D2) == ("DROP", "in flight", None)
    store.finish("job", D2)
    assert store.record("job", D2, *RECORDED) is False
    assert decision(store, D2)[0] == "RUN"
    assert store.record("job", D2, *RECORDED) is True
    assert store.record("job", D2, "wrong", b"wrong") is False
    store.finish("job", D2)
    assert decision(store, D2) == ("REPLAY", "recorded result", RECORDED)


def test_size_eviction_and_write_recency():
    store = fence(max_jobs=2)
    for job in ("a", "b"):
        decision(store, job=job)
    assert store.record("a", D1, *RECORDED) is True
    decision(store, job="c")
    assert decision(store, job="a")[0] == "REPLAY"
    assert decision(store, job="b")[0] == "RUN"


def test_age_eviction_and_write_refresh():
    now = [0.0]
    store = fence(max_age=10, clock=lambda: now[0])
    decision(store)
    now[0] = 9
    assert store.record("job", D1, *RECORDED) is True
    now[0] = 11
    assert decision(store)[0] == "REPLAY"
    now[0] = 20
    assert decision(store)[0] == "RUN"
    now[0] = 31
    assert store.record("job", D1, *RECORDED) is False


def test_evicted_job_never_replays_another_dispatch():
    store = fence(max_jobs=1)
    decision(store)
    store.record("job", D1, *RECORDED)
    decision(store, job="evict")
    assert decision(store, D2)[0] == "RUN"
    assert store.record("job", D2, "second", b"second") is True
    assert decision(store) == ("DROP", "stale attempt", None)


def test_replay_only_cannot_seed_or_supersede_running_entry():
    store = fence()
    assert decision(store, replay_only=True)[0] == "DROP"
    assert store.record("job", D1, *RECORDED) is False
    assert decision(store)[0] == "RUN"
    assert decision(store, D2, replay_only=True)[0] == "DROP"
    assert store.record("job", D1, *RECORDED) is True
    assert decision(store, replay_only=True) == ("REPLAY", "recorded result", RECORDED)


@pytest.mark.parametrize("value", [0, -1, 1_000_001, True, 1.5, "1", None])
def test_invalid_max_jobs(value):
    with pytest.raises(ValueError, match="dispatch_fence_max_jobs"):
        Agent(dispatch_fence_max_jobs=value)


@pytest.mark.parametrize("value", [0, -1, float("nan"), float("inf"), True, "1", None,
                                  pytest.param(10**400, id="overflow")])
def test_invalid_max_age(value):
    with pytest.raises(ValueError, match="dispatch_fence_max_age"):
        Agent(dispatch_fence_max_age=value)


def test_default_bounds():
    agent = Agent()
    assert (agent._dispatch_fence.max_jobs, agent._dispatch_fence.max_age) == (1024, 3600.0)


class Harness:
    def __init__(self, production=False, block=False, **options):
        self.key = ec.generate_private_key(ec.SECP256R1())
        self.production, self.block, self.calls = production, block, 0
        self.started, self.release = asyncio.Event(), asyncio.Event()
        self.nats, self.store, self.metrics = RecordingNATS([]), InMemoryBlobStore(), RecordingMetrics()
        self.agent = (production_agent({"k1": self.key.public_key()}, InMemoryReplayStore())
                      if production else Agent(**options))
        self.agent._store, self.agent._nc = self.store, self.nats
        self.agent._metrics = self.metrics
        self.agent._trust_admitting = True
        if production:
            activate_production_session(self.agent)
            config = self.agent._trust_settings.config
            config.proof_key_id, config.proof_private_key = "k1", self.key
        self.agent.job(TOPIC)(self.handler)

    async def handler(self, ctx, data):
        self.calls += 1
        self.started.set()
        if self.block:
            await self.release.wait()
        return {"call": self.calls}

    def request(self, attempt=1, dispatch_id="d1", worker=None):
        packet = make_packet(audience=TOPIC)
        packet.job_request.context_ptr = "redis://ctx:job-1"
        packet.job_request.dispatch.CopyFrom(job_pb2.DispatchIdentity(
            dispatch_id=dispatch_id, attempt=attempt,
            assigned_worker_id=worker if worker is not None else self.agent._sender_id))
        return packet

    def wire(self, packet):
        if self.production:
            return seal_production_event(packet, key=self.key, key_id="k1", audience=TOPIC)
        return finalize_packet(packet, None).SerializeToString(deterministic=True)

    async def deliver(self, raw, direct=False):
        await self.store.set("ctx:job-1", b"{}")
        message = SimpleNamespace(data=raw, subject=TOPIC)
        if direct:
            await self.agent._on_direct_msg(message)
        else:
            await self.agent._on_msg(message, self.agent._handlers[TOPIC])

    def results(self):
        raw = [data for subject, data in self.nats.published if subject == SUBJECT_RESULT]
        if not self.production:
            return [buspacket_pb2.BusPacket.FromString(data) for data in raw]
        trust = ProductionTrust(audience=SUBJECT_RESULT, tenant="tenant-a",
                                sender=self.agent._sender_id, public_keys={"k1": self.key.public_key()})
        return [verify_production_packet(data, trust) for data in raw]


@pytest.mark.parametrize("production,resign,direct", [
    (False, False, False), (False, False, True),
    (True, False, False), (True, True, False), (True, False, True),
], ids=["A1", "A1-direct", "A2", "A3", "A2-direct"])
def test_exact_redelivery_replays_result(production, resign, direct):
    async def run():
        h = Harness(production)
        packet = h.request()
        raw = h.wire(packet)
        await h.deliver(raw, direct)
        metrics_before = list(h.metrics.calls)
        if not production:
            packet.trace_id = "new-envelope-trace"
            raw = h.wire(packet)
        elif resign:
            raw = h.wire(packet)
        await h.deliver(raw, direct)
        results = h.results()
        assert (h.calls, len(results)) == (1, 2)
        assert results[0].job_result == results[1].job_result
        assert results[0].job_result.SerializeToString(deterministic=True) == results[1].job_result.SerializeToString(deterministic=True)
        assert [r.trace_id for r in results] == ["trace-1", "trace-1"]
        assert h.metrics.calls == metrics_before
        with patch.object(h.agent._dispatch_fence, "record", wraps=h.agent._dispatch_fence.record) as record:
            await h.deliver(raw, direct)
            assert (h.calls, len(h.results()), record.call_count) == (1, 3, 0)
        if production:
            assert results[0].job_result.dispatch == packet.job_request.dispatch
            assert results[0].job_result.identity == packet.job_request.identity
            assert results[0].signature_metadata.message_id != results[1].signature_metadata.message_id
    asyncio.run(run())


@pytest.mark.parametrize("production,future", [(False, False), (True, False), (False, True)])
def test_running_duplicate_or_future_is_dropped(production, future):
    async def run():
        h = Harness(production, block=True)
        raw = h.wire(h.request())
        active = asyncio.create_task(h.deliver(raw))
        try:
            await asyncio.wait_for(h.started.wait(), 1)
            duplicate = h.wire(h.request(3, "future")) if future else raw
            await asyncio.wait_for(h.deliver(duplicate), 0.2)
            assert (h.calls, len(h.results())) == (1, 0)
        finally:
            h.release.set()
            await asyncio.wait_for(active, 1)
        assert (h.calls, len(h.results())) == (1, 1)
        await h.deliver(raw)
        assert (h.calls, len(h.results())) == (1, 2)
    asyncio.run(run())


def test_new_attempt_runs_and_stale_drops():
    async def run():
        h = Harness()
        await h.deliver(h.wire(h.request()))
        await h.deliver(h.wire(h.request(2, "d2")))
        assert (h.calls, len(h.results())) == (2, 2)
        assert h.results()[0].job_result != h.results()[1].job_result
        await h.deliver(h.wire(h.request(1, "stale")))
        assert (h.calls, len(h.results())) == (2, 2)
    asyncio.run(run())


def test_foreign_worker_is_dropped():
    async def run():
        h = Harness()
        await h.deliver(h.wire(h.request(worker="foreign")))
        assert (h.calls, len(h.results())) == (0, 0)
    asyncio.run(run())


def test_legacy_requests_remain_unfenced():
    async def run():
        h = Harness(dispatch_fence_max_jobs=1)
        packet = h.request()
        packet.job_request.ClearField("dispatch")
        raw = h.wire(packet)
        await h.deliver(raw)
        await h.deliver(raw)
        assert (h.calls, len(h.results())) == (2, 2)
        assert h.agent._dispatch_fence.record("job-1", D1, *RECORDED) is False
    asyncio.run(run())


def test_publish_failure_is_replayed_without_handler():
    async def run():
        h = Harness()
        publish = h.nats.publish
        failed = False
        async def fail_once(subject, data):
            nonlocal failed
            if subject == SUBJECT_RESULT and not failed:
                failed = True
                raise RuntimeError("publish failed")
            await publish(subject, data)
        h.nats.publish = fail_once
        raw = h.wire(h.request())
        with pytest.raises(RuntimeError, match="publish failed"):
            await h.deliver(raw)
        assert (h.calls, len(h.results())) == (1, 0)
        await h.deliver(raw)
        assert (h.calls, len(h.results())) == (1, 1)
    asyncio.run(run())


def test_production_unknown_duplicate_never_claims_a_dispatch():
    async def run():
        h = Harness(production=True)
        raw = h.wire(h.request())
        assert h.agent._decode_admitted_packet(raw, TOPIC) is not None
        await h.deliver(raw)
        assert (h.calls, len(h.results())) == (0, 0)
        dispatch = ("d1", 1, h.agent._sender_id)
        assert h.agent._dispatch_fence.record("job-1", dispatch, *RECORDED) is False
        await h.deliver(h.wire(h.request()))
        assert (h.calls, len(h.results())) == (1, 1)
    asyncio.run(run())


def test_readme_documents_dispatch_fence():
    for term in ("dispatch_fence_max_jobs", "dispatch_fence_max_age", "in memory", "run_worker"):
        assert readme_lines_with(term), term
    for state, event, result in [("unknown", "dispatch", "RUN"), ("DONE", "recorded", "REPLAY"),
                                 ("DONE", "no result", "RUN"), ("RUNNING", "exact", "DROP"),
                                 ("RUNNING", "active+1", "RUN"), ("DONE", "newer", "RUN"),
                                 ("any", "digest", "DROP"), ("any", "stale", "DROP")]:
        assert readme_lines_with("|", state, event, result), (state, event, result)
