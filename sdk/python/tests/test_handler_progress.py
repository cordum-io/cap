"""Handler event contract: ordered progress, bounded artifacts, terminal fencing."""
from __future__ import annotations

import asyncio
import logging
from dataclasses import replace
from datetime import datetime, timedelta, timezone
from pathlib import Path
from types import SimpleNamespace

import pytest
from cryptography.hazmat.primitives.asymmetric import ec

from cap import JobFailure
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2 as E
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionSignatureError, ProductionTrust, verify_production_packet
from cap.runtime import Agent, Context, InMemoryBlobStore
from cap.subjects import SUBJECT_PROGRESS, SUBJECT_RESULT
from cap.validate import validate_bus_packet
from cap.worker_trust import WorkerTrustMode
from test_runtime_resources import (
    CONFIG, JOB_ID, SCHEDULER_KEY, TRUSTED, RecordingBackend, job_message,
    put_input, registry, started,
)
from worker_trust_runtime_support import TOPIC, WORKER_ID, RecordingNATS

CANARY = "secret-handler-event-canary"
FAILED, SUCCEEDED = E.JOB_STATUS_FAILED, E.JOB_STATUS_SUCCEEDED


class EventBackend(RecordingBackend):
    def __init__(self, fail=False):
        super().__init__()
        self.artifacts = []
        self.fail_artifact = fail

    async def externalize(self, request):
        await asyncio.sleep(0)  # deterministic T3 and concurrent slot reservation
        if request.purpose.startswith("job-artifact:"):
            self.artifacts.append(request)
            if self.fail_artifact:
                raise RuntimeError(CANARY)
        return await super().externalize(request)


async def drive(handler, *, backend=None, nats=None, max_bytes=1 << 20, blob=False, **options):
    backend = backend if backend is not None else EventBackend()
    nats = nats if nats is not None else RecordingNATS()
    reg, contexts = registry(backend, max_bytes), []
    store = InMemoryBlobStore()
    resources = {"store": store} if blob else {"resource_registry": reg, "result_backend_id": "memory"}
    agent = Agent(connect_fn=nats.connect, sender_id=WORKER_ID, heartbeat_interval=60,
                  worker_trust_mode="warn", worker_trust=CONFIG, **resources, **options)

    @agent.job(TOPIC)
    async def invoke(ctx, data):
        contexts.append(ctx)
        return await handler(ctx)

    if blob:
        await store.set("ctx:" + JOB_ID, b"{}")
        message = job_message(ptr="redis://ctx:" + JOB_ID)
    else:
        message = job_message(await put_input(backend, content=b"{}"))
    await started(agent)
    nats.published.clear()  # exclude the startup capability handshake, not job traffic
    try:
        await agent._on_msg(message, agent._handlers[TOPIC])
    finally:
        await agent.close()
    packets = [buspacket_pb2.BusPacket.FromString(raw) for _, raw in nats.published]
    results = [p.job_result for p in packets if p.HasField("job_result")]
    assert len(results) == 1
    return SimpleNamespace(result=results[0], packets=packets, nats=nats, backend=backend,
                           registry=reg, contexts=contexts, agent=agent)


def assert_failure(run, message):
    r = run.result
    assert (r.status, r.error_code_enum, r.error_code, r.error_message) == (FAILED, 201, "", message)
    assert list(r.artifact_refs) == []
    assert all(CANARY.encode() not in raw for _, raw in run.nats.published)


def test_progress_order():
    async def handler(ctx):
        await ctx.report_progress("upper")
        await ctx.report_progress("suffix")
        return {}

    run = asyncio.run(drive(handler))
    assert [s for s, _ in run.nats.published] == [SUBJECT_PROGRESS, SUBJECT_PROGRESS, SUBJECT_RESULT]
    for packet, marker in zip(run.packets, ["upper", "suffix"]):
        progress = packet.job_progress
        assert (progress.job_id, progress.status, progress.message, progress.percent, progress.step_id) == (
            JOB_ID, E.JOB_STATUS_RUNNING, marker, 0, "")
        assert (packet.trace_id, packet.sender_id, packet.protocol_version) == ("trace-" + JOB_ID, WORKER_ID, 1)
        assert validate_bus_packet(packet) == []
        assert (progress.result_ptr, list(progress.artifact_ptrs), list(progress.artifact_refs)) == ("", [], [])
        assert not progress.HasField("result_ref")
    assert run.result.status == SUCCEEDED


def test_failure_after_progress():
    async def handler(ctx):
        await ctx.report_progress("prepare")
        raise RuntimeError(CANARY)

    run = asyncio.run(drive(handler))
    assert [s for s, _ in run.nats.published] == [SUBJECT_PROGRESS, SUBJECT_RESULT]
    assert run.packets[0].job_progress.message == "prepare"
    assert (run.result.status, run.result.error_message) == (FAILED, "handler failed")
    assert all(CANARY.encode() not in raw for _, raw in run.nats.published)


