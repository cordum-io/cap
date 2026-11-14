"""cap.Agent with an operator ResourceRegistry: context_ref in, result_ref out."""
from __future__ import annotations

import asyncio
import hashlib
import json
import logging
from dataclasses import dataclass, field, replace
from datetime import datetime, timedelta, timezone
from types import SimpleNamespace
from typing import Any, Callable, Dict, List, Optional, Tuple
from unittest.mock import patch

import pytest
from cryptography.hazmat.primitives.asymmetric import ec

from cap.packet_boundary import finalize_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionTrust, verify_production_packet
from cap.resource_registry import (
    InMemoryResourceBackend,
    ResourceBackendRegistration,
    ResourceRegistry,
    TrustedResourceContext,
)
from cap.runtime import Agent, InMemoryBlobStore
from cap.subjects import SUBJECT_RESULT
from cap.worker_trust import WorkerTrustMode
from cap.worker_trust_runtime import WorkerTrustLifecycle
from worker_trust_runtime_support import TOPIC, WORKER_ID, RecordingNATS, session, trust_config

CANARY = "cap-canary-d35b"
JOB_ID = "job-resources-1"
JSON = "application/json"
SCHEDULER_KEY = ec.generate_private_key(ec.SECP256R1())
CONFIG = replace(trust_config(), scheduler_public_keys={"scheduler-pin": SCHEDULER_KEY.public_key()})
TENANT = CONFIG.tenant_id
TRUSTED = TrustedResourceContext(TENANT, JOB_ID)
INPUT = {"x": 1, "note": CANARY}
INPUT_BYTES = json.dumps(INPUT).encode("utf-8")
OUTPUT = {"y": 2}

E = job_pb2  # ErrorCode and JobStatus values are module-level ints in the generated code.
FAILED, SUCCEEDED = E.JOB_STATUS_FAILED, E.JOB_STATUS_SUCCEEDED
REQUIRED = (E.ERROR_CODE_JOB_INVALID_INPUT, "context ref required")
REFUSED = (E.ERROR_CODE_JOB_INVALID_INPUT, "context ref refused")
TOO_LARGE = (E.ERROR_CODE_JOB_RESOURCE_EXHAUSTED, "context exceeds max size")
DECODE = (E.ERROR_CODE_JOB_INVALID_INPUT, "context decode failed")
FETCH = (E.ERROR_CODE_UNSPECIFIED, "context fetch failed")
WRITE = (E.ERROR_CODE_UNSPECIFIED, "result write failed")
RESULT_TOO_LARGE = (E.ERROR_CODE_JOB_RESOURCE_EXHAUSTED, "result exceeds max size")


class RecordingMetrics:
    def __init__(self) -> None:
        self.calls: List[Tuple[Any, ...]] = []

    def on_job_received(self, *args: Any) -> None:
        self.calls.append(("received",) + args)

    def on_job_completed(self, *args: Any) -> None:
        self.calls.append(("completed",) + args)

    def on_job_failed(self, *args: Any) -> None:
        self.calls.append(("failed",) + args)

    def on_heartbeat_sent(self, *args: Any) -> None:
        self.calls.append(("heartbeat",) + args)


class RecordingBackend:
    """The in-memory backend, recording each call's trusted context; it can fail or block."""

    def __init__(self, fail: str = "") -> None:
        self.inner = InMemoryResourceBackend()
        self.fail = fail
        self.calls: List[Tuple[str, TrustedResourceContext]] = []
        self.entered = asyncio.Event()

    async def resolve(self, request: Any) -> Any:
        self.calls.append(("resolve", request.trusted_context))
        self.entered.set()
        await asyncio.sleep(0)  # lets concurrent jobs interleave
        if self.fail == "resolve":
            raise RuntimeError(CANARY)
        if self.fail == "block":
            await asyncio.Event().wait()
        return await self.inner.resolve(request)

    async def externalize(self, request: Any) -> Any:
        self.calls.append(("externalize", request.trusted_context))
        if self.fail == "externalize":
            raise RuntimeError(CANARY)
        return await self.inner.externalize(request)

    def count(self, kind: str) -> int:
        return sum(1 for name, _ in self.calls if name == kind)


def registry(backend: Any, max_bytes: int = 1 << 20) -> ResourceRegistry:
    return ResourceRegistry([ResourceBackendRegistration("memory", max_bytes, backend, backend)])


async def put_input(backend: RecordingBackend, content: bytes = INPUT_BYTES, media_type: str = JSON,
                    tenant: str = TENANT) -> Any:
    """Externalize a job input straight into the backend, outside the agent's recording."""
    return await registry(backend.inner).externalize(
        "memory", content, media_type=media_type, purpose="job-context",
        expires_at=datetime.now(timezone.utc) + timedelta(hours=1),
        trusted=TrustedResourceContext(tenant, JOB_ID))


def make_agent(nats: RecordingNATS, calls: List[Any], output: Callable[[Any, Any], Any] = lambda c, d: OUTPUT,
               **options: Any) -> Agent:
    agent = Agent(connect_fn=nats.connect, sender_id=WORKER_ID, heartbeat_interval=60,
                  worker_trust_mode="warn", worker_trust=CONFIG, worker_trust_timeout=1.0,
                  worker_trust_retries=1, logger=logging.getLogger("cap.test.runtime.resources"), **options)

    @agent.job(TOPIC)
    async def handler(ctx: Any, data: Any) -> Any:
        calls.append(data)
        return output(ctx, data)

    return agent


async def started(agent: Agent) -> Agent:
    with patch.object(WorkerTrustLifecycle, "_exchange_once", return_value=session("live")):
        await agent.start()
    return agent


def job_message(ref: Any = None, ptr: str = "", job_id: str = JOB_ID) -> Any:
    packet = buspacket_pb2.BusPacket(trace_id="trace-" + job_id, sender_id=CONFIG.expected_scheduler_id,
                                     protocol_version=1)
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(job_pb2.JobRequest(job_id=job_id, topic=TOPIC, context_ptr=ptr))
    if ref is not None:
        packet.job_request.context_ref.CopyFrom(ref)
    signed = finalize_packet(packet, SCHEDULER_KEY)
    return SimpleNamespace(data=signed.SerializeToString(deterministic=True))


def results(nats: RecordingNATS) -> Dict[str, job_pb2.JobResult]:
    packets = [buspacket_pb2.BusPacket.FromString(data) for subject, data in nats.published
               if subject == SUBJECT_RESULT]
    by_job = {packet.job_result.job_id: packet.job_result for packet in packets}
    assert len(by_job) == len(packets)  # one result per job
    return by_job


async def run_job(agent: Agent, nats: RecordingNATS, message: Any) -> job_pb2.JobResult:
    await agent._on_msg(message, agent._handlers[TOPIC])
    published = results(nats)
    assert list(published) == [JOB_ID]
    return published[JOB_ID]


def keep(ref: Any) -> Any:
    return ref


def drop(ref: Any) -> Any:
    return None


def expire(ref: Any) -> Any:
    ref.expires_at.FromDatetime(datetime.now(timezone.utc) - timedelta(hours=1))
    return ref


def unknown_resolver(ref: Any) -> Any:
    ref.resolver_id = "elsewhere"
    return ref


def tamper(ref: Any) -> Any:
    ref.sha256 = hashlib.sha256(b"tampered").digest()
    return ref


@dataclass
class Case:
    id: str
    expect: Tuple[int, str]  # error_code_enum, error_message; status is always FAILED
    calls: int = 0  # handler calls
    resolves: int = 0  # backend resolve calls
    edit: Callable[[Any], Any] = keep
    ptr: str = ""
    media_type: str = JSON
    tenant: str = TENANT
    max_bytes: int = 1 << 20
    fail: str = ""
    output: Any = field(default_factory=lambda: OUTPUT)
    agent: Dict[str, Any] = field(default_factory=dict)