def test_concurrent_progress_order():
    async def handler(ctx):
        await asyncio.gather(ctx.report_progress("a"), ctx.report_progress("b"))
        return {}

    run = asyncio.run(drive(handler))
    assert [p.job_progress.message for p in run.packets[:-1]] == ["a", "b"]
    assert run.result.status == SUCCEEDED


def test_production_progress_is_bound_and_sealed():
    async def scenario():
        key = ec.generate_private_key(ec.SECP256R1())
        trust = ProductionTrust(audience=TOPIC, tenant="tenant-a", sender="scheduler-a",
                                public_keys={"scheduler-key": SCHEDULER_KEY.public_key()})
        store, nats = InMemoryBlobStore(), RecordingNATS()
        agent = Agent(store=store, sender_id="worker-a", production_trust=trust,
                      replay_store=InMemoryReplayStore())

        @agent.job(TOPIC)
        async def handler(ctx, data):
            await ctx.report_progress("upper")
            await ctx.report_progress("suffix")
            return {}

        agent._nc = nats
        agent._trust_settings = SimpleNamespace(mode=WorkerTrustMode.ENFORCE, config=SimpleNamespace(
            tenant_id="tenant-a", expected_scheduler_id="scheduler-a",
            proof_key_id="worker-key", proof_private_key=key))
        agent._worker_trust = SimpleNamespace(session_token=lambda: "session-a")
        agent._trust_admitting = True
        packet = buspacket_pb2.BusPacket(trace_id="trace-a", sender_id="scheduler-a")
        req = E.JobRequest(job_id="job-a", topic=TOPIC, context_ptr="redis://ctx:job-a")
        req.identity.CopyFrom(E.IdentityBinding(tenant_id="tenant-a", principal_id="p-a"))
        req.dispatch.CopyFrom(E.DispatchIdentity(dispatch_id="dispatch-a", attempt=1, assigned_worker_id="worker-a"))
        packet.job_request.CopyFrom(req)
        packet.identity.CopyFrom(req.identity)
        await store.set("ctx:job-a", b"{}")
        await agent._on_msg(SimpleNamespace(data=b""), agent._handlers[TOPIC], packet)
        return nats, req, key

    nats, req, key = asyncio.run(scenario())
    assert [s for s, _ in nats.published] == [SUBJECT_PROGRESS, SUBJECT_PROGRESS, SUBJECT_RESULT]
    trust = ProductionTrust(audience=SUBJECT_PROGRESS, tenant="tenant-a", sender="worker-a",
                            public_keys={"worker-key": key.public_key()})
    for (_, raw), marker in zip(nats.published[:2], ["upper", "suffix"]):
        packet = verify_production_packet(raw, trust)
        progress = packet.job_progress
        assert (progress.job_id, progress.message, progress.status) == (req.job_id, marker, E.JOB_STATUS_RUNNING)
        assert progress.dispatch == req.dispatch and progress.identity == req.identity
        assert packet.identity == req.identity and packet.auth_token == "session-a"
        with pytest.raises(ProductionSignatureError):
            verify_production_packet(raw, replace(trust, audience=SUBJECT_RESULT))
    result = verify_production_packet(nats.published[-1][1], replace(trust, audience=SUBJECT_RESULT))
    assert result.job_result.status == SUCCEEDED


@pytest.mark.parametrize("marker,error", [(None, TypeError), (b"x", TypeError), ("", ValueError),
    ("a\nb", ValueError), ("\x00", ValueError), ("é" * 129, ValueError), ("a" * 257, ValueError)])
def test_marker_validation(marker, error):
    async def handler(ctx):
        with pytest.raises(error):
            await ctx.report_progress(marker)
        await ctx.report_progress("a" * 256)
        return {}

    run = asyncio.run(drive(handler))
    assert run.result.status == SUCCEEDED
    assert [p.job_progress.message for p in run.packets[:-1]] == ["a" * 256]


def test_progress_publish_error_propagates():
    class BrokenNATS(RecordingNATS):
        async def publish(self, subject, raw):
            if subject == SUBJECT_PROGRESS:
                raise ConnectionError(CANARY)
            await super().publish(subject, raw)

    seen = []

    async def handler(ctx):
        try:
            await ctx.report_progress("prepare")
        except ConnectionError:
            seen.append("connection-error")
            raise

    run = asyncio.run(drive(handler, nats=BrokenNATS()))
    assert seen == ["connection-error"]
    assert [s for s, _ in run.nats.published] == [SUBJECT_RESULT]
    assert (run.result.status, run.result.error_message) == (FAILED, "handler failed")


@pytest.mark.parametrize("limit", [2, 8])
def test_two_artifacts_resolve_in_order(limit):
    async def scenario():
        async def handler(ctx):
            await ctx.attach_artifact("summary", "text/plain", b"HELLO")
            await ctx.attach_artifact("data", "application/json", b'{"a":1}')
            return {}

        before = datetime.now(timezone.utc)
        run = await drive(handler, max_artifacts=limit, result_ref_ttl=120)
        after = datetime.now(timezone.utc)
        assert run.result.status == SUCCEEDED and run.result.HasField("result_ref")
        refs = run.result.artifact_refs
        assert [(r.purpose, r.media_type, r.size_bytes, r.resolver_id) for r in refs] == [
            ("job-artifact:summary", "text/plain", 5, "memory"),
            ("job-artifact:data", "application/json", 7, "memory")]
        for ref, content in zip(refs, [b"HELLO", b'{"a":1}']):
            resolved = await run.registry.resolve(ref, TRUSTED)
            assert (resolved.content, resolved.media_type) == (content, ref.media_type)
            assert before + timedelta(seconds=120) <= ref.expires_at.ToDatetime(tzinfo=timezone.utc) <= after + timedelta(seconds=120)
        assert [r.trusted_context for r in run.backend.artifacts] == [TRUSTED, TRUSTED]

    asyncio.run(scenario())


@pytest.mark.parametrize("limit", [0, 1])
def test_artifact_count_bound(limit):
    caught = []

    async def handler(ctx):
        for i in range(limit + 1):
            try:
                await ctx.attach_artifact("a" + str(i), "text/plain", b"HELLO")
            except JobFailure as exc:
                caught.append(exc.error_code_enum)
                raise

    run = asyncio.run(drive(handler, max_artifacts=limit))
    assert caught == [201]
    assert_failure(run, "artifact count exceeds max")
    assert len(run.backend.artifacts) == limit


@pytest.mark.parametrize("content", [b"HELLO", b"HELLO!"])
def test_artifact_byte_bound(content):
    async def handler(ctx):
        await ctx.attach_artifact("summary", "text/plain", content)
        return {}

    run = asyncio.run(drive(handler, max_artifact_bytes=5))
    if len(content) == 5:
        assert run.result.status == SUCCEEDED and len(run.result.artifact_refs) == 1
        assert len(run.backend.artifacts) == 1
    else:
        assert_failure(run, "artifact exceeds max size")
        assert run.backend.artifacts == []


@pytest.mark.parametrize("outcome", ["return", "typed", "exception"])
def test_latch_overrides_handler_outcome(outcome):
    async def handler(ctx):
        with pytest.raises(JobFailure):
            await ctx.attach_artifact("first", "text/plain", b"x")
        if outcome == "typed":
            raise JobFailure(202)
        if outcome == "exception":
            raise RuntimeError(CANARY)
        return {}

    assert_failure(asyncio.run(drive(handler, max_artifacts=0)), "artifact count exceeds max")


@pytest.mark.parametrize("mode", ["backend", "missing", "size"])
def test_artifact_externalize_failure_has_no_context(mode):
    caught = []

    async def handler(ctx):
        try:
            await ctx.attach_artifact("summary", "text/plain", b"HELLO")
        except JobFailure as exc:
            caught.append((exc.error_code_enum, exc.__cause__, exc.__context__))
        return {}

    run = asyncio.run(drive(handler, backend=EventBackend(fail=mode == "backend"),
                            blob=mode == "missing", max_bytes=4 if mode == "size" else 1024))
    assert caught == [(201, None, None)]
    assert_failure(run, "artifact exceeds max size" if mode == "size" else "artifact write failed")


@pytest.mark.parametrize("name,media,content,error", [
    *[(n, "text/plain", b"x", ValueError) for n in ["Bad", "a:b", "", "a" * 65, "a\n"]],
    (b"x", "text/plain", b"x", TypeError),
    *[("a", m, b"x", ValueError) for m in ["Text/Plain", "text/plain; charset=utf-8", "text", "text/plain\n", "a/" + "b" * 126]],
    ("a", None, b"x", TypeError), ("a", "text/plain", "str", TypeError),
    ("a", "text/plain", bytearray(b"x"), TypeError), ("a", "text/plain", b"", ValueError),
])
def test_artifact_argument_errors_do_not_latch(name, media, content, error):
    async def handler(ctx):
        with pytest.raises(error):
            await ctx.attach_artifact(name, media, content)
        return {}

    run = asyncio.run(drive(handler))
    assert run.result.status == SUCCEEDED and list(run.result.artifact_refs) == []
    assert run.backend.artifacts == []