CASES = [
    Case("missing-context-ref", REQUIRED, edit=drop),
    Case("refused-expired", REFUSED, edit=expire),
    Case("refused-unknown-resolver", REFUSED, edit=unknown_resolver),
    Case("refused-digest-tampered", REFUSED, edit=tamper, resolves=1),
    Case("refused-incompatible-dual-pointer", REFUSED, ptr="redis://ctx:" + JOB_ID),
    # The in-memory backend keys entries by the TRUSTED tenant, so another tenant's
    # ref is a backend miss: the registry reports ResourceBackendUnavailableError.
    Case("other-tenant-ref", FETCH, tenant="tenant-other", resolves=1),
    Case("declared-over-max-context-bytes", TOO_LARGE, agent={"max_context_bytes": 16}),
    Case("over-backend-max-bytes", TOO_LARGE, max_bytes=16),
    Case("non-json-media-type", DECODE, media_type="text/plain"),
    Case("backend-raises", FETCH, fail="resolve", resolves=1),
    Case("result-write-fails", WRITE, fail="externalize", calls=1, resolves=1),
    Case("result-over-backend-bound", RESULT_TOO_LARGE, max_bytes=64, output={"big": "a" * 100},
         calls=1, resolves=1),
]


async def drive(case: Case) -> Tuple[job_pb2.JobResult, List[bytes], RecordingMetrics, List[Any], RecordingBackend]:
    nats, calls, metrics, backend = RecordingNATS(), [], RecordingMetrics(), RecordingBackend(case.fail)
    ref = case.edit(await put_input(backend, media_type=case.media_type, tenant=case.tenant))
    agent = make_agent(nats, calls, lambda c, d: case.output, resource_registry=registry(backend, case.max_bytes),
                       result_backend_id="memory", metrics=metrics, **case.agent)
    await started(agent)
    try:
        result = await run_job(agent, nats, job_message(ref, case.ptr))
    finally:
        await agent.close()
    return result, [data for _, data in nats.published], metrics, calls, backend


class TestFailuresBeforeAndAfterTheHandler:
    @pytest.mark.parametrize("case", CASES, ids=[case.id for case in CASES])
    def test_case(self, case: Case) -> None:
        result, raw, metrics, calls, backend = asyncio.run(drive(case))
        enum_value, message = case.expect
        assert (result.status, result.error_code_enum, result.error_code, result.error_message) == (
            FAILED, enum_value, "", message)
        assert len(calls) == case.calls  # zero handler calls for every input refusal
        assert backend.count("resolve") == case.resolves
        assert result.result_ptr == "" and not result.HasField("result_ref")
        assert all(CANARY.encode("utf-8") not in payload for payload in raw)
        assert CANARY not in repr(metrics.calls)
        assert [call for call in metrics.calls if call[0] == "failed"] == [("failed", JOB_ID, message)]


async def succeed(**options: Any) -> SimpleNamespace:
    nats, calls, backend = RecordingNATS(), [], RecordingBackend()
    reg = registry(backend)
    ref = await put_input(backend)
    agent = await started(make_agent(nats, calls, resource_registry=reg, result_backend_id="memory", **options))
    try:
        before = datetime.now(timezone.utc)
        result = await run_job(agent, nats, job_message(ref))
        after = datetime.now(timezone.utc)
    finally:
        await agent.close()
    agent_calls = list(backend.calls)
    resolved = await reg.resolve(result.result_ref, TRUSTED)
    return SimpleNamespace(result=result, calls=calls, backend_calls=agent_calls, resolved=resolved,
                           before=before, after=after)


class TestRegistryRoundTrip:
    def test_success_resolves_context_ref_and_publishes_only_result_ref(self) -> None:
        run = asyncio.run(succeed(result_ref_ttl=120.0))
        assert run.calls == [INPUT]
        assert run.result.status == SUCCEEDED and run.result.error_message == ""
        assert run.result.result_ptr == ""
        assert run.result.HasField("result_ref")
        assert run.resolved.content == json.dumps(OUTPUT).encode("utf-8")
        assert run.resolved.media_type == JSON
        ref = run.result.result_ref
        assert (ref.resolver_id, ref.media_type, ref.purpose) == ("memory", JSON, "job-result")
        assert ref.uri.startswith("memory://%s/" % TENANT)
        expires = ref.expires_at.ToDatetime(tzinfo=timezone.utc)
        ttl = timedelta(seconds=120)
        assert run.before + ttl - timedelta(microseconds=1) <= expires <= run.after + ttl

    def test_trusted_context_is_config_tenant_and_request_job_id(self) -> None:
        run = asyncio.run(succeed())
        assert run.backend_calls == [("resolve", TRUSTED), ("externalize", TRUSTED)]

    def test_retries_rerun_only_the_handler(self) -> None:
        attempts: List[int] = []

        def flaky(ctx: Any, data: Any) -> Any:
            attempts.append(1)
            if len(attempts) < 3:
                raise RuntimeError(CANARY)
            return OUTPUT

        async def scenario() -> Tuple[job_pb2.JobResult, RecordingBackend]:
            nats, backend = RecordingNATS(), RecordingBackend()
            ref = await put_input(backend)
            agent = await started(make_agent(nats, [], flaky, resource_registry=registry(backend),
                                             result_backend_id="memory", retries=2))
            try:
                return await run_job(agent, nats, job_message(ref)), backend
            finally:
                await agent.close()

        result, backend = asyncio.run(scenario())
        assert result.status == SUCCEEDED and len(attempts) == 3
        assert (backend.count("resolve"), backend.count("externalize")) == (1, 1)

    def test_concurrent_jobs_use_their_own_trusted_context(self) -> None:
        async def scenario() -> Tuple[Dict[str, job_pb2.JobResult], RecordingBackend, ResourceRegistry]:
            nats, backend = RecordingNATS(), RecordingBackend()
            reg = registry(backend)
            ref = await put_input(backend)
            agent = await started(make_agent(nats, [], lambda c, d: {"job": c.job_id},
                                             resource_registry=reg, result_backend_id="memory"))
            try:
                await asyncio.gather(*(agent._on_msg(job_message(ref, job_id=job), agent._handlers[TOPIC])
                                       for job in ("job-a", "job-b")))
            finally:
                await agent.close()
            return results(nats), backend, reg

        published, backend, reg = asyncio.run(scenario())
        for job in ("job-a", "job-b"):
            trusted = TrustedResourceContext(TENANT, job)
            assert sorted(name for name, ctx in backend.calls if ctx == trusted) == ["externalize", "resolve"]
            resolved = asyncio.run(reg.resolve(published[job].result_ref, trusted))
            assert json.loads(resolved.content) == {"job": job}

    def test_cancel_during_resolve_propagates_and_publishes_nothing(self) -> None:
        async def scenario() -> Tuple[RecordingNATS, List[Any]]:
            nats, calls, backend = RecordingNATS(), [], RecordingBackend("block")
            ref = await put_input(backend)
            agent = await started(make_agent(nats, calls, resource_registry=registry(backend),
                                             result_backend_id="memory"))
            try:
                task = asyncio.ensure_future(agent._on_msg(job_message(ref), agent._handlers[TOPIC]))
                await asyncio.wait_for(backend.entered.wait(), timeout=1)
                task.cancel()
                with pytest.raises(asyncio.CancelledError):
                    await task
            finally:
                await agent.close()
            return nats, calls

        nats, calls = asyncio.run(scenario())
        assert calls == [] and results(nats) == {}


REG = registry(InMemoryResourceBackend())