def test_retry_resets_artifacts():
    attempts = []

    async def handler(ctx):
        name = "first" if not attempts else "second"
        attempts.append(name)
        await ctx.attach_artifact(name, "text/plain", b"x")
        await ctx.report_progress(name)
        if name == "first":
            raise RuntimeError(CANARY)
        return {}

    run = asyncio.run(drive(handler, retries=1, max_artifacts=1))
    assert attempts == ["first", "second"]
    assert run.result.status == SUCCEEDED
    assert [r.purpose for r in run.result.artifact_refs] == ["job-artifact:second"]
    assert [p.job_progress.message for p in run.packets[:-1]] == attempts


def test_concurrent_artifacts_reserve_slots():
    async def handler(ctx):
        await asyncio.gather(*(ctx.attach_artifact(n, "text/plain", b"x") for n in ["a", "b"]),
                             return_exceptions=True)
        return {}

    run = asyncio.run(drive(handler, max_artifacts=1))
    assert_failure(run, "artifact count exceeds max")
    assert len(run.backend.artifacts) == 1


@pytest.mark.parametrize("option,values", [("max_artifacts", [-1, 1001, True, 1.0, "1", None]),
    ("max_artifact_bytes", [0, -1, True, 1.5, "1", None])])
def test_invalid_options(option, values):
    for value in values:
        with pytest.raises(ValueError):
            Agent(**{option: value})


def test_option_defaults_and_boundaries():
    agent = Agent()
    assert (agent._max_artifacts, agent._max_artifact_bytes) == (8, 2 * 1024 * 1024)
    for value in [0, 1000]:
        assert Agent(max_artifacts=value)._max_artifacts == value


def test_hooks_after_terminal_publish_nothing():
    async def scenario():
        async def handler(ctx):
            await ctx.report_progress("before")
            return {}

        run = await drive(handler)
        ctx = run.contexts[0]
        before = (len(run.nats.published), run.backend.count("externalize"))
        assert before == (2, 1)
        assert await ctx.report_progress("late") is None
        assert await ctx.attach_artifact("late", "text/plain", b"x") is None
        assert (len(run.nats.published), run.backend.count("externalize")) == before
        with pytest.raises(ValueError):
            await ctx.report_progress("")
        with pytest.raises(ValueError):
            await ctx.attach_artifact("Bad", "text/plain", b"x")

    asyncio.run(scenario())


def test_direct_terminal_publish_closes_hooks():
    from cap.job_events import _JobEvents

    async def scenario():
        nats, agent = RecordingNATS(), Agent(worker_trust_mode="off")
        agent._nc = nats
        packet = buspacket_pb2.BusPacket(job_request=E.JobRequest(job_id=JOB_ID))

        async def publish(marker):
            await agent._publish_progress(packet, marker, None)

        ctx = Context(job=packet.job_request, packet=packet, logger=logging.LoggerAdapter(logging.getLogger(), {}),
                      _events=_JobEvents(publish, None, 8, 1024))
        await agent._publish_result(ctx, E.JobResult(job_id=JOB_ID, status=SUCCEEDED))
        assert await ctx.report_progress("late") is None
        assert [s for s, _ in nats.published] == [SUBJECT_RESULT]

    asyncio.run(scenario())


def test_unawaited_marker_is_dropped():
    tasks = []

    async def scenario():
        async def handler(ctx):
            tasks.append(asyncio.create_task(ctx.report_progress("bg")))
            return {}

        run = await drive(handler)
        await asyncio.gather(*tasks)
        assert run.result.status == SUCCEEDED
        assert [s for s, _ in run.nats.published] == [SUBJECT_RESULT]

    asyncio.run(scenario())


def test_unbound_context_validates_but_does_nothing():
    async def scenario():
        ctx = Context(job=E.JobRequest(), packet=buspacket_pb2.BusPacket(),
                      logger=logging.LoggerAdapter(logging.getLogger(), {}))
        assert await ctx.report_progress("valid") is None
        assert await ctx.attach_artifact("valid", "text/plain", b"x") is None
        with pytest.raises(ValueError):
            await ctx.report_progress("")
        with pytest.raises(TypeError):
            await ctx.attach_artifact("a", "text/plain", "x")

    asyncio.run(scenario())


def test_readme_documents_progress_and_artifacts():
    readme = (Path(__file__).resolve().parents[1] / "README.md").read_text(encoding="utf-8")
    for name in ["report_progress", "attach_artifact", "max_artifacts", "max_artifact_bytes", "job-artifact:<name>"]:
        assert name in readme
    for message in ["artifact count exceeds max", "artifact exceeds max size", "artifact write failed"]:
        assert len([line for line in readme.splitlines() if all(s in line for s in [
            "|", "`JOB_STATUS_FAILED`", "`ERROR_CODE_JOB_RESOURCE_EXHAUSTED`", "`" + message + "`"
        ])]) == 1