class TestOptionsAndStartup:
    @pytest.mark.parametrize("options", [
        {"resource_registry": REG},
        {"result_backend_id": "memory"},
        {"resource_registry": REG, "result_backend_id": "elsewhere"},
        {"resource_registry": object(), "result_backend_id": "memory"},
        *({"resource_registry": REG, "result_backend_id": "memory", "result_ref_ttl": ttl}
          for ttl in (0, -1, True, float("nan"), float("inf"), "60")),
    ])
    def test_invalid_options_raise_at_construction(self, options: Dict[str, Any]) -> None:
        with pytest.raises(ValueError):
            Agent(connect_fn=RecordingNATS().connect, sender_id=WORKER_ID, **options)

    def test_registry_without_worker_trust_fails_before_connect(self) -> None:
        async def scenario() -> int:
            nats = RecordingNATS()
            agent = Agent(connect_fn=nats.connect, sender_id=WORKER_ID, worker_trust_mode="off",
                          resource_registry=REG, result_backend_id="memory")
            agent.job(TOPIC)(lambda ctx, data: data)
            with pytest.raises(RuntimeError, match="resource registry requires worker trust configuration"):
                await agent.start()
            return nats.connect_count

        assert asyncio.run(scenario()) == 0

    def test_registry_agent_never_creates_a_redis_store(self) -> None:
        async def scenario() -> Any:
            agent = make_agent(RecordingNATS(), [], resource_registry=REG, result_backend_id="memory")
            with patch("cap.runtime.RedisBlobStore") as redis_store:
                await started(agent)
                await agent.close()
            return redis_store

        asyncio.run(scenario()).assert_not_called()

    def test_without_registry_context_ptr_and_result_ptr_are_unchanged(self) -> None:
        async def scenario() -> Tuple[job_pb2.JobResult, List[Any], InMemoryBlobStore]:
            nats, calls, store = RecordingNATS(), [], InMemoryBlobStore()
            agent = await started(make_agent(nats, calls, store=store))
            await store.set("ctx:" + JOB_ID, INPUT_BYTES)
            try:
                return await run_job(agent, nats, job_message(ptr="redis://ctx:" + JOB_ID)), calls, store
            finally:
                await agent.close()

        result, calls, store = asyncio.run(scenario())
        assert calls == [INPUT] and result.status == SUCCEEDED
        assert result.result_ptr == "redis://res:" + JOB_ID and not result.HasField("result_ref")
        assert store._data["res:" + JOB_ID] == json.dumps(OUTPUT).encode("utf-8")


class TestProductionWithRegistry:
    def test_missing_ref_publishes_a_sealed_failure_without_the_handler(self) -> None:
        worker_key = ec.generate_private_key(ec.SECP256R1())
        trust = ProductionTrust(audience="job.worker.pool-a", tenant="tenant-a", sender="scheduler-a",
                                public_keys={"scheduler-key": SCHEDULER_KEY.public_key()})
        agent = Agent(sender_id="worker-a", production_trust=trust, replay_store=InMemoryReplayStore(),
                      resource_registry=REG, result_backend_id="memory")
        calls: List[Any] = []
        agent.job(TOPIC)(lambda ctx, data: calls.append(data))
        nats = RecordingNATS()
        agent._nc = nats
        agent._trust_settings = SimpleNamespace(mode=WorkerTrustMode.ENFORCE, config=SimpleNamespace(
            tenant_id="tenant-a", expected_scheduler_id="scheduler-a",
            proof_key_id="worker-key", proof_private_key=worker_key))
        agent._worker_trust = SimpleNamespace(session_token=lambda: "session-a")
        agent._trust_admitting = True
        packet = buspacket_pb2.BusPacket(trace_id="trace-a", sender_id="scheduler-a")
        packet.job_request.CopyFrom(job_pb2.JobRequest(job_id="job-a", topic=TOPIC))
        packet.job_request.identity.CopyFrom(job_pb2.IdentityBinding(tenant_id="tenant-a", principal_id="p-a"))
        packet.identity.CopyFrom(packet.job_request.identity)

        asyncio.run(agent._on_msg(SimpleNamespace(data=b""), agent._handlers[TOPIC], packet))

        verify = ProductionTrust(audience=SUBJECT_RESULT, tenant="tenant-a", sender="worker-a",
                                 public_keys={"worker-key": worker_key.public_key()})
        [(subject, raw)] = nats.published
        sealed = verify_production_packet(raw, verify).job_result
        assert subject == SUBJECT_RESULT and calls == []
        assert (sealed.job_id, sealed.status, sealed.error_code_enum, sealed.error_message) == (
            "job-a", FAILED, *REQUIRED)
        assert sealed.identity.tenant_id == "tenant-a"
